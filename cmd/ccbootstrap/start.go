package cmd

import (
	"fmt"

	"github.com/abdessama-cto/ccb/internal/config"
	"github.com/abdessama-cto/ccb/internal/tui"
	"github.com/spf13/cobra"
)

var startCmd = &cobra.Command{
	Use:     "start",
	Aliases: []string{"s"},
	Short:   "Guided onboarding — configure AI if needed, then bootstrap",
	Long: `Recommended entry point for first-time users.

ccb start will:
  1. Check if an AI provider is configured.
  2. If not, open the settings editor so you can paste your API key.
  3. Run 'ccb init' in the current directory.

Already configured? This is equivalent to running 'ccb init' directly.`,
	RunE: runStart,
}

func init() {
	rootCmd.AddCommand(startCmd)
}

func runStart(cmd *cobra.Command, args []string) error {
	cfg, _ := config.Load()

	if !cfg.AI.IsConfigured() {
		tui.Banner(Version)
		tui.Info("No AI provider configured yet — let's set one up first.")
		fmt.Println()

		if err := runSettings(cmd, args); err != nil {
			return err
		}

		cfg, _ = config.Load()
		if !cfg.AI.IsConfigured() {
			return fmt.Errorf("AI provider still not configured — run 'ccb settings' and try again")
		}

		fmt.Println()
		tui.Success("AI configured — starting bootstrap...")
		fmt.Println()
	}

	return runInit(cmd, args)
}
