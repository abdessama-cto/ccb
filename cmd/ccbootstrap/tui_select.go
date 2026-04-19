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
	styleDim      = lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
	styleTitle2   = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("212"))
	styleSub      = lipgloss.NewStyle().Foreground(lipgloss.Color("244"))
	styleLabel    = lipgloss.NewStyle().Foreground(lipgloss.Color("39"))
	styleDetail   = lipgloss.NewStyle().Foreground(lipgloss.Color("243"))
	styleCursor   = lipgloss.NewStyle().Foreground(lipgloss.Color("212")).Bold(true)
	styleChecked  = lipgloss.NewStyle().Foreground(lipgloss.Color("120"))
	styleUncheck  = lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
	styleCount    = lipgloss.NewStyle().Foreground(lipgloss.Color("212")).Bold(true)
	styleHint     = lipgloss.NewStyle().Foreground(lipgloss.Color("238"))
	styleSepLine  = lipgloss.NewStyle().Foreground(lipgloss.Color("62"))
	styleNewSkill = lipgloss.NewStyle().Foreground(lipgloss.Color("220")).Bold(true)
	stylePreview  = lipgloss.NewStyle().
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
	// SkillRef links a CheckItem back to the remote skill it was added from.
	// Empty for items that came from the AI proposals (not the skills.sh search).
	SkillRef *skills.Skill
}

// ─── Focus area ───────────────────────────────────────────────────────────────

type focusArea int

const (
	focusMain   focusArea = iota // navigating the main (AI proposed) list
	focusSearch                  // navigating skills.sh search results
)

// ─── Async search messages ───────────────────────────────────────────────────

type searchStartedMsg struct{ query string }
type searchResultMsg struct {
	query   string
	results []skills.Skill
	err     error
}

// runSearch returns a tea.Cmd that performs the skills.sh search in the
// background and emits a searchResultMsg when done.
func runSearch(query string) tea.Cmd {
	return func() tea.Msg {
		res, err := skills.Search(query, 100)
		return searchResultMsg{query: query, results: res, err: err}
	}
}

// ─── Bubbletea Model ──────────────────────────────────────────────────────────

type checkModel struct {
	title      string
	subtitle   string
	items      []CheckItem
	cursor     int
	offset     int
	maxVisible int
	searchable bool
	searching  bool
	search     textinput.Model
	focus      focusArea

	// Remote search state
	searchLoading bool
	searchError   string
	searchQuery   string         // query tied to the current results
	remoteResults []skills.Skill // last page of results from skills.sh

	previewText string

	confirmed bool
	width     int
}

func newCheckModel(title, subtitle string, items []CheckItem, searchable bool) checkModel {
	ti := textinput.New()
	ti.Placeholder = "search skills.sh (e.g. supabase, stripe, react)…"
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

// visibleMain returns indices into m.items that match the current query.
// Only applies when focus is Main and the user has typed a filter into the
// search box in non-search mode. In our new flow the search box is dedicated
// to the remote API, so this filter is only active when NOT in search mode.
func (m checkModel) visibleMain() []int {
	var out []int
	for i := range m.items {
		out = append(out, i)
	}
	return out
}

// alreadyInListByRef returns true if a skill (matched by ID or label) is
// already in the main selection list.
func (m checkModel) alreadyInListByRef(s skills.Skill) bool {
	for _, it := range m.items {
		if it.SkillRef != nil && it.SkillRef.ID == s.ID {
			return true
		}
		if strings.EqualFold(it.Label, s.SkillID) || strings.EqualFold(it.Label, s.Name) {
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

	case searchResultMsg:
		// Only apply if this result matches the most recent query we sent
		if msg.query != m.searchQuery {
			return m, nil
		}
		m.searchLoading = false
		if msg.err != nil {
			m.searchError = msg.err.Error()
			m.remoteResults = nil
			return m, nil
		}
		m.searchError = ""
		m.remoteResults = msg.results
		m.cursor = 0
		m.offset = 0
		// Auto-focus the result list if we got any results
		if len(m.remoteResults) > 0 {
			m.focus = focusSearch
			m.searching = false
			m.search.Blur()
		}
		m.updatePreview()
		return m, nil

	case tea.KeyMsg:
		// ── Search text input handling ──────────────────────────────────────
		if m.searching {
			switch msg.String() {
			case "ctrl+c":
				m.confirmed = true
				return m, tea.Quit
			case "esc":
				m.searching = false
				m.search.Blur()
				m.remoteResults = nil
				m.searchError = ""
				m.previewText = ""
				m.focus = focusMain
				m.cursor = 0
				m.offset = 0
			case "enter":
				q := strings.TrimSpace(m.search.Value())
				if q == "" {
					m.searching = false
					m.search.Blur()
					return m, nil
				}
				m.searchQuery = q
				m.searchLoading = true
				m.searchError = ""
				m.remoteResults = nil
				return m, runSearch(q)
			default:
				var cmd tea.Cmd
				m.search, cmd = m.search.Update(msg)
				return m, cmd
			}
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
			if m.focus == focusSearch || len(m.remoteResults) > 0 {
				m.focus = focusMain
				m.remoteResults = nil
				m.searchError = ""
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
			if m.searchable && len(m.remoteResults) > 0 {
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
				// Add skill result to the main selection list
				if m.cursor < len(m.remoteResults) {
					rs := m.remoteResults[m.cursor]
					if !m.alreadyInListByRef(rs) {
						ref := rs
						label := rs.SkillID
						if label == "" {
							label = rs.Name
						}
						detail := rs.Source
						if rs.Installs > 0 {
							detail = fmt.Sprintf("%s  ·  %d installs", rs.Source, rs.Installs)
						}
						m.items = append(m.items, CheckItem{
							Label:    label,
							Detail:   detail,
							Selected: true,
							SkillRef: &ref,
						})
					} else {
						// Toggle existing
						for i, it := range m.items {
							if it.SkillRef != nil && it.SkillRef.ID == rs.ID {
								m.items[i].Selected = !m.items[i].Selected
								break
							}
							if strings.EqualFold(it.Label, rs.SkillID) {
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
			}
		}
	}
	return m, nil
}

func (m checkModel) currentLimit() int {
	if m.focus == focusSearch {
		return len(m.remoteResults)
	}
	return len(m.visibleMain())
}

func (m *checkModel) updatePreview() {
	if m.focus == focusSearch && m.cursor < len(m.remoteResults) {
		rs := m.remoteResults[m.cursor]
		m.previewText = fmt.Sprintf("%s  ·  %d installs\nhttps://skills.sh/s/%s",
			rs.Source, rs.Installs, rs.ID)
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
		hints = []string{"↑/↓ navigate results", "SPACE add to selection", "TAB switch list", "ENTER confirm", "ESC back"}
	} else {
		hints = []string{"↑/↓ navigate", "SPACE toggle", "ENTER confirm", "a/n all/none"}
		if m.searchable {
			hints = append(hints, "/ search skills.sh")
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

	mainMaxVis := m.maxVisible
	if len(m.remoteResults) > 0 || m.searchLoading || m.searchError != "" {
		mainMaxVis = 5
	}

	detailWidth := m.width - 14
	if detailWidth < 30 {
		detailWidth = 30
	}
	indent := "        "

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
		sb.WriteString(styleHint.Render("  ENTER to search skills.sh  ·  ESC to cancel"))
		sb.WriteString("\n")
	} else if m.searchable && len(m.remoteResults) == 0 && !m.searchLoading && m.searchError == "" {
		sb.WriteString(styleHint.Render("  Press  /  to search skills.sh — 800+ skills across the community"))
		sb.WriteString("\n")
	}

	// ── Loading / error ───────────────────────────────────────────────────
	if m.searchLoading {
		sep := styleSepLine.Render(strings.Repeat("─", 60))
		sb.WriteString(sep + "\n")
		sb.WriteString(styleDim.Render(fmt.Sprintf("  ⏳ searching skills.sh for %q…\n", m.searchQuery)))
	} else if m.searchError != "" {
		sep := styleSepLine.Render(strings.Repeat("─", 60))
		sb.WriteString(sep + "\n")
		sb.WriteString(styleHint.Render(fmt.Sprintf("  ❌ search failed: %s\n", m.searchError)))
	}

	// ── Remote search results ─────────────────────────────────────────────
	if len(m.remoteResults) > 0 {
		sep := styleSepLine.Render(strings.Repeat("─", 60))
		sb.WriteString(sep + "\n")
		label := fmt.Sprintf("  🌐 skills.sh results for %q  (%d found)", m.searchQuery, len(m.remoteResults))
		if m.focus == focusSearch {
			sb.WriteString(styleTitle2.Render(label) + styleDim.Render("   ← focus here"))
		} else {
			sb.WriteString(styleDim.Render(label) + styleHint.Render("   (TAB to focus)"))
		}
		sb.WriteString("\n\n")

		diskEnd := m.offset + m.maxVisible
		if m.focus != focusSearch {
			diskEnd = 5
		}
		if diskEnd > len(m.remoteResults) {
			diskEnd = len(m.remoteResults)
		}
		diskStart := 0
		if m.focus == focusSearch {
			diskStart = m.offset
		}

		diskIndent := "        "
		for pos := diskStart; pos < diskEnd; pos++ {
			rs := m.remoteResults[pos]
			isCursor := m.focus == focusSearch && pos == m.cursor

			inList := m.alreadyInListByRef(rs)
			box := styleUncheck.Render("☐")
			if inList {
				box = styleChecked.Render("☑")
			}

			name := styleNewSkill.Render(rs.SkillID)
			detail := fmt.Sprintf("%s  ·  %d installs", rs.Source, rs.Installs)
			desc := styleDetail.Width(detailWidth).Render(detail)

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

		if m.focus == focusSearch && len(m.remoteResults) > m.maxVisible {
			sb.WriteString(styleDim.Render(fmt.Sprintf(
				"\n  %d–%d of %d results\n", diskStart+1, diskEnd, len(m.remoteResults))))
		}

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
// When searchable=true, "/" opens a search against skills.sh.
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
