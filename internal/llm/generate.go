package llm

import (
	"bufio"
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

// File block delimiters used by the plain-text generation protocol. This
// format avoids JSON entirely so the LLM doesn't have to escape quotes,
// backslashes, or newlines inside long freeform content — which is the
// root cause of parse failures we kept hitting.
const (
	fileOpenMarker  = "=== FILE:"
	fileCloseMarker = "=== END FILE ==="
)

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
The audience is a developer who will invoke Claude on THIS codebase daily. Your
goal is to produce files that MAKE THEIR LIFE EASIER — not to lecture them.

PHILOSOPHY — follow it in every file:
- Prefer guidance over commandments. Claude Code works best with context, not
  "rules". Never use all-caps imperatives ("NEVER", "ALWAYS", "MUST"). Instead,
  describe how things work here and why: "When X, prefer Y because Z".
- Point to real code. Cite concrete file paths, function names, and modules
  you observed — not abstract principles. Every claim should be grounded.
- Short and useful beats long and exhaustive. If a file has nothing
  project-specific to say, keep it tight. Generic boilerplate is noise.
- Small projects deserve small configs. A 2k-LOC CLI does not need a 10-rule
  manifesto. Scale the content to the project.

OUTPUT FORMAT — emit blocks with these exact markers on their own lines:

=== FILE: <relative/path> ===
<raw file content — no escaping, no JSON, no triple backticks wrapping>
=== END FILE ===

- The closing marker is exactly "=== END FILE ===" on its own line.
- Do not add commentary outside the blocks. Produce every file in the manifest.

CLAUDE CODE TOOL NAMES (for agent "tools:" frontmatter — use these exact names,
never invent others):

  Read, Write, Edit, Bash, Grep, Glob, WebFetch, WebSearch, Agent, BashOutput, KillShell, Skill, Task, TodoWrite, NotebookEdit

AGENT FILES (.claude/agents/*.md) — EVERY agent MUST have:

1. YAML frontmatter:
     ---
     name: kebab-case-name
     description: one sentence that makes it obvious when Claude should invoke this agent (include example trigger phrases).
     tools: Read, Edit, Bash  (comma-separated; pick from the list above, only what the agent actually needs)
     ---
2. A BODY — at least 25 lines of actionable content:
     - A "## When to use this agent" section with 2-3 concrete trigger scenarios from THIS codebase.
     - A "## How this agent works" section: the step-by-step approach it follows.
     - A "## Relevant files in this project" section naming 3-8 real paths the agent will touch.
     - Optional: examples of good vs. bad output, or common pitfalls.

   An agent with only frontmatter is a BUG — do not emit one.

SKILL FILES (.claude/skills/*.md) — each skill is a procedure Claude can run.
Every skill MUST have:

1. YAML frontmatter:
     ---
     name: kebab-case-name
     description: 1-2 sentences describing what the skill teaches Claude to do in this project.
     ---
2. A BODY with a concrete, step-by-step procedure (at least 20 lines):
     - A "## When to apply" section with 1-2 trigger situations.
     - A numbered "## Steps" section — actionable steps grounded in THIS codebase (reference real files and functions).
     - A "## Example" section (optional but encouraged): one small before/after or shell snippet.

RULES FILES (.claude/rules/0N-*.md) — guidance, not commandments:

- Phrase things as "When you're doing X, here's how this project does it" instead of "NEVER do X".
- Ground every item in a real file or a real pattern seen in the code.
- Keep each file under 40 lines. If you cannot fill 10 useful lines, skip
  generic ones — a short rules file beats a padded one.
- 05-project-specific.md in particular should contain ONLY things unique to
  this project. Do not duplicate anything already stated in CLAUDE.md.

CLAUDE.md:

- Start with 2-3 sentences explaining what the project does and who uses it.
- "## Stack" — bulleted, concise.
- "## How to run / test / build" — the actual commands for this project.
  Skip this section if nothing is obvious from the code.
- "## Key modules" — a short guided tour of 4-8 directories or files with
  one-line descriptions of each.
- "## Conventions" — patterns you observed in the code (not generic best
  practices). 4-8 items.
- "## Guidance for Claude" — replaces the old "Strict Rules" section. Max 6
  items, phrased as guidance ("When adding X, follow Y"), not as commandments.
  Reference the real rules files for detail instead of duplicating content.

docs/architecture.md:

- Target ~60-120 lines for small projects, longer only if the project is big.
- Cover: high-level flow, module breakdown (each module = 1-3 lines), external
  integrations, and any unusual patterns. Concrete paths > diagrams unless the
  architecture is genuinely graph-shaped.

EVIDENCE CHECK — before referencing any file or function, confirm it appears
in the "Project understanding" block below. If it doesn't, do not invent it.

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

	sb.WriteString("## Project size hint\n")
	sb.WriteString(projectSizeHint(fp))
	sb.WriteString("\n\n")

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
	sb.WriteString("- `.claude/rules/04-code-quality.md` — quality + security guidance for this language/framework\n")
	sb.WriteString("- `.claude/rules/05-project-specific.md` — guidance unique to THIS project (no overlap with CLAUDE.md)\n")
	sb.WriteString("- `docs/architecture.md` — architecture documentation for this codebase\n\n")

	if len(agents) > 0 {
		sb.WriteString("### Selected agents — produce one file per agent (each with a FULL body, not just frontmatter)\n")
		for _, a := range agents {
			sb.WriteString(fmt.Sprintf("- `.claude/agents/%s` — name: %s — %s — reason: %s\n",
				a.Filename, a.Name, a.Description, a.Reason))
		}
		sb.WriteString("\n")
	}

	if len(skills) > 0 {
		sb.WriteString("### Selected skills — produce one file per skill (each with a FULL body, not just frontmatter)\n")
		for _, s := range skills {
			sb.WriteString(fmt.Sprintf("- `.claude/skills/%s` — name: %s — %s — reason: %s\n",
				s.Filename, s.Name, s.Description, s.Reason))
		}
		sb.WriteString("\n")
	}

	if len(rules) > 0 {
		sb.WriteString("### Selected project-specific guidance — include in 05-project-specific.md\n")
		for _, r := range rules {
			sb.WriteString(fmt.Sprintf("- %s (reason: %s)\n", r.Rule, r.Reason))
		}
		sb.WriteString("\n")
	}

	return sb.String()
}

// projectSizeHint nudges the LLM to scale the output to project size so small
// repos don't get 10-rule manifestos.
func projectSizeHint(fp *analyzer.ProjectFingerprint) string {
	switch {
	case fp.LOC < 3_000:
		return "This is a small codebase. Keep CLAUDE.md under ~80 lines, each rule file under ~25 lines, and do not pad with generic content."
	case fp.LOC < 20_000:
		return "This is a mid-sized codebase. CLAUDE.md around 100-140 lines is appropriate; rule files 25-40 lines each."
	default:
		return "This is a large codebase. You can afford more detail in CLAUDE.md and docs/architecture.md, but still avoid generic fluff — ground every claim in real files."
	}
}

// parseGeneration scans the LLM output for `=== FILE: path ===` / `=== END FILE ===`
// blocks and collects them into a GenerationResult. Content between markers
// is preserved verbatim — no unescaping required.
//
// Backward compatibility: if the LLM returned JSON (older format), we fall
// back to the JSON parser with the sanitizer.
func parseGeneration(raw string) (*GenerationResult, error) {
	files := scanFileBlocks(raw)
	if len(files) > 0 {
		return &GenerationResult{Files: sanitizePaths(files)}, nil
	}

	// Fallback: try legacy JSON parse (still sanitized).
	cleaned := StripJSONFences(raw)
	var res GenerationResult
	if err := json.Unmarshal([]byte(cleaned), &res); err != nil {
		preview := raw
		if len(preview) > 400 {
			preview = preview[:400]
		}
		return nil, fmt.Errorf("generation parse failed: no file blocks found and JSON fallback failed: %w (preview: %s)", err, preview)
	}
	return &GenerationResult{Files: sanitizePaths(res.Files)}, nil
}

// scanFileBlocks extracts every `=== FILE: path === ... === END FILE ===`
// block from the LLM output. Tolerant of whitespace and surrounding prose.
func scanFileBlocks(raw string) []GeneratedFile {
	var files []GeneratedFile
	scanner := bufio.NewScanner(strings.NewReader(raw))
	// Allow long lines — default buffer is 64KB which is too small for full files.
	buf := make([]byte, 0, 1024*1024)
	scanner.Buffer(buf, 8*1024*1024)

	var (
		inBlock bool
		curPath string
		curBody strings.Builder
	)

	flush := func() {
		if inBlock && curPath != "" {
			files = append(files, GeneratedFile{
				Path:    curPath,
				Content: strings.TrimRight(curBody.String(), "\n") + "\n",
			})
		}
		inBlock = false
		curPath = ""
		curBody.Reset()
	}

	for scanner.Scan() {
		line := scanner.Text()
		trimmed := strings.TrimSpace(line)

		if !inBlock {
			if path, ok := parseFileOpenLine(trimmed); ok {
				inBlock = true
				curPath = path
				curBody.Reset()
			}
			continue
		}

		// Inside a block.
		if trimmed == fileCloseMarker {
			flush()
			continue
		}
		// Handle back-to-back blocks where the LLM forgot the closing marker.
		if path, ok := parseFileOpenLine(trimmed); ok {
			flush()
			inBlock = true
			curPath = path
			curBody.Reset()
			continue
		}

		curBody.WriteString(line)
		curBody.WriteByte('\n')
	}

	// Final unclosed block — accept it rather than drop the LLM's work.
	if inBlock && curPath != "" {
		files = append(files, GeneratedFile{
			Path:    curPath,
			Content: strings.TrimRight(curBody.String(), "\n") + "\n",
		})
	}
	return files
}

// parseFileOpenLine returns the path if the line is an opening marker.
// Accepts "=== FILE: path ===" and "=== FILE: path===" variants.
func parseFileOpenLine(line string) (string, bool) {
	if !strings.HasPrefix(line, fileOpenMarker) {
		return "", false
	}
	rest := strings.TrimPrefix(line, fileOpenMarker)
	rest = strings.TrimSpace(rest)
	// Strip the trailing "===".
	rest = strings.TrimSuffix(rest, "===")
	path := strings.TrimSpace(rest)
	if path == "" {
		return "", false
	}
	return path, true
}

// sanitizePaths drops empty/unsafe paths so we never escape the project root.
func sanitizePaths(files []GeneratedFile) []GeneratedFile {
	clean := make([]GeneratedFile, 0, len(files))
	for _, f := range files {
		p := strings.TrimSpace(f.Path)
		if p == "" || strings.Contains(p, "..") || strings.HasPrefix(p, "/") {
			continue
		}
		// Strip backticks if the LLM wrapped the path.
		p = strings.Trim(p, "`'\" ")
		clean = append(clean, GeneratedFile{Path: p, Content: f.Content})
	}
	return clean
}
