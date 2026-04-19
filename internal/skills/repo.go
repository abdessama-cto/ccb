// Package skills handles discovery of Claude skills from a cached clone of
// ComposioHQ/awesome-claude-skills.
package skills

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"time"
)

// SkillsRepoURL is the upstream we clone for the skills library.
const SkillsRepoURL = "https://github.com/ComposioHQ/awesome-claude-skills.git"

// maxCacheAge controls how often we attempt to `git pull` the repo.
const maxCacheAge = 24 * time.Hour

// CacheDir returns the local path to the awesome-claude-skills clone.
func CacheDir() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".ccbootstrap", "skills-cache")
}

// EnsureRepo clones awesome-claude-skills if missing, or pulls it if stale.
// It returns the cache directory path. Errors from pull are swallowed — a
// slightly stale cache is still useful.
func EnsureRepo() (string, error) {
	dir := CacheDir()
	gitDir := filepath.Join(dir, ".git")

	if _, err := os.Stat(gitDir); os.IsNotExist(err) {
		if err := os.MkdirAll(filepath.Dir(dir), 0755); err != nil {
			return dir, fmt.Errorf("mkdir cache parent: %w", err)
		}
		cmd := exec.Command("git", "clone", "--depth", "1", SkillsRepoURL, dir)
		if out, err := cmd.CombinedOutput(); err != nil {
			return dir, fmt.Errorf("clone awesome-claude-skills: %w\n%s", err, out)
		}
		return dir, nil
	}

	// Pull if stale
	fetchHead := filepath.Join(gitDir, "FETCH_HEAD")
	if info, err := os.Stat(fetchHead); err == nil && time.Since(info.ModTime()) < maxCacheAge {
		return dir, nil // fresh enough
	}
	pull := exec.Command("git", "-C", dir, "pull", "--ff-only", "--quiet")
	_, _ = pull.CombinedOutput() // non-fatal — stale clone is still useful
	return dir, nil
}
