package tui

import (
	"fmt"
	"strings"

	"github.com/fatih/color"
)

var (
	Green  = color.New(color.FgGreen).SprintFunc()
	Yellow = color.New(color.FgYellow).SprintFunc()
	Cyan   = color.New(color.FgCyan).SprintFunc()
	Red    = color.New(color.FgRed).SprintFunc()
	Bold   = color.New(color.Bold).SprintFunc()
	Dim    = color.New(color.Faint).SprintFunc()
)

func Info(msg string)    { fmt.Printf("%s %s\n", Cyan("▸"), msg) }
func Success(msg string) { fmt.Printf("%s %s\n", Green("✓"), msg) }
func Warn(msg string)    { fmt.Fprintf(color.Error, "%s  %s\n", Yellow("⚠"), msg) }
func Err(msg string)     { fmt.Fprintf(color.Error, "%s %s\n", Red("✗"), msg) }

func Banner(version string) {
	fmt.Printf(`
   %s %s
   %s

`,
		Green("🌱 ccbootstrap"),
		Dim("— Claude Code Project Bootstrapper"),
		Dim(fmt.Sprintf("v%s · macOS Apple Silicon", version)),
	)
}

func Box(title string, lines []string) {
	width := len(title) + 4
	for _, l := range lines {
		if len(l)+4 > width {
			width = len(l) + 4
		}
	}
	border := strings.Repeat("─", width)
	fmt.Printf("\n%s %s\n%s\n", Bold("⚙️"), Bold(title), border)
	for _, l := range lines {
		fmt.Printf("  %s\n", l)
	}
	fmt.Println(border)
}
