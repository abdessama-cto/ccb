package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
)

// addCmd is the parent for `ccb add <type>` subcommands.
var addCmd = &cobra.Command{
	Use:     "add",
	Aliases: []string{"a"},
	Short:   "Add a skill, agent, or rule to an installed project",
	Long: `Add an item to the .claude/ configuration of the current project.

Examples:
  ccb add skill                      # interactive picker across 800+ skills
  ccb add skill changelog-generator  # install a specific skill
  ccb add agent                      # generate a new agent with AI
  ccb add rule "always log SQL queries in dev"
`,
}

func init() {
	rootCmd.AddCommand(addCmd)
	addCmd.AddCommand(addSkillCmd)
	addCmd.AddCommand(addAgentCmd)
	addCmd.AddCommand(addRuleCmd)
}

// requireProject returns the current working directory if it contains a
// .claude/ directory, or an error otherwise.
func requireProject() (string, error) {
	wd, err := os.Getwd()
	if err != nil {
		return "", err
	}
	claudeDir := filepath.Join(wd, ".claude")
	if _, statErr := os.Stat(claudeDir); os.IsNotExist(statErr) {
		return "", fmt.Errorf("no .claude/ found in %s — run 'ccb start' first", wd)
	}
	return wd, nil
}

// kebabName normalises a user-provided name to a kebab-case file-safe slug.
func kebabName(s string) string {
	s = strings.TrimSpace(strings.ToLower(s))
	s = strings.ReplaceAll(s, " ", "-")
	s = strings.ReplaceAll(s, "_", "-")
	return s
}
