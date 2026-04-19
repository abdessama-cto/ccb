package cmd

import (
	"fmt"
	"time"

	"github.com/abdessama-cto/ccb/internal/skills"
	"github.com/abdessama-cto/ccb/internal/tui"
	"github.com/spf13/cobra"
)

var syncCmd = &cobra.Command{
	Use:   "sync",
	Short: "Ping skills.sh to verify the catalog is reachable",
	Long: `Skills are now pulled on demand from https://skills.sh — there is no
local cache to refresh. This command simply verifies the API is reachable
and prints a sample response to confirm the connection works.`,
	RunE: runSync,
}

func init() {
	rootCmd.AddCommand(syncCmd)
}

func runSync(cmd *cobra.Command, args []string) error {
	sp := tui.StartSpinner("Pinging https://skills.sh ...")
	start := time.Now()
	results, err := skills.Search("test", 1)
	elapsed := time.Since(start).Round(time.Millisecond)
	if err != nil {
		sp.Fail("skills.sh unreachable")
		return fmt.Errorf("skills.sh unreachable: %w", err)
	}
	sp.Success(fmt.Sprintf("Catalog reachable (%s, %d sample result)", elapsed, len(results)))
	if len(results) > 0 {
		s := results[0]
		fmt.Printf("  sample: %s  ·  %s  ·  %d installs\n", tui.Cyan(s.SkillID), tui.Dim(s.Source), s.Installs)
	}
	return nil
}
