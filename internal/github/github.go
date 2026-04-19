package github

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// CheckAuth verifies gh CLI is authenticated
func CheckAuth() (string, error) {
	out, err := exec.Command("gh", "auth", "status").CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("GitHub auth failed. Run: gh auth login\n%s", string(out))
	}
	// Extract username
	for _, line := range strings.Split(string(out), "\n") {
		if strings.Contains(line, "Logged in to github.com account") {
			parts := strings.Fields(line)
			for i, p := range parts {
				if p == "account" && i+1 < len(parts) {
					return parts[i+1], nil
				}
			}
		}
	}
	return "authenticated", nil
}

// CloneRepo clones a GitHub repo into destDir
func CloneRepo(repoURL, destDir string) error {
	if err := os.MkdirAll(filepath.Dir(destDir), 0755); err != nil {
		return err
	}
	// Shallow clone for speed
	cmd := exec.Command("git", "clone", "--depth", "200", repoURL, destDir)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// CountCommits returns commit count
func CountCommits(repoDir string) int {
	out, err := exec.Command("git", "-C", repoDir, "rev-list", "--count", "HEAD").Output()
	if err != nil {
		return 0
	}
	var n int
	fmt.Sscanf(strings.TrimSpace(string(out)), "%d", &n)
	return n
}

// CurrentBranch returns the current branch name
func CurrentBranch(repoDir string) string {
	out, err := exec.Command("git", "-C", repoDir, "branch", "--show-current").Output()
	if err != nil {
		return "main"
	}
	return strings.TrimSpace(string(out))
}

// CreateBranchAndPush creates (or reuses) a branch, commits all changes, and pushes.
// If the branch already exists locally, it switches to it instead of re-creating it.
func CreateBranchAndPush(repoDir, branch, commitMsg string) error {
	// Step 1: create or switch to branch
	checkoutNew := exec.Command("git", "-C", repoDir, "checkout", "-b", branch)
	if out, err := checkoutNew.CombinedOutput(); err != nil {
		// Branch already exists — try switching to it
		checkoutExisting := exec.Command("git", "-C", repoDir, "checkout", branch)
		if out2, err2 := checkoutExisting.CombinedOutput(); err2 != nil {
			return fmt.Errorf("checkout branch %q: %w\n%s\n%s", branch, err2, out, out2)
		}
	}

	// Step 2: stage all changes
	if out, err := exec.Command("git", "-C", repoDir, "add", ".").CombinedOutput(); err != nil {
		return fmt.Errorf("git add: %w\n%s", err, out)
	}

	// Step 3: commit (skip if nothing to commit)
	commitCmd := exec.Command("git", "-C", repoDir, "commit", "-m", commitMsg)
	if out, err := commitCmd.CombinedOutput(); err != nil {
		msg := strings.ToLower(string(out))
		if strings.Contains(msg, "nothing to commit") || strings.Contains(msg, "nothing added") {
			// Nothing new to commit — push existing branch state
		} else {
			return fmt.Errorf("git commit: %w\n%s", err, out)
		}
	}

	// Step 4: push
	pushCmd := exec.Command("git", "-C", repoDir, "push", "--set-upstream", "origin", branch)
	pushCmd.Stdout = os.Stdout
	pushCmd.Stderr = os.Stderr
	if err := pushCmd.Run(); err != nil {
		return fmt.Errorf("git push: %w", err)
	}
	return nil
}

// CreatePR creates a pull request via gh CLI
func CreatePR(repoDir, title, body, branch string) (string, error) {
	cmd := exec.Command("gh", "pr", "create",
		"--title", title,
		"--body", body,
		"--head", branch,
	)
	cmd.Dir = repoDir
	out, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

// RepoNameFromURL extracts "owner-repo" from a GitHub URL
func RepoNameFromURL(repoURL string) string {
	u := strings.TrimSuffix(repoURL, ".git")
	u = strings.TrimSuffix(u, "/")
	parts := strings.Split(u, "/")
	if len(parts) >= 2 {
		return parts[len(parts)-2] + "-" + parts[len(parts)-1]
	}
	return "unknown-repo"
}
