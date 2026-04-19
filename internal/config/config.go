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
	Enabled bool   `yaml:"enabled"`
	Provider string `yaml:"provider"` // "openai" | "gemini" | "ollama"

	// OpenAI
	OpenAIKey   string `yaml:"openai_key,omitempty"`
	OpenAIModel string `yaml:"openai_model"`

	// Google Gemini
	GeminiKey   string `yaml:"gemini_key,omitempty"`
	GeminiModel string `yaml:"gemini_model"`

	// Ollama (local)
	OllamaURL   string `yaml:"ollama_url"`
	OllamaModel string `yaml:"ollama_model"`

	MonthlyBudgetUSD float64 `yaml:"monthly_budget_usd"`
}

// ActiveKey returns the API key for the currently selected provider
func (a *AIConfig) ActiveKey() string {
	switch a.Provider {
	case "gemini":
		return a.GeminiKey
	case "ollama":
		return "ollama" // Ollama doesn't need a real key
	default:
		return a.OpenAIKey
	}
}

// ActiveModel returns the model for the currently selected provider
func (a *AIConfig) ActiveModel() string {
	switch a.Provider {
	case "gemini":
		return a.GeminiModel
	case "ollama":
		return a.OllamaModel
	default:
		return a.OpenAIModel
	}
}

// IsConfigured returns true if the active provider has what it needs to work
func (a *AIConfig) IsConfigured() bool {
	if !a.Enabled {
		return false
	}
	switch a.Provider {
	case "gemini":
		return a.GeminiKey != ""
	case "ollama":
		return true // no key needed
	default:
		return a.OpenAIKey != ""
	}
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
			Enabled:          true,
			Provider:         "openai",
			OpenAIModel:      "gpt-5.4-mini",     // Fast + capable default
			GeminiModel:      "gemini-2.5-flash", // Stable fast default
			OllamaURL:        "http://localhost:11434",
			OllamaModel:      "llama3.2",
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
