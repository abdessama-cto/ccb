package cmd

import (
	"fmt"
	"os"

	"github.com/abdessama-cto/ccb/internal/config"
	"github.com/abdessama-cto/ccb/internal/tui"
	"github.com/spf13/cobra"
)

// Version is set at build time via -ldflags
var Version = "0.2.0"

var rootCmd = &cobra.Command{
	Use:   "ccbootstrap",
	Short: "Claude Code Project Bootstrapper for Mac Apple Silicon",
	Long: `ccbootstrap — Analyze any GitHub repo and automatically generate
a complete Claude Code configuration (CLAUDE.md, .claude/, docs/).
Everything automated: clone, analyze, questionnaire, generate, commit, push, PR.`,
	Version: Version,
	PersistentPreRun: func(cmd *cobra.Command, args []string) {
		// Ensure config dir exists on every command
		_ = config.EnsureDirs()
	},
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		tui.Err(err.Error())
		os.Exit(1)
	}
}

func init() {
	rootCmd.SetVersionTemplate(fmt.Sprintf(
		"ccbootstrap %s · Mac Apple Silicon (arm64-darwin)\n"+
			"Repo: https://github.com/abdessama-cto/ccb\n",
		Version,
	))
}
