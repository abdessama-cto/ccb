package skills

import (
	"os"
	"path/filepath"
	"testing"
)

// TestScanDiskSkillsWalksRecursively verifies the walker finds SKILL.md at
// both top-level and nested depths (mimicking awesome-claude-skills layout).
func TestScanDiskSkillsWalksRecursively(t *testing.T) {
	root := t.TempDir()

	makeSkill := func(rel, name, desc string) {
		full := filepath.Join(root, rel)
		if err := os.MkdirAll(filepath.Dir(full), 0755); err != nil {
			t.Fatal(err)
		}
		content := "---\nname: " + name + "\ndescription: " + desc + "\n---\n\n# body"
		if err := os.WriteFile(full, []byte(content), 0644); err != nil {
			t.Fatal(err)
		}
	}

	makeSkill("top-skill/SKILL.md", "top-skill", "top level skill")
	makeSkill("composio-skills/slack-automation/SKILL.md", "slack-automation", "posts to slack")
	makeSkill("document-skills/pdf/SKILL.md", "pdf", "parses PDFs")
	// noise that should be ignored
	if err := os.WriteFile(filepath.Join(root, "README.md"), []byte("noise"), 0644); err != nil {
		t.Fatal(err)
	}

	found := ScanDiskSkills(root)
	if len(found) != 3 {
		t.Fatalf("expected 3 skills, got %d: %+v", len(found), found)
	}

	names := map[string]bool{}
	for _, s := range found {
		names[s.Name] = true
	}
	for _, want := range []string{"top-skill", "slack-automation", "pdf"} {
		if !names[want] {
			t.Errorf("missing skill %q in results", want)
		}
	}
}
