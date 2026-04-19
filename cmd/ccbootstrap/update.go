package cmd

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"time"

	"github.com/abdessama-cto/ccb/internal/tui"
	"github.com/spf13/cobra"
)

var updateCmd = &cobra.Command{
	Use:   "update",
	Short: "Update ccb to the latest version",
	RunE:  runUpdate,
}

func init() {
	rootCmd.AddCommand(updateCmd)
}

func runUpdate(cmd *cobra.Command, args []string) error {
	checkSpin := tui.StartSpinner("Checking for updates...")

	arch := fmt.Sprintf("%s-%s", runtime.GOOS, hostArch())

	latestVersion, err := fetchLatestVersion()
	if err != nil {
		checkSpin.Fail("Could not check for updates")
		return err
	}

	if latestVersion == "v"+Version {
		checkSpin.Success(fmt.Sprintf("Already up to date (%s)", Version))
		return nil
	}
	checkSpin.Success(fmt.Sprintf("New version available: %s → %s", Version, latestVersion))

	exe, err := os.Executable()
	if err != nil {
		return fmt.Errorf("could not determine current binary path: %w", err)
	}

	baseURL := fmt.Sprintf("https://github.com/abdessama-cto/ccb/releases/download/%s", latestVersion)
	candidates := []string{
		fmt.Sprintf("%s/ccb-%s", baseURL, arch),
		fmt.Sprintf("%s/ccbootstrap-%s", baseURL, arch),
	}
	tmpPath := exe + ".new"

	dlSpin := tui.StartSpinner(fmt.Sprintf("Downloading %s...", latestVersion))
	var lastErr error
	var downloaded bool
	for _, url := range candidates {
		if err := downloadToFile(url, tmpPath); err != nil {
			lastErr = err
			_ = os.Remove(tmpPath)
			continue
		}
		downloaded = true
		break
	}
	if !downloaded {
		dlSpin.Fail("Download failed")
		return fmt.Errorf("download failed: %w", lastErr)
	}
	dlSpin.Success(fmt.Sprintf("Downloaded %s", latestVersion))

	if err := os.Chmod(tmpPath, 0755); err != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("chmod failed: %w", err)
	}

	if err := os.Rename(tmpPath, exe); err != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("could not replace binary: %w", err)
	}

	tui.Success(fmt.Sprintf("Updated to %s — restart your shell if needed.", latestVersion))
	return nil
}

func fetchLatestVersion() (string, error) {
	client := &http.Client{Timeout: 15 * time.Second}
	req, _ := http.NewRequest("GET", "https://api.github.com/repos/abdessama-cto/ccb/releases/latest", nil)
	req.Header.Set("Accept", "application/vnd.github+json")
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("could not reach GitHub API: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return "", fmt.Errorf("GitHub API returned status %d", resp.StatusCode)
	}
	var body struct {
		TagName string `json:"tag_name"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return "", fmt.Errorf("could not parse GitHub API response: %w", err)
	}
	if body.TagName == "" {
		return "", fmt.Errorf("could not determine latest version from GitHub API")
	}
	return body.TagName, nil
}

// hostArch returns the real CPU arch, not the Go toolchain's arch.
// On macOS, a Go binary built for amd64 running under Rosetta on Apple
// Silicon reports runtime.GOARCH == "amd64", but the host is arm64.
// sysctl.proc_translated == 1 means we're running under Rosetta, so
// the real hardware is arm64.
func hostArch() string {
	if runtime.GOOS == "darwin" && runtime.GOARCH == "amd64" {
		out, err := exec.Command("sysctl", "-n", "sysctl.proc_translated").Output()
		if err == nil && strings.TrimSpace(string(out)) == "1" {
			return "arm64"
		}
	}
	return runtime.GOARCH
}

func downloadToFile(url, dest string) error {
	client := &http.Client{Timeout: 120 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return fmt.Errorf("status %d from %s", resp.StatusCode, url)
	}
	f, err := os.Create(dest)
	if err != nil {
		return err
	}
	defer f.Close()
	if _, err := io.Copy(f, resp.Body); err != nil {
		return err
	}
	return nil
}
