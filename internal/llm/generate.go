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
Produce the full content of every file listed in the OUTPUT MANIFEST below.

CONTENT RULES:
- Use the project understanding, the wizard answers, and the selected items as input.
- Tailor every file to THIS project. Do NOT write generic boilerplate that would apply anywhere.
- Agents and skills: include YAML frontmatter with "name", "description", and "tools" when relevant.
  Agent description should make it clear when Claude should invoke the agent.
- Rules files: concise, actionable, enforceable. Cite project-specific examples where useful.
- CLAUDE.md: start with a one-paragraph purpose statement, then list the stack, key modules,
  project conventions, and strict rules. Keep it under ~150 lines.
- docs/architecture.md: expand on architecture with module breakdown, data flow, external services.

OUTPUT FORMAT — STRICTLY FOLLOW THIS (NOT JSON):

For every file, emit a block delimited by these exact markers on their own lines:

=== FILE: <relative/path> ===
<raw file content here — no escaping, no JSON, no backticks wrapping>
=== END FILE ===

Rules for the format:
- The opening marker MUST be "=== FILE: " followed by the path and " ===" on a single line.
- The closing marker MUST be exactly "=== END FILE ===" on its own line.
- Put the raw file content between the markers, exactly as it should be written to disk.
- You do NOT need to escape quotes, backslashes, or newlines — write content as-is.
- Do NOT wrap the content in triple backticks or any code fence.
- Do NOT add commentary, preamble, or trailing prose outside the blocks.
- Produce every file from the manifest in order.

Example (for illustration only — produce the actual files requested below):

=== FILE: CLAUDE.md ===
# My Project

Some prose with "quotes" and \backslashes\ that need no escaping.
=== END FILE ===

=== FILE: .claude/rules/01-core-behavior.md ===
# Core Behavior
- Rule 1
- Rule 2
=== END FILE ===

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
