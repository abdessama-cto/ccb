package analyzer

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// SemanticContext holds raw file contents for AI analysis
type SemanticContext struct {
	Files    map[string]string
	TokenEst int
}

// ExtractSemanticContext reads the project intelligently:
// 1. Always reads structural/description files first
// 2. Reads ALL source files, skipping ignored dirs and binary/generated files
// 3. Orders by importance (routes, controllers, services before generic files)
func ExtractSemanticContext(repoDir string) *SemanticContext {
	ctx := &SemanticContext{Files: make(map[string]string)}

	// ── Phase A: Structural files (always read, highest priority) ────────────
	for _, rel := range structuralFiles {
		full := filepath.Join(repoDir, rel)
		if content := smartRead(full, rel); content != "" {
			ctx.Files[rel] = content
		}
	}

	// ── Directory tree (4 levels deep) ───────────────────────────────────────
	ctx.Files["__dir_tree__"] = buildDirTree(repoDir, 4)

	// ── Phase B: All source files (smart-filtered) ───────────────────────────
	_ = filepath.Walk(repoDir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info == nil {
			return nil
		}

		if info.IsDir() {
			if shouldIgnoreDir(info.Name()) {
				return filepath.SkipDir
			}
			return nil
		}

		rel, _ := filepath.Rel(repoDir, path)
		if _, already := ctx.Files[rel]; already {
			return nil
		}

		ext := strings.ToLower(filepath.Ext(info.Name()))
		if !isSourceFile(ext, info.Name()) {
			return nil
		}

		// Skip files > 100KB (likely generated/large data)
		if info.Size() > 100_000 {
			ctx.Files[rel] = fmt.Sprintf("[ file skipped: too large (%dKB) ]", info.Size()/1024)
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

// BuildAIPrompt builds prompt with no character limit (for Gemini 1M context)
func BuildAIPrompt(ctx *SemanticContext, fp *ProjectFingerprint) string {
	return buildPrompt(ctx, fp, 0)
}

// BuildAIPromptLimited builds prompt with a character limit
func BuildAIPromptLimited(ctx *SemanticContext, fp *ProjectFingerprint, maxChars int) string {
	return buildPrompt(ctx, fp, maxChars)
}

func buildPrompt(ctx *SemanticContext, fp *ProjectFingerprint, maxChars int) string {
	var sb strings.Builder

	sb.WriteString(`You are a senior software architect analyzing a codebase.
Study all the files below carefully, then return a single JSON object.

CRITICAL: Return ONLY the JSON — no markdown code fences, no text before or after, just the raw JSON.

{
  "project_name": "exact name of the project",
  "purpose": "2-3 sentences: what this app does, who uses it, and the business value",
  "domain": "e.g. e-commerce, SaaS platform, mobile backend, developer tool...",
  "architecture": "concise description of the architecture (e.g. Next.js SSR + REST API + PostgreSQL via Prisma)",
  "key_features": ["feature1", "feature2", "feature3"],
  "main_modules": [
    "module_name: what it does",
    "module_name: what it does"
  ],
  "api_endpoints": [
    "METHOD /path — what it does",
    "METHOD /path — what it does"
  ],
  "external_services": [
    "ServiceName: how it is used"
  ],
  "conventions": [
    "naming or structural convention observed in the code"
  ],
  "tech_notes": "important patterns, gotchas, or architectural decisions Claude should know",
  "what_claude_should_know": "essential context for an AI assistant working on this codebase daily"
}

`)

	sb.WriteString(fmt.Sprintf("Stack detected: %s\nTotal files: %d\n\n", fp.StackString(), fp.Files))

	// Write files in importance order
	orderedKeys := rankFiles(ctx.Files)
	charCount := sb.Len()

	for _, key := range orderedKeys {
		content := ctx.Files[key]
		chunk := fmt.Sprintf("=== %s ===\n%s\n\n", key, content)

		if maxChars > 0 && charCount+len(chunk) > maxChars {
			sb.WriteString(fmt.Sprintf("[ ... %d more files omitted (context limit) ]\n", len(orderedKeys)))
			break
		}
		sb.WriteString(chunk)
		charCount += len(chunk)
	}

	return sb.String()
}

// rankFiles orders files by their importance signal
func rankFiles(files map[string]string) []string {
	type scored struct {
		key   string
		score int
	}

	list := make([]scored, 0, len(files))
	for k := range files {
		list = append(list, scored{k, scoreFile(k)})
	}

	sort.Slice(list, func(i, j int) bool {
		return list[i].score > list[j].score
	})

	result := make([]string, len(list))
	for i, s := range list {
		result[i] = s.key
	}
	return result
}

func scoreFile(path string) int {
	lower := strings.ToLower(path)
	score := 100 - strings.Count(path, "/")*5 // prefer root-level files

	highValue := map[string]int{
		"readme":      80, "package.json": 70, "composer.json": 70,
		"go.mod": 70, "pyproject": 70,
		"__dir_tree__": 65,
		"route":        60, "routes": 60, "router": 60,
		"controller": 55, "handler": 55,
		"service":    50, "repository": 50,
		"model":      48, "schema": 48, "entity": 48, "prisma": 48,
		"api":        45, "graphql": 45,
		"middleware": 40, "auth": 40, "guard": 40,
		"config":     35, "constant": 35, "env": 35,
		"main":       30, "index": 30, "app": 30, "server": 30,
		"module":     25,
		"migration":  20,
		"test":       10, "spec": 10,
	}

	for keyword, boost := range highValue {
		if strings.Contains(lower, keyword) {
			score += boost
		}
	}

	// Penalise obvious noise
	noise := []string{".min.", ".map", "package-lock", "yarn.lock", "pnpm-lock", ".d.ts", ".snap"}
	for _, n := range noise {
		if strings.Contains(lower, n) {
			score -= 60
		}
	}

	return score
}

// ─── Helper functions ─────────────────────────────────────────────────────────

// shouldIgnoreDir returns true for directories that should be skipped
func shouldIgnoreDir(name string) bool {
	ignored := map[string]bool{
		// Package managers
		"node_modules": true, "vendor": true,
		// Build outputs
		"dist": true, "build": true, "out": true, ".next": true,
		"public/build": true,
		// Python
		"__pycache__": true, ".mypy_cache": true, ".pytest_cache": true,
		"venv": true, ".venv": true, "env": true,
		// Cache/tools
		".git": true, ".idea": true, ".vscode": true,
		".turbo": true, ".cache": true, ".parcel-cache": true,
		// Laravel specific
		"storage": true, "bootstrap/cache": true,
		// Misc
		"tmp": true, "temp": true, "logs": true, "coverage": true,
		".nyc_output": true, "storybook-static": true,
	}
	return ignored[name]
}

// isSourceFile returns true for files worth reading
func isSourceFile(ext, name string) bool {
	// Skip lock files by name
	lockFiles := map[string]bool{
		"package-lock.json": true, "yarn.lock": true, "pnpm-lock.yaml": true,
		"composer.lock": true, "Gemfile.lock": true, "Cargo.lock": true,
		"poetry.lock": true, "go.sum": true,
	}
	if lockFiles[name] {
		return false
	}

	// Skip minified/map files
	if strings.Contains(name, ".min.") || strings.HasSuffix(name, ".map") {
		return false
	}

	// Skip TypeScript declaration files
	if strings.HasSuffix(name, ".d.ts") {
		return false
	}

	source := map[string]bool{
		// JS/TS ecosystem
		".ts": true, ".tsx": true, ".js": true, ".jsx": true,
		".vue": true, ".svelte": true, ".astro": true,
		// Backend
		".go": true, ".py": true, ".rb": true, ".rs": true,
		".java": true, ".kt": true, ".php": true, ".cs": true,
		".swift": true, ".dart": true, ".ex": true, ".exs": true,
		// Config/schema
		".json": true, ".yaml": true, ".yml": true, ".toml": true,
		".prisma": true, ".graphql": true, ".gql": true,
		// Docs
		".md": true, ".mdx": true,
		// Shell
		".sh": true, ".bash": true,
		// Templates
		".html": true, ".ejs": true, ".hbs": true, ".blade": true,
		// DB
		".sql": true,
		// Env
		".env": true,
	}
	return source[ext]
}

// structuralFiles are always read regardless of the walk
var structuralFiles = []string{
	"README.md", "readme.md", "README.rst", "README.txt",
	"package.json", "composer.json", "pyproject.toml",
	"go.mod", "Gemfile", "Cargo.toml", "pom.xml",
	".env.example", ".env.sample", ".env.local.example",
	"docker-compose.yml", "docker-compose.yaml", "Dockerfile",
	"next.config.js", "next.config.ts", "next.config.mjs",
	"nuxt.config.ts", "nuxt.config.js",
	"vite.config.ts", "vite.config.js",
	"tailwind.config.js", "tailwind.config.ts",
	"tsconfig.json",
	"app/Http/Kernel.php",
	"config/app.php",
	"manage.py",
}

// smartRead reads a file with filtering
func smartRead(path, relPath string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}

	// Binary check
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

	// Minified file check
	for _, line := range lines[:min2(len(lines), 3)] {
		if len(line) > 2000 {
			return "[ minified/generated — skipped ]"
		}
	}

	// Special processing for known structured files
	if relPath == "package.json" {
		return extractPackageJSONSummary(content)
	}
	if relPath == "composer.json" {
		return extractComposerJSONSummary(content)
	}

	// Truncate large files: keep head (250 lines) + tail (30 lines)
	if len(lines) > 280 {
		head := strings.Join(lines[:250], "\n")
		tail := strings.Join(lines[len(lines)-30:], "\n")
		return fmt.Sprintf("%s\n\n[ ... %d lines omitted ... ]\n\n%s",
			head, len(lines)-280, tail)
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
			if shouldIgnoreDir(entry.Name()) {
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
				result = append(result, "  [ ... more ... ]")
				depCount++
			}
			continue
		}
		for _, field := range []string{`"name"`, `"version"`, `"description"`, `"main"`, `"type"`, `"author"`, `"repository"`, `"keywords"`, `"homepage"`} {
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
			} else if reqCount < 25 {
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
