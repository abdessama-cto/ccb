package cmd

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/abdessama-cto/ccb/internal/config"
	"github.com/abdessama-cto/ccb/internal/llm"
	"github.com/abdessama-cto/ccb/internal/tui"
	"github.com/spf13/cobra"
)

var settingsCmd = &cobra.Command{
	Use:   "settings",
	Short: "Configure ccbootstrap credentials and preferences",
	RunE:  runSettings,
}

func init() {
	rootCmd.AddCommand(settingsCmd)
}

func runSettings(cmd *cobra.Command, args []string) error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	for {
		printSettingsMenu(cfg)

		fmt.Print("Choice: ")
		reader := bufio.NewReader(os.Stdin)
		input, _ := reader.ReadString('\n')
		choice := strings.TrimSpace(input)

		switch choice {
		// ── Provider
		case "1":
			cfg.AI.Provider = promptChoice("AI Provider", []string{
				"openai", "gemini", "ollama",
			}, cfg.AI.Provider)
		// ── OpenAI
		case "2":
			cfg.AI.OpenAIKey = promptSecret("OpenAI API Key", cfg.AI.OpenAIKey)
		case "3":
			cfg.AI.OpenAIModel = promptChoice("OpenAI Model", llm.OpenAIModels, cfg.AI.OpenAIModel)
		// ── Gemini
		case "4":
			cfg.AI.GeminiKey = promptSecret("Gemini API Key", cfg.AI.GeminiKey)
		case "5":
			cfg.AI.GeminiModel = promptChoice("Gemini Model", llm.GeminiModels, cfg.AI.GeminiModel)
		// ── Ollama
		case "6":
			cfg.AI.OllamaURL = promptInput("Ollama base URL", cfg.AI.OllamaURL)
		case "7":
			cfg.AI.OllamaModel = promptChoice("Ollama Model", llm.OllamaModels, cfg.AI.OllamaModel)
		// ── General AI
		case "8":
			cfg.AI.Enabled = !cfg.AI.Enabled
		case "9":
			val := promptInput("Monthly budget (USD)", fmt.Sprintf("%.2f", cfg.AI.MonthlyBudgetUSD))
			fmt.Sscanf(val, "%f", &cfg.AI.MonthlyBudgetUSD)
		// ── UI
		case "10":
			cfg.UI.Language = promptChoice("Language", []string{"auto", "en", "fr", "es", "de"}, cfg.UI.Language)
		case "11":
			cfg.UI.ColorScheme = promptChoice("Color scheme", []string{"auto", "dark", "light"}, cfg.UI.ColorScheme)
		// ── Defaults
		case "12":
			cfg.Defaults.Profile = promptChoice("Default profile", []string{"balanced", "strict", "lightweight"}, cfg.Defaults.Profile)
		case "13":
			cfg.Defaults.AutoPR = !cfg.Defaults.AutoPR
		case "14":
			cfg.Defaults.AutoRunTests = !cfg.Defaults.AutoRunTests
		case "s", "S":
			if err := config.Save(cfg); err != nil {
				tui.Err("Failed to save: " + err.Error())
			} else {
				tui.Success("Settings saved to " + config.ConfigFile)
			}
			return nil
		case "q", "Q":
			fmt.Println("No changes saved.")
			return nil
		default:
			tui.Warn("Unknown option: " + choice)
		}
	}
}

func printSettingsMenu(cfg *config.Config) {
	aiStatus := tui.Green("✅ enabled")
	if !cfg.AI.Enabled {
		aiStatus = tui.Red("❌ disabled")
	}

	// Provider status
	providerLabel := tui.Cyan(cfg.AI.Provider)
	activeModel := cfg.AI.ActiveModel()

	// Key statuses
	openAIKeyStatus := keyStatus(cfg.AI.OpenAIKey)
	geminiKeyStatus := keyStatus(cfg.AI.GeminiKey)
	ollamaStatus := tui.Dim(cfg.AI.OllamaURL)

	autoPR := boolStr(cfg.Defaults.AutoPR)
	autoTest := boolStr(cfg.Defaults.AutoRunTests)

	lines := []string{
		"",
		tui.Bold("🤖 AI Provider"),
		fmt.Sprintf("   [1] Active provider      %s  (model: %s)", providerLabel, tui.Cyan(activeModel)),
		fmt.Sprintf("   [8] AI Enabled           %s", aiStatus),
		fmt.Sprintf("   [9] Monthly budget       $%.2f", cfg.AI.MonthlyBudgetUSD),
		"",
		tui.Bold("🔑 OpenAI"),
		fmt.Sprintf("   [2] API Key              %s", openAIKeyStatus),
		fmt.Sprintf("   [3] Model                %s", tui.Cyan(cfg.AI.OpenAIModel)),
		"",
		tui.Bold("💎 Google Gemini"),
		fmt.Sprintf("   [4] API Key              %s", geminiKeyStatus),
		fmt.Sprintf("   [5] Model                %s", tui.Cyan(cfg.AI.GeminiModel)),
		"",
		tui.Bold("🦙 Ollama (local)"),
		fmt.Sprintf("   [6] URL                  %s", ollamaStatus),
		fmt.Sprintf("   [7] Model                %s", tui.Cyan(cfg.AI.OllamaModel)),
		"",
		tui.Bold("🎨 UI"),
		fmt.Sprintf("   [10] Language            %s", cfg.UI.Language),
		fmt.Sprintf("   [11] Color scheme        %s", cfg.UI.ColorScheme),
		"",
		tui.Bold("📦 Defaults"),
		fmt.Sprintf("   [12] Profile             %s", tui.Cyan(cfg.Defaults.Profile)),
		fmt.Sprintf("   [13] Auto-create PR      %s", autoPR),
		fmt.Sprintf("   [14] Auto-run tests      %s", autoTest),
		"",
		fmt.Sprintf("   [s] Save & quit   [q] Quit without saving"),
		"",
	}

	tui.Box("ccbootstrap Settings — v"+Version, lines)
}

func keyStatus(key string) string {
	if key == "" {
		return tui.Red("not set")
	}
	masked := key
	if len(masked) > 8 {
		masked = masked[:8] + "••••••••"
	}
	return tui.Green("✅ ") + masked
}

func boolStr(v bool) string {
	if v {
		return tui.Green("yes")
	}
	return tui.Red("no")
}

func promptInput(label, current string) string {
	fmt.Printf("  %s [%s]: ", tui.Bold(label), tui.Dim(current))
	reader := bufio.NewReader(os.Stdin)
	val, _ := reader.ReadString('\n')
	val = strings.TrimSpace(val)
	if val == "" {
		return current
	}
	return val
}

func promptSecret(label, current string) string {
	masked := "(empty)"
	if current != "" {
		masked = current[:min(8, len(current))] + "••••"
	}
	fmt.Printf("  %s [%s]: ", tui.Bold(label), tui.Dim(masked))
	reader := bufio.NewReader(os.Stdin)
	val, _ := reader.ReadString('\n')
	val = strings.TrimSpace(val)
	if val == "" {
		return current
	}
	return val
}

func promptChoice(label string, choices []string, current string) string {
	fmt.Printf("\n  %s:\n", tui.Bold(label))
	for i, c := range choices {
		marker := "  "
		if c == current {
			marker = tui.Green("▶ ")
		}
		fmt.Printf("    %s[%d] %s\n", marker, i+1, c)
	}
	fmt.Print("  Choice [enter to keep current]: ")
	reader := bufio.NewReader(os.Stdin)
	val, _ := reader.ReadString('\n')
	val = strings.TrimSpace(val)
	var idx int
	if _, err := fmt.Sscanf(val, "%d", &idx); err == nil {
		if idx >= 1 && idx <= len(choices) {
			return choices[idx-1]
		}
	}
	return current
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
