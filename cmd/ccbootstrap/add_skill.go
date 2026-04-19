package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/abdessama-cto/ccb/internal/skills"
	"github.com/abdessama-cto/ccb/internal/tui"
	"github.com/spf13/cobra"
)

var addSkillCmd = &cobra.Command{
	Use:   "skill [name]",
	Short: "Add one or more skills from awesome-claude-skills",
	Long: `Add skills to .claude/skills/.

Without arguments: opens an interactive picker with all 800+ catalog skills.
Press "/" to search, SPACE to toggle, ENTER to install the selected ones.

With a name: installs that skill directly by folder name (e.g. "changelog-generator").`,
	Args: cobra.ArbitraryArgs,
	RunE: runAddSkill,
}

func runAddSkill(cmd *cobra.Command, args []string) error {
	projectDir, err := requireProject()
	if err != nil {
		return err
	}

	tui.Info("Loading skill catalog (awesome-claude-skills)...")
	catalog := skills.LoadSkills()
	if len(catalog) == 0 {
		return fmt.Errorf("skill catalog empty — is git installed and network reachable?")
	}
	tui.Success(fmt.Sprintf("%d skills available in the catalog", len(catalog)))

	// ── Direct install by name ───────────────────────────────────────────────
	if len(args) > 0 {
		return installSkillsByName(projectDir, catalog, args)
	}

	// ── Interactive picker ───────────────────────────────────────────────────
	// Show a searchable checkbox pre-populated with the full catalog (first 200)
	// and let the user search the rest via "/".
	items := make([]CheckItem, 0, 20)
	preview := catalog
	if len(preview) > 20 {
		preview = preview[:20]
	}
	for _, s := range preview {
		items = append(items, CheckItem{
			Label:    s.FolderName,
			Detail:   s.Description,
			Selected: false,
		})
	}

	result := InteractiveCheckbox(
		"🔧 Add skills — "+fmt.Sprintf("%d available", len(catalog)),
		"Press / to search the full catalog. SPACE to toggle. ENTER to install.",
		items,
		true,
	)

	selected := map[string]bool{}
	for _, it := range result {
		if it.Selected {
			selected[it.Label] = true
		}
	}
	if len(selected) == 0 {
		tui.Warn("Nothing selected — no skills installed")
		return nil
	}

	var names []string
	for k := range selected {
		names = append(names, k)
	}
	return installSkillsByName(projectDir, catalog, names)
}

// installSkillsByName copies SKILL.md files from the catalog into the current
// project's .claude/skills/ directory.
func installSkillsByName(projectDir string, catalog []skills.DiskSkill, names []string) error {
	skillsDir := filepath.Join(projectDir, ".claude", "skills")
	if err := os.MkdirAll(skillsDir, 0755); err != nil {
		return err
	}

	installed := 0
	for _, n := range names {
		match := findSkill(catalog, n)
		if match == nil {
			tui.Warn(fmt.Sprintf("Skill %q not found — did you mean something else? Try: ccb search %s", n, n))
			continue
		}
		data, err := os.ReadFile(match.SkillPath)
		if err != nil {
			tui.Warn(fmt.Sprintf("Could not read %s: %s", match.SkillPath, err))
			continue
		}
		dest := filepath.Join(skillsDir, match.FolderName+".md")
		if err := os.WriteFile(dest, data, 0644); err != nil {
			return fmt.Errorf("write %s: %w", dest, err)
		}
		rel, _ := filepath.Rel(projectDir, dest)
		fmt.Printf("  %s %s\n", tui.Green("✓"), rel)
		installed++
	}

	if installed == 0 {
		return fmt.Errorf("no skills installed")
	}
	tui.Success(fmt.Sprintf("Installed %d skill(s)", installed))
	return nil
}

// findSkill looks up a skill by exact folder name or by case-insensitive match.
func findSkill(catalog []skills.DiskSkill, query string) *skills.DiskSkill {
	for i := range catalog {
		if catalog[i].FolderName == query {
			return &catalog[i]
		}
	}
	lower := strings.ToLower(query)
	for i := range catalog {
		if strings.EqualFold(catalog[i].FolderName, lower) {
			return &catalog[i]
		}
	}
	return nil
}
