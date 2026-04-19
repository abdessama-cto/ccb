package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/abdessama-cto/ccb/internal/tui"
	"github.com/spf13/cobra"
)

var listCmd = &cobra.Command{
	Use:     "list [type]",
	Aliases: []string{"ls"},
	Short:   "List installed agents, skills, rules, commands, and hooks",
	Long: `Show what is installed in .claude/ for the current project.

Without type: shows every category.
With type: shows only that category.

Examples:
  ccb list
  ccb list skills
  ccb list agents`,
	Args:      cobra.MaximumNArgs(1),
	ValidArgs: []string{"agents", "skills", "rules", "commands", "hooks"},
	RunE:      runList,
}

func init() {
	rootCmd.AddCommand(listCmd)
}

func runList(cmd *cobra.Command, args []string) error {
	projectDir, err := requireProject()
	if err != nil {
		return err
	}

	filter := ""
	if len(args) > 0 {
		filter = strings.ToLower(args[0])
	}

	sections := []struct {
		key    string
		label  string
		dir    string
		suffix string
	}{
		{"agents", "🤖 Agents", filepath.Join(projectDir, ".claude", "agents"), ".md"},
		{"skills", "🔧 Skills", filepath.Join(projectDir, ".claude", "skills"), ".md"},
		{"rules", "📐 Rules", filepath.Join(projectDir, ".claude", "rules"), ".md"},
		{"commands", "⚡ Commands", filepath.Join(projectDir, ".claude", "commands"), ".md"},
		{"hooks", "🪝 Hooks", filepath.Join(projectDir, ".claude", "hooks"), ".sh"},
	}

	fmt.Printf("\n%s %s\n", tui.Bold("📦"), tui.Bold("Installed in .claude/"))

	total := 0
	for _, sec := range sections {
		if filter != "" && filter != sec.key {
			continue
		}
		files := listFiles(sec.dir, sec.suffix)
		fmt.Printf("\n  %s  %s\n", tui.Bold(sec.label), tui.Dim(fmt.Sprintf("(%d)", len(files))))
		if len(files) == 0 {
			fmt.Printf("    %s\n", tui.Dim("(none)"))
			continue
		}
		for _, f := range files {
			name := strings.TrimSuffix(f, sec.suffix)
			desc := extractFrontmatterDesc(filepath.Join(sec.dir, f))
			if desc != "" {
				fmt.Printf("    %s  %s\n", tui.Cyan(name), tui.Dim(truncate(desc, 60)))
			} else {
				fmt.Printf("    %s\n", tui.Cyan(name))
			}
			total++
		}
	}
	fmt.Printf("\n%s Total: %d item(s)\n\n", tui.Green("✓"), total)
	return nil
}

func listFiles(dir, suffix string) []string {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	var out []string
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		if suffix != "" && !strings.HasSuffix(e.Name(), suffix) {
			continue
		}
		out = append(out, e.Name())
	}
	sort.Strings(out)
	return out
}

// extractFrontmatterDesc reads the `description:` field from a markdown file's
// YAML frontmatter, or returns an empty string if none is present.
func extractFrontmatterDesc(path string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	content := string(data)
	if !strings.HasPrefix(strings.TrimSpace(content), "---") {
		return ""
	}
	lines := strings.Split(content, "\n")
	fenceCount := 0
	for _, line := range lines {
		t := strings.TrimSpace(line)
		if t == "---" {
			fenceCount++
			if fenceCount == 2 {
				return ""
			}
			continue
		}
		if fenceCount == 1 && strings.HasPrefix(t, "description:") {
			val := strings.TrimSpace(strings.TrimPrefix(t, "description:"))
			return strings.Trim(val, `"'`)
		}
	}
	return ""
}
