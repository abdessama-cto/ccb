package cmd

import (
	"fmt"

	"github.com/abdessama-cto/ccb/internal/config"
	"github.com/abdessama-cto/ccb/internal/tui"
	"github.com/spf13/cobra"
)

var settingsCmd = &cobra.Command{
	Use:   "settings",
	Short: "Configure ccb credentials and preferences",
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
	saved, err := RunSettingsWizard(cfg)
	if err != nil {
		return err
	}
	if saved {
		tui.Success("Settings saved to " + config.ConfigFile)
	} else {
		fmt.Println("No changes saved.")
	}
	return nil
}
