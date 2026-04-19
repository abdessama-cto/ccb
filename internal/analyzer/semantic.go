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
	Files    map[string]string // path → content (trimmed)
	Summary  string            // populated by AI after extraction
	TokenEst int               // rough token estimate
}

// ExtractSemanticContext reads the most important files in the repo
// for AI understanding: entry points, routing, config descriptions, etc.
func ExtractSemanticContext(repoDir string) *SemanticContext {
	ctx := &SemanticContext{
		Files: make(map[string]string),
	}

	// Priority ordered list of file patterns to read
	candidates := []string{
		// Project description files
		"README.md", "readme.md", "README.rst", "README.txt",
		"package.json",
		"composer.json",
		"pyproject.toml", "setup.py", "setup.cfg",
		"go.mod",
		"Gemfile",
		"Cargo.toml",
		".env.example",

		// Application entry points
		"main.go", "cmd/main.go",
		"main.ts", "main.js",
		"src/main.ts", "src/main.js",
		"index.ts", "index.js",
		"src/index.ts", "src/index.js",
		"app.ts", "app.js",
		"src/app.ts", "src/app.js", "src/app.module.ts",
		"server.ts", "server.js",
		"src/server.ts", "src/server.js",
		"manage.py",
		"wsgi.py", "asgi.py",
		"app.py", "main.py", "run.py",
		"artisan",
		"bootstrap/app.php",
		"config/app.php",
	}

	for _, rel := range candidates {
		full := filepath.Join(repoDir, rel)
		if content := smartRead(full, rel); content != "" {
			ctx.Files[rel] = content
		}
	}

	// Read routing / controllers (max 5 files per category)
	routePatterns := []struct {
		glob    string
		maxRead int
	}{
		{"routes/*.php", 3},
		{"routes/api.php", 1},
		{"routes/web.php", 1},
		{"routes/*.ts", 3},
		{"pages/api/**/*.ts", 3},
		{"src/routes/*.ts", 3},
		{"src/routes/*.js", 3},
		{"Controllers/*.php", 3},
		{"app/Http/Controllers/*.php", 3},
		{"src/controllers/*.ts", 3},
		{"src/modules/**/*.controller.ts", 3},
		{"internal/**/handler*.go", 3},
		{"internal/**/router*.go", 2},
		{"api/**/*.py", 3},
		{"views/*.py", 2},
	}

	for _, rp := range routePatterns {
		files, _ := filepath.Glob(filepath.Join(repoDir, rp.glob))
		read := 0
		for _, f := range files {
			if read >= rp.maxRead {
				break
			}
			rel, _ := filepath.Rel(repoDir, f)
			if _, already := ctx.Files[rel]; already {
				continue
			}
			if content := smartRead(f, rel); content != "" {
				ctx.Files[rel] = content
				read++
			}
		}
	}

	// Add a directory tree overview (lightweight)
	ctx.Files["__dir_tree__"] = buildDirTree(repoDir, 3)

	// Estimate tokens (rough: 4 chars per token)
	total := 0
	for _, v := range ctx.Files {
		total += len(v)
	}
	ctx.TokenEst = total / 4

	return ctx
}

// smartRead reads a file intelligently:
// - Skips binary files
// - Strips minified content
// - Truncates large files keeping head + tail
// - Strips lock file details from package.json
func smartRead(path, relPath string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}

	// Skip binary files
	for _, b := range data[:min2(len(data), 512)] {
		if b == 0 {
			return "[ binary file — skipped ]"
		}
	}

	content := string(data)

	// Special handling for package.json: keep only meaningful fields
	if relPath == "package.json" {
		return extractPackageJSONSummary(content)
	}

	// Special handling for composer.json
	if relPath == "composer.json" {
		return extractComposerJSONSummary(content)
	}

	// Skip minified files (lines too long)
	lines := strings.Split(content, "\n")
	for _, line := range lines[:min2(len(lines), 5)] {
		if len(line) > 500 {
			return "[ minified file — skipped ]"
		}
	}

	// Truncate large files: keep first 150 lines + last 30 lines
	maxLines := 150
	if len(lines) > maxLines+30 {
		head := strings.Join(lines[:maxLines], "\n")
		tail := strings.Join(lines[len(lines)-30:], "\n")
		return fmt.Sprintf("%s\n\n[ ... %d lines truncated ... ]\n\n%s",
			head, len(lines)-maxLines-30, tail)
	}

	return content
}

// buildDirTree builds a human-readable directory tree (depth limited)
func buildDirTree(root string, maxDepth int) string {
	var sb strings.Builder
	sb.WriteString(filepath.Base(root) + "/\n")

	ignoreDirs := map[string]bool{
		"node_modules": true, "vendor": true, ".git": true,
		"dist": true, "build": true, ".next": true, "__pycache__": true,
		"coverage": true, ".nyc_output": true, "venv": true, ".venv": true,
	}

	var walk func(dir string, depth int, prefix string)
	walk = func(dir string, depth int, prefix string) {
		if depth > maxDepth {
			return
		}
		entries, err := os.ReadDir(dir)
		if err != nil {
			return
		}
		// Sort: dirs first, then files
		sort.Slice(entries, func(i, j int) bool {
			if entries[i].IsDir() != entries[j].IsDir() {
				return entries[i].IsDir()
			}
			return entries[i].Name() < entries[j].Name()
		})

		for i, entry := range entries {
			if ignoreDirs[entry.Name()] {
				continue
			}
			isLast := i == len(entries)-1
			connector := "├── "
			nextPrefix := prefix + "│   "
			if isLast {
				connector = "└── "
				nextPrefix = prefix + "    "
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

// BuildAIPrompt creates the prompt to send to GPT for project understanding
func BuildAIPrompt(ctx *SemanticContext, fp *ProjectFingerprint) string {
	var sb strings.Builder

	sb.WriteString(`You are analyzing a software project to generate a comprehensive understanding.
Based on the files below, provide a structured JSON response with:
{
  "project_name": "...",
  "purpose": "1-2 sentences: what this app does",
  "domain": "e.g. e-commerce, SaaS, portfolio, API, mobile backend...",
  "architecture": "brief description of the architecture",
  "key_features": ["feature1", "feature2", "feature3"],
  "main_modules": ["module1: description", "module2: description"],
  "api_endpoints": ["GET /api/users", "POST /api/auth/login", "..."],
  "external_services": ["Stripe", "SendGrid", "AWS S3", ...],
  "conventions": ["convention1", "convention2"],
  "tech_notes": "important technical details Claude should know",
  "what_claude_should_know": "key context for an AI assistant working on this project"
}

Only respond with valid JSON. No explanation outside the JSON.

` + fmt.Sprintf("Detected stack: %s\n\n", fp.StackString()))

	// Add files in order of importance
	priority := []string{
		"README.md", "readme.md", "package.json", "composer.json",
		"pyproject.toml", "go.mod", "Gemfile",
		"__dir_tree__",
	}

	written := map[string]bool{}

	// Write priority files first
	for _, key := range priority {
		if content, ok := ctx.Files[key]; ok {
			sb.WriteString(fmt.Sprintf("=== %s ===\n%s\n\n", key, content))
			written[key] = true
		}
	}

	// Then entry points & routes
	for key, content := range ctx.Files {
		if written[key] {
			continue
		}
		sb.WriteString(fmt.Sprintf("=== %s ===\n%s\n\n", key, content))
	}

	return sb.String()
}

// ── Helpers ──────────────────────────────────────────────────────────────────

func extractPackageJSONSummary(content string) string {
	// Extract key fields only (name, description, scripts, dependencies sans les versions)
	lines := strings.Split(content, "\n")
	var result []string
	inScripts := false
	inDeps := false
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
			} else if depCount < 20 {
				// Extract only dep name
				parts := strings.Split(trimmed, ":")
				if len(parts) > 0 {
					result = append(result, "  "+strings.Trim(parts[0], `" `)+"")
				}
				depCount++
			} else if depCount == 20 {
				result = append(result, fmt.Sprintf("  [ ... and more dependencies ]"))
				depCount++
			}
			continue
		}

		// Always include name, version, description, main, type fields
		for _, field := range []string{`"name"`, `"version"`, `"description"`, `"main"`, `"type"`, `"author"`, `"repository"`} {
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
			} else if reqCount < 15 {
				parts := strings.Split(trimmed, ":")
				if len(parts) > 0 {
					result = append(result, "  "+strings.Trim(parts[0], `" `))
				}
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
