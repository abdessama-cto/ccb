package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/abdessama-cto/ccb/internal/cache"
	"github.com/abdessama-cto/ccb/internal/config"
	"github.com/abdessama-cto/ccb/internal/skills"
	"github.com/abdessama-cto/ccb/internal/tui"
	"github.com/spf13/cobra"
)

var doctorCmd = &cobra.Command{
	Use:   "doctor",
	Short: "Audit the ccb setup and the current project's .claude/ config",
	Long: `Run a battery of checks:
  - AI provider is configured with a valid-looking key
  - awesome-claude-skills cache is present and fresh
  - .claude/settings.json is valid JSON
  - .claude/hooks/*.sh are executable
  - .claude/ has the expected subdirectories

Reports ✅ pass, ⚠️  warn, or ❌ fail for each check.`,
	RunE: runDoctor,
}

func init() {
	rootCmd.AddCommand(doctorCmd)
}

type check struct {
	label   string
	status  string // "pass" | "warn" | "fail"
	message string
}

func runDoctor(cmd *cobra.Command, args []string) error {
	var checks []check

	// ── 1. Global config (always check) ──────────────────────────────────────
	cfg, _ := config.Load()
	if cfg.AI.IsConfigured() {
		checks = append(checks, check{
			label:   fmt.Sprintf("AI provider (%s)", cfg.AI.Provider),
			status:  "pass",
			message: fmt.Sprintf("model: %s", cfg.AI.ActiveModel()),
		})
	} else {
		checks = append(checks, check{
			label:   "AI provider",
			status:  "fail",
			message: "not configured — run 'ccb settings'",
		})
	}

	// ── 2. skills.sh reachability ────────────────────────────────────────────
	start := time.Now()
	if err := skills.Reachable(); err != nil {
		checks = append(checks, check{
			label:   "skills.sh API",
			status:  "fail",
			message: "unreachable: " + err.Error(),
		})
	} else {
		checks = append(checks, check{
			label:   "skills.sh API",
			status:  "pass",
			message: fmt.Sprintf("reachable (%s)", time.Since(start).Round(time.Millisecond)),
		})
	}

	// ── 3. Project-level checks (only if inside a bootstrapped project) ──────
	wd, _ := os.Getwd()
	claudeDir := filepath.Join(wd, ".claude")
	if _, err := os.Stat(claudeDir); os.IsNotExist(err) {
		checks = append(checks, check{
			label:   "Current project",
			status:  "warn",
			message: "no .claude/ here — run 'ccb start' to bootstrap",
		})
		printReport(checks)
		return nil
	}

	// settings.json
	settingsPath := filepath.Join(claudeDir, "settings.json")
	if data, err := os.ReadFile(settingsPath); err == nil {
		var v map[string]interface{}
		if jsonErr := json.Unmarshal(data, &v); jsonErr == nil {
			checks = append(checks, check{label: ".claude/settings.json", status: "pass"})
		} else {
			checks = append(checks, check{label: ".claude/settings.json", status: "fail", message: "invalid JSON: " + jsonErr.Error()})
		}
	} else {
		checks = append(checks, check{label: ".claude/settings.json", status: "fail", message: "missing"})
	}

	// hooks executable
	hooksDir := filepath.Join(claudeDir, "hooks")
	if entries, err := os.ReadDir(hooksDir); err == nil {
		var nonExec []string
		for _, e := range entries {
			if !strings.HasSuffix(e.Name(), ".sh") {
				continue
			}
			full := filepath.Join(hooksDir, e.Name())
			info, _ := os.Stat(full)
			if info == nil || info.Mode().Perm()&0111 == 0 {
				nonExec = append(nonExec, e.Name())
			}
		}
		if len(nonExec) == 0 {
			checks = append(checks, check{label: ".claude/hooks/*.sh executable", status: "pass"})
		} else {
			checks = append(checks, check{
				label:   ".claude/hooks/*.sh executable",
				status:  "fail",
				message: fmt.Sprintf("not executable: %s — run: chmod +x .claude/hooks/*.sh", strings.Join(nonExec, ", ")),
			})
		}
	}

	// expected subdirs
	for _, sub := range []string{"rules", "agents", "skills", "commands", "hooks"} {
		p := filepath.Join(claudeDir, sub)
		if _, err := os.Stat(p); os.IsNotExist(err) {
			checks = append(checks, check{
				label:   ".claude/" + sub + "/",
				status:  "warn",
				message: "missing — some features may not work",
			})
		}
	}

	// Cached analysis
	cached, _ := cache.Load(wd)
	if cached != nil {
		age := time.Since(cached.CreatedAt).Round(time.Hour)
		checks = append(checks, check{
			label:   "Cached analysis",
			status:  "pass",
			message: fmt.Sprintf("%s, %d files, %s ago", cached.Model, cached.FilesCount, age),
		})
	} else {
		checks = append(checks, check{
			label:   "Cached analysis",
			status:  "warn",
			message: "no .ccb/analysis.json — 'ccb add agent' will fail until you reanalyze",
		})
	}

	printReport(checks)
	return nil
}

func printReport(checks []check) {
	fmt.Printf("\n%s ccb doctor report\n\n", tui.Bold("🩺"))
	passed, warned, failed := 0, 0, 0
	for _, c := range checks {
		var icon string
		switch c.status {
		case "pass":
			icon = tui.Green("✅")
			passed++
		case "warn":
			icon = "⚠️ "
			warned++
		default:
			icon = tui.Red("❌")
			failed++
		}
		line := fmt.Sprintf("  %s %s", icon, c.label)
		if c.message != "" {
			line += "  " + tui.Dim(c.message)
		}
		fmt.Println(line)
	}
	fmt.Printf("\n  %s %d pass · %d warn · %d fail\n\n",
		tui.Bold("Summary:"), passed, warned, failed)
}
