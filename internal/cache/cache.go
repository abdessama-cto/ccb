// Package cache handles persisting and loading AI analysis results per project.
// The analysis file is stored at <projectDir>/.ccbootstrap/analysis.json.
package cache

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/abdessama-cto/ccb/internal/analyzer"
	"github.com/abdessama-cto/ccb/internal/llm"
)

const (
	cacheDir      = ".ccbootstrap"
	cacheFilename = "analysis.json"
	currentVersion = "2"
)

// Analysis is the full persisted analysis for a project.
type Analysis struct {
	Version     string                      `json:"version"`
	CreatedAt   time.Time                   `json:"created_at"`
	Provider    string                      `json:"provider"`
	Model       string                      `json:"model"`
	FilesCount  int                         `json:"files_count"`
	TokensEst   int                         `json:"tokens_estimated"`
	Fingerprint *analyzer.ProjectFingerprint `json:"fingerprint"`
	Understanding *llm.ProjectUnderstanding  `json:"understanding"`
}

// Save persists the analysis to <projectDir>/.ccbootstrap/analysis.json
func Save(projectDir, provider, model string, filesCount, tokensEst int,
	fp *analyzer.ProjectFingerprint, u *llm.ProjectUnderstanding) error {

	dir := filepath.Join(projectDir, cacheDir)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("creating cache dir: %w", err)
	}

	a := &Analysis{
		Version:       currentVersion,
		CreatedAt:     time.Now(),
		Provider:      provider,
		Model:         model,
		FilesCount:    filesCount,
		TokensEst:     tokensEst,
		Fingerprint:   fp,
		Understanding: u,
	}

	data, err := json.MarshalIndent(a, "", "  ")
	if err != nil {
		return fmt.Errorf("marshalling cache: %w", err)
	}

	path := filepath.Join(dir, cacheFilename)
	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("writing cache: %w", err)
	}
	return nil
}

// Load reads the cached analysis from <projectDir>/.ccbootstrap/analysis.json.
// Returns nil, nil if no cache exists.
func Load(projectDir string) (*Analysis, error) {
	path := filepath.Join(projectDir, cacheDir, cacheFilename)
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("reading cache: %w", err)
	}

	var a Analysis
	if err := json.Unmarshal(data, &a); err != nil {
		return nil, fmt.Errorf("parsing cache: %w", err)
	}

	// Reject stale format versions
	if a.Version != currentVersion {
		return nil, nil
	}
	return &a, nil
}

// Age returns a human-readable age string for the cache entry.
func (a *Analysis) Age() string {
	d := time.Since(a.CreatedAt)
	switch {
	case d < time.Minute:
		return "just now"
	case d < time.Hour:
		return fmt.Sprintf("%dm ago", int(d.Minutes()))
	case d < 24*time.Hour:
		return fmt.Sprintf("%dh ago", int(d.Hours()))
	default:
		return fmt.Sprintf("%dd ago", int(d.Hours()/24))
	}
}

// Summary returns a one-line summary for display.
func (a *Analysis) Summary() string {
	return fmt.Sprintf("%s/%s · %d files · %s",
		a.Provider, a.Model, a.FilesCount, a.Age())
}

// CachePath returns the path to the cache file for a given project directory.
func CachePath(projectDir string) string {
	return filepath.Join(projectDir, cacheDir, cacheFilename)
}
