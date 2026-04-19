package cmd

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/abdessama-cto/ccb/internal/config"
	"github.com/abdessama-cto/ccb/internal/generator"
	"github.com/abdessama-cto/ccb/internal/tui"
	"github.com/spf13/cobra"
)

var uninstallFlags struct {
	yes          bool
	keepBinary   bool
	keepGlobal   bool
	keepProject  bool
}

var uninstallCmd = &cobra.Command{
	Use:   "uninstall",
	Short: "Remove ccb from this system and clean up the current project",
	Long: `Remove every trace of ccb:

  1. Files ccb generated in the current project (from the manifest)
  2. The per-project cache (.ccbootstrap/ inside the project)
  3. The global config (~/.ccbootstrap/)
  4. The ccb binary itself (the currently-running executable)

Every destructive step asks for confirmation unless --yes is passed.`,
	RunE: runUninstall,
}

func init() {
	rootCmd.AddCommand(uninstallCmd)
	uninstallCmd.Flags().BoolVarP(&uninstallFlags.yes, "yes", "y", false, "Skip all confirmations")
	uninstallCmd.Flags().BoolVar(&uninstallFlags.keepBinary, "keep-binary", false, "Do not delete the ccb binary")
	uninstallCmd.Flags().BoolVar(&uninstallFlags.keepGlobal, "keep-global", false, "Do not delete ~/.ccbootstrap/")
	uninstallCmd.Flags().BoolVar(&uninstallFlags.keepProject, "keep-project", false, "Do not touch files in the current project")
}

func runUninstall(cmd *cobra.Command, args []string) error {
	tui.Banner(Version)

	wd, _ := os.Getwd()

	fmt.Println("This will remove:")
	if !uninstallFlags.keepProject {
		fmt.Printf("  %s  files ccb generated in %s (from manifest)\n", tui.Bold("•"), tui.Cyan(wd))
		fmt.Printf("  %s  %s/.ccbootstrap/ (per-project cache)\n", tui.Bold("•"), tui.Cyan(wd))
	}
	if !uninstallFlags.keepGlobal {
		home, _ := os.UserHomeDir()
		fmt.Printf("  %s  %s (global config)\n", tui.Bold("•"), tui.Cyan(filepath.Join(home, ".ccbootstrap")))
	}
	if !uninstallFlags.keepBinary {
		exe, _ := os.Executable()
		fmt.Printf("  %s  %s (binary)\n", tui.Bold("•"), tui.Cyan(exe))
	}
	fmt.Println()

	if !uninstallFlags.yes && !confirm("Proceed? [y/N]: ") {
		tui.Info("Aborted. Nothing was removed.")
		return nil
	}

	// ── Step 1: per-project cleanup ──────────────────────────────────────────
	if !uninstallFlags.keepProject {
		if err := cleanProject(wd); err != nil {
			tui.Warn(fmt.Sprintf("Project cleanup had errors: %s", err.Error()))
		}
	}

	// ── Step 2: global config ────────────────────────────────────────────────
	if !uninstallFlags.keepGlobal {
		if err := os.RemoveAll(config.ConfigDir); err != nil {
			tui.Warn(fmt.Sprintf("Could not remove %s: %s", config.ConfigDir, err.Error()))
		} else {
			tui.Success(fmt.Sprintf("Removed global config: %s", config.ConfigDir))
		}
	}

	// ── Step 3: binary ───────────────────────────────────────────────────────
	if !uninstallFlags.keepBinary {
		if err := removeBinary(); err != nil {
			tui.Warn(err.Error())
		}
	}

	fmt.Println()
	tui.Success("Uninstall complete. Thank you for using ccb 👋")
	fmt.Println(tui.Dim("  (If your shell still resolves 'ccb', run: hash -r)"))
	return nil
}

// cleanProject removes files ccb tracked in the manifest and deletes empty
// dirs ccb created. If no manifest exists, it deletes the .claude/ tree as
// a best-effort fallback.
func cleanProject(projectDir string) error {
	m, err := generator.LoadManifest(projectDir)
	if err != nil {
		return err
	}

	if m == nil {
		tui.Warn("No manifest found — falling back to broad cleanup of .claude/")
		return fallbackCleanProject(projectDir)
	}

	tui.Info(fmt.Sprintf("Removing %d file(s) from the manifest...", len(m.Files)))
	removed := 0
	for _, rel := range m.Files {
		full := filepath.Join(projectDir, rel)
		if err := os.Remove(full); err == nil {
			fmt.Printf("  %s %s\n", tui.Green("✓"), rel)
			removed++
		}
	}
	tui.Success(fmt.Sprintf("%d file(s) removed", removed))

	// Remove directories ccb created, deepest-first, but only if empty.
	dirs := append([]string{}, m.Dirs...)
	sort.Slice(dirs, func(i, j int) bool { return len(dirs[i]) > len(dirs[j]) })
	for _, rel := range dirs {
		full := filepath.Join(projectDir, rel)
		_ = os.Remove(full) // fails if non-empty, which is desired
	}

	// Per-project cache
	_ = os.RemoveAll(filepath.Join(projectDir, ".ccbootstrap"))
	tui.Success(fmt.Sprintf("Removed %s/.ccbootstrap/", projectDir))

	if m.BackupFrom != "" {
		fmt.Println()
		rel, _ := filepath.Rel(projectDir, m.BackupFrom)
		tui.Warn(fmt.Sprintf("Your original .claude/ is still at %s — restore with:", tui.Bold(rel)))
		fmt.Printf("  mv %s .claude\n", rel)
	}
	return nil
}

// fallbackCleanProject is used when the project was bootstrapped before the
// manifest feature existed. It removes the whole .claude/ directory and the
// files ccb historically creates, after warning the user.
func fallbackCleanProject(projectDir string) error {
	targets := []string{
		".claude",
		".ccbootstrap",
		"CLAUDE.md",
		"docs/architecture.md",
		"docs/progress.md",
	}
	for _, rel := range targets {
		full := filepath.Join(projectDir, rel)
		if _, err := os.Stat(full); os.IsNotExist(err) {
			continue
		}
		if err := os.RemoveAll(full); err != nil {
			tui.Warn(fmt.Sprintf("Could not remove %s: %s", rel, err.Error()))
			continue
		}
		fmt.Printf("  %s %s\n", tui.Green("✓"), rel)
	}
	return nil
}

func removeBinary() error {
	exe, err := os.Executable()
	if err != nil {
		return fmt.Errorf("could not determine binary path: %w", err)
	}
	// Resolve symlinks so we remove the real file, not a dangling symlink.
	resolved, err := filepath.EvalSymlinks(exe)
	if err == nil {
		exe = resolved
	}

	if err := os.Remove(exe); err != nil {
		return fmt.Errorf("could not remove binary %s: %w", exe, err)
	}
	tui.Success(fmt.Sprintf("Removed binary: %s", exe))

	// Also clean up a legacy ccbootstrap binary if it lives next to us.
	legacy := filepath.Join(filepath.Dir(exe), "ccbootstrap")
	if _, statErr := os.Stat(legacy); statErr == nil {
		_ = os.Remove(legacy)
		tui.Success(fmt.Sprintf("Removed legacy binary: %s", legacy))
	}
	return nil
}

func confirm(prompt string) bool {
	fmt.Print(prompt)
	r := bufio.NewReader(os.Stdin)
	line, _ := r.ReadString('\n')
	line = strings.ToLower(strings.TrimSpace(line))
	return line == "y" || line == "yes"
}
