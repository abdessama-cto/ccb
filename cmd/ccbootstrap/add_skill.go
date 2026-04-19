package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/abdessama-cto/ccb/internal/skills"
	"github.com/abdessama-cto/ccb/internal/tui"
	"github.com/spf13/cobra"
)

var addSkillCmd = &cobra.Command{
	Use:   "skill [query or name]",
	Short: "Add a skill from skills.sh",
	Long: `Add a skill to .claude/skills/ by searching skills.sh.

Without arguments: opens an interactive search picker.
With arguments: searches skills.sh and installs the first exact match (or the
top result for a multi-word query).

Examples:
  ccb add skill
  ccb add skill supabase-postgres-best-practices
  ccb add skill stripe`,
	Args: cobra.ArbitraryArgs,
	RunE: runAddSkill,
}

func runAddSkill(cmd *cobra.Command, args []string) error {
	projectDir, err := requireProject()
	if err != nil {
		return err
	}

	// ── Interactive mode (no args) ───────────────────────────────────────────
	if len(args) == 0 {
		return runAddSkillInteractive(projectDir)
	}

	// ── Direct-install mode ──────────────────────────────────────────────────
	query := args[0]
	searchSpin := tui.StartSpinner(fmt.Sprintf("Searching skills.sh for %q...", query))
	results, err := skills.Search(query, 20)
	if err != nil {
		searchSpin.Fail("Search failed")
		return fmt.Errorf("search failed: %w", err)
	}
	if len(results) == 0 {
		searchSpin.Fail(fmt.Sprintf("No skills matched %q", query))
		return fmt.Errorf("no skills matched %q", query)
	}
	searchSpin.Success(fmt.Sprintf("Found %d result(s)", len(results)))

	// Prefer an exact match on skillId, else the top result.
	chosen := results[0]
	for _, r := range results {
		if r.SkillID == query {
			chosen = r
			break
		}
	}
	installSpin := tui.StartSpinner(fmt.Sprintf("Installing %s from %s (%d installs)...",
		chosen.SkillID, chosen.Source, chosen.Installs))
	err = fetchAndWriteSkill(projectDir, &chosen)
	if err != nil {
		installSpin.Fail("Install failed")
		return err
	}
	installSpin.Success(fmt.Sprintf("Installed %s at .claude/skills/%s.md", chosen.SkillID, chosen.SkillID))
	return nil
}

// runAddSkillInteractive shows the TUI picker with an empty main list; the
// user drives the flow via "/" search on skills.sh.
func runAddSkillInteractive(projectDir string) error {
	tui.Info("Press / to search skills.sh, SPACE to add, ENTER when done.")
	result := InteractiveCheckbox(
		"🔧 Add skills from skills.sh",
		"Search the catalog with /, add with SPACE, confirm with ENTER.",
		[]CheckItem{},
		true,
	)

	installed := 0
	for _, it := range result {
		if !it.Selected || it.SkillRef == nil {
			continue
		}
		ref := it.SkillRef
		if err := fetchAndWriteSkill(projectDir, ref); err != nil {
			tui.Warn(fmt.Sprintf("  %s: %s — skipping", ref.SkillID, err.Error()))
			continue
		}
		fmt.Printf("  %s .claude/skills/%s.md\n", tui.Green("✓"), ref.SkillID)
		installed++
	}
	if installed == 0 {
		tui.Warn("Nothing selected — no skills installed")
		return nil
	}
	tui.Success(fmt.Sprintf("Installed %d skill(s)", installed))
	return nil
}

// fetchAndWriteSkill downloads a SKILL.md from GitHub raw and writes it into
// the project's .claude/skills/ directory.
func fetchAndWriteSkill(projectDir string, s *skills.Skill) error {
	if err := skills.FetchContent(s); err != nil {
		return err
	}
	if s.Content == "" {
		return fmt.Errorf("empty content")
	}

	skillsDir := filepath.Join(projectDir, ".claude", "skills")
	if err := os.MkdirAll(skillsDir, 0755); err != nil {
		return err
	}
	dest := filepath.Join(skillsDir, s.SkillID+".md")
	if err := os.WriteFile(dest, []byte(s.Content), 0644); err != nil {
		return err
	}
	return nil
}
