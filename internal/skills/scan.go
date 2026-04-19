package skills

import (
	"os"
	"path/filepath"
	"strings"
)

// DiskSkill is a skill parsed from a SKILL.md file on disk.
type DiskSkill struct {
	Name        string // from frontmatter `name:` or folder name
	Description string // from frontmatter `description:` (first 120 chars)
	FolderName  string // directory name (used as install key)
	SkillPath   string // absolute path to SKILL.md
	Preview     string // first ~300 chars of content after frontmatter
}

// DefaultSkillsDir returns the cache path where awesome-claude-skills is cloned.
func DefaultSkillsDir() string {
	return CacheDir()
}

// LoadSkills ensures the awesome-claude-skills repo is present (cloning or
// pulling if needed) and returns the scanned SKILL.md index.
func LoadSkills() []DiskSkill {
	dir, _ := EnsureRepo()
	return ScanDiskSkills(dir)
}

// ScanDiskSkills walks the skills directory recursively and returns every
// SKILL.md it finds, parsed into DiskSkill entries. It handles the layout
// of awesome-claude-skills, which mixes top-level skills with nested
// groups like composio-skills/<app>/SKILL.md and document-skills/<sub>/SKILL.md.
func ScanDiskSkills(skillsDir string) []DiskSkill {
	if skillsDir == "" {
		skillsDir = DefaultSkillsDir()
	}

	var skills []DiskSkill
	_ = filepath.Walk(skillsDir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info == nil {
			return nil
		}
		if info.IsDir() {
			name := info.Name()
			// Skip VCS and hidden dirs to keep the walk fast.
			if name == ".git" || strings.HasPrefix(name, ".") {
				return filepath.SkipDir
			}
			return nil
		}
		if info.Name() != "SKILL.md" {
			return nil
		}

		folderName := filepath.Base(filepath.Dir(path))
		data, readErr := os.ReadFile(path)
		if readErr != nil {
			return nil
		}

		ds := DiskSkill{
			FolderName: folderName,
			SkillPath:  path,
			Name:       folderName,
		}
		ds.Name, ds.Description, ds.Preview = parseSkillMD(string(data), folderName)
		skills = append(skills, ds)
		return nil
	})
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
