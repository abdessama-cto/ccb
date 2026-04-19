package llm

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/abdessama-cto/ccb/internal/analyzer"
)

// AgentProposal describes a Claude Code subagent suggested by the LLM.
type AgentProposal struct {
	ID          string `json:"id"`
	Filename    string `json:"filename"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Reason      string `json:"reason"`
	Selected    bool   `json:"selected"`
}

// RuleProposal describes a project-specific rule suggested by the LLM.
type RuleProposal struct {
	ID       string `json:"id"`
	Rule     string `json:"rule"`
	Reason   string `json:"reason"`
	Selected bool   `json:"selected"`
}

// SkillProposal describes a Claude Code skill suggested by the LLM.
type SkillProposal struct {
	ID          string `json:"id"`
	Filename    string `json:"filename"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Reason      string `json:"reason"`
	Selected    bool   `json:"selected"`
}

// Proposals is the full set of LLM suggestions for the project.
type Proposals struct {
	Agents []AgentProposal `json:"agents"`
	Rules  []RuleProposal  `json:"rules"`
	Skills []SkillProposal `json:"skills"`
}

// GenerateProposals asks the LLM to propose agents, rules, and skills,
// using the project understanding, the fingerprint, and the wizard answers.
func GenerateProposals(cfg Config, u *ProjectUnderstanding, fp *analyzer.ProjectFingerprint, answers []WizardAnswer) (*Proposals, error) {
	prompt := buildProposalsPrompt(u, fp, answers)
	raw, err := CallLLM(cfg, prompt)
	if err != nil {
		return nil, err
	}
	return parseProposals(raw)
}

func buildProposalsPrompt(u *ProjectUnderstanding, fp *analyzer.ProjectFingerprint, answers []WizardAnswer) string {
	var sb strings.Builder
	sb.WriteString(`You are configuring Claude Code for a specific project.
Propose the agents, rules, and skills that would genuinely help an engineer
working on THIS codebase. Quality > quantity.

RULES:
- Every proposal MUST be justified by something observed in the project (a file, a pattern, a domain, a wizard answer).
- DO NOT propose generic agents that would apply to any project unless clearly justified.
- Filenames: lowercase kebab-case, .md extension (e.g. "payment-webhook-guard.md").
- "selected" = true means the item should be enabled by default. Set false for items
  that are useful but optional.
- Limits: up to 8 agents, up to 12 rules, up to 10 skills.

Return ONLY this JSON (no markdown fences, no preamble):
{
  "agents": [
    {
      "id": "snake_case_id",
      "filename": "kebab-case.md",
      "name": "kebab-case-name",
      "description": "one-line description of what the agent does and when to invoke it",
      "reason": "why this specific agent for THIS project — cite code/domain signals",
      "selected": true
    }
  ],
  "rules": [
    {
      "id": "snake_case_id",
      "rule": "the rule text as it should appear in the rules file",
      "reason": "why this rule for THIS project",
      "selected": true
    }
  ],
  "skills": [
    {
      "id": "snake_case_id",
      "filename": "kebab-case.md",
      "name": "kebab-case-name",
      "description": "what the skill teaches Claude to do",
      "reason": "why this skill for THIS project",
      "selected": true
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
	sb.WriteString(fmt.Sprintf("Language: %s\nTests: %s\n", fp.Language, fp.TestFrameworksString()))
	sb.WriteString(fmt.Sprintf("CI: %v · Docker: %v · .env: %v\n\n", fp.HasCI, fp.HasDocker, fp.HasEnvFile))

	if len(answers) > 0 {
		sb.WriteString("## Wizard answers (engineer's intent)\n")
		for _, a := range answers {
			sb.WriteString(fmt.Sprintf("- [%s] %s → %q\n", a.ID, a.Question, a.Value))
		}
		sb.WriteString("\n")
	}

	return sb.String()
}

func parseProposals(raw string) (*Proposals, error) {
	raw = StripJSONFences(raw)

	var p Proposals
	if err := json.Unmarshal([]byte(raw), &p); err != nil {
		preview := raw
		if len(preview) > 300 {
			preview = preview[:300]
		}
		return nil, fmt.Errorf("proposals JSON parse failed: %w (raw: %s)", err, preview)
	}

	// Sanity-cap
	if len(p.Agents) > 12 {
		p.Agents = p.Agents[:12]
	}
	if len(p.Rules) > 20 {
		p.Rules = p.Rules[:20]
	}
	if len(p.Skills) > 15 {
		p.Skills = p.Skills[:15]
	}

	// Ensure filenames end in .md
	for i := range p.Agents {
		p.Agents[i].Filename = ensureMD(p.Agents[i].Filename, p.Agents[i].Name)
	}
	for i := range p.Skills {
		p.Skills[i].Filename = ensureMD(p.Skills[i].Filename, p.Skills[i].Name)
	}

	return &p, nil
}

func ensureMD(filename, fallback string) string {
	f := strings.TrimSpace(filename)
	if f == "" {
		f = fallback
	}
	f = strings.ToLower(strings.ReplaceAll(f, " ", "-"))
	if !strings.HasSuffix(f, ".md") {
		f += ".md"
	}
	return f
}
