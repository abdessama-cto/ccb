package cmd

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/abdessama-cto/ccb/internal/analyzer"
	"github.com/abdessama-cto/ccb/internal/generator"
	ghpkg "github.com/abdessama-cto/ccb/internal/github"
	"github.com/abdessama-cto/ccb/internal/skills"
	"github.com/abdessama-cto/ccb/internal/tui"
	"github.com/spf13/cobra"
)

var initFlags struct {
	yes              bool
	profile          string
	dryRun           bool
	skipQuestionnaire bool
	branch           string
	noPR             bool
}

var initCmd = &cobra.Command{
	Use:   "init [repo-url|.]",
	Short: "Bootstrap Claude Code config (GitHub repo or local directory)",
	Long: `Two modes:
  1. ccbootstrap init <github-url>  — clone & bootstrap a GitHub repo
  2. ccbootstrap init               — bootstrap the current local directory
  3. ccbootstrap init .             — same as above

Generates CLAUDE.md, .claude/ (rules, hooks, commands), docs/, installs skills, and opens a PR.`,
	Args: cobra.RangeArgs(0, 1),
	RunE: runInit,
}

func init() {
	rootCmd.AddCommand(initCmd)
	initCmd.Flags().BoolVarP(&initFlags.yes, "yes", "y", false, "Skip confirmations (non-interactive)")
	initCmd.Flags().StringVar(&initFlags.profile, "profile", "balanced", "Profile: balanced | strict | lightweight")
	initCmd.Flags().BoolVar(&initFlags.dryRun, "dry-run", false, "Generate without pushing or creating PR")
	initCmd.Flags().BoolVar(&initFlags.skipQuestionnaire, "skip-questionnaire", false, "Use profile defaults, skip questionnaire")
	initCmd.Flags().StringVar(&initFlags.branch, "branch", "", "Custom branch name (default: ccbootstrap/initial-setup)")
	initCmd.Flags().BoolVar(&initFlags.noPR, "no-pr", false, "Push without creating a PR")
}

func runInit(cmd *cobra.Command, args []string) error {
	startTime := time.Now()
	tui.Banner(Version)

	// Determine mode: local or GitHub
	useLocal := len(args) == 0 || args[0] == "." || args[0] == ""
	isGitHubURL := len(args) > 0 && (strings.HasPrefix(args[0], "http") || strings.HasPrefix(args[0], "git@"))

	// If no args and not --yes, ask the user to choose
	if !useLocal && !isGitHubURL && len(args) > 0 {
		useLocal = true // treat anything that's not a URL as a local path
	}

	var repoURL string
	var destDir string
	var err error

	if useLocal {
		// ── LOCAL MODE ────────────────────────────────────────────────────
		destDir, err = os.Getwd()
		if err != nil {
			return fmt.Errorf("could not determine current directory: %w", err)
		}
		// Verify it's a git repo
		if _, statErr := os.Stat(filepath.Join(destDir, ".git")); statErr != nil {
			return fmt.Errorf("current directory is not a git repository: %s", destDir)
		}
		// Try to get the remote URL for fingerprinting
		if remoteOut, remoteErr := exec.Command("git", "-C", destDir, "remote", "get-url", "origin").Output(); remoteErr == nil {
			repoURL = strings.TrimSpace(string(remoteOut))
		} else {
			repoURL = destDir // fallback
		}
		tui.Success(fmt.Sprintf("Using local project: %s", tui.Bold(filepath.Base(destDir))))
	} else {
		// ── GITHUB CLONE MODE ─────────────────────────────────────────────
		repoURL = args[0]

		tui.Info("Checking GitHub auth...")
		user, authErr := ghpkg.CheckAuth()
		if authErr != nil {
			return authErr
		}
		tui.Success(fmt.Sprintf("GitHub auth OK (user: @%s)", user))

		repoName := ghpkg.RepoNameFromURL(repoURL)
		homeDir, _ := os.UserHomeDir()
		projectsDir := filepath.Join(homeDir, ".ccbootstrap", "projects")
		destDir = filepath.Join(projectsDir, repoName)

		if _, statErr := os.Stat(destDir); statErr == nil {
			tui.Warn(fmt.Sprintf("Already cloned. Pulling latest changes in %s...", destDir))
			pullCmd := exec.Command("git", "-C", destDir, "pull")
			pullCmd.Stdout = os.Stdout
			pullCmd.Stderr = os.Stderr
			_ = pullCmd.Run()
		} else {
			tui.Info(fmt.Sprintf("Cloning %s...", repoURL))
			if cloneErr := ghpkg.CloneRepo(repoURL, destDir); cloneErr != nil {
				return fmt.Errorf("clone failed: %w", cloneErr)
			}
			tui.Success("Clone complete")
		}
	}

	// ── Step 3: Analyze ───────────────────────────────────────────────────
	tui.Info("Analyzing codebase...")
	commits := ghpkg.CountCommits(destDir)
	fp, err := analyzer.Analyze(destDir, repoURL, commits)
	if err != nil {
		return fmt.Errorf("analysis failed: %w", err)
	}

	printFingerprint(fp)

	// ── Step 4: Questionnaire ─────────────────────────────────────────────
	var q *generator.Questionnaire
	if initFlags.skipQuestionnaire || initFlags.yes {
		q = defaultQuestionnaire(initFlags.profile)
		tui.Info(fmt.Sprintf("Using profile: %s (skipping questionnaire)", initFlags.profile))
	} else {
		q, err = runQuestionnaire(fp)
		if err != nil {
			return err
		}
	}

	// Set branch name
	if initFlags.branch != "" {
		q.BranchName = initFlags.branch
	} else if q.BranchName == "" {
		q.BranchName = "ccbootstrap/initial-setup"
	}

	// ── Step 5: Generate ──────────────────────────────────────────────────
	tui.Info("Generating configuration...")
	if err := generator.Generate(destDir, fp, q); err != nil {
		return fmt.Errorf("generation failed: %w", err)
	}
	tui.Success("CLAUDE.md generated")
	tui.Success(".claude/ rules, hooks, commands generated")
	tui.Success("docs/ structure generated")

	if initFlags.dryRun {
		tui.Warn("Dry-run mode: skipping skills, tests, git push, and PR creation.")
		tui.Success(fmt.Sprintf("Config generated in: %s", destDir))
		return nil
	}

	// ── Step 6: Install skills ─────────────────────────────────────────────
	if q.InstallSkills {
		recommendedSkills := skills.RecommendedSkills(fp.Stack)
		tui.Info(fmt.Sprintf("Installing %d recommended skills...", len(recommendedSkills)))
		for _, skill := range recommendedSkills {
			tui.Info(fmt.Sprintf("  npx skills add %s", skill))
			if err := skills.Install(destDir, skill); err != nil {
				tui.Warn(fmt.Sprintf("  Could not install %s: %s", skill, err.Error()))
			} else {
				tui.Success(fmt.Sprintf("  %s installed", skill))
			}
		}
	}

	// ── Step 7: Run tests ─────────────────────────────────────────────────
	if q.RunTests && len(fp.TestFrameworks) > 0 {
		tui.Info("Running existing test suite for regression check...")
		testCmd := buildTestCmdForStack(fp)
		if testCmd != nil {
			testCmd.Dir = destDir
			testCmd.Stdout = os.Stdout
			testCmd.Stderr = os.Stderr
			if err := testCmd.Run(); err != nil {
				tui.Warn("Some tests failed — review before merging the PR.")
			} else {
				tui.Success("All tests green")
			}
		}
	}

	// ── Step 8: Git push + PR ─────────────────────────────────────────────
	tui.Info("Creating branch and committing...")
	commitMsg := fmt.Sprintf("chore(claude): bootstrap Claude Code via ccbootstrap v%s", Version)
	if err := ghpkg.CreateBranchAndPush(destDir, q.BranchName, commitMsg); err != nil {
		return fmt.Errorf("git push failed: %w", err)
	}
	tui.Success(fmt.Sprintf("Pushed branch: %s", q.BranchName))

	var prURL string
	if !initFlags.noPR && q.CreatePR {
		tui.Info("Creating PR...")
		prURL, err = ghpkg.CreatePR(
			destDir,
			"chore(claude): bootstrap Claude Code via ccbootstrap",
			fmt.Sprintf("Generated by [ccbootstrap](https://github.com/abdessama-cto/ccb) v%s.\n\n"+
				"### What's included\n"+
				"- `CLAUDE.md` — project context for Claude\n"+
				"- `.claude/settings.json` — permissions\n"+
				"- `.claude/rules/` — 4 behavior rules\n"+
				"- `.claude/hooks/` — automation hooks\n"+
				"- `.claude/commands/` — slash commands\n"+
				"- `docs/` — architecture, progress, decisions\n", Version),
			q.BranchName,
		)
		if err != nil {
			tui.Warn("PR creation failed: " + err.Error())
		} else {
			tui.Success("PR created: " + prURL)
		}
	}

	elapsed := time.Since(startTime).Round(time.Second)
	fmt.Printf("\n%s Done in %s\n\n", tui.Green("🎉"), tui.Bold(elapsed.String()))
	if prURL != "" {
		fmt.Printf("%s PR: %s\n\n", tui.Bold("📎"), prURL)
	}
	fmt.Printf("Next steps:\n  cd %s\n  claude\n  Then run: /context\n\n", destDir)
	return nil
}

// ─── Questionnaire ────────────────────────────────────────────────────────────

func runQuestionnaire(fp *analyzer.ProjectFingerprint) (*generator.Questionnaire, error) {
	fmt.Printf("\n%s Quick setup — answer %s questions\n\n",
		tui.Bold("📝"), tui.Cyan("10"))

	q := &generator.Questionnaire{}
	reader := bufio.NewReader(os.Stdin)

	q.Goal = selectOption(reader, "1/10 Primary goal?", []string{
		"quality", "ship-fast", "stability", "refactor",
	}, "quality")

	q.WorkflowStyle = selectOption(reader, "2/10 Workflow style?", []string{
		"plan-execute", "vibe", "spec-driven",
	}, "plan-execute")

	q.TeamSize = selectOption(reader, "3/10 Team size?", []string{
		"solo", "small (2-5)", "medium (5-15)", "large (15+)",
	}, "solo")

	q.AutoFormatHook = confirmQ(reader, "4/10 Auto-format files after each edit?", true)
	q.SecretScanHook = confirmQ(reader, "5/10 Scan for secrets before writing files?", true)
	q.AutoCommitHook = confirmQ(reader, "6/10 Auto-commit after each edit (on feature branches)?", false)
	q.DesktopNotify = confirmQ(reader, "7/10 Desktop notification when Claude finishes a task?", true)
	q.PushGuardHook = confirmQ(reader, "8/10 Block git push if tests fail?", len(fp.TestFrameworks) > 0)
	q.AuditLogHook = confirmQ(reader, "9/10 Audit log of all bash commands?", false)
	q.InstallSkills = confirmQ(reader, "10/10 Install recommended skills?", true)
	q.RunTests = len(fp.TestFrameworks) > 0
	q.CreatePR = !initFlags.noPR

	fmt.Printf("\n%s Questionnaire done\n\n", tui.Green("✓"))
	return q, nil
}

func defaultQuestionnaire(profile string) *generator.Questionnaire {
	q := &generator.Questionnaire{
		Goal:           "quality",
		WorkflowStyle:  "plan-execute",
		TeamSize:       "solo",
		SecretScanHook: true,
		DesktopNotify:  true,
		RunTests:       true,
		InstallSkills:  true,
		CreatePR:       true,
	}
	switch profile {
	case "strict":
		q.AutoFormatHook = true
		q.AutoCommitHook = true
		q.PushGuardHook = true
		q.AuditLogHook = true
		q.WorkflowStyle = "spec-driven"
	case "lightweight":
		q.SecretScanHook = false
		q.DesktopNotify = false
		q.InstallSkills = false
		q.WorkflowStyle = "vibe"
	default: // balanced
		q.AutoFormatHook = true
		q.PushGuardHook = true
	}
	return q
}

func selectOption(r *bufio.Reader, prompt string, options []string, defaultVal string) string {
	fmt.Printf("  %s\n", tui.Bold(prompt))
	for i, o := range options {
		marker := "  "
		if o == defaultVal {
			marker = tui.Green("▶ ")
		}
		fmt.Printf("    %s[%d] %s\n", marker, i+1, o)
	}
	fmt.Print("  Choice (enter for default): ")
	input, _ := r.ReadString('\n')
	input = strings.TrimSpace(input)
	if input == "" {
		return defaultVal
	}
	var idx int
	n, _ := fmt.Sscanf(input, "%d", &idx)
	if n == 1 && idx >= 1 && idx <= len(options) {
		return options[idx-1]
	}
	return defaultVal
}

func confirmQ(r *bufio.Reader, prompt string, defaultVal bool) bool {
	hint := "Y/n"
	if !defaultVal {
		hint = "y/N"
	}
	fmt.Printf("  %s %s: ", tui.Bold(prompt), tui.Dim("["+hint+"]"))
	input, _ := r.ReadString('\n')
	input = strings.ToLower(strings.TrimSpace(input))
	if input == "" {
		return defaultVal
	}
	return input == "y" || input == "yes"
}

// ─── Helpers ──────────────────────────────────────────────────────────────────

func printFingerprint(fp *analyzer.ProjectFingerprint) {
	fmt.Printf("\n%s Project fingerprint\n", tui.Bold("📊"))
	fmt.Printf("   %-12s : %s\n", "Stack", tui.Cyan(fp.StackString()))
	fmt.Printf("   %-12s : %s across %d files\n", "Size", formatLOC(fp.LOC), fp.Files)
	fmt.Printf("   %-12s : %s\n", "Age", fp.Age)
	fmt.Printf("   %-12s : %s\n", "Tests", fp.TestFrameworksString())
	fmt.Printf("   %-12s : %s\n", "CI/CD", boolEmoji2(fp.HasCI))
	fmt.Printf("   %-12s : %s\n", "Docker", boolEmoji2(fp.HasDocker))
	fmt.Printf("   %-12s : %s\n", "Secrets (.env)", boolEmoji2(fp.HasEnvFile))
	fmt.Println()
}

func boolEmoji2(v bool) string {
	if v {
		return "✅"
	}
	return "❌ not found"
}

func formatLOC(loc int) string {
	if loc >= 1000 {
		return fmt.Sprintf("%dk LOC", loc/1000)
	}
	return fmt.Sprintf("%d LOC", loc)
}

func buildTestCmdForStack(fp *analyzer.ProjectFingerprint) *exec.Cmd {
	for _, s := range fp.Stack {
		switch {
		case strings.Contains(s, "Laravel"):
			return exec.Command("php", "artisan", "test", "--parallel")
		case strings.Contains(s, "NestJS"), strings.Contains(s, "Next"), strings.Contains(s, "React"):
			return exec.Command("npm", "test")
		case strings.Contains(s, "Django"), strings.Contains(s, "FastAPI"), strings.Contains(s, "Flask"):
			return exec.Command("pytest")
		case s == "Go" || len(s) > 2 && s[:3] == "Go/":
			return exec.Command("go", "test", "./...")
		case strings.Contains(s, "Rails"):
			return exec.Command("bundle", "exec", "rspec")
		}
	}
	return nil
}
