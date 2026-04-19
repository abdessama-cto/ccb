package generator

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"time"
)

// ManifestFilename is where ccb writes the list of every file it created in a
// bootstrapped project. `ccb uninstall` reads this file to know what to remove.
const ManifestFilename = ".ccb-manifest.json"

// Manifest records the files created by a ccb run so they can be removed later.
type Manifest struct {
	CreatedAt  time.Time `json:"created_at"`
	Version    string    `json:"version"`
	Files      []string  `json:"files"`      // project-relative paths
	Dirs       []string  `json:"dirs"`       // project-relative dirs ccb created (removed if empty)
	BackupFrom string    `json:"backup_from,omitempty"`
}

// ManifestPath returns the absolute path to the manifest file inside the
// project's .ccbootstrap/ cache directory.
func ManifestPath(projectDir string) string {
	return filepath.Join(projectDir, ".ccbootstrap", ManifestFilename)
}

// WriteManifest persists a manifest to disk.
func WriteManifest(projectDir string, m Manifest) error {
	path := ManifestPath(projectDir)
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	sort.Strings(m.Files)
	sort.Strings(m.Dirs)
	data, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}

// LoadManifest reads a previously-written manifest. Returns nil, nil if missing.
func LoadManifest(projectDir string) (*Manifest, error) {
	data, err := os.ReadFile(ManifestPath(projectDir))
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var m Manifest
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, err
	}
	return &m, nil
}
