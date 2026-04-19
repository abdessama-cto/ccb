package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/abdessama-cto/ccb/internal/skills"
	"github.com/abdessama-cto/ccb/internal/tui"
	"github.com/spf13/cobra"
)

var syncCmd = &cobra.Command{
	Use:   "sync",
	Short: "Pull the latest awesome-claude-skills catalog",
	Long: `Force-refresh the local clone of ComposioHQ/awesome-claude-skills
stored at ~/.ccbootstrap/skills-cache/.

ccb normally auto-pulls if the cache is older than 24h. Use 'ccb sync' to
force an immediate refresh.`,
	RunE: runSync,
}

func init() {
	rootCmd.AddCommand(syncCmd)
}

func runSync(cmd *cobra.Command, args []string) error {
	dir := skills.CacheDir()

	// If not cloned yet, clone now.
	if _, err := os.Stat(filepath.Join(dir, ".git")); os.IsNotExist(err) {
		tui.Info("Skills cache not found — cloning...")
		if _, err := skills.EnsureRepo(); err != nil {
			return err
		}
	} else {
		tui.Info("Pulling latest awesome-claude-skills...")
		pull := exec.Command("git", "-C", dir, "pull", "--ff-only")
		out, err := pull.CombinedOutput()
		if err != nil {
			return fmt.Errorf("git pull failed: %w\n%s", err, out)
		}
		fmt.Println(string(out))
	}

	catalog := skills.ScanDiskSkills(dir)
	tui.Success(fmt.Sprintf("Catalog refreshed — %d skills available", len(catalog)))
	return nil
}
