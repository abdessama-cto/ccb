package analyzer

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// ProjectFingerprint holds all detected info about a repo
type ProjectFingerprint struct {
	RepoURL    string
	Stack      []string // e.g. ["Laravel 10", "Vue.js 3", "PostgreSQL"]
	Language   string   // primary language
	LOC        int
	Files      int
	Commits    int
	TestFrameworks []string
	HasCI      bool
	HasDocker  bool
	HasEnvFile bool
	Age        string // "greenfield" or "brownfield"
	Coverage   string // "high (>70%)", "medium", "low", "unknown"
}

func (f *ProjectFingerprint) StackString() string {
	return strings.Join(f.Stack, " + ")
}

func (f *ProjectFingerprint) TestFrameworksString() string {
	if len(f.TestFrameworks) == 0 {
		return "none detected"
	}
	return strings.Join(f.TestFrameworks, ", ")
}

// Analyze performs static analysis on a cloned repo
func Analyze(repoDir string, repoURL string, commits int) (*ProjectFingerprint, error) {
	fp := &ProjectFingerprint{
		RepoURL: repoURL,
		Commits: commits,
	}

	fp.Stack = detectStack(repoDir)
	fp.Language = detectPrimaryLanguage(repoDir)
	fp.LOC, fp.Files = countLOC(repoDir)
	fp.TestFrameworks = detectTests(repoDir)
	fp.HasCI = hasCI(repoDir)
	fp.HasDocker = hasDocker(repoDir)
	fp.HasEnvFile = hasEnvFile(repoDir)

	if commits > 100 {
		fp.Age = fmt.Sprintf("brownfield (%d commits)", commits)
	} else {
		fp.Age = fmt.Sprintf("greenfield (%d commits)", commits)
	}

	fp.Coverage = detectCoverage(repoDir, fp.TestFrameworks)
	return fp, nil
}

func detectStack(dir string) []string {
	var stack []string

	// PHP / Laravel
	if fileContains(filepath.Join(dir, "composer.json"), "laravel/framework") {
		version := extractVersion(filepath.Join(dir, "composer.json"), "laravel/framework")
		stack = append(stack, "Laravel "+version)
	}

	// Node.js frameworks
	pkgJSON := filepath.Join(dir, "package.json")
	if exists(pkgJSON) {
		content := readFile(pkgJSON)
		if strings.Contains(content, `"next"`) {
			stack = append(stack, "Next.js")
		} else if strings.Contains(content, `"@nestjs/core"`) {
			stack = append(stack, "NestJS")
		} else if strings.Contains(content, `"express"`) {
			stack = append(stack, "Express.js")
		}
		if strings.Contains(content, `"react"`) && !strings.Contains(content, `"next"`) {
			stack = append(stack, "React")
		}
		if strings.Contains(content, `"vue"`) {
			stack = append(stack, "Vue.js")
		}
		if strings.Contains(content, `"svelte"`) {
			stack = append(stack, "Svelte")
		}
		if strings.Contains(content, `"typescript"`) {
			stack = append(stack, "TypeScript")
		}
	}

	// Python
	reqFiles := []string{"requirements.txt", "pyproject.toml", "Pipfile"}
	for _, rf := range reqFiles {
		content := readFile(filepath.Join(dir, rf))
		if content == "" {
			continue
		}
		if strings.Contains(strings.ToLower(content), "django") {
			stack = append(stack, "Django")
		} else if strings.Contains(strings.ToLower(content), "fastapi") {
			stack = append(stack, "FastAPI")
		} else if strings.Contains(strings.ToLower(content), "flask") {
			stack = append(stack, "Flask")
		}
		break
	}

	// Go
	if exists(filepath.Join(dir, "go.mod")) {
		content := readFile(filepath.Join(dir, "go.mod"))
		if strings.Contains(content, "github.com/gin-gonic") {
			stack = append(stack, "Go/Gin")
		} else if strings.Contains(content, "github.com/gofiber") {
			stack = append(stack, "Go/Fiber")
		} else {
			stack = append(stack, "Go")
		}
	}

	// Ruby on Rails
	if fileContains(filepath.Join(dir, "Gemfile"), "rails") {
		stack = append(stack, "Ruby on Rails")
	}

	// Databases
	for _, f := range []string{"docker-compose.yml", "docker-compose.yaml", ".env", ".env.example"} {
		content := readFile(filepath.Join(dir, f))
		if strings.Contains(strings.ToLower(content), "postgres") && !contains(stack, "PostgreSQL") {
			stack = append(stack, "PostgreSQL")
		}
		if strings.Contains(strings.ToLower(content), "mysql") || strings.Contains(strings.ToLower(content), "mariadb") {
			if !contains(stack, "MySQL") {
				stack = append(stack, "MySQL")
			}
		}
		if strings.Contains(strings.ToLower(content), "redis") && !contains(stack, "Redis") {
			stack = append(stack, "Redis")
		}
		if strings.Contains(strings.ToLower(content), "mongodb") && !contains(stack, "MongoDB") {
			stack = append(stack, "MongoDB")
		}
	}

	if len(stack) == 0 {
		stack = append(stack, "Unknown stack")
	}
	return stack
}

func detectPrimaryLanguage(dir string) string {
	counts := map[string]int{}
	_ = filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}
		if strings.Contains(path, "/.git/") || strings.Contains(path, "/vendor/") || strings.Contains(path, "/node_modules/") {
			return nil
		}
		ext := strings.ToLower(filepath.Ext(path))
		switch ext {
		case ".php":
			counts["PHP"]++
		case ".ts", ".tsx":
			counts["TypeScript"]++
		case ".js", ".jsx":
			counts["JavaScript"]++
		case ".py":
			counts["Python"]++
		case ".go":
			counts["Go"]++
		case ".rb":
			counts["Ruby"]++
		case ".java":
			counts["Java"]++
		case ".rs":
			counts["Rust"]++
		}
		return nil
	})
	max, lang := 0, "Unknown"
	for l, c := range counts {
		if c > max {
			max, lang = c, l
		}
	}
	return lang
}

func countLOC(dir string) (int, int) {
	// Use wc -l via find to count lines efficiently
	out, err := exec.Command("bash", "-c",
		fmt.Sprintf(`find "%s" -type f \( -name "*.php" -o -name "*.ts" -o -name "*.tsx" -o -name "*.js" -o -name "*.jsx" -o -name "*.py" -o -name "*.go" -o -name "*.rb" \) ! -path "*/vendor/*" ! -path "*/node_modules/*" ! -path "*/.git/*" | xargs wc -l 2>/dev/null | tail -1`, dir),
	).Output()
	loc := 0
	if err == nil {
		fmt.Sscanf(strings.TrimSpace(string(out)), "%d", &loc)
	}

	// Count source files
	out2, err2 := exec.Command("bash", "-c",
		fmt.Sprintf(`find "%s" -type f \( -name "*.php" -o -name "*.ts" -o -name "*.tsx" -o -name "*.js" -o -name "*.py" -o -name "*.go" -o -name "*.rb" \) ! -path "*/vendor/*" ! -path "*/node_modules/*" ! -path "*/.git/*" | wc -l`, dir),
	).Output()
	files := 0
	if err2 == nil {
		fmt.Sscanf(strings.TrimSpace(string(out2)), "%d", &files)
	}
	return loc, files
}

func detectTests(dir string) []string {
	var frameworks []string
	if exists(filepath.Join(dir, "phpunit.xml")) || exists(filepath.Join(dir, "phpunit.xml.dist")) {
		frameworks = append(frameworks, "PHPUnit")
	}
	if fileContains(filepath.Join(dir, "package.json"), "jest") {
		frameworks = append(frameworks, "Jest")
	}
	if fileContains(filepath.Join(dir, "package.json"), "vitest") {
		frameworks = append(frameworks, "Vitest")
	}
	if exists(filepath.Join(dir, "pytest.ini")) || exists(filepath.Join(dir, "pyproject.toml")) && fileContains(filepath.Join(dir, "pyproject.toml"), "pytest") {
		frameworks = append(frameworks, "pytest")
	}
	if exists(filepath.Join(dir, "go.mod")) {
		frameworks = append(frameworks, "go test")
	}
	if fileContains(filepath.Join(dir, "Gemfile"), "rspec") {
		frameworks = append(frameworks, "RSpec")
	}
	return frameworks
}

func hasCI(dir string) bool {
	return exists(filepath.Join(dir, ".github", "workflows")) ||
		exists(filepath.Join(dir, ".gitlab-ci.yml")) ||
		exists(filepath.Join(dir, "Jenkinsfile")) ||
		exists(filepath.Join(dir, ".circleci"))
}

func hasDocker(dir string) bool {
	return exists(filepath.Join(dir, "Dockerfile")) ||
		exists(filepath.Join(dir, "docker-compose.yml")) ||
		exists(filepath.Join(dir, "docker-compose.yaml"))
}

func hasEnvFile(dir string) bool {
	return exists(filepath.Join(dir, ".env")) || exists(filepath.Join(dir, ".env.example"))
}

func detectCoverage(dir string, frameworks []string) string {
	if len(frameworks) == 0 {
		return "none"
	}
	// Check for coverage config files
	if exists(filepath.Join(dir, "coverage")) || exists(filepath.Join(dir, ".nyc_output")) {
		return "configured"
	}
	return "unknown"
}

// Helpers
func exists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func readFile(path string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return string(data)
}

func fileContains(path, substr string) bool {
	return strings.Contains(readFile(path), substr)
}

func extractVersion(composerPath, pkg string) string {
	content := readFile(composerPath)
	idx := strings.Index(content, `"`+pkg+`"`)
	if idx < 0 {
		return ""
	}
	rest := content[idx+len(pkg)+3:]
	end := strings.IndexAny(rest, `"`)
	if end < 0 {
		return ""
	}
	v := strings.TrimLeft(rest[:end], "^~>=<")
	parts := strings.Split(v, ".")
	if len(parts) > 0 {
		return parts[0]
	}
	return v
}

func contains(slice []string, s string) bool {
	for _, v := range slice {
		if v == s {
			return true
		}
	}
	return false
}
