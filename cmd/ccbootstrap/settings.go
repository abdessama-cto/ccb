package cmd

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/abdessama-cto/ccb/internal/config"
	"github.com/abdessama-cto/ccb/internal/tui"
	"github.com/spf13/cobra"
)

var settingsCmd = &cobra.Command{
	Use:   "settings",
	Short: "Configure ccbootstrap credentials and preferences",
	Long:  "Interactive settings menu to configure API keys, AI models, and default behaviors.",
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
		case "1":
			cfg.AI.APIKey = promptSecret("OpenAI API Key", cfg.AI.APIKey)
		case "2":
			cfg.AI.Model = promptChoice("AI Model", []string{
				"gpt-4o-mini",
				"gpt-4o",
				"o1-mini",
				"o1",
			}, cfg.AI.Model)
		case "3":
			cfg.AI.Enabled = !cfg.AI.Enabled
		case "4":
			val := promptInput("Monthly budget (USD)", fmt.Sprintf("%.2f", cfg.AI.MonthlyBudgetUSD))
			fmt.Sscanf(val, "%f", &cfg.AI.MonthlyBudgetUSD)
		case "5":
			cfg.UI.Language = promptChoice("Language", []string{"auto", "en", "fr", "es", "de"}, cfg.UI.Language)
		case "6":
			cfg.UI.ColorScheme = promptChoice("Color scheme", []string{"auto", "dark", "light"}, cfg.UI.ColorScheme)
		case "7":
			cfg.Defaults.Profile = promptChoice("Default profile", []string{"balanced", "strict", "lightweight"}, cfg.Defaults.Profile)
		case "8":
			cfg.Defaults.AutoPR = !cfg.Defaults.AutoPR
		case "9":
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
	keyStatus := tui.Dim("not set")
	if cfg.AI.APIKey != "" {
		masked := cfg.AI.APIKey
		if len(masked) > 8 {
			masked = masked[:8] + "••••••••"
		}
		keyStatus = tui.Green("✅ ") + masked
	}
	autoPR := boolStr(cfg.Defaults.AutoPR)
	autoTest := boolStr(cfg.Defaults.AutoRunTests)

	lines := []string{
		"",
		tui.Bold("🔑 Credentials"),
		fmt.Sprintf("   [1] OpenAI API Key     %s", keyStatus),
		"",
		tui.Bold("🤖 AI Assistant"),
		fmt.Sprintf("   [2] Model              %s", tui.Cyan(cfg.AI.Model)),
		fmt.Sprintf("   [3] AI Enabled         %s", aiStatus),
		fmt.Sprintf("   [4] Monthly budget     $%.2f", cfg.AI.MonthlyBudgetUSD),
		"",
		tui.Bold("🎨 UI"),
		fmt.Sprintf("   [5] Language           %s", cfg.UI.Language),
		fmt.Sprintf("   [6] Color scheme       %s", cfg.UI.ColorScheme),
		"",
		tui.Bold("📦 Defaults"),
		fmt.Sprintf("   [7] Profile            %s", tui.Cyan(cfg.Defaults.Profile)),
		fmt.Sprintf("   [8] Auto-create PR     %s", autoPR),
		fmt.Sprintf("   [9] Auto-run tests     %s", autoTest),
		"",
		fmt.Sprintf("   [s] Save & quit   [q] Quit without saving"),
		"",
	}

	tui.Box("ccbootstrap Settings — v"+Version, lines)
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
