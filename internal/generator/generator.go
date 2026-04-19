package generator

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/abdessama-cto/ccb/internal/analyzer"
	"github.com/abdessama-cto/ccb/internal/llm"
)

// Questionnaire holds answers from the interactive setup
type Questionnaire struct {
	Goal             string // quality | ship-fast | stability | refactor
	WorkflowStyle    string // plan-execute | vibe | spec-driven
	TeamSize         string // solo | small | medium | large
	AutoFormatHook   bool
	SecretScanHook   bool
	AutoCommitHook   bool
	DesktopNotify    bool
	PushGuardHook    bool
	AuditLogHook     bool
	InstallSkills    bool
	RunTests         bool
	CreatePR         bool
	BranchName       string
}

// Generate creates the full Claude Code config in targetDir
func Generate(targetDir string, fp *analyzer.ProjectFingerprint, q *Questionnaire, understanding *llm.ProjectUnderstanding) error {
	dirs := []string{
		filepath.Join(targetDir, ".claude", "rules"),
		filepath.Join(targetDir, ".claude", "hooks"),
		filepath.Join(targetDir, ".claude", "commands"),
		filepath.Join(targetDir, ".claude", "agents"),
		filepath.Join(targetDir, "docs", "decisions"),
		filepath.Join(targetDir, "docs", "solutions"),
		filepath.Join(targetDir, "docs", "brainstorms"),
	}
	for _, d := range dirs {
		if err := os.MkdirAll(d, 0755); err != nil {
			return fmt.Errorf("mkdir %s: %w", d, err)
		}
	}

	files := map[string]string{
		filepath.Join(targetDir, "CLAUDE.md"):                                   generateClaudeMD(fp, q, understanding),
		filepath.Join(targetDir, ".claude", "settings.json"):                    generateSettings(fp, q),
		filepath.Join(targetDir, ".claude", "rules", "01-core-behavior.md"):     generateCoreRules(q),
		filepath.Join(targetDir, ".claude", "rules", "02-git-workflow.md"):      generateGitRules(q),
		filepath.Join(targetDir, ".claude", "rules", "03-testing.md"):           generateTestingRules(fp, q),
		filepath.Join(targetDir, ".claude", "rules", "04-code-quality.md"):      generateQualityRules(fp),
		filepath.Join(targetDir, ".claude", "commands", "context.md"):           generateContextCommand(),
		filepath.Join(targetDir, ".claude", "commands", "ship.md"):              generateShipCommand(fp),
		filepath.Join(targetDir, ".claude", "commands", "review.md"):            generateReviewCommand(),
		filepath.Join(targetDir, ".claude", "commands", "test.md"):              generateTestCommand(fp),
		filepath.Join(targetDir, ".claude", "commands", "progress.md"):          generateProgressCommand(),
		filepath.Join(targetDir, "docs", "architecture.md"):                     generateArchitectureMD(fp, understanding),
		filepath.Join(targetDir, "docs", "progress.md"):                         generateProgressMD(),
	}

	// Conditional hooks
	if q.AutoFormatHook {
		files[filepath.Join(targetDir, ".claude", "hooks", "post-edit-format.sh")] = hookAutoFormat(fp)
	}
	files[filepath.Join(targetDir, ".claude", "hooks", "session-start-context.sh")] = hookSessionStart()
	if q.SecretScanHook {
		files[filepath.Join(targetDir, ".claude", "hooks", "pre-edit-secret-scan.sh")] = hookSecretScan()
	}
	if q.AutoCommitHook {
		files[filepath.Join(targetDir, ".claude", "hooks", "post-edit-auto-commit.sh")] = hookAutoCommit()
	}
	if q.DesktopNotify {
		files[filepath.Join(targetDir, ".claude", "hooks", "stop-notify.sh")] = hookStopNotify()
	}
	if q.PushGuardHook {
		files[filepath.Join(targetDir, ".claude", "hooks", "pre-push-tests.sh")] = hookPrePushTests(fp)
	}
	if q.AuditLogHook {
		files[filepath.Join(targetDir, ".claude", "hooks", "pre-bash-audit.sh")] = hookAuditLog()
	}

	for path, content := range files {
		if err := writeFile(path, content); err != nil {
			return fmt.Errorf("write %s: %w", path, err)
		}
	}

	// Make hooks executable
	hooksDir := filepath.Join(targetDir, ".claude", "hooks")
	entries, _ := os.ReadDir(hooksDir)
	for _, e := range entries {
		_ = os.Chmod(filepath.Join(hooksDir, e.Name()), 0755)
	}

	return nil
}

// ─── CLAUDE.md ────────────────────────────────────────────────────────────────

func generateClaudeMD(fp *analyzer.ProjectFingerprint, q *Questionnaire, understanding *llm.ProjectUnderstanding) string {
	repoName := repoNameFromURL(fp.RepoURL)

	// Use AI understanding if available
	purposeSection := "_No AI understanding available — add your OpenAI key with: ccbootstrap settings_"
	archSection := ""
	featuresSection := ""
	moduleSection := ""
	conventionsSection := ""
	whatClaudeSection := ""
	extServicesSection := ""

	if understanding != nil {
		if understanding.ProjectName != "" {
			repoName = understanding.ProjectName
		}
		if understanding.Purpose != "" {
			purposeSection = understanding.Purpose
		}
		if understanding.Architecture != "" {
			archSection = "\n## Architecture\n" + understanding.Architecture
		}
		if len(understanding.KeyFeatures) > 0 {
			featuresSection = "\n## Key Features\n" + formatList(understanding.KeyFeatures)
		}
		if len(understanding.MainModules) > 0 {
			moduleSection = "\n## Main Modules\n" + formatList(understanding.MainModules)
		}
		if len(understanding.Conventions) > 0 {
			conventionsSection = "\n## Project-Specific Conventions\n" + formatList(understanding.Conventions)
		}
		if understanding.WhatClaudeKnows != "" {
			whatClaudeSection = "\n## What Claude Should Know\n" + understanding.WhatClaudeKnows
		}
		if len(understanding.ExternalServices) > 0 {
			extServicesSection = "\n## External Services\n" + formatList(understanding.ExternalServices)
		}
	}

	return fmt.Sprintf(`# Project: %s

> Generated by [ccbootstrap](https://github.com/abdessama-cto/ccb) on %s

## Purpose
%s
%s%s%s%s%s%s
## Tech Stack
%s
## Project Profile
- **Goal**: %s
- **Workflow**: %s
- **Team size**: %s
- **LOC**: %s across %d files
- **Commits**: %d (%s)
- **Tests**: %s
- **CI/CD**: %s
- **Docker**: %s

## Commands
| Command | Purpose |
|---|---|
%s

## Strict Rules
- Never refactor code outside the scope of the requested task
- Always confirm before modifying files outside the main task scope
- Read @docs/solutions/ before proposing solutions to recurring problems
- Update @docs/progress.md at the end of each session

## References
- @docs/architecture.md — full architecture details
- @docs/decisions/ — Architecture Decision Records
- @docs/solutions/ — Solved problems with context
- @docs/progress.md — current session progress
`,
		repoName,
		time.Now().Format("2006-01-02"),
		purposeSection,
		archSection,
		featuresSection,
		moduleSection,
		conventionsSection,
		whatClaudeSection,
		extServicesSection,
		formatStack(fp.Stack),
		q.Goal,
		q.WorkflowStyle,
		q.TeamSize,
		formatLOC(fp.LOC),
		fp.Files,
		fp.Commits,
		fp.Age,
		fp.TestFrameworksString(),
		boolEmoji(fp.HasCI),
		boolEmoji(fp.HasDocker),
		buildCommandsTable(fp),
	)
}

// ─── .claude/settings.json ────────────────────────────────────────────────────

func generateSettings(fp *analyzer.ProjectFingerprint, q *Questionnaire) string {
	askList := buildAskList(fp)
	return fmt.Sprintf(`{
  "permissions": {
    "allow": [
      "Bash(git status)",
      "Bash(git diff*)",
      "Bash(git log*)",
      "Bash(ls*)",
      "Bash(cat*)",
      "Bash(grep*)",
      "Bash(find*)",
      "Bash(echo*)",
      "Read(*)",%s
    ],
    "deny": [
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
      "Read(**/id_rsa)"
    ],
    "ask": [%s
    ]
  },
  "env": {
    "CLAUDE_AUTOCOMPACT_PCT_OVERRIDE": "75"
  }
}
`,
		buildAllowList(fp),
		askList,
	)
}

func buildAllowList(fp *analyzer.ProjectFingerprint) string {
	extras := []string{}
	for _, s := range fp.Stack {
		switch {
		case strings.Contains(s, "Laravel"):
			extras = append(extras, `      "Bash(php artisan*)"`, `      "Bash(composer*)"`)
		case strings.Contains(s, "Next") || strings.Contains(s, "React") || strings.Contains(s, "Node") || strings.Contains(s, "NestJS"):
			extras = append(extras, `      "Bash(npm run*)"`, `      "Bash(npx*)"`)
		case strings.Contains(s, "Django") || strings.Contains(s, "FastAPI") || strings.Contains(s, "Flask"):
			extras = append(extras, `      "Bash(python*)"`, `      "Bash(pytest*)"`)
		case s == "Go" || strings.HasPrefix(s, "Go/"):
			extras = append(extras, `      "Bash(go build*)"`, `      "Bash(go test*)"`)
		}
	}
	if len(extras) == 0 {
		return ""
	}
	return "\n" + strings.Join(extras, ",\n") + ","
}

func buildAskList(fp *analyzer.ProjectFingerprint) string {
	asks := []string{`
      "Bash(git push*)"`,
		`"Bash(git reset*)"`,
		`"Bash(git rebase*)"`,
	}
	for _, s := range fp.Stack {
		if strings.Contains(s, "Laravel") {
			asks = append(asks, `"Bash(php artisan migrate*)"`, `"Bash(composer update)"`)
		}
		if strings.Contains(s, "Next") || strings.Contains(s, "NestJS") {
			asks = append(asks, `"Bash(npm install)"`)
		}
	}
	return "\n      " + strings.Join(asks, ",\n      ")
}

// ─── Rules ────────────────────────────────────────────────────────────────────

func generateCoreRules(q *Questionnaire) string {
	workflowDesc := map[string]string{
		"plan-execute": "Always plan before implementing. Write a brief plan in a code block, wait for confirmation, then execute.",
		"vibe":         "Move fast and iterate. Implement directly, explain as you go.",
		"spec-driven":  "Follow the spec strictly. Ask for clarification on ambiguities before implementing.",
	}
	return fmt.Sprintf(`# Core Behavior Rules

## Workflow Style: %s
%s

## General Rules
- Be concise and precise in explanations
- Don't add unrequested features or refactor out-of-scope code
- Prefer editing existing code over creating new files when possible
- Always read before writing — understand the context first

## Goal Alignment: %s
- Prioritize %s over speed
- Ask before breaking changes
`,
		q.WorkflowStyle,
		workflowDesc[q.WorkflowStyle],
		q.Goal,
		q.Goal,
	)
}

func generateGitRules(q *Questionnaire) string {
	return `# Git Workflow Rules

## Branching
- Never commit directly to main/master
- Use descriptive branch names: feat/, fix/, chore/, refactor/
- One feature per branch

## Commits
- Follow Conventional Commits: feat:, fix:, chore:, docs:, refactor:, test:
- Keep commits atomic and focused
- Write commit messages in present tense

## PR Guidelines
- Open a PR for every feature or fix
- Describe what changed and why in the PR body
- Link related issues
`
}

func generateTestingRules(fp *analyzer.ProjectFingerprint, q *Questionnaire) string {
	testCmd := buildTestCommand(fp)
	return fmt.Sprintf(`# Testing Rules

## Test Command
` + "```bash\n" + testCmd + "\n```" + `

## Rules
- Write a test for every new feature or bug fix
- Run the test suite before committing
- Do not modify tests to make them pass — fix the implementation
- Tests must be green before pushing
`)
}

func generateQualityRules(fp *analyzer.ProjectFingerprint) string {
	return fmt.Sprintf(`# Code Quality Rules

## Language: %s

## Principles
- Keep functions small and focused (< 50 lines)
- Avoid deeply nested conditionals (max 3 levels)
- Prefer explicit over implicit
- No commented-out code in PRs
- Remove unused imports and variables

## Security
- Never hardcode secrets or API keys
- Use environment variables for configuration
- Validate all user inputs
- Use parameterized queries for database access
`, fp.Language)
}

// ─── Commands ─────────────────────────────────────────────────────────────────

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
1. Run the test suite: ` + "`%s`" + `
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

// ─── Hooks ────────────────────────────────────────────────────────────────────

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
osascript -e 'display notification "Claude Code finished your task" with title "ccbootstrap" sound name "Glass"' 2>/dev/null || true
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

func generateArchitectureMD(fp *analyzer.ProjectFingerprint, understanding *llm.ProjectUnderstanding) string {
	if understanding != nil && understanding.Architecture != "" {
		endpoints := ""
		if len(understanding.APIEndpoints) > 0 {
			endpoints = "\n## API Endpoints\n" + formatList(understanding.APIEndpoints)
		}
		modules := ""
		if len(understanding.MainModules) > 0 {
			modules = "\n## Modules\n" + formatList(understanding.MainModules)
		}
		tech := ""
		if understanding.TechNotes != "" {
			tech = "\n## Technical Notes\n" + understanding.TechNotes
		}
		return fmt.Sprintf(`# Architecture

> Auto-generated by ccbootstrap with AI analysis.

## Overview
%s

## Stack
%s%s%s%s
## External Services
%s
`, understanding.Architecture, formatStack(fp.Stack), modules, endpoints, tech,
			formatList(understanding.ExternalServices))
	}
	// Fallback: static
	return fmt.Sprintf(`# Architecture

> Auto-generated by ccbootstrap — update this file to reflect the real architecture.

## Stack
%s

## Structure
_Document your directory structure here._

## Key Design Decisions
_See docs/decisions/ for Architecture Decision Records._

## Data Flow
_Describe how data flows through the system._

## External Dependencies
_List external services, APIs, and their purposes._
`, formatStack(fp.Stack))
}

func generateProgressMD() string {
	return fmt.Sprintf(`# Session Progress

## %s — Initial Setup

- ✅ Bootstrapped with ccbootstrap
- ✅ Generated CLAUDE.md, .claude/, docs/ structure

## Next Steps
- [ ] Review and update CLAUDE.md with project-specific context
- [ ] Add architecture details to docs/architecture.md
- [ ] Start development
`, time.Now().Format("2006-01-02"))
}

// ─── Helpers ──────────────────────────────────────────────────────────────────

func writeFile(path, content string) error {
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		return err
	}
	return nil
}

func repoNameFromURL(url string) string {
	u := strings.TrimSuffix(url, ".git")
	u = strings.TrimSuffix(u, "/")
	parts := strings.Split(u, "/")
	if len(parts) > 0 {
		return parts[len(parts)-1]
	}
	return "project"
}

func formatStack(stack []string) string {
	result := ""
	for _, s := range stack {
		result += "- " + s + "\n"
	}
	return result
}

func formatLOC(loc int) string {
	if loc >= 1000 {
		return fmt.Sprintf("%dk", loc/1000)
	}
	return fmt.Sprintf("%d", loc)
}

func boolEmoji(v bool) string {
	if v {
		return "✅"
	}
	return "❌"
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

func buildCommandsTable(fp *analyzer.ProjectFingerprint) string {
	var rows []string
	for _, s := range fp.Stack {
		switch {
		case strings.Contains(s, "Laravel"):
			rows = append(rows,
				"| `php artisan serve` | Start dev server |",
				"| `php artisan test --parallel` | Run test suite |",
				"| `php artisan migrate` | Run migrations |",
				"| `composer install` | Install PHP deps |",
			)
		case strings.Contains(s, "Next"):
			rows = append(rows,
				"| `npm run dev` | Start dev server |",
				"| `npm test` | Run test suite |",
				"| `npm run build` | Production build |",
			)
		case strings.Contains(s, "NestJS"):
			rows = append(rows,
				"| `npm run start:dev` | Start dev server |",
				"| `npm test` | Run test suite |",
				"| `npm run build` | Production build |",
			)
		case s == "Go" || strings.HasPrefix(s, "Go/"):
			rows = append(rows,
				"| `go run .` | Start app |",
				"| `go test ./...` | Run test suite |",
				"| `go build -o app .` | Build binary |",
			)
		case strings.Contains(s, "Django"):
			rows = append(rows,
				"| `python manage.py runserver` | Start dev server |",
				"| `pytest` | Run test suite |",
				"| `python manage.py migrate` | Run migrations |",
			)
		}
	}
	if len(rows) == 0 {
		rows = append(rows, "| _(add your commands here)_ | _ |")
	}
	return strings.Join(rows, "\n")
}
func formatList(items []string) string {
	result := ""
	for _, item := range items {
		result += "- " + item + "\n"
	}
	return result
}
