package config

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"gopkg.in/yaml.v3"
)

var ConfigDir = filepath.Join(os.Getenv("HOME"), ".ccbootstrap")
var ConfigFile = filepath.Join(ConfigDir, "config.yaml")

type AIConfig struct {
	Enabled         bool    `yaml:"enabled"`
	Provider        string  `yaml:"provider"`
	Model           string  `yaml:"model"`
	APIKey          string  `yaml:"api_key,omitempty"`
	MonthlyBudgetUSD float64 `yaml:"monthly_budget_usd"`
}

type UIConfig struct {
	Language    string `yaml:"language"`
	ColorScheme string `yaml:"color_scheme"`
	Verbosity   string `yaml:"verbosity"`
}

type DefaultsConfig struct {
	Profile           string `yaml:"profile"`
	AutoPR            bool   `yaml:"auto_pr"`
	AutoRunTests      bool   `yaml:"auto_run_tests"`
	SkipConfirmations bool   `yaml:"skip_confirmations"`
}

type Config struct {
	Version          string         `yaml:"version,omitempty"`
	InstalledAt      string         `yaml:"installed_at,omitempty"`
	InstallerVersion string         `yaml:"installer_version,omitempty"`
	AI               AIConfig       `yaml:"ai"`
	UI               UIConfig       `yaml:"ui"`
	Defaults         DefaultsConfig `yaml:"defaults"`
}

func Default() *Config {
	return &Config{
		AI: AIConfig{
			Enabled:         true,
			Provider:        "openai",
			Model:           "gpt-4o-mini",
			MonthlyBudgetUSD: 5.0,
		},
		UI: UIConfig{
			Language:    "auto",
			ColorScheme: "auto",
			Verbosity:   "normal",
		},
		Defaults: DefaultsConfig{
			Profile:           "balanced",
			AutoPR:            true,
			AutoRunTests:      true,
			SkipConfirmations: false,
		},
	}
}

func EnsureDirs() error {
	dirs := []string{
		ConfigDir,
		filepath.Join(ConfigDir, "cache", "templates"),
		filepath.Join(ConfigDir, "projects"),
		filepath.Join(ConfigDir, "logs"),
	}
	for _, d := range dirs {
		if err := os.MkdirAll(d, 0755); err != nil {
			return fmt.Errorf("failed to create dir %s: %w", d, err)
		}
	}
	return nil
}

func Load() (*Config, error) {
	_ = EnsureDirs()
	cfg := Default()
	data, err := os.ReadFile(ConfigFile)
	if err != nil {
		if os.IsNotExist(err) {
			return cfg, nil
		}
		return cfg, err
	}
	if err := yaml.Unmarshal(data, cfg); err != nil {
		return Default(), nil
	}
	return cfg, nil
}

func Save(cfg *Config) error {
	_ = EnsureDirs()
	header := "# ccbootstrap config — edit via: ccbootstrap settings\n"
	data, err := yaml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}
	return os.WriteFile(ConfigFile, append([]byte(header), data...), 0600)
}

func InitIfMissing(version, installerVersion string) error {
	if _, err := os.Stat(ConfigFile); err == nil {
		return nil // already exists
	}
	cfg := Default()
	cfg.Version = version
	cfg.InstalledAt = time.Now().UTC().Format(time.RFC3339)
	cfg.InstallerVersion = installerVersion
	return Save(cfg)
}
