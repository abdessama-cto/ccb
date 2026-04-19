package llm

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/abdessama-cto/ccb/internal/analyzer"
)

// GeneratedFile is one file produced by the final generation LLM call.
type GeneratedFile struct {
	Path    string `json:"path"`
	Content string `json:"content"`
}

// GenerationResult holds all files the LLM produced.
type GenerationResult struct {
	Files []GeneratedFile `json:"files"`
}

// GenerateFiles makes a single LLM call that produces every dynamic file
// Claude Code needs: CLAUDE.md, rules, agents, skills, docs. Structural
// files like settings.json and shell hooks are NOT generated here — they
// stay deterministic in the generator package.
func GenerateFiles(
	cfg Config,
	u *ProjectUnderstanding,
	fp *analyzer.ProjectFingerprint,
	answers []WizardAnswer,
	agents []AgentProposal,
	rules []RuleProposal,
	skills []SkillProposal,
) (*GenerationResult, error) {
	prompt := buildGenerationPrompt(u, fp, answers, agents, rules, skills) + "\n\n" + LanguageDirective(cfg)
	raw, err := CallLLM(cfg, prompt)
	if err != nil {
		return nil, err
	}
	return parseGeneration(raw)
}

func buildGenerationPrompt(
	u *ProjectUnderstanding,
	fp *analyzer.ProjectFingerprint,
	answers []WizardAnswer,
	agents []AgentProposal,
	rules []RuleProposal,
	skills []SkillProposal,
) string {
	var sb strings.Builder
	sb.WriteString(`You are writing the Claude Code configuration for a specific project.
Produce the full content of every file listed in the OUTPUT MANIFEST below.

RULES:
- Use the project understanding, the wizard answers, and the selected items as input.
- Tailor every file to THIS project. Do NOT write generic boilerplate that would apply anywhere.
- Agents and skills: include YAML frontmatter with "name", "description", and "tools" when relevant.
  Agent description should make it clear when Claude should invoke the agent.
- Rules files: concise, actionable, enforceable. Cite project-specific examples where useful.
- CLAUDE.md: start with a one-paragraph purpose statement, then list the stack, key modules,
  project conventions, and strict rules. Keep it under ~150 lines.
- docs/architecture.md: expand on architecture with module breakdown, data flow, external services.

Return ONLY this JSON (no markdown fences, no preamble, no trailing text):
{
  "files": [
    { "path": "CLAUDE.md", "content": "..." },
    { "path": ".claude/rules/01-core-behavior.md", "content": "..." },
    ...
  ]
}

`)

	sb.WriteString("## Project understanding\n")
	if b, err := json.MarshalIndent(u, "", "  "); err == nil {
		sb.Write(b)
		sb.WriteString("\n\n")
	}

	sb.WriteString("## Static fingerprint\n")
	sb.WriteString(fmt.Sprintf("Stack: %s\nLanguage: %s\nLOC: %d across %d files\nTests: %s\nCI: %v · Docker: %v · .env: %v\n\n",
		fp.StackString(), fp.Language, fp.LOC, fp.Files,
		fp.TestFrameworksString(), fp.HasCI, fp.HasDocker, fp.HasEnvFile))

	if len(answers) > 0 {
		sb.WriteString("## Wizard answers\n")
		for _, a := range answers {
			sb.WriteString(fmt.Sprintf("- [%s] %s → %q\n", a.ID, a.Question, a.Value))
		}
		sb.WriteString("\n")
	}

	sb.WriteString("## OUTPUT MANIFEST — produce every file listed below\n\n")

	sb.WriteString("### Always produce\n")
	sb.WriteString("- `CLAUDE.md` — main Claude Code project file\n")
	sb.WriteString("- `.claude/rules/01-core-behavior.md` — workflow + collaboration style\n")
	sb.WriteString("- `.claude/rules/02-git-workflow.md` — branching, commits, PR discipline\n")
	sb.WriteString("- `.claude/rules/03-testing.md` — testing expectations for this stack\n")
	sb.WriteString("- `.claude/rules/04-code-quality.md` — quality + security rules for this language/framework\n")
	sb.WriteString("- `.claude/rules/05-project-specific.md` — rules derived from the wizard + selected rules below\n")
	sb.WriteString("- `docs/architecture.md` — architecture documentation for this codebase\n\n")

	if len(agents) > 0 {
		sb.WriteString("### Selected agents — produce one file per agent\n")
		for _, a := range agents {
			sb.WriteString(fmt.Sprintf("- `.claude/agents/%s` — name: %s — %s — reason: %s\n",
				a.Filename, a.Name, a.Description, a.Reason))
		}
		sb.WriteString("\n")
	}

	if len(skills) > 0 {
		sb.WriteString("### Selected skills — produce one file per skill\n")
		for _, s := range skills {
			sb.WriteString(fmt.Sprintf("- `.claude/skills/%s` — name: %s — %s — reason: %s\n",
				s.Filename, s.Name, s.Description, s.Reason))
		}
		sb.WriteString("\n")
	}

	if len(rules) > 0 {
		sb.WriteString("### Selected project-specific rules — include in 05-project-specific.md\n")
		for _, r := range rules {
			sb.WriteString(fmt.Sprintf("- %s (reason: %s)\n", r.Rule, r.Reason))
		}
		sb.WriteString("\n")
	}

	return sb.String()
}

func parseGeneration(raw string) (*GenerationResult, error) {
	raw = StripJSONFences(raw)

	var res GenerationResult
	if err := json.Unmarshal([]byte(raw), &res); err != nil {
		preview := raw
		if len(preview) > 400 {
			preview = preview[:400]
		}
		return nil, fmt.Errorf("generation JSON parse failed: %w (raw preview: %s)", err, preview)
	}

	// Filter out empty or unsafe paths
	clean := make([]GeneratedFile, 0, len(res.Files))
	for _, f := range res.Files {
		p := strings.TrimSpace(f.Path)
		if p == "" || strings.Contains(p, "..") || strings.HasPrefix(p, "/") {
			continue
		}
		clean = append(clean, GeneratedFile{Path: p, Content: f.Content})
	}
	res.Files = clean
	return &res, nil
}
