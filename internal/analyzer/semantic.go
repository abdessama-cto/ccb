package analyzer

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// SemanticContext holds raw file contents extracted for AI understanding
type SemanticContext struct {
	Files    map[string]string // path → content
	TokenEst int               // rough token estimate (~4 chars per token)
}

// ExtractSemanticContext reads ALL significant files in the repo for AI analysis.
// It is designed to maximise context sent to the LLM (takes advantage of 1M+ context windows).
func ExtractSemanticContext(repoDir string) *SemanticContext {
	ctx := &SemanticContext{Files: make(map[string]string)}

	// 1. Always read description/config files first
	for _, rel := range priorityFiles {
		full := filepath.Join(repoDir, rel)
		if content := smartRead(full, rel); content != "" {
			ctx.Files[rel] = content
		}
	}

	// 2. Directory tree overview
	ctx.Files["__dir_tree__"] = buildDirTree(repoDir, 4)

	// 3. Read ALL source files recursively (skip ignored dirs & binary files)
	_ = filepath.Walk(repoDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		name := info.Name()

		// Skip ignored directories
		if info.IsDir() {
			if ignoredDirs[name] {
				return filepath.SkipDir
			}
			return nil
		}

		rel, _ := filepath.Rel(repoDir, path)

		// Skip if already read
		if _, already := ctx.Files[rel]; already {
			return nil
		}

		// Only read source-like files
		ext := strings.ToLower(filepath.Ext(name))
		if !sourceExtensions[ext] {
			return nil
		}

		// Skip very large files (>200KB) — probably generated
		if info.Size() > 200_000 {
			ctx.Files[rel] = fmt.Sprintf("[ file too large (%dKB) — skipped ]", info.Size()/1024)
			return nil
		}

		if content := smartRead(path, rel); content != "" {
			ctx.Files[rel] = content
		}
		return nil
	})

	// Estimate tokens
	total := 0
	for _, v := range ctx.Files {
		total += len(v)
	}
	ctx.TokenEst = total / 4
	return ctx
}

// BuildAIPrompt creates the prompt for project understanding.
// maxChars = 0 means no limit (use for Gemini 1M+).
func BuildAIPrompt(ctx *SemanticContext, fp *ProjectFingerprint) string {
	return buildPromptWithLimit(ctx, fp, 0)
}

// BuildAIPromptLimited creates the prompt with a character limit.
func BuildAIPromptLimited(ctx *SemanticContext, fp *ProjectFingerprint, maxChars int) string {
	return buildPromptWithLimit(ctx, fp, maxChars)
}

func buildPromptWithLimit(ctx *SemanticContext, fp *ProjectFingerprint, maxChars int) string {
	var sb strings.Builder

	sb.WriteString(`You are analyzing a software project codebase.
Based on ALL the files provided, return a SINGLE valid JSON object (no markdown fences, no explanation outside the JSON):

{
  "project_name": "name of the project",
  "purpose": "2-3 sentences: what this app does, for whom, and why",
  "domain": "e.g. e-commerce, SaaS, portfolio, API backend, mobile app...",
  "architecture": "clear description of the architecture pattern and how components interact",
  "key_features": ["feature1", "feature2", "feature3", "..."],
  "main_modules": ["module: what it does", "module: what it does", "..."],
  "api_endpoints": ["METHOD /path — description", "..."],
  "external_services": ["ServiceName: purpose", "..."],
  "conventions": ["convention or coding pattern observed", "..."],
  "tech_notes": "important technical details, gotchas, or patterns Claude should know when working on this",
  "what_claude_should_know": "key context for an AI assistant working on this codebase day-to-day"
}

IMPORTANT:
- Return ONLY the JSON object. Do not wrap it in markdown code fences.
- Be specific and concrete, not generic.
- Infer business logic and purpose from the actual code, not just file names.

`)

	sb.WriteString(fmt.Sprintf("Detected stack: %s\n\n", fp.StackString()))

	// Write files in priority order
	written := map[string]bool{}
	charCount := len(sb.String())

	// Priority: description → tree → routes/controllers → other source files
	orderedKeys := orderedFileKeys(ctx.Files)

	for _, key := range orderedKeys {
		content := ctx.Files[key]
		chunk := fmt.Sprintf("=== %s ===\n%s\n\n", key, content)

		if maxChars > 0 && charCount+len(chunk) > maxChars {
			// Add a note and stop
			sb.WriteString("[ ... additional files omitted due to context limit ... ]\n")
			break
		}

		sb.WriteString(chunk)
		written[key] = true
		charCount += len(chunk)
	}

	return sb.String()
}

// orderedFileKeys returns file keys sorted by importance
func orderedFileKeys(files map[string]string) []string {
	priority := []string{
		"README.md", "readme.md", "README.rst",
		"package.json", "composer.json", "pyproject.toml", "go.mod", "Gemfile", "Cargo.toml",
		"__dir_tree__",
	}

	result := make([]string, 0, len(files))
	seen := map[string]bool{}

	// Priority first
	for _, k := range priority {
		if _, ok := files[k]; ok {
			result = append(result, k)
			seen[k] = true
		}
	}

	// Routes/controllers next (high signal)
	remaining := []string{}
	for k := range files {
		if seen[k] {
			continue
		}
		remaining = append(remaining, k)
	}

	sort.Slice(remaining, func(i, j int) bool {
		return fileImportance(remaining[i]) > fileImportance(remaining[j])
	})

	result = append(result, remaining...)
	return result
}

// fileImportance scores a file path for ordering
func fileImportance(path string) int {
	lower := strings.ToLower(path)
	score := 0

	highSignal := []string{
		"route", "router", "controller", "handler", "api", "service",
		"module", "main", "index", "app", "server", "config",
		"model", "schema", "entity", "migration",
		"middleware", "auth", "guard",
	}
	for _, kw := range highSignal {
		if strings.Contains(lower, kw) {
			score += 10
		}
	}

	// Prefer shorter paths (closer to root = more structural)
	score -= strings.Count(path, "/") * 2

	return score
}

// ─── Internal helpers ─────────────────────────────────────────────────────────

var ignoredDirs = map[string]bool{
	"node_modules": true, "vendor": true, ".git": true,
	"dist": true, "build": true, ".next": true, "out": true,
	"__pycache__": true, ".mypy_cache": true,
	"coverage": true, ".nyc_output": true,
	"venv": true, ".venv": true, "env": true,
	".idea": true, ".vscode": true,
	"storage": true, "bootstrap/cache": true,
	"public/build": true, "public/hot": true,
	".turbo": true, ".cache": true,
	"tmp": true, "temp": true, "logs": true,
}

var sourceExtensions = map[string]bool{
	// Web
	".ts": true, ".tsx": true, ".js": true, ".jsx": true, ".vue": true, ".svelte": true,
	".css": true, ".scss": true, ".less": true,
	// Backend
	".go": true, ".py": true, ".rb": true, ".rs": true, ".java": true, ".kt": true,
	".php": true, ".cs": true, ".swift": true, ".dart": true,
	// Config/Data
	".json": true, ".yaml": true, ".yml": true, ".toml": true, ".env": true,
	// Docs
	".md": true, ".mdx": true, ".txt": true,
	// Shell
	".sh": true, ".bash": true, ".zsh": true,
	// Templates
	".html": true, ".blade.php": true, ".ejs": true, ".hbs": true, ".jinja2": true,
	// DB
	".sql": true, ".prisma": true, ".graphql": true, ".gql": true,
}

// priorityFiles are always read first regardless of walking
var priorityFiles = []string{
	"README.md", "readme.md", "README.rst", "README.txt",
	"package.json", "composer.json", "pyproject.toml",
	"go.mod", "Gemfile", "Cargo.toml",
	".env.example", ".env.sample",
	"docker-compose.yml", "docker-compose.yaml",
	"Dockerfile",
}

// smartRead reads a file with smart filtering
func smartRead(path, relPath string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}

	// Skip binary files
	sample := data
	if len(sample) > 512 {
		sample = sample[:512]
	}
	for _, b := range sample {
		if b == 0 {
			return ""
		}
	}

	content := string(data)
	lines := strings.Split(content, "\n")

	// Skip minified files
	for _, line := range lines[:min2(len(lines), 5)] {
		if len(line) > 2000 {
			return "[ minified/generated file — skipped ]"
		}
	}

	// Special handling for config files
	if relPath == "package.json" {
		return extractPackageJSONSummary(content)
	}
	if relPath == "composer.json" {
		return extractComposerJSONSummary(content)
	}

	// Truncate very large files: keep first 300 lines + last 50 lines
	maxLines := 300
	if len(lines) > maxLines+50 {
		head := strings.Join(lines[:maxLines], "\n")
		tail := strings.Join(lines[len(lines)-50:], "\n")
		return fmt.Sprintf("%s\n\n[ ... %d lines truncated ... ]\n\n%s",
			head, len(lines)-maxLines-50, tail)
	}

	return content
}

func buildDirTree(root string, maxDepth int) string {
	var sb strings.Builder
	sb.WriteString(filepath.Base(root) + "/\n")

	var walk func(dir string, depth int, prefix string)
	walk = func(dir string, depth int, prefix string) {
		if depth > maxDepth {
			return
		}
		entries, err := os.ReadDir(dir)
		if err != nil {
			return
		}
		sort.Slice(entries, func(i, j int) bool {
			if entries[i].IsDir() != entries[j].IsDir() {
				return entries[i].IsDir()
			}
			return entries[i].Name() < entries[j].Name()
		})
		for i, entry := range entries {
			if ignoredDirs[entry.Name()] {
				continue
			}
			isLast := i == len(entries)-1
			connector, nextPrefix := "├── ", prefix+"│   "
			if isLast {
				connector, nextPrefix = "└── ", prefix+"    "
			}
			if entry.IsDir() {
				sb.WriteString(prefix + connector + entry.Name() + "/\n")
				walk(filepath.Join(dir, entry.Name()), depth+1, nextPrefix)
			} else {
				sb.WriteString(prefix + connector + entry.Name() + "\n")
			}
		}
	}
	walk(root, 1, "")
	return sb.String()
}

func extractPackageJSONSummary(content string) string {
	lines := strings.Split(content, "\n")
	var result []string
	inScripts, inDeps := false, false
	depCount := 0

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.Contains(trimmed, `"scripts"`) {
			inScripts = true
			result = append(result, line)
			continue
		}
		if strings.Contains(trimmed, `"dependencies"`) || strings.Contains(trimmed, `"devDependencies"`) {
			inDeps = true
			depCount = 0
			result = append(result, line)
			continue
		}
		if inScripts {
			result = append(result, line)
			if strings.Contains(trimmed, "}") {
				inScripts = false
			}
			continue
		}
		if inDeps {
			if strings.Contains(trimmed, "}") {
				inDeps = false
				result = append(result, line)
			} else if depCount < 30 {
				parts := strings.SplitN(trimmed, ":", 2)
				result = append(result, "  "+strings.Trim(parts[0], `" `))
				depCount++
			} else if depCount == 30 {
				result = append(result, "  [ ... more deps ... ]")
				depCount++
			}
			continue
		}
		for _, field := range []string{`"name"`, `"version"`, `"description"`, `"main"`, `"type"`, `"author"`, `"repository"`, `"keywords"`} {
			if strings.Contains(trimmed, field) {
				result = append(result, line)
				break
			}
		}
	}
	return strings.Join(result, "\n")
}

func extractComposerJSONSummary(content string) string {
	lines := strings.Split(content, "\n")
	var result []string
	inRequire := false
	reqCount := 0

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.Contains(trimmed, `"require"`) {
			inRequire = true
			result = append(result, line)
			continue
		}
		if inRequire {
			if strings.Contains(trimmed, "}") {
				inRequire = false
				result = append(result, line)
			} else if reqCount < 20 {
				parts := strings.SplitN(trimmed, ":", 2)
				result = append(result, "  "+strings.Trim(parts[0], `" `))
				reqCount++
			}
			continue
		}
		for _, field := range []string{`"name"`, `"description"`, `"type"`, `"keywords"`, `"authors"`} {
			if strings.Contains(trimmed, field) {
				result = append(result, line)
				break
			}
		}
	}
	return strings.Join(result, "\n")
}

func min2(a, b int) int {
	if a < b {
		return a
	}
	return b
}
