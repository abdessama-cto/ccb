package cmd

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/abdessama-cto/ccb/internal/skills"
)

// ─── Styles ───────────────────────────────────────────────────────────────────

var (
	styleDim       = lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
	styleTitle2    = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("212"))
	styleSub       = lipgloss.NewStyle().Foreground(lipgloss.Color("244"))
	styleLabel     = lipgloss.NewStyle().Foreground(lipgloss.Color("39"))
	styleDetail    = lipgloss.NewStyle().Foreground(lipgloss.Color("243"))
	styleCursor    = lipgloss.NewStyle().Foreground(lipgloss.Color("212")).Bold(true)
	styleChecked   = lipgloss.NewStyle().Foreground(lipgloss.Color("120"))
	styleUncheck   = lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
	styleCount     = lipgloss.NewStyle().Foreground(lipgloss.Color("212")).Bold(true)
	styleHint      = lipgloss.NewStyle().Foreground(lipgloss.Color("238"))
	styleSepLine   = lipgloss.NewStyle().Foreground(lipgloss.Color("62"))
	styleNewSkill  = lipgloss.NewStyle().Foreground(lipgloss.Color("220")).Bold(true)
	stylePreview   = lipgloss.NewStyle().
			Foreground(lipgloss.Color("251")).
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("62")).
			Padding(0, 1).
			MarginTop(1)

	styleSearchBox = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("212")).
			Padding(0, 1)

	styleRowActive = lipgloss.NewStyle().Background(lipgloss.Color("235"))
)

// ─── CheckItem ────────────────────────────────────────────────────────────────

// CheckItem represents a selectable item in the interactive checkbox list.
type CheckItem struct {
	Label    string
	Detail   string
	Selected bool
}

// ─── Focus area ───────────────────────────────────────────────────────────────

type focusArea int

const (
	focusMain   focusArea = iota // navigating the main (AI proposed) list
	focusSearch                  // navigating search results from disk
)

// ─── Bubbletea Model ──────────────────────────────────────────────────────────

type checkModel struct {
	title      string
	subtitle   string
	items      []CheckItem  // AI-proposed items
	cursor     int          // cursor in current focus area
	offset     int          // scroll offset for current focus
	maxVisible int
	searchable bool
	searching  bool
	search     textinput.Model
	focus      focusArea

	// Disk skill index (loaded once when / first pressed)
	diskLoaded  bool
	diskIndex   []skills.DiskSkill // all skills from disk
	diskResults []skills.DiskSkill // filtered results from disk

	// Preview pane: shows description of focused disk skill
	previewText string

	confirmed bool
	width     int
}

func newCheckModel(title, subtitle string, items []CheckItem, searchable bool) checkModel {
	ti := textinput.New()
	ti.Placeholder = "type to filter…"
	ti.CharLimit = 60
	ti.Width = 42
	ti.PromptStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("212"))
	ti.TextStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("15"))

	return checkModel{
		title:      title,
		subtitle:   subtitle,
		items:      items,
		cursor:     0,
		offset:     0,
		maxVisible: 10,
		searchable: searchable,
		search:     ti,
		width:      80,
		focus:      focusMain,
	}
}

// visibleMain returns indices into m.items that match the current query
func (m checkModel) visibleMain() []int {
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

// alreadyInList returns true if a disk skill name is already in items
func (m checkModel) alreadyInList(folderName string) bool {
	for _, it := range m.items {
		if strings.EqualFold(it.Label, folderName) ||
			strings.EqualFold(strings.ReplaceAll(it.Label, " ", "-"), folderName) {
			return true
		}
	}
	return false
}

func (m checkModel) Init() tea.Cmd { return nil }

func (m checkModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {

	case tea.WindowSizeMsg:
		m.width = msg.Width
		rows := msg.Height - 14
		if rows < 4 {
			rows = 4
		}
		if rows > 18 {
			rows = 18
		}
		m.maxVisible = rows

	case tea.KeyMsg:
		// ── Search text input handling ──────────────────────────────────────
		if m.searching {
			switch msg.String() {
			case "esc":
				m.searching = false
				m.search.SetValue("")
				m.search.Blur()
				m.diskResults = nil
				m.previewText = ""
				m.focus = focusMain
				m.cursor = 0
				m.offset = 0
			case "enter":
				m.searching = false
				m.search.Blur()
				if len(m.diskResults) > 0 {
					m.focus = focusSearch
					m.cursor = 0
					m.offset = 0
					m.updatePreview()
				}
			case "tab":
				// switch between main list and search results
				if m.focus == focusMain {
					if len(m.diskResults) > 0 {
						m.focus = focusSearch
						m.cursor = 0
					}
				} else {
					m.focus = focusMain
					m.cursor = 0
				}
			default:
				var cmd tea.Cmd
				m.search, cmd = m.search.Update(msg)
				// Re-filter disk results live
				q := m.search.Value()
				if q == "" {
					m.diskResults = nil
					m.previewText = ""
				} else {
					if !m.diskLoaded {
						m.diskIndex = skills.LoadSkills()
						m.diskLoaded = true
					}
					// Filter out skills already in proposals
					all := skills.Search(m.diskIndex, q)
					m.diskResults = nil
					for _, ds := range all {
						m.diskResults = append(m.diskResults, ds)
					}
				}
				m.cursor = 0
				m.offset = 0
				m.updatePreview()
				return m, cmd
			}
			m.updatePreview()
			return m, nil
		}

		// ── Normal navigation ───────────────────────────────────────────────
		switch msg.String() {
		case "ctrl+c":
			m.confirmed = true
			return m, tea.Quit

		case "enter":
			m.confirmed = true
			return m, tea.Quit

		case "esc":
			if m.focus == focusSearch || len(m.diskResults) > 0 {
				m.focus = focusMain
				m.diskResults = nil
				m.search.SetValue("")
				m.previewText = ""
				m.cursor = 0
			} else {
				m.confirmed = true
				return m, tea.Quit
			}

		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
				if m.cursor < m.offset {
					m.offset = m.cursor
				}
				m.updatePreview()
			}

		case "down", "j":
			limit := m.currentLimit()
			if m.cursor < limit-1 {
				m.cursor++
				if m.cursor >= m.offset+m.maxVisible {
					m.offset = m.cursor - m.maxVisible + 1
				}
				m.updatePreview()
			}

		case "tab":
			// switch focus between lists
			if m.searchable && len(m.diskResults) > 0 {
				if m.focus == focusMain {
					m.focus = focusSearch
				} else {
					m.focus = focusMain
				}
				m.cursor = 0
				m.offset = 0
				m.updatePreview()
			}

		case " ":
			if m.focus == focusMain {
				vis := m.visibleMain()
				if m.cursor < len(vis) {
					i := vis[m.cursor]
					m.items[i].Selected = !m.items[i].Selected
				}
			} else if m.focus == focusSearch {
				// Add disk skill to the list and select it
				if m.cursor < len(m.diskResults) {
					ds := m.diskResults[m.cursor]
					if !m.alreadyInList(ds.FolderName) {
						m.items = append(m.items, CheckItem{
							Label:    ds.FolderName,
							Detail:   ds.Description,
							Selected: true,
						})
					} else {
						// Toggle existing
						for i, it := range m.items {
							if strings.EqualFold(it.Label, ds.FolderName) {
								m.items[i].Selected = !m.items[i].Selected
								break
							}
						}
					}
				}
			}

		case "a":
			if m.focus == focusMain {
				for i := range m.items {
					m.items[i].Selected = true
				}
			}

		case "n":
			if m.focus == focusMain {
				for i := range m.items {
					m.items[i].Selected = false
				}
			}

		case "/":
			if m.searchable {
				m.searching = true
				m.search.Focus()
				if !m.diskLoaded {
					m.diskIndex = skills.ScanDiskSkills("")
					m.diskLoaded = true
				}
			}
		}
	}
	return m, nil
}

func (m checkModel) currentLimit() int {
	if m.focus == focusSearch {
		return len(m.diskResults)
	}
	return len(m.visibleMain())
}

func (m *checkModel) updatePreview() {
	if m.focus == focusSearch && m.cursor < len(m.diskResults) {
		ds := m.diskResults[m.cursor]
		m.previewText = ds.Description
		if m.previewText == "" {
			m.previewText = ds.Preview
		}
	} else {
		m.previewText = ""
	}
}

// ─── View ─────────────────────────────────────────────────────────────────────

func (m checkModel) View() string {
	if m.confirmed {
		return ""
	}

	var sb strings.Builder
	sb.WriteString("\n")

	// Title
	sb.WriteString(styleTitle2.Render("  " + m.title))
	sb.WriteString("\n")
	if m.subtitle != "" {
		sb.WriteString(styleSub.Render("  " + m.subtitle))
		sb.WriteString("\n")
	}
	sb.WriteString("\n")

	// Keybind bar
	var hints []string
	if m.focus == focusSearch {
		hints = []string{"↑/↓ navigate results", "SPACE add", "TAB switch list", "ENTER confirm", "ESC back"}
	} else {
		hints = []string{"↑/↓ navigate", "SPACE toggle", "ENTER confirm", "a/n all/none"}
		if m.searchable {
			hints = append(hints, "/ search disk")
		}
	}
	sb.WriteString(styleHint.Render("  " + strings.Join(hints, "  ·  ")))
	sb.WriteString("\n\n")

	// ── Main (AI proposed) list ────────────────────────────────────────────
	vis := m.visibleMain()
	end := m.offset + m.maxVisible
	if end > len(vis) {
		end = len(vis)
	}

	if len(vis) == 0 && m.search.Value() != "" && m.focus == focusMain {
		sb.WriteString(styleDim.Render("  (no AI proposals match filter)"))
		sb.WriteString("\n")
	}

	mainOffset := 0
	mainEnd := end
	mainMaxVis := m.maxVisible

	// When disk results are shown, shrink main list
	if len(m.diskResults) > 0 {
		mainMaxVis = 5
		if mainEnd > mainOffset+mainMaxVis {
			mainEnd = mainOffset + mainMaxVis
		}
	}

	detailWidth := m.width - 14
	if detailWidth < 30 {
		detailWidth = 30
	}
	indent := "        " // 8 spaces — aligns under label

	for pos := 0; pos < len(vis) && pos < mainMaxVis; pos++ {
		i := vis[pos]
		it := m.items[i]
		isCursor := m.focus == focusMain && pos == m.cursor

		box := styleChecked.Render("☑")
		if !it.Selected {
			box = styleUncheck.Render("☐")
		}
		label := styleLabel.Render(it.Label)
		detail := styleDetail.Width(detailWidth).Render(it.Detail)

		var row string
		if isCursor {
			arrow := styleCursor.Render("▶")
			row = fmt.Sprintf("  %s %s  %s\n%s%s", arrow, box, label, indent, detail)
			sb.WriteString(styleRowActive.Render(row))
		} else {
			row = fmt.Sprintf("    %s  %s\n%s%s", box, label, indent, detail)
			sb.WriteString(row)
		}
		sb.WriteString("\n")
	}
	if len(vis) > mainMaxVis {
		sb.WriteString(styleDim.Render(fmt.Sprintf("  … %d more in list\n", len(vis)-mainMaxVis)))
	}

	// ── Search bar ─────────────────────────────────────────────────────────
	sb.WriteString("\n")
	if m.searching {
		sb.WriteString(styleSearchBox.Render("  / " + m.search.View()))
		sb.WriteString("\n")
	} else if m.searchable {
		q := m.search.Value()
		if q != "" {
			sb.WriteString(styleDim.Render(fmt.Sprintf("  Filter: %s  (TAB to switch focus · ESC to clear)", q)))
		} else {
			sb.WriteString(styleHint.Render("  Press  /  to search 100+ skills from disk"))
		}
		sb.WriteString("\n")
	}

	// ── Disk search results ─────────────────────────────────────────────────
	if len(m.diskResults) > 0 {
		sep := styleSepLine.Render(strings.Repeat("─", 60))
		sb.WriteString(sep + "\n")
		label := "  🔍 Skills from disk"
		if m.focus == focusSearch {
			label = styleTitle2.Render("  🔍 Skills from disk") + styleDim.Render(" ← focus here")
		} else {
			label = styleDim.Render(label) + styleHint.Render("  (TAB to navigate)")
		}
		sb.WriteString(label + "\n\n")

		diskEnd := m.offset + m.maxVisible
		if m.focus != focusSearch {
			diskEnd = 5 // preview only
		}
		if diskEnd > len(m.diskResults) {
			diskEnd = len(m.diskResults)
		}
		diskStart := 0
		if m.focus == focusSearch {
			diskStart = m.offset
		}

		diskIndent := "        "
		for pos := diskStart; pos < diskEnd; pos++ {
			ds := m.diskResults[pos]
			isCursor := m.focus == focusSearch && pos == m.cursor

			inList := m.alreadyInList(ds.FolderName)
			box := styleUncheck.Render("☐")
			if inList {
				box = styleChecked.Render("☑")
			}

			name := styleNewSkill.Render(ds.FolderName)
			desc := styleDetail.Width(detailWidth).Render(ds.Description)

			var row string
			if isCursor {
				arrow := styleCursor.Render("▶")
				row = fmt.Sprintf("  %s %s  %s\n%s%s", arrow, box, name, diskIndent, desc)
				sb.WriteString(styleRowActive.Render(row))
			} else {
				row = fmt.Sprintf("    %s  %s\n%s%s", box, name, diskIndent, desc)
				sb.WriteString(row)
			}
			sb.WriteString("\n")
		}

		if m.focus == focusSearch && len(m.diskResults) > m.maxVisible {
			sb.WriteString(styleDim.Render(fmt.Sprintf(
				"\n  %d–%d of %d results\n", diskStart+1, diskEnd, len(m.diskResults))))
		}

		// Preview pane
		if m.previewText != "" {
			wrapped := tuiWordWrap(m.previewText, m.width-12)
			sb.WriteString(stylePreview.Render("  " + wrapped))
			sb.WriteString("\n")
		}
	}

	// ── Footer ─────────────────────────────────────────────────────────────
	selected := 0
	for _, it := range m.items {
		if it.Selected {
			selected++
		}
	}
	sb.WriteString(styleCount.Render(fmt.Sprintf("\n  ● %d / %d selected\n",
		selected, len(m.items))))

	return sb.String()
}

// ─── Public API ───────────────────────────────────────────────────────────────

// InteractiveCheckbox shows a Bubbletea-powered checkbox selector.
// When searchable=true, / searches real SKILL.md files from disk.
func InteractiveCheckbox(title, subtitle string, items []CheckItem, searchable bool) []CheckItem {
	m := newCheckModel(title, subtitle, items, searchable)

	p := tea.NewProgram(m, tea.WithAltScreen())
	finalModel, err := p.Run()
	if err != nil {
		return fallbackCheckbox(title, subtitle, items)
	}

	if fm, ok := finalModel.(checkModel); ok {
		return fm.items
	}
	return items
}

// ─── Helpers ──────────────────────────────────────────────────────────────────

// tuiWordWrap wraps text at maxWidth characters, for use in the TUI preview pane.
func tuiWordWrap(text string, maxWidth int) string {
	if maxWidth <= 0 || len(text) <= maxWidth {
		return text
	}
	words := strings.Fields(text)
	var lines []string
	var current string
	for _, w := range words {
		if len(current)+len(w)+1 > maxWidth {
			if current != "" {
				lines = append(lines, current)
			}
			current = w
		} else {
			if current == "" {
				current = w
			} else {
				current += " " + w
			}
		}
	}
	if current != "" {
		lines = append(lines, current)
	}
	return strings.Join(lines, "\n  ")
}

// ─── Fallback (CI / pipes / non-TTY) ─────────────────────────────────────────

func fallbackCheckbox(title, subtitle string, items []CheckItem) []CheckItem {
	fmt.Printf("\n  %s\n", styleTitle2.Render(title))
	if subtitle != "" {
		fmt.Printf("  %s\n\n", styleSub.Render(subtitle))
	}
	for i, it := range items {
		check := styleChecked.Render("[x]")
		if !it.Selected {
			check = styleUncheck.Render("[ ]")
		}
		fmt.Printf("  %s %2d. %-32s %s\n", check, i+1,
			styleLabel.Render(it.Label), styleDetail.Render(it.Detail))
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
