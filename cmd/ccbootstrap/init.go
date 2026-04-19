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
	"github.com/abdessama-cto/ccb/internal/cache"
	"github.com/abdessama-cto/ccb/internal/config"
	"github.com/abdessama-cto/ccb/internal/generator"
	ghpkg "github.com/abdessama-cto/ccb/internal/github"
	"github.com/abdessama-cto/ccb/internal/llm"
	"github.com/abdessama-cto/ccb/internal/skills"
	"github.com/abdessama-cto/ccb/internal/tui"
	"github.com/spf13/cobra"
)

var initFlags struct {
	yes               bool
	profile           string
	dryRun            bool
	skipQuestionnaire bool
}

var initCmd = &cobra.Command{
	Use:     "init [repo-url|.]",
	Aliases: []string{"i"},
	Short:   "Bootstrap Claude Code config (GitHub repo or local directory)",
	Long: `Three modes:
  1. ccb init <github-url>  — clone & bootstrap a GitHub repo
  2. ccb init               — bootstrap the current local directory
  3. ccb init .             — same as above

Generates CLAUDE.md, .claude/ (rules, hooks, commands, agents, skills), docs/.
Everything tailored to this project by AI. No git push — you keep control.`,
	Args: cobra.RangeArgs(0, 1),
	RunE: runInit,
}

func init() {
	rootCmd.AddCommand(initCmd)
	initCmd.Flags().BoolVarP(&initFlags.yes, "yes", "y", false, "Skip wizard confirmations (use AI defaults)")
	initCmd.Flags().StringVar(&initFlags.profile, "profile", "balanced", "Hook profile: balanced | strict | lightweight")
	initCmd.Flags().BoolVar(&initFlags.dryRun, "dry-run", false, "Generate files without running tests")
	initCmd.Flags().BoolVar(&initFlags.skipQuestionnaire, "skip-questionnaire", false, "Skip wizard, use AI-suggested defaults")
}

func runInit(cmd *cobra.Command, args []string) error {
	startTime := time.Now()
	tui.Banner(Version)

	// ── Step 1: Determine mode (local / clone) ───────────────────────────────
	destDir, repoURL, err := resolveDestination(args)
	if err != nil {
		return err
	}

	// ── Step 2: Static analysis ──────────────────────────────────────────────
	tui.Info("Analyzing codebase (static)...")
	commits := ghpkg.CountCommits(destDir)
	fp, err := analyzer.Analyze(destDir, repoURL, commits)
	if err != nil {
		return fmt.Errorf("analysis failed: %w", err)
	}
	printFingerprint(fp)

	// ── Step 3: AI configuration check ───────────────────────────────────────
	cfg, _ := config.Load()
	if !cfg.AI.IsConfigured() {
		tui.Err("No AI provider configured. ccb requires an LLM to tailor the config.")
		tui.Info("Run: ccb settings  (or: ccb start  for a guided setup)")
		return fmt.Errorf("AI provider not configured")
	}

	llmCfg := llm.Config{
		Provider:        llm.Provider(cfg.AI.Provider),
		Model:           cfg.AI.ActiveModel(),
		APIKey:          cfg.AI.ActiveKey(),
		OllamaURL:       cfg.AI.OllamaURL,
		MaxContextChars: contextLimitForProvider(cfg.AI.Provider),
	}

	// ── Step 3b: Semantic AI analysis (with cache) ───────────────────────────
	understanding, err := ensureUnderstanding(destDir, fp, cfg, llmCfg)
	if err != nil {
		return err
	}

	// ── Step 4: Wizard — dynamic AI-driven questions ─────────────────────────
	answers, err := runDynamicWizard(llmCfg, understanding, fp)
	if err != nil {
		tui.Warn(fmt.Sprintf("Wizard skipped: %s", err.Error()))
		answers = nil
	}

	// ── Step 5: Proposals — AI-driven agents/rules/skills ────────────────────
	tui.Info("Asking AI for tailored agents, rules, and skills...")
	proposals, err := llm.GenerateProposals(llmCfg, understanding, fp, answers)
	if err != nil {
		return fmt.Errorf("proposals generation failed: %w", err)
	}
	tui.Success(fmt.Sprintf("AI proposed %d agents, %d rules, %d skills",
		len(proposals.Agents), len(proposals.Rules), len(proposals.Skills)))

	if !initFlags.yes && !initFlags.skipQuestionnaire {
		proposals = ConfirmProposals(proposals)
	}
	selectedAgents := SelectedAgents(proposals)
	selectedRules := SelectedRules(proposals)
	selectedSkills := SelectedSkills(proposals)

	// Split skills into LLM-generated vs external (skills.sh fetched)
	var llmSkills, extSkills []llm.SkillProposal
	for _, s := range selectedSkills {
		if s.ExternalID != "" {
			extSkills = append(extSkills, s)
		} else {
			llmSkills = append(llmSkills, s)
		}
	}

	// ── Step 6: Generate files — single LLM call with progress ───────────────
	tui.Info("Generating Claude Code configuration (single LLM call)...")
	genResult, err := llm.GenerateFiles(llmCfg, understanding, fp, answers,
		selectedAgents, selectedRules, llmSkills)
	if err != nil {
		return fmt.Errorf("file generation failed: %w", err)
	}
	tui.Success(fmt.Sprintf("AI returned %d files", len(genResult.Files)))

	// Fetch external skills (skills.sh) directly from GitHub raw.
	if len(extSkills) > 0 {
		tui.Info(fmt.Sprintf("Fetching %d skill(s) from skills.sh...", len(extSkills)))
		for _, s := range extSkills {
			ref := skills.Skill{ID: s.ExternalID, Source: s.ExternalSource, SkillID: s.Name}
			if err := skills.FetchContent(&ref); err != nil {
				tui.Warn(fmt.Sprintf("  %s: %s — skipping", s.Name, err.Error()))
				continue
			}
			genResult.Files = append(genResult.Files, llm.GeneratedFile{
				Path:    ".claude/skills/" + s.Filename,
				Content: ref.Content,
			})
		}
	}

	// Build hook questionnaire from profile
	q := hookSettingsForProfile(initFlags.profile, fp)

	if err := generator.Generate(destDir, fp, q, understanding, genResult.Files); err != nil {
		return fmt.Errorf("generation failed: %w", err)
	}

	// ── Step 7: Run tests (regression check) ─────────────────────────────────
	if !initFlags.dryRun && q.RunTests && len(fp.TestFrameworks) > 0 {
		tui.Info("Running existing test suite for regression check...")
		testCmd := buildTestCmdForStack(fp)
		if testCmd != nil {
			testCmd.Dir = destDir
			testCmd.Stdout = os.Stdout
			testCmd.Stderr = os.Stderr
			if err := testCmd.Run(); err != nil {
				tui.Warn("Some tests failed — review before committing.")
			} else {
				tui.Success("All tests green")
			}
		}
	}

	// ── Final summary ────────────────────────────────────────────────────────
	elapsed := time.Since(startTime).Round(time.Second)
	fmt.Printf("\n%s Done in %s\n\n", tui.Green("🎉"), tui.Bold(elapsed.String()))
	fmt.Printf("Next steps:\n")
	fmt.Printf("  cd %s\n", destDir)
	fmt.Printf("  git status                               # review the generated files\n")
	fmt.Printf("  git add CLAUDE.md .claude/ docs/\n")
	fmt.Printf("  git commit -m \"chore: bootstrap Claude Code config\"\n")
	fmt.Printf("  claude                                    # then run: /context\n\n")
	return nil
}

// ─── Helpers ──────────────────────────────────────────────────────────────────

// resolveDestination figures out whether we are bootstrapping a local dir or
// cloning a GitHub repo, and returns destDir + repoURL.
func resolveDestination(args []string) (destDir, repoURL string, err error) {
	useLocal := len(args) == 0 || args[0] == "." || args[0] == ""
	isGitHubURL := len(args) > 0 && (strings.HasPrefix(args[0], "http") || strings.HasPrefix(args[0], "git@"))
	if !useLocal && !isGitHubURL && len(args) > 0 {
		useLocal = true
	}

	if useLocal {
		destDir, err = os.Getwd()
		if err != nil {
			return "", "", fmt.Errorf("could not determine current directory: %w", err)
		}
		if _, statErr := os.Stat(filepath.Join(destDir, ".git")); statErr != nil {
			return "", "", fmt.Errorf("current directory is not a git repository: %s", destDir)
		}
		if remoteOut, remoteErr := exec.Command("git", "-C", destDir, "remote", "get-url", "origin").Output(); remoteErr == nil {
			repoURL = strings.TrimSpace(string(remoteOut))
		} else {
			repoURL = destDir
		}
		tui.Success(fmt.Sprintf("Using local project: %s", tui.Bold(filepath.Base(destDir))))
		return destDir, repoURL, nil
	}

	// Clone mode
	repoURL = args[0]
	tui.Info("Checking GitHub auth...")
	user, authErr := ghpkg.CheckAuth()
	if authErr != nil {
		return "", "", authErr
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
			return "", "", fmt.Errorf("clone failed: %w", cloneErr)
		}
		tui.Success("Clone complete")
	}
	return destDir, repoURL, nil
}

// contextLimitForProvider returns the character budget for the full-codebase prompt.
// Ollama is limited because local models typically have smaller context windows.
// All cloud providers (OpenAI, Gemini) assume ~1M tokens (~4M chars).
func contextLimitForProvider(provider string) int {
	if provider == "ollama" {
		return 100_000
	}
	return 4_000_000
}

// ensureUnderstanding loads the cached AI analysis or runs a fresh one.
func ensureUnderstanding(destDir string, fp *analyzer.ProjectFingerprint, cfg *config.Config, llmCfg llm.Config) (*llm.ProjectUnderstanding, error) {
	// ── Cache check ──────────────────────────────────────────────────────────
	if cached, loadErr := cache.Load(destDir); loadErr == nil && cached != nil {
		tui.Info(fmt.Sprintf("\n  📦 Cached analysis found: %s", cached.Summary()))
		useCache := true
		if !initFlags.yes {
			answer := askLine("  Reuse it? [Y/n]: ", "y")
			answer = strings.ToLower(strings.TrimSpace(answer))
			useCache = answer == "" || answer == "y" || answer == "yes"
		}
		if useCache {
			tui.Success("  Using cached analysis ✓")
			printUnderstanding(cached.Understanding)
			return cached.Understanding, nil
		}
	}

	// ── Fresh analysis ───────────────────────────────────────────────────────
	tui.Info(fmt.Sprintf("Reading full codebase for AI understanding (%s / %s)...",
		cfg.AI.Provider, cfg.AI.ActiveModel()))
	semCtx := analyzer.ExtractSemanticContext(destDir)
	tui.Info(fmt.Sprintf("  %d files · ~%dk tokens extracted",
		len(semCtx.Files), semCtx.TokenEst/1000))

	prompt := analyzer.BuildAIPromptLimited(semCtx, fp, llmCfg.MaxContextChars)
	tui.Info(fmt.Sprintf("  Sending %dk chars to %s...", len(prompt)/1000, cfg.AI.ActiveModel()))

	understanding, err := llm.UnderstandProject(llmCfg, prompt)
	if err != nil {
		return nil, fmt.Errorf("AI understanding failed: %w", err)
	}
	printUnderstanding(understanding)

	// Persist to cache
	if saveErr := cache.Save(
		destDir, cfg.AI.Provider, cfg.AI.ActiveModel(),
		len(semCtx.Files), semCtx.TokenEst, fp, understanding,
	); saveErr != nil {
		tui.Warn(fmt.Sprintf("Could not cache analysis: %s", saveErr.Error()))
	} else {
		tui.Success(fmt.Sprintf("  Analysis cached to %s", cache.CachePath(destDir)))
	}
	return understanding, nil
}

// runDynamicWizard asks the LLM to generate project-specific questions, then
// displays them in a wizard UI. Returns the answers or uses defaults in --yes mode.
func runDynamicWizard(llmCfg llm.Config, u *llm.ProjectUnderstanding, fp *analyzer.ProjectFingerprint) ([]llm.WizardAnswer, error) {
	tui.Info("Asking AI for project-specific questions...")
	questions, err := llm.GenerateWizardQuestions(llmCfg, u, fp)
	if err != nil {
		return nil, err
	}
	if len(questions) == 0 {
		tui.Info("AI had no project-specific questions — skipping wizard")
		return nil, nil
	}
	tui.Success(fmt.Sprintf("Generated %d question(s) tailored to this project", len(questions)))

	if initFlags.yes || initFlags.skipQuestionnaire {
		tui.Info("Skipping wizard — using AI-suggested defaults")
		return WizardDefaults(questions), nil
	}

	answers := RunWizard(questions)
	if answers == nil {
		tui.Warn("Wizard aborted — using AI-suggested defaults")
		return WizardDefaults(questions), nil
	}
	return answers, nil
}

// hookSettingsForProfile derives the hook configuration from the --profile flag.
func hookSettingsForProfile(profile string, fp *analyzer.ProjectFingerprint) *generator.Questionnaire {
	q := &generator.Questionnaire{
		SecretScanHook: true,
		DesktopNotify:  true,
		InstallSkills:  true,
		RunTests:       true,
	}
	switch profile {
	case "strict":
		q.AutoFormatHook = true
		q.AutoCommitHook = true
		q.PushGuardHook = true
		q.AuditLogHook = true
	case "lightweight":
		q.SecretScanHook = false
		q.DesktopNotify = false
		q.InstallSkills = false
	default: // balanced
		q.AutoFormatHook = true
		q.PushGuardHook = len(fp.TestFrameworks) > 0
	}
	return q
}

// askLine prints a prompt and reads one line from stdin. Enter = default.
func askLine(prompt, defaultVal string) string {
	fmt.Print(prompt)
	r := bufio.NewReader(os.Stdin)
	line, _ := r.ReadString('\n')
	line = strings.TrimSpace(line)
	if line == "" {
		return defaultVal
	}
	return line
}

// ─── Display helpers ──────────────────────────────────────────────────────────

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

func printUnderstanding(u *llm.ProjectUnderstanding) {
	const width = 68

	line := func(s string) string {
		visible := visibleLen(s)
		pad := width - 2 - visible
		if pad < 0 {
			pad = 0
		}
		return "║ " + s + strings.Repeat(" ", pad) + " ║"
	}

	divider := "╠" + strings.Repeat("═", width) + "╗"
	top := "╔" + strings.Repeat("═", width) + "╗"
	bottom := "╚" + strings.Repeat("═", width) + "╝"
	sep := "║" + strings.Repeat("─", width) + "║"

	fmt.Println()
	fmt.Println(top)
	title := "  🧠  AI Project Understanding"
	fmt.Println(line(tui.Bold(title)))
	fmt.Println(divider)

	name := u.ProjectName
	if name == "" {
		name = "Unknown project"
	}
	domain := u.Domain
	nameStr := tui.Cyan(tui.Bold(name))
	if domain != "" {
		nameStr += tui.Dim("  ·  " + domain)
	}
	fmt.Println(line("  " + nameStr))
	fmt.Println(sep)

	if u.Purpose != "" {
		fmt.Println(line(tui.Bold("  PURPOSE")))
		for _, l := range wordWrap(u.Purpose, width-4) {
			fmt.Println(line("  " + l))
		}
		fmt.Println(sep)
	}
	if u.Architecture != "" {
		fmt.Println(line(tui.Bold("  ARCHITECTURE")))
		for _, l := range wordWrap(u.Architecture, width-4) {
			fmt.Println(line("  " + l))
		}
		fmt.Println(sep)
	}
	if len(u.KeyFeatures) > 0 {
		fmt.Println(line(tui.Bold("  ✦ KEY FEATURES")))
		for _, f := range u.KeyFeatures {
			fmt.Println(line("    " + tui.Cyan("•") + " " + f))
		}
		fmt.Println(sep)
	}
	if len(u.MainModules) > 0 {
		fmt.Println(line(tui.Bold("  📁 MAIN MODULES")))
		for _, m := range u.MainModules {
			parts := strings.SplitN(m, ":", 2)
			if len(parts) == 2 {
				row := tui.Cyan(fmt.Sprintf("    %-18s", strings.TrimSpace(parts[0]))) +
					tui.Dim(strings.TrimSpace(parts[1]))
				fmt.Println(line(row))
			} else {
				fmt.Println(line("    " + m))
			}
		}
		fmt.Println(sep)
	}
	if len(u.APIEndpoints) > 0 {
		fmt.Println(line(tui.Bold("  🌐 API ENDPOINTS")))
		shown := u.APIEndpoints
		if len(shown) > 8 {
			shown = shown[:8]
		}
		for _, e := range shown {
			parts := strings.SplitN(e, "—", 2)
			if len(parts) == 2 {
				row := tui.Green(fmt.Sprintf("    %-28s", strings.TrimSpace(parts[0]))) +
					tui.Dim(strings.TrimSpace(parts[1]))
				fmt.Println(line(row))
			} else {
				fmt.Println(line("    " + e))
			}
		}
		if len(u.APIEndpoints) > 8 {
			fmt.Println(line(tui.Dim(fmt.Sprintf("    ... +%d more endpoints", len(u.APIEndpoints)-8))))
		}
		fmt.Println(sep)
	}
	if len(u.ExternalServices) > 0 {
		fmt.Println(line(tui.Bold("  🔌 EXTERNAL SERVICES")))
		row := "    " + strings.Join(colorServices(u.ExternalServices), tui.Dim(" · "))
		fmt.Println(line(row))
		fmt.Println(sep)
	}
	if len(u.Conventions) > 0 {
		fmt.Println(line(tui.Bold("  📐 CONVENTIONS")))
		for _, c := range u.Conventions {
			fmt.Println(line("    " + tui.Dim("›") + " " + c))
		}
		fmt.Println(sep)
	}
	if u.WhatClaudeKnows != "" {
		fmt.Println(line(tui.Bold("  💡 FOR CLAUDE")))
		for _, l := range wordWrap(u.WhatClaudeKnows, width-4) {
			fmt.Println(line("  " + tui.Dim(l)))
		}
	}

	fmt.Println(bottom)
	fmt.Println()
}

func wordWrap(s string, maxWidth int) []string {
	words := strings.Fields(s)
	var lines []string
	current := ""
	for _, w := range words {
		if current == "" {
			current = w
		} else if len(current)+1+len(w) <= maxWidth {
			current += " " + w
		} else {
			lines = append(lines, current)
			current = w
		}
	}
	if current != "" {
		lines = append(lines, current)
	}
	if len(lines) == 0 {
		return []string{s}
	}
	return lines
}

func visibleLen(s string) int {
	inEscape := false
	count := 0
	for _, r := range s {
		if r == '\033' {
			inEscape = true
			continue
		}
		if inEscape {
			if r == 'm' {
				inEscape = false
			}
			continue
		}
		count++
	}
	return count
}

func colorServices(services []string) []string {
	result := make([]string, len(services))
	for i, s := range services {
		parts := strings.SplitN(s, ":", 2)
		result[i] = tui.Cyan(strings.TrimSpace(parts[0]))
	}
	return result
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
