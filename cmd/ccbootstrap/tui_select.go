package cmd

import (
	"fmt"
	"os"
	"strings"

	"golang.org/x/sys/unix"

	"github.com/abdessama-cto/ccb/internal/tui"
)

// ─── Interactive checkbox selector ────────────────────────────────────────────
// Navigate with ↑/↓, toggle with SPACE, confirm with ENTER, search with /

// CheckItem represents a selectable item in the interactive list
type CheckItem struct {
	Label    string
	Detail   string
	Selected bool
}

// InteractiveCheckbox renders a full-screen interactive checkbox list.
// Returns the updated items with Selected fields modified.
func InteractiveCheckbox(title, subtitle string, items []CheckItem, searchable bool) []CheckItem {
	if !isTerminal() {
		// Fallback to number-based selection if not a TTY
		return fallbackCheckbox(title, subtitle, items)
	}

	term := &rawTerm{}
	if err := term.enable(); err != nil {
		return fallbackCheckbox(title, subtitle, items)
	}
	defer term.restore()

	// Make a working copy
	result := make([]CheckItem, len(items))
	copy(result, items)

	cursor := 0
	searchMode := false
	searchQuery := ""
	filtered := result // current visible items (indices into result)
	filteredIdx := make([]int, len(result))
	for i := range filteredIdx {
		filteredIdx[i] = i
	}
	offset := 0 // scroll offset
	maxVisible := 12

	redraw := func() {
		// Recompute filtered list
		if searchQuery != "" {
			filteredIdx = nil
			q := strings.ToLower(searchQuery)
			for i, it := range result {
				if strings.Contains(strings.ToLower(it.Label), q) ||
					strings.Contains(strings.ToLower(it.Detail), q) {
					filteredIdx = append(filteredIdx, i)
				}
			}
		} else {
			filteredIdx = make([]int, len(result))
			for i := range filteredIdx {
				filteredIdx[i] = i
			}
		}
		if cursor >= len(filteredIdx) {
			cursor = len(filteredIdx) - 1
		}
		if cursor < 0 {
			cursor = 0
		}

		// Scroll window
		if cursor < offset {
			offset = cursor
		}
		if cursor >= offset+maxVisible {
			offset = cursor - maxVisible + 1
		}

		clearScreen()

		// Header
		fmt.Printf("  %s\n", tui.Bold(title))
		if subtitle != "" {
			fmt.Printf("  %s\n", tui.Dim(subtitle))
		}
		fmt.Printf("  %s\n\n",
			tui.Dim("↑↓ navigate  SPACE toggle  ENTER confirm"+func() string {
				if searchable {
					return "  / search"
				}
				return ""
			}()))

		// Items
		end := offset + maxVisible
		if end > len(filteredIdx) {
			end = len(filteredIdx)
		}

		if len(filteredIdx) == 0 {
			fmt.Printf("  %s\n", tui.Dim("No results for: "+searchQuery))
		}

		for pos := offset; pos < end; pos++ {
			i := filteredIdx[pos]
			it := result[i]

			isCursor := pos == cursor
			check := tui.Green("[x]")
			if !it.Selected {
				check = tui.Dim("[ ]")
			}

			label := tui.Cyan(fmt.Sprintf("%-32s", it.Label))
			detail := tui.Dim(it.Detail)

			if isCursor {
				arrow := tui.Green("▶")
				fmt.Printf("  %s %s %s %s\n", arrow, check, label, detail)
			} else {
				fmt.Printf("    %s %s %s\n", check, label, detail)
			}
		}

		// Scroll indicator
		if len(filteredIdx) > maxVisible {
			shown := end - offset
			fmt.Printf("\n  %s\n", tui.Dim(fmt.Sprintf("  Showing %d-%d of %d items (scroll with ↑↓)", offset+1, offset+shown, len(filteredIdx))))
		}

		// Search bar
		if searchMode {
			fmt.Printf("\n  %s %s%s\n", tui.Bold("Search:"), tui.Cyan(searchQuery), tui.Green("█"))
		}

		// Footer
		selected := 0
		for _, it := range result {
			if it.Selected {
				selected++
			}
		}
		fmt.Printf("\n  %s %d/%d selected\n",
			tui.Bold("●"), selected, len(result))
	}

	redraw()
	_ = filtered

	for {
		b, seq := readKey()

		if searchMode {
			switch {
			case seq == "ESC" || b == 27:
				searchMode = false
				searchQuery = ""
			case seq == "ENTER" || b == 13 || b == 10:
				searchMode = false
			case b == 127 || b == 8: // backspace
				if len(searchQuery) > 0 {
					searchQuery = searchQuery[:len(searchQuery)-1]
				}
			case b >= 32 && b < 127: // printable
				searchQuery += string(rune(b))
				cursor = 0
				offset = 0
			}
		} else {
			switch {
			case seq == "UP":
				if cursor > 0 {
					cursor--
				}
			case seq == "DOWN":
				if cursor < len(filteredIdx)-1 {
					cursor++
				}
			case b == ' ': // space = toggle
				if cursor < len(filteredIdx) {
					i := filteredIdx[cursor]
					result[i].Selected = !result[i].Selected
				}
			case seq == "ENTER" || b == 13 || b == 10: // enter = confirm
				return result
			case b == 'a' || b == 'A': // select all
				for i := range result {
					result[i].Selected = true
				}
			case b == 'n' || b == 'N': // select none
				for i := range result {
					result[i].Selected = false
				}
			case b == '/' && searchable: // search
				searchMode = true
				searchQuery = ""
			case seq == "ESC":
				// close search if open, otherwise return
				return result
			}
		}
		redraw()
	}
}

// ─── Terminal raw mode ────────────────────────────────────────────────────────

type rawTerm struct {
	origState unix.Termios
}

func (t *rawTerm) enable() error {
	fd := int(os.Stdin.Fd())
	orig, err := unix.IoctlGetTermios(fd, unix.TIOCGETA)
	if err != nil {
		return err
	}
	t.origState = *orig

	raw := *orig
	// Disable canonical mode and echo
	raw.Lflag &^= unix.ECHO | unix.ECHOE | unix.ECHOK | unix.ECHONL
	raw.Lflag &^= unix.ICANON
	raw.Iflag &^= unix.ICRNL | unix.INPCK | unix.ISTRIP
	raw.Cflag |= unix.CS8
	raw.Oflag &^= unix.OPOST
	raw.Cc[unix.VMIN] = 1
	raw.Cc[unix.VTIME] = 0

	return unix.IoctlSetTermios(fd, unix.TIOCSETA, &raw)
}

func (t *rawTerm) restore() {
	_ = unix.IoctlSetTermios(int(os.Stdin.Fd()), unix.TIOCSETA, &t.origState)
}

// readKey reads one keypress and returns the byte + a semantic name
func readKey() (byte, string) {
	buf := make([]byte, 4)
	n, _ := os.Stdin.Read(buf)
	if n == 0 {
		return 0, ""
	}
	b := buf[0]

	// Escape sequences
	if b == 27 && n > 1 {
		if n >= 3 && buf[1] == '[' {
			switch buf[2] {
			case 'A':
				return 0, "UP"
			case 'B':
				return 0, "DOWN"
			case 'C':
				return 0, "RIGHT"
			case 'D':
				return 0, "LEFT"
			}
		}
		return 27, "ESC"
	}
	if b == 27 {
		return 27, "ESC"
	}
	if b == 13 {
		return b, "ENTER"
	}
	return b, ""
}

func clearScreen() {
	// Move cursor to top-left and clear from cursor to end of screen
	fmt.Print("\033[H\033[J")
}

func isTerminal() bool {
	fd := int(os.Stdin.Fd())
	_, err := unix.IoctlGetTermios(fd, unix.TIOCGETA)
	return err == nil
}

// ─── Fallback (non-TTY, pipes, CI) ───────────────────────────────────────────

func fallbackCheckbox(title, subtitle string, items []CheckItem) []CheckItem {
	fmt.Printf("\n  %s\n", tui.Bold(title))
	if subtitle != "" {
		fmt.Printf("  %s\n\n", tui.Dim(subtitle))
	}
	for i, it := range items {
		check := tui.Green("[x]")
		if !it.Selected {
			check = tui.Dim("[ ]")
		}
		fmt.Printf("  %s %2d. %-32s %s\n", check, i+1, tui.Cyan(it.Label), tui.Dim(it.Detail))
	}
	fmt.Print("\n  Toggle (e.g. \"3 5\") or enter: ")

	var line string
	fmt.Scanln(&line)
	for _, tok := range strings.Fields(line) {
		n := 0
		fmt.Sscanf(tok, "%d", &n)
		if n >= 1 && n <= len(items) {
			items[n-1].Selected = !items[n-1].Selected
		}
	}
	return items
}
