package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"

	"github.com/abdessama-cto/ccb/internal/tui"
	"github.com/spf13/cobra"
)

var selfUpdateCmd = &cobra.Command{
	Use:   "self-update",
	Short: "Update ccb to the latest version",
	RunE:  runSelfUpdate,
}

func init() {
	rootCmd.AddCommand(selfUpdateCmd)
}

func runSelfUpdate(cmd *cobra.Command, args []string) error {
	tui.Info("Checking for updates...")

	// Determine binary arch suffix
	arch := "darwin-arm64"
	if runtime.GOARCH == "amd64" {
		arch = "darwin-amd64"
	}

	// Fetch latest version from GitHub API
	out, err := exec.Command("curl", "-fsSL",
		"https://api.github.com/repos/abdessama-cto/ccb/releases/latest",
	).Output()
	if err != nil {
		return fmt.Errorf("could not reach GitHub API: %w", err)
	}

	// Parse tag_name with jq if available
	var latestVersion string
	jqCmd := exec.Command("jq", "-r", ".tag_name")
	jqOut, jqErr := execInput(jqCmd, out)
	if jqErr == nil {
		latestVersion = strings.TrimSpace(string(jqOut))
	}

	if latestVersion == "" || latestVersion == "null" {
		return fmt.Errorf("could not determine latest version from GitHub API")
	}

	if latestVersion == "v"+Version {
		tui.Success(fmt.Sprintf("Already up to date (%s)", Version))
		return nil
	}

	tui.Info(fmt.Sprintf("New version available: %s → %s", Version, latestVersion))

	// Download new binary
	exe, err := os.Executable()
	if err != nil {
		return fmt.Errorf("could not determine current binary path: %w", err)
	}

	binaryURL := fmt.Sprintf(
		"https://github.com/abdessama-cto/ccb/releases/download/%s/ccb-%s",
		latestVersion, arch,
	)
	tmpPath := exe + ".new"

	tui.Info(fmt.Sprintf("Downloading %s...", latestVersion))
	dlCmd := exec.Command("curl", "-fsSL", "-o", tmpPath, binaryURL)
	dlCmd.Stdout = os.Stdout
	dlCmd.Stderr = os.Stderr
	if err := dlCmd.Run(); err != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("download failed: %w", err)
	}

	if err := os.Chmod(tmpPath, 0755); err != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("chmod failed: %w", err)
	}

	// Atomic replace
	if err := os.Rename(tmpPath, exe); err != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("could not replace binary: %w", err)
	}

	tui.Success(fmt.Sprintf("Updated to %s — restart your shell if needed.", latestVersion))
	return nil
}

// exec.Command.Input is not stdlib — add helper
func execInput(cmd *exec.Cmd, input []byte) ([]byte, error) {
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, err
	}
	go func() {
		defer stdin.Close()
		stdin.Write(input)
	}()
	return cmd.Output()
}
