package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/abdessama-cto/ccb/internal/cache"
	"github.com/abdessama-cto/ccb/internal/config"
	"github.com/abdessama-cto/ccb/internal/llm"
	"github.com/abdessama-cto/ccb/internal/tui"
	"github.com/spf13/cobra"
)

var addAgentCmd = &cobra.Command{
	Use:   "agent [name or description]",
	Short: "Generate a new agent tailored to this project",
	Long: `Ask the AI to write a new Claude Code agent for the current project.

Without arguments: the AI picks the most valuable missing agent for the project.
With arguments: the AI creates an agent around the topic you provide.

Examples:
  ccb add agent
  ccb add agent payment-webhook-guard
  ccb add agent "reviews database migrations"`,
	Args: cobra.ArbitraryArgs,
	RunE: runAddAgent,
}

func runAddAgent(cmd *cobra.Command, args []string) error {
	projectDir, err := requireProject()
	if err != nil {
		return err
	}

	cfg, _ := config.Load()
	if !cfg.AI.IsConfigured() {
		return fmt.Errorf("AI not configured — run 'ccb settings'")
	}

	cached, err := cache.Load(projectDir)
	if err != nil || cached == nil {
		return fmt.Errorf("no cached project analysis found — run 'ccb init' or 'ccb reanalyze' first")
	}

	nameHint := strings.TrimSpace(strings.Join(args, " "))
	llmCfg := llm.Config{
		Provider:        llm.Provider(cfg.AI.Provider),
		Model:           cfg.AI.ActiveModel(),
		APIKey:          cfg.AI.ActiveKey(),
		OllamaURL:       cfg.AI.OllamaURL,
		MaxContextChars: contextLimitForProvider(cfg.AI.Provider),
		Language:        cfg.UI.Language,
	}

	var sp *tui.Spinner
	if nameHint != "" {
		sp = tui.StartSpinner(fmt.Sprintf("Generating agent for %q...", nameHint))
	} else {
		sp = tui.StartSpinner("Asking AI for the most valuable missing agent for this project...")
	}

	agent, err := llm.GenerateAgent(llmCfg, cached.Understanding, cached.Fingerprint, nameHint)
	if err != nil {
		sp.Fail("Agent generation failed")
		return fmt.Errorf("agent generation failed: %w", err)
	}
	sp.Success(fmt.Sprintf("Agent ready: %s", agent.Name))

	agentsDir := filepath.Join(projectDir, ".claude", "agents")
	if err := os.MkdirAll(agentsDir, 0755); err != nil {
		return err
	}

	dest := filepath.Join(agentsDir, agent.Filename)
	if _, statErr := os.Stat(dest); statErr == nil {
		tui.Warn(fmt.Sprintf("%s already exists — overwriting", agent.Filename))
	}
	if err := os.WriteFile(dest, []byte(agent.Content), 0644); err != nil {
		return fmt.Errorf("write %s: %w", dest, err)
	}

	rel, _ := filepath.Rel(projectDir, dest)
	tui.Success(fmt.Sprintf("Created agent: %s", rel))
	fmt.Printf("  Name: %s\n", tui.Cyan(agent.Name))
	return nil
}
