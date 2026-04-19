package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/abdessama-cto/ccb/internal/tui"
	"github.com/spf13/cobra"
)

var addRuleCmd = &cobra.Command{
	Use:   "rule <text>",
	Short: "Append a project-specific rule to .claude/rules/05-project-specific.md",
	Long: `Add a single rule to the project-specific rules file.

Example:
  ccb add rule "Always use parameterized queries for database access"
  ccb add rule "Never log user PII even in debug mode"`,
	Args: cobra.MinimumNArgs(1),
	RunE: runAddRule,
}

func runAddRule(cmd *cobra.Command, args []string) error {
	projectDir, err := requireProject()
	if err != nil {
		return err
	}

	rule := strings.TrimSpace(strings.Join(args, " "))
	if rule == "" {
		return fmt.Errorf("rule text cannot be empty")
	}

	path := filepath.Join(projectDir, ".claude", "rules", "05-project-specific.md")
	existing := ""
	if data, err := os.ReadFile(path); err == nil {
		existing = string(data)
	}

	// Dedup: skip if the exact line already exists.
	for _, line := range strings.Split(existing, "\n") {
		if strings.TrimSpace(line) == "- "+rule {
			tui.Warn("Rule already present — skipping")
			return nil
		}
	}

	if existing == "" {
		existing = "# Project-Specific Rules\n\n> Auto-maintained by ccb — add rules via: ccb add rule \"...\"\n\n## Rules\n\n"
	} else if !strings.Contains(existing, "## Rules") {
		existing += "\n## Rules\n\n"
	} else if !strings.HasSuffix(existing, "\n") {
		existing += "\n"
	}

	existing += "- " + rule + "\n"

	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	if err := os.WriteFile(path, []byte(existing), 0644); err != nil {
		return err
	}

	rel, _ := filepath.Rel(projectDir, path)
	tui.Success(fmt.Sprintf("Rule added to %s", rel))
	fmt.Printf("  • %s\n", rule)
	return nil
}
