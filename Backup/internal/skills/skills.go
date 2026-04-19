package skills

import (
	"fmt"
	"os"
	"os/exec"
)

// Install runs `npx skills add <owner/repo>` in the given directory
func Install(repoDir, skillRepo string) error {
	cmd := exec.Command("npx", "skills", "add", skillRepo)
	cmd.Dir = repoDir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("skills add %s failed: %w", skillRepo, err)
	}
	return nil
}

// RecommendedSkills returns a curated list of skills for the detected stack
func RecommendedSkills(stack []string) []string {
	skillSet := map[string]bool{}

	// Always include meta-skill
	skillSet["vercel-labs/skills"] = true
	skillSet["anthropics/skills"] = true

	for _, s := range stack {
		switch {
		case contains(s, "Laravel"):
			skillSet["obra/superpowers"] = true
		case contains(s, "Next"), contains(s, "React"):
			skillSet["vercel-labs/agent-skills"] = true
		case contains(s, "NestJS"):
			skillSet["obra/superpowers"] = true
		case contains(s, "Django"), contains(s, "FastAPI"), contains(s, "Flask"):
			skillSet["obra/superpowers"] = true
		case s == "Go" || len(s) > 2 && s[:3] == "Go/":
			skillSet["obra/superpowers"] = true
		}
	}

	result := make([]string, 0, len(skillSet))
	// Always first
	result = append(result, "vercel-labs/skills", "anthropics/skills")
	for k := range skillSet {
		if k != "vercel-labs/skills" && k != "anthropics/skills" {
			result = append(result, k)
		}
	}
	return result
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsStr(s, substr))
}

func containsStr(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
