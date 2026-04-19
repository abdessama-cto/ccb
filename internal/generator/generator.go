package generator

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/abdessama-cto/ccb/internal/analyzer"
	"github.com/abdessama-cto/ccb/internal/llm"
	"github.com/abdessama-cto/ccb/internal/tui"
)

// Questionnaire holds the hook-related settings derived from profile defaults.
// Dynamic project questions now live in the wizard (llm.WizardQuestion).
type Questionnaire struct {
	AutoFormatHook bool
	SecretScanHook bool
	AutoCommitHook bool
	DesktopNotify  bool
	PushGuardHook  bool
	AuditLogHook   bool
	InstallSkills  bool
	RunTests       bool
}

// Generate writes the Claude Code configuration to targetDir.
// It writes two kinds of files:
//   1. LLM-generated content (CLAUDE.md, rules, agents, skills, docs/architecture.md)
//   2. Deterministic infrastructure (settings.json, hooks/*.sh, commands/*.md, docs/progress.md)
func Generate(
	targetDir string,
	fp *analyzer.ProjectFingerprint,
	q *Questionnaire,
	understanding *llm.ProjectUnderstanding,
	llmFiles []llm.GeneratedFile,
) error {
	if err := ensureDirs(targetDir); err != nil {
		return err
	}

	// Deterministic structural files — always written.
	deterministic := map[string]string{
		filepath.Join(targetDir, ".claude", "settings.json"):           generateSettings(fp, q),
		filepath.Join(targetDir, ".claude", "commands", "context.md"):  generateContextCommand(),
		filepath.Join(targetDir, ".claude", "commands", "ship.md"):     generateShipCommand(fp),
		filepath.Join(targetDir, ".claude", "commands", "review.md"):   generateReviewCommand(),
		filepath.Join(targetDir, ".claude", "commands", "test.md"):     generateTestCommand(fp),
		filepath.Join(targetDir, ".claude", "commands", "progress.md"): generateProgressCommand(),
		filepath.Join(targetDir, "docs", "progress.md"):                generateProgressMD(),
	}

	// Conditional hooks
	if q.AutoFormatHook {
		deterministic[filepath.Join(targetDir, ".claude", "hooks", "post-edit-format.sh")] = hookAutoFormat(fp)
	}
	deterministic[filepath.Join(targetDir, ".claude", "hooks", "session-start-context.sh")] = hookSessionStart()
	if q.SecretScanHook {
		deterministic[filepath.Join(targetDir, ".claude", "hooks", "pre-edit-secret-scan.sh")] = hookSecretScan()
	}
	if q.AutoCommitHook {
		deterministic[filepath.Join(targetDir, ".claude", "hooks", "post-edit-auto-commit.sh")] = hookAutoCommit()
	}
	if q.DesktopNotify {
		deterministic[filepath.Join(targetDir, ".claude", "hooks", "stop-notify.sh")] = hookStopNotify()
	}
	if q.PushGuardHook {
		deterministic[filepath.Join(targetDir, ".claude", "hooks", "pre-push-tests.sh")] = hookPrePushTests(fp)
	}
	if q.AuditLogHook {
		deterministic[filepath.Join(targetDir, ".claude", "hooks", "pre-bash-audit.sh")] = hookAuditLog()
	}

	// ── Write LLM-generated files with progress display ─────────────────────
	if len(llmFiles) > 0 {
		fmt.Println()
		tui.Info("Writing AI-generated files:")
		if err := writeWithProgress(targetDir, llmFiles); err != nil {
			return err
		}
	}

	// ── Write deterministic files ─────────────────────────────────────────────
	tui.Info("Writing infrastructure files (settings, hooks, commands):")
	paths := make([]string, 0, len(deterministic))
	for p := range deterministic {
		paths = append(paths, p)
	}
	sort.Strings(paths)
	for _, p := range paths {
		if err := writeFile(p, deterministic[p]); err != nil {
			return fmt.Errorf("write %s: %w", p, err)
		}
		rel, _ := filepath.Rel(targetDir, p)
		fmt.Printf("  %s %s\n", tui.Green("✓"), tui.Dim(rel))
	}

	// Fallback: if the LLM didn't produce docs/architecture.md, write a stub.
	archPath := filepath.Join(targetDir, "docs", "architecture.md")
	if _, err := os.Stat(archPath); os.IsNotExist(err) {
		_ = writeFile(archPath, generateArchitectureStub(fp, understanding))
	}

	// Make hooks executable
	hooksDir := filepath.Join(targetDir, ".claude", "hooks")
	entries, _ := os.ReadDir(hooksDir)
	for _, e := range entries {
		_ = os.Chmod(filepath.Join(hooksDir, e.Name()), 0755)
	}

	return nil
}

func ensureDirs(targetDir string) error {
	dirs := []string{
		filepath.Join(targetDir, ".claude", "rules"),
		filepath.Join(targetDir, ".claude", "hooks"),
		filepath.Join(targetDir, ".claude", "commands"),
		filepath.Join(targetDir, ".claude", "agents"),
		filepath.Join(targetDir, ".claude", "skills"),
		filepath.Join(targetDir, "docs", "decisions"),
		filepath.Join(targetDir, "docs", "solutions"),
		filepath.Join(targetDir, "docs", "brainstorms"),
	}
	for _, d := range dirs {
		if err := os.MkdirAll(d, 0755); err != nil {
			return fmt.Errorf("mkdir %s: %w", d, err)
		}
	}
	return nil
}

func writeWithProgress(targetDir string, files []llm.GeneratedFile) error {
	// Sort by path for predictable output
	sort.Slice(files, func(i, j int) bool { return files[i].Path < files[j].Path })

	for _, f := range files {
		full := filepath.Join(targetDir, f.Path)
		if err := os.MkdirAll(filepath.Dir(full), 0755); err != nil {
			return fmt.Errorf("mkdir %s: %w", filepath.Dir(full), err)
		}
		if err := writeFile(full, f.Content); err != nil {
			return fmt.Errorf("write %s: %w", f.Path, err)
		}
		size := formatBytes(len(f.Content))
		fmt.Printf("  %s %-50s %s\n", tui.Green("✓"), f.Path, tui.Dim(size))
	}
	return nil
}

func formatBytes(n int) string {
	if n < 1024 {
		return fmt.Sprintf("%d B", n)
	}
	return fmt.Sprintf("%.1f KB", float64(n)/1024)
}

// ─── .claude/settings.json ────────────────────────────────────────────────────

func generateSettings(fp *analyzer.ProjectFingerprint, q *Questionnaire) string {
	type settings struct {
		Permissions struct {
			Allow []string `json:"allow"`
			Deny  []string `json:"deny"`
			Ask   []string `json:"ask"`
		} `json:"permissions"`
		Env map[string]string `json:"env"`
	}

	s := settings{}
	s.Permissions.Allow = buildAllowPatterns(fp)
	s.Permissions.Deny = []string{
		"Bash(rm -rf*)",
		"Bash(rm -fr*)",
		"Bash(sudo rm*)",
		"Bash(git push --force*)",
		"Bash(git reset --hard*)",
		"Write(.env)",
		"Write(.env.local)",
		"Write(.env.production)",
		"Write(**/secrets/**)",
		"Read(**/*.pem)",
		"Read(**/*.key)",
		"Read(**/id_rsa)",
	}
	s.Permissions.Ask = buildAskPatterns(fp)
	s.Env = map[string]string{"CLAUDE_AUTOCOMPACT_PCT_OVERRIDE": "75"}

	data, _ := json.MarshalIndent(s, "", "  ")
	return string(data) + "\n"
}

func buildAllowPatterns(fp *analyzer.ProjectFingerprint) []string {
	base := []string{
		"Bash(git status)",
		"Bash(git diff*)",
		"Bash(git log*)",
		"Bash(ls*)",
		"Bash(cat*)",
		"Bash(grep*)",
		"Bash(find*)",
		"Bash(echo*)",
		"Read(*)",
	}
	for _, stk := range fp.Stack {
		switch {
		case strings.Contains(stk, "Laravel"):
			base = append(base, "Bash(php artisan*)", "Bash(composer*)")
		case strings.Contains(stk, "Next") || strings.Contains(stk, "React") || strings.Contains(stk, "Node") || strings.Contains(stk, "NestJS"):
			base = append(base, "Bash(npm run*)", "Bash(npx*)")
		case strings.Contains(stk, "Django") || strings.Contains(stk, "FastAPI") || strings.Contains(stk, "Flask"):
			base = append(base, "Bash(python*)", "Bash(pytest*)")
		case stk == "Go" || strings.HasPrefix(stk, "Go/"):
			base = append(base, "Bash(go build*)", "Bash(go test*)")
		}
	}
	return base
}

func buildAskPatterns(fp *analyzer.ProjectFingerprint) []string {
	asks := []string{
		"Bash(git push*)",
		"Bash(git reset*)",
		"Bash(git rebase*)",
	}
	for _, stk := range fp.Stack {
		if strings.Contains(stk, "Laravel") {
			asks = append(asks, "Bash(php artisan migrate*)", "Bash(composer update)")
		}
		if strings.Contains(stk, "Next") || strings.Contains(stk, "NestJS") {
			asks = append(asks, "Bash(npm install)")
		}
	}
	return asks
}

// ─── Commands (.claude/commands/*.md) ─────────────────────────────────────────

func generateContextCommand() string {
	return `# /context — Show current context usage and project state

Analyze the current session:
1. Show token usage estimate
2. List open files and their sizes
3. Show current git branch and uncommitted changes
4. Display the last 5 entries in docs/progress.md
5. List any TODO comments in recently touched files
`
}

func generateShipCommand(fp *analyzer.ProjectFingerprint) string {
	testCmd := buildTestCommand(fp)
	return fmt.Sprintf(`# /ship — Prepare and ship current changes

Execute this sequence:
1. Run the test suite: `+"`%s`"+`
2. If tests fail, stop and report failures
3. Run git diff --staged to review changes
4. Create a conventional commit message based on the changes
5. Suggest a PR title and description
6. Confirm with user before pushing
`, testCmd)
}

func generateReviewCommand() string {
	return `# /review — Code review checklist

Review the current changes against:
1. **Correctness**: Does the code do what it's supposed to?
2. **Tests**: Are there tests? Do they cover edge cases?
3. **Security**: Any hardcoded secrets? SQL injection risks? Unvalidated inputs?
4. **Performance**: Any N+1 queries? Unnecessary loops?
5. **Maintainability**: Is the code readable? Are functions too long?
6. **Conventions**: Does it follow the project's coding conventions?

Provide a structured report with: ✅ Good | ⚠️ Concern | ❌ Must fix
`
}

func generateTestCommand(fp *analyzer.ProjectFingerprint) string {
	testCmd := buildTestCommand(fp)
	return fmt.Sprintf(`# /test — Run test suite

Run: `+"`%s`"+`

Then:
1. Report pass/fail count
2. List any failing tests with their error messages
3. Suggest fixes for failing tests if the cause is clear
4. Do NOT modify tests to make them pass — fix the implementation
`, testCmd)
}

func generateProgressCommand() string {
	return `# /progress — Update session progress

Append to docs/progress.md:
1. Date and time
2. What was accomplished this session
3. What's in progress (if any)
4. Next steps / blockers
5. Any important decisions made

Keep entries concise (< 10 lines each).
`
}

// ─── Hooks (.claude/hooks/*.sh) ───────────────────────────────────────────────

func hookAutoFormat(fp *analyzer.ProjectFingerprint) string {
	return `#!/bin/bash
# .claude/hooks/post-edit-format.sh — Auto-format after edit (PostToolUse)
FILE=$(echo "$CLAUDE_TOOL_INPUT" | jq -r '.file_path // .path // ""' 2>/dev/null)
[[ -z "$FILE" ]] && exit 0

case "$FILE" in
  *.php) [[ -x ./vendor/bin/pint ]] && ./vendor/bin/pint "$FILE" >/dev/null 2>&1 ;;
  *.js|*.ts|*.vue|*.jsx|*.tsx|*.json)
    npx prettier --write "$FILE" >/dev/null 2>&1 || true ;;
  *.py) ruff format "$FILE" >/dev/null 2>&1 || black "$FILE" >/dev/null 2>&1 || true ;;
  *.go) gofmt -w "$FILE" 2>/dev/null || true ;;
  *.rb) rubocop -a "$FILE" >/dev/null 2>&1 || true ;;
esac
exit 0
`
}

func hookSessionStart() string {
	return `#!/bin/bash
# .claude/hooks/session-start-context.sh — Inject git context (SessionStart)
BRANCH=$(git branch --show-current 2>/dev/null || echo "detached")
LAST_COMMIT=$(git log -1 --pretty="%s (%cr)" 2>/dev/null || echo "none")
UNCOMMITTED=$(git status --porcelain 2>/dev/null | wc -l | tr -d ' ')
PROGRESS=""
[[ -f docs/progress.md ]] && PROGRESS=$(tail -30 docs/progress.md)

cat <<EOF
{
  "additionalContext": "Git: $BRANCH | Last: $LAST_COMMIT | Uncommitted: $UNCOMMITTED files\n\nRecent progress:\n$PROGRESS"
}
EOF
`
}

func hookSecretScan() string {
	return `#!/bin/bash
# .claude/hooks/pre-edit-secret-scan.sh — Scan content for secrets (PreToolUse)
CONTENT=$(echo "$CLAUDE_TOOL_INPUT" | jq -r '.content // .new_string // ""' 2>/dev/null)
[[ -z "$CONTENT" ]] && exit 0

PATTERNS=(
  'AKIA[0-9A-Z]{16}'
  'sk-[a-zA-Z0-9]{48}'
  'sk-ant-[a-zA-Z0-9-]{95}'
  'ghp_[a-zA-Z0-9]{36}'
  'AIza[0-9A-Za-z\-_]{35}'
  'xox[baprs]-[0-9a-zA-Z-]{10,48}'
  'BEGIN (RSA |DSA |EC |OPENSSH |)PRIVATE KEY'
)

for pattern in "${PATTERNS[@]}"; do
  if echo "$CONTENT" | grep -qE "$pattern" 2>/dev/null; then
    echo "🚫 Secret pattern detected: $pattern" >&2
    echo "Remove the secret and use environment variables instead." >&2
    exit 2
  fi
done
exit 0
`
}

func hookAutoCommit() string {
	return `#!/bin/bash
# .claude/hooks/post-edit-auto-commit.sh — Atomic commit after edit (PostToolUse)
FILE=$(echo "$CLAUDE_TOOL_INPUT" | jq -r '.file_path // .path // ""' 2>/dev/null)
[[ -z "$FILE" ]] && exit 0
[[ ! -d .git ]] && exit 0

BRANCH=$(git branch --show-current 2>/dev/null)
[[ "$BRANCH" == "main" || "$BRANCH" == "master" ]] && exit 0

git add "$FILE" 2>/dev/null || exit 0
SHORT=$(basename "$FILE")
git commit -m "chore(ai): edit $SHORT" --no-verify 2>/dev/null || true
exit 0
`
}

func hookStopNotify() string {
	return `#!/bin/bash
# .claude/hooks/stop-notify.sh — Desktop notification on task complete (Stop)
osascript -e 'display notification "Claude Code finished your task" with title "ccb" sound name "Glass"' 2>/dev/null || true
exit 0
`
}

func hookPrePushTests(fp *analyzer.ProjectFingerprint) string {
	testCmd := buildTestCommand(fp)
	return fmt.Sprintf(`#!/bin/bash
# .claude/hooks/pre-push-tests.sh — Run tests before git push (PreToolUse)
CMD=$(echo "$CLAUDE_TOOL_INPUT" | jq -r '.command // ""' 2>/dev/null)
[[ "$CMD" != *"git push"* ]] && exit 0

echo "🧪 Running tests before push..." >&2
%s
if [[ $? -ne 0 ]]; then
  echo "❌ Tests failing. Push blocked. Fix tests first." >&2
  exit 2
fi
echo "✅ Tests green. Push proceeding." >&2
exit 0
`, testCmd)
}

func hookAuditLog() string {
	return `#!/bin/bash
# .claude/hooks/pre-bash-audit.sh — Audit log for bash commands (PreToolUse)
CMD=$(echo "$CLAUDE_TOOL_INPUT" | jq -r '.command // ""' 2>/dev/null)
[[ -z "$CMD" ]] && exit 0

TIMESTAMP=$(date -Iseconds 2>/dev/null || date)
SESSION="${CLAUDE_SESSION_ID:-unknown}"
LOG_DIR=".claude/audit"
mkdir -p "$LOG_DIR"
printf '%s [session:%s] %s\n' "$TIMESTAMP" "$SESSION" "$CMD" >> "$LOG_DIR/bash-commands.log"
exit 0
`
}

// ─── Docs ─────────────────────────────────────────────────────────────────────

func generateArchitectureStub(fp *analyzer.ProjectFingerprint, u *llm.ProjectUnderstanding) string {
	if u != nil && u.Architecture != "" {
		return fmt.Sprintf(`# Architecture

> Auto-generated fallback — the AI did not produce docs/architecture.md this run.

## Overview
%s

## Stack
%s
`, u.Architecture, formatStack(fp.Stack))
	}
	return fmt.Sprintf(`# Architecture

> Auto-generated by ccb — update this file to reflect the real architecture.

## Stack
%s

## Structure
_Document your directory structure here._
`, formatStack(fp.Stack))
}

func generateProgressMD() string {
	return fmt.Sprintf(`# Session Progress

## %s — Initial Setup

- ✅ Bootstrapped with ccb
- ✅ Generated CLAUDE.md, .claude/, docs/ structure

## Next Steps
- [ ] Review the AI-generated CLAUDE.md and refine
- [ ] Review .claude/rules/ and adjust if needed
- [ ] Start development
`, time.Now().Format("2006-01-02"))
}

// ─── Helpers ──────────────────────────────────────────────────────────────────

func writeFile(path, content string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(content), 0644)
}

func formatStack(stack []string) string {
	result := ""
	for _, s := range stack {
		result += "- " + s + "\n"
	}
	return result
}

func buildTestCommand(fp *analyzer.ProjectFingerprint) string {
	for _, s := range fp.Stack {
		switch {
		case strings.Contains(s, "Laravel"):
			return "php artisan test --parallel"
		case strings.Contains(s, "NestJS"), strings.Contains(s, "Next"), strings.Contains(s, "React"):
			return "npm test"
		case strings.Contains(s, "Django"), strings.Contains(s, "FastAPI"), strings.Contains(s, "Flask"):
			return "pytest"
		case s == "Go" || strings.HasPrefix(s, "Go/"):
			return "go test ./..."
		case strings.Contains(s, "Rails"):
			return "bundle exec rspec"
		}
	}
	return "# No test command detected — add yours here"
}
