// Package skills handles skill discovery from the local Antigravity skills directory.
package skills

import (
	"os"
	"path/filepath"
	"strings"
)

// DiskSkill is a skill found in the user's skills directory.
type DiskSkill struct {
	Name        string // from frontmatter `name:` or folder name
	Description string // from frontmatter `description:` (first 120 chars)
	FolderName  string // directory name (used as install key)
	SkillPath   string // absolute path to SKILL.md
	Preview     string // first ~300 chars of content after frontmatter
}

// DefaultSkillsDir returns the path to the user's Antigravity skills directory.
func DefaultSkillsDir() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".gemini", "antigravity", "skills")
}

// ScanDiskSkills reads all SKILL.md files from the skills directory and
// returns a list of parsed skills with name, description, and preview.
func ScanDiskSkills(skillsDir string) []DiskSkill {
	if skillsDir == "" {
		skillsDir = DefaultSkillsDir()
	}

	entries, err := os.ReadDir(skillsDir)
	if err != nil {
		return nil
	}

	var skills []DiskSkill
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		skillMD := filepath.Join(skillsDir, e.Name(), "SKILL.md")
		data, err := os.ReadFile(skillMD)
		if err != nil {
			continue
		}

		ds := DiskSkill{
			FolderName: e.Name(),
			SkillPath:  skillMD,
			Name:       e.Name(), // fallback
		}

		content := string(data)
		ds.Name, ds.Description, ds.Preview = parseSkillMD(content, e.Name())
		skills = append(skills, ds)
	}
	return skills
}

// Search returns skills from the list that match the query.
// It searches name, description, and preview text.
func Search(skills []DiskSkill, query string) []DiskSkill {
	if query == "" {
		return skills
	}
	q := strings.ToLower(query)
	var out []DiskSkill
	for _, s := range skills {
		if strings.Contains(strings.ToLower(s.Name), q) ||
			strings.Contains(strings.ToLower(s.Description), q) ||
			strings.Contains(strings.ToLower(s.Preview), q) {
			out = append(out, s)
		}
	}
	return out
}

// parseSkillMD extracts name, description, and preview from SKILL.md content.
func parseSkillMD(content, fallbackName string) (name, description, preview string) {
	name = fallbackName
	lines := strings.Split(content, "\n")

	inFrontmatter := false
	frontmatterEnd := 0
	fenceCount := 0

	for i, line := range lines {
		trimmed := strings.TrimSpace(line)

		if trimmed == "---" {
			fenceCount++
			if fenceCount == 1 {
				inFrontmatter = true
				continue
			}
			if fenceCount == 2 {
				inFrontmatter = false
				frontmatterEnd = i + 1
				break
			}
		}

		if inFrontmatter {
			if strings.HasPrefix(trimmed, "name:") {
				val := strings.TrimSpace(strings.TrimPrefix(trimmed, "name:"))
				val = strings.Trim(val, `"'`)
				if val != "" {
					name = val
				}
			}
			if strings.HasPrefix(trimmed, "description:") {
				val := strings.TrimSpace(strings.TrimPrefix(trimmed, "description:"))
				val = strings.Trim(val, `"'`)
				if val != "" {
					description = val
				}
			}
		}
	}

	// Trim description to 120 chars
	if len(description) > 120 {
		// Cut at last word boundary
		cut := 117
		for cut > 0 && description[cut] != ' ' {
			cut--
		}
		description = description[:cut] + "…"
	}

	// Build preview from content after frontmatter (exclude markdown headings)
	if frontmatterEnd < len(lines) {
		var previewLines []string
		for _, l := range lines[frontmatterEnd:] {
			t := strings.TrimSpace(l)
			if t == "" || strings.HasPrefix(t, "#") {
				continue
			}
			previewLines = append(previewLines, t)
			if len(strings.Join(previewLines, " ")) > 300 {
				break
			}
		}
		preview = strings.Join(previewLines, " ")
		if len(preview) > 300 {
			preview = preview[:297] + "…"
		}
	}

	return name, description, preview
}
