package cmd

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// ─── Styles ───────────────────────────────────────────────────────────────────

var (
	styleDim     = lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
	styleTitle   = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("15"))
	styleSub     = lipgloss.NewStyle().Foreground(lipgloss.Color("244"))
	styleLabel   = lipgloss.NewStyle().Foreground(lipgloss.Color("39"))
	styleDetail  = lipgloss.NewStyle().Foreground(lipgloss.Color("243"))
	styleCursor  = lipgloss.NewStyle().Foreground(lipgloss.Color("212")).Bold(true)
	styleChecked = lipgloss.NewStyle().Foreground(lipgloss.Color("120"))
	styleUncheck = lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
	styleCount   = lipgloss.NewStyle().Foreground(lipgloss.Color("212")).Bold(true)
	styleHint    = lipgloss.NewStyle().Foreground(lipgloss.Color("238"))

	styleBorder = lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("62")).
		Padding(0, 1)

	styleSearchBox = lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("212")).
		Padding(0, 1)

	styleTitle2 = lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("212")).
		MarginBottom(0)
)

// ─── CheckItem ────────────────────────────────────────────────────────────────

// CheckItem represents a selectable item in the interactive list
type CheckItem struct {
	Label    string
	Detail   string
	Selected bool
}

// ─── Bubbletea Model ──────────────────────────────────────────────────────────

type checkModel struct {
	title      string
	subtitle   string
	items      []CheckItem
	cursor     int
	offset     int   // scroll offset
	maxVisible int   // rows visible at once
	searchable bool
	searching  bool
	search     textinput.Model
	confirmed  bool
	width      int
}

func newCheckModel(title, subtitle string, items []CheckItem, searchable bool) checkModel {
	ti := textinput.New()
	ti.Placeholder = "type to filter…"
	ti.CharLimit = 50
	ti.Width = 40
	ti.PromptStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("212"))
	ti.TextStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("15"))

	return checkModel{
		title:      title,
		subtitle:   subtitle,
		items:      items,
		cursor:     0,
		offset:     0,
		maxVisible: 12,
		searchable: searchable,
		search:     ti,
		width:      80,
	}
}

// visible returns indices of items that match current search
func (m checkModel) visible() []int {
	q := strings.ToLower(m.search.Value())
	var out []int
	for i, it := range m.items {
		if q == "" || strings.Contains(strings.ToLower(it.Label), q) ||
			strings.Contains(strings.ToLower(it.Detail), q) {
			out = append(out, i)
		}
	}
	return out
}

func (m checkModel) Init() tea.Cmd { return nil }

func (m checkModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {

	case tea.WindowSizeMsg:
		m.width = msg.Width
		rows := msg.Height - 10
		if rows < 4 {
			rows = 4
		}
		if rows > 24 {
			rows = 24
		}
		m.maxVisible = rows

	case tea.KeyMsg:
		if m.searching {
			switch msg.String() {
			case "esc":
				m.searching = false
				m.search.SetValue("")
				m.search.Blur()
				m.cursor = 0
				m.offset = 0
			case "enter":
				m.searching = false
				m.search.Blur()
			default:
				var cmd tea.Cmd
				m.search, cmd = m.search.Update(msg)
				m.cursor = 0
				m.offset = 0
				return m, cmd
			}
			return m, nil
		}

		vis := m.visible()
		switch msg.String() {
		case "ctrl+c", "q":
			m.confirmed = true
			return m, tea.Quit

		case "enter":
			m.confirmed = true
			return m, tea.Quit

		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
				if m.cursor < m.offset {
					m.offset = m.cursor
				}
			}

		case "down", "j":
			if m.cursor < len(vis)-1 {
				m.cursor++
				if m.cursor >= m.offset+m.maxVisible {
					m.offset = m.cursor - m.maxVisible + 1
				}
			}

		case " ":
			if m.cursor < len(vis) {
				i := vis[m.cursor]
				m.items[i].Selected = !m.items[i].Selected
			}

		case "a":
			for i := range m.items {
				m.items[i].Selected = true
			}

		case "n":
			for i := range m.items {
				m.items[i].Selected = false
			}

		case "/":
			if m.searchable {
				m.searching = true
				m.search.Focus()
			}
		}
	}
	return m, nil
}

func (m checkModel) View() string {
	if m.confirmed {
		return ""
	}

	vis := m.visible()
	end := m.offset + m.maxVisible
	if end > len(vis) {
		end = len(vis)
	}

	var sb strings.Builder

	// ── Title ──────────────────────────────────────────────────────────────
	sb.WriteString("\n")
	sb.WriteString(styleTitle2.Render(m.title))
	sb.WriteString("\n")
	if m.subtitle != "" {
		sb.WriteString(styleSub.Render("  " + m.subtitle))
		sb.WriteString("\n")
	}
	sb.WriteString("\n")

	// ── Keybind hint ───────────────────────────────────────────────────────
	hints := []string{"↑/↓ navigate", "SPACE toggle", "ENTER confirm", "a/n all/none"}
	if m.searchable {
		hints = append(hints, "/ search")
	}
	sb.WriteString(styleHint.Render("  " + strings.Join(hints, "  ·  ")))
	sb.WriteString("\n\n")

	// ── Items ──────────────────────────────────────────────────────────────
	if len(vis) == 0 {
		sb.WriteString(styleDim.Render("  No results for: " + m.search.Value()))
		sb.WriteString("\n")
	}

	for pos := m.offset; pos < end; pos++ {
		i := vis[pos]
		it := m.items[i]
		isCursor := pos == m.cursor

		// Checkbox
		var box string
		if it.Selected {
			box = styleChecked.Render("☑")
		} else {
			box = styleUncheck.Render("☐")
		}

		// Label + detail
		label := styleLabel.Render(fmt.Sprintf("%-30s", it.Label))
		detail := styleDetail.Render(it.Detail)

		if isCursor {
			arrow := styleCursor.Render("▶")
			row := fmt.Sprintf("  %s %s  %s  %s", arrow, box, label, detail)
			sb.WriteString(lipgloss.NewStyle().
				Background(lipgloss.Color("235")).
				Render(row))
		} else {
			row := fmt.Sprintf("    %s  %s  %s", box, label, detail)
			sb.WriteString(row)
		}
		sb.WriteString("\n")
	}

	// ── Scroll indicator ───────────────────────────────────────────────────
	if len(vis) > m.maxVisible {
		sb.WriteString(styleDim.Render(fmt.Sprintf(
			"\n  %d–%d of %d  (scroll with ↑/↓)\n", m.offset+1, end, len(vis))))
	}

	// ── Search bar ─────────────────────────────────────────────────────────
	if m.searching {
		sb.WriteString("\n")
		sb.WriteString(styleSearchBox.Render("  / " + m.search.View()))
		sb.WriteString("\n")
	} else if m.searchable && !m.searching {
		sb.WriteString(styleHint.Render("\n  Press  /  to search"))
		sb.WriteString("\n")
	}

	// ── Footer: count ──────────────────────────────────────────────────────
	selected := 0
	for _, it := range m.items {
		if it.Selected {
			selected++
		}
	}
	footer := fmt.Sprintf("\n  %s  %d / %d selected",
		styleCount.Render("●"), selected, len(m.items))
	sb.WriteString(footer)
	sb.WriteString("\n")

	return sb.String()
}

// ─── Public API ───────────────────────────────────────────────────────────────

// InteractiveCheckbox shows a Bubbletea-powered checkbox selector.
// Returns items with Updated Selected fields.
func InteractiveCheckbox(title, subtitle string, items []CheckItem, searchable bool) []CheckItem {
	m := newCheckModel(title, subtitle, items, searchable)

	p := tea.NewProgram(m, tea.WithAltScreen())
	finalModel, err := p.Run()
	if err != nil {
		// Fallback if terminal not supported
		return fallbackCheckbox(title, subtitle, items)
	}

	if fm, ok := finalModel.(checkModel); ok {
		return fm.items
	}
	return items
}

// ─── Fallback (CI / pipes / non-TTY) ─────────────────────────────────────────

func fallbackCheckbox(title, subtitle string, items []CheckItem) []CheckItem {
	fmt.Printf("\n  %s\n", styleTitle.Render(title))
	if subtitle != "" {
		fmt.Printf("  %s\n\n", styleSub.Render(subtitle))
	}
	for i, it := range items {
		check := styleChecked.Render("[x]")
		if !it.Selected {
			check = styleUncheck.Render("[ ]")
		}
		fmt.Printf("  %s %2d. %-32s %s\n",
			check, i+1,
			styleLabel.Render(it.Label),
			styleDetail.Render(it.Detail))
	}
	fmt.Print("\n  Toggle (e.g. \"3 5\") or Enter: ")

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
