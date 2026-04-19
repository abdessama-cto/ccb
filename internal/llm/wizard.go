package llm

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/abdessama-cto/ccb/internal/analyzer"
)

// WizardQuestion is a single question the LLM asks the user about the project.
type WizardQuestion struct {
	ID       string   `json:"id"`
	Question string   `json:"question"`
	Subtitle string   `json:"subtitle"`
	Type     string   `json:"type"` // "choice" | "yesno" | "text"
	Options  []string `json:"options"`
	Default  string   `json:"default"`
}

// WizardAnswer holds the user's answer to a WizardQuestion.
type WizardAnswer struct {
	ID       string `json:"id"`
	Question string `json:"question"`
	Value    string `json:"value"`
}

// GenerateWizardQuestions asks the LLM to produce 1 to 10 questions that are
// specific to the project being bootstrapped, based on the AI understanding
// and the static fingerprint.
func GenerateWizardQuestions(cfg Config, u *ProjectUnderstanding, fp *analyzer.ProjectFingerprint) ([]WizardQuestion, error) {
	prompt := buildWizardPrompt(u, fp) + "\n\n" + LanguageDirective(cfg)
	raw, err := CallLLM(cfg, prompt)
	if err != nil {
		return nil, err
	}
	return parseWizardQuestions(raw)
}

func buildWizardPrompt(u *ProjectUnderstanding, fp *analyzer.ProjectFingerprint) string {
	var sb strings.Builder
	sb.WriteString(`You are configuring Claude Code for a specific project.
Based on the AI understanding below, generate between 1 and 10 questions to ask the engineer.

STRICT RULES:
- Questions MUST be specific to THIS project (its code, domain, stack, conventions, risks you observed).
- DO NOT ask generic questions like "Team size?", "Workflow style?", "Primary goal?", "Install skills?".
- Focus on: architecture decisions not obvious from the code, domain-specific risks (payment, auth, multi-tenant, compliance),
  conventions to enforce, gotchas detected, ambiguities that only the engineer can resolve.
- Prefer FEWER high-signal questions over many generic ones. 1 excellent question is better than 5 mediocre ones.
- Every question must have a safe/conservative default so the wizard can be skipped with --yes.

Return ONLY this JSON (no markdown fences, no preamble, no trailing text):
{
  "questions": [
    {
      "id": "snake_case_id",
      "question": "Clear question text ending with a question mark?",
      "subtitle": "Why this question matters, referencing specific files, patterns, or risks you observed",
      "type": "yesno" | "choice" | "text",
      "options": ["only for type=choice, 2-5 items"],
      "default": "recommended/safer default answer"
    }
  ]
}

`)

	sb.WriteString("## Project understanding\n")
	if b, err := json.MarshalIndent(u, "", "  "); err == nil {
		sb.Write(b)
		sb.WriteString("\n\n")
	}

	sb.WriteString("## Static fingerprint\n")
	sb.WriteString(fmt.Sprintf("Stack: %s\n", fp.StackString()))
	sb.WriteString(fmt.Sprintf("Language: %s\n", fp.Language))
	sb.WriteString(fmt.Sprintf("LOC: %d across %d files\n", fp.LOC, fp.Files))
	sb.WriteString(fmt.Sprintf("Tests: %s\n", fp.TestFrameworksString()))
	sb.WriteString(fmt.Sprintf("CI: %v · Docker: %v · .env: %v\n", fp.HasCI, fp.HasDocker, fp.HasEnvFile))
	sb.WriteString(fmt.Sprintf("Age: %s\n", fp.Age))

	return sb.String()
}

func parseWizardQuestions(raw string) ([]WizardQuestion, error) {
	raw = StripJSONFences(raw)

	var wrapper struct {
		Questions []WizardQuestion `json:"questions"`
	}
	if err := json.Unmarshal([]byte(raw), &wrapper); err != nil {
		preview := raw
		if len(preview) > 300 {
			preview = preview[:300]
		}
		return nil, fmt.Errorf("wizard JSON parse failed: %w (raw: %s)", err, preview)
	}

	// Validate + cap
	clean := make([]WizardQuestion, 0, len(wrapper.Questions))
	for _, q := range wrapper.Questions {
		if strings.TrimSpace(q.Question) == "" {
			continue
		}
		if q.Type == "" {
			q.Type = "yesno"
		}
		clean = append(clean, q)
	}
	if len(clean) > 10 {
		clean = clean[:10]
	}
	return clean, nil
}
