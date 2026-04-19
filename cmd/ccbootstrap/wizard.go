package cmd

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/abdessama-cto/ccb/internal/llm"
)

// ─── Styles ───────────────────────────────────────────────────────────────────

var (
	wizStyleTitle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("212"))
	wizStyleSub   = lipgloss.NewStyle().Foreground(lipgloss.Color("244"))
	wizStyleDim   = lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
	wizStyleHint  = lipgloss.NewStyle().Foreground(lipgloss.Color("238"))
	wizStyleHit   = lipgloss.NewStyle().Foreground(lipgloss.Color("120"))
	wizStyleOpt   = lipgloss.NewStyle().Foreground(lipgloss.Color("39"))
	wizStyleCur   = lipgloss.NewStyle().Foreground(lipgloss.Color("212")).Bold(true)
	wizStyleProg  = lipgloss.NewStyle().Foreground(lipgloss.Color("62"))
)

// ─── Model ────────────────────────────────────────────────────────────────────

type wizardModel struct {
	questions []llm.WizardQuestion
	answers   []llm.WizardAnswer
	step      int

	// For choice/yesno
	cursor int

	// For free text
	text textinput.Model

	done     bool
	aborted  bool
	width    int
}

func newWizardModel(questions []llm.WizardQuestion) wizardModel {
	ti := textinput.New()
	ti.Placeholder = "type your answer…"
	ti.CharLimit = 200
	ti.Width = 60
	ti.Focus()

	m := wizardModel{
		questions: questions,
		answers:   make([]llm.WizardAnswer, 0, len(questions)),
		text:      ti,
		width:     80,
	}
	return m
}

func (m wizardModel) Init() tea.Cmd { return textinput.Blink }

func (m wizardModel) currentQ() *llm.WizardQuestion {
	if m.step < 0 || m.step >= len(m.questions) {
		return nil
	}
	return &m.questions[m.step]
}

func (m wizardModel) optionsFor(q *llm.WizardQuestion) []string {
	if q == nil {
		return nil
	}
	switch q.Type {
	case "yesno":
		return []string{"yes", "no"}
	case "choice":
		return q.Options
	}
	return nil
}

func (m *wizardModel) recordAnswer(value string) {
	q := m.currentQ()
	if q == nil {
		return
	}
	m.answers = append(m.answers, llm.WizardAnswer{
		ID:       q.ID,
		Question: q.Question,
		Value:    value,
	})
}

func (m wizardModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		return m, nil

	case tea.KeyMsg:
		q := m.currentQ()
		if q == nil {
			return m, tea.Quit
		}

		// Text input handling
		if q.Type == "text" {
			switch msg.String() {
			case "ctrl+c", "esc":
				m.aborted = true
				return m, tea.Quit
			case "enter":
				val := strings.TrimSpace(m.text.Value())
				if val == "" {
					val = q.Default
				}
				m.recordAnswer(val)
				m.advance()
				if m.done {
					return m, tea.Quit
				}
				m.text.SetValue("")
				m.text.Focus()
				return m, nil
			default:
				var cmd tea.Cmd
				m.text, cmd = m.text.Update(msg)
				return m, cmd
			}
		}

		// Choice/yesno navigation
		opts := m.optionsFor(q)
		switch msg.String() {
		case "ctrl+c":
			m.aborted = true
			return m, tea.Quit
		case "esc":
			// Use default
			m.recordAnswer(q.Default)
			m.advance()
			if m.done {
				return m, tea.Quit
			}
			m.cursor = 0
			return m, nil
		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
			}
		case "down", "j":
			if m.cursor < len(opts)-1 {
				m.cursor++
			}
		case "enter", " ":
			if m.cursor >= 0 && m.cursor < len(opts) {
				m.recordAnswer(opts[m.cursor])
				m.advance()
				if m.done {
					return m, tea.Quit
				}
				m.cursor = 0
				m.text.SetValue("")
				m.text.Focus()
				return m, nil
			}
		}
	}
	return m, nil
}

func (m *wizardModel) advance() {
	m.step++
	if m.step >= len(m.questions) {
		m.done = true
	}
}

func (m wizardModel) View() string {
	if m.done || m.aborted {
		return ""
	}
	q := m.currentQ()
	if q == nil {
		return ""
	}

	var sb strings.Builder
	total := len(m.questions)
	step := m.step + 1

	// Progress bar
	barWidth := 40
	filled := (step * barWidth) / total
	bar := strings.Repeat("█", filled) + strings.Repeat("░", barWidth-filled)
	sb.WriteString("\n  ")
	sb.WriteString(wizStyleProg.Render(bar))
	sb.WriteString(fmt.Sprintf("  %s\n\n", wizStyleDim.Render(fmt.Sprintf("Question %d of %d", step, total))))

	// Title
	sb.WriteString("  ")
	sb.WriteString(wizStyleTitle.Render(q.Question))
	sb.WriteString("\n")

	if strings.TrimSpace(q.Subtitle) != "" {
		maxw := m.width - 4
		if maxw < 40 {
			maxw = 40
		}
		for _, ln := range wordWrap(q.Subtitle, maxw) {
			sb.WriteString("  ")
			sb.WriteString(wizStyleSub.Render(ln))
			sb.WriteString("\n")
		}
	}

	sb.WriteString("\n")

	// Body
	switch q.Type {
	case "text":
		sb.WriteString("  " + m.text.View())
		sb.WriteString("\n")
		if q.Default != "" {
			sb.WriteString("  ")
			sb.WriteString(wizStyleDim.Render(fmt.Sprintf("(Enter to accept default: %q)", q.Default)))
			sb.WriteString("\n")
		}
	default:
		opts := m.optionsFor(q)
		for i, o := range opts {
			marker := "  "
			label := wizStyleOpt.Render(o)
			if i == m.cursor {
				marker = wizStyleCur.Render("▶ ")
				label = wizStyleCur.Render(o)
			}
			star := ""
			if o == q.Default {
				star = wizStyleHit.Render(" (default)")
			}
			sb.WriteString(fmt.Sprintf("  %s%s%s\n", marker, label, star))
		}
	}

	// Footer hints
	sb.WriteString("\n  ")
	switch q.Type {
	case "text":
		sb.WriteString(wizStyleHint.Render("ENTER submit  ·  ESC abort"))
	default:
		sb.WriteString(wizStyleHint.Render("↑/↓ navigate  ·  ENTER select  ·  ESC skip (use default)"))
	}
	sb.WriteString("\n")

	return sb.String()
}

// ─── Public API ───────────────────────────────────────────────────────────────

// RunWizard displays the full-screen wizard and returns the collected answers.
// Returns nil if the user aborted with Ctrl+C.
func RunWizard(questions []llm.WizardQuestion) []llm.WizardAnswer {
	if len(questions) == 0 {
		return nil
	}

	m := newWizardModel(questions)
	p := tea.NewProgram(m, tea.WithAltScreen())
	final, err := p.Run()
	if err != nil {
		return wizardFallback(questions)
	}
	fm, ok := final.(wizardModel)
	if !ok || fm.aborted {
		return nil
	}
	return fm.answers
}

// WizardDefaults fills the answers slice with each question's default value.
// Used in --yes / --skip-questionnaire mode.
func WizardDefaults(questions []llm.WizardQuestion) []llm.WizardAnswer {
	out := make([]llm.WizardAnswer, 0, len(questions))
	for _, q := range questions {
		out = append(out, llm.WizardAnswer{
			ID:       q.ID,
			Question: q.Question,
			Value:    q.Default,
		})
	}
	return out
}

// ─── Fallback (non-TTY) ─────────────────────────────────────────────────────

func wizardFallback(questions []llm.WizardQuestion) []llm.WizardAnswer {
	out := make([]llm.WizardAnswer, 0, len(questions))
	for i, q := range questions {
		fmt.Printf("\n[%d/%d] %s\n", i+1, len(questions), q.Question)
		if q.Subtitle != "" {
			fmt.Printf("      %s\n", q.Subtitle)
		}
		switch q.Type {
		case "yesno":
			fmt.Printf("      Answer (yes/no) [default: %s]: ", q.Default)
		case "choice":
			fmt.Printf("      Options: %s [default: %s]: ", strings.Join(q.Options, " / "), q.Default)
		default:
			fmt.Printf("      Answer [default: %q]: ", q.Default)
		}
		var line string
		fmt.Scanln(&line)
		line = strings.TrimSpace(line)
		if line == "" {
			line = q.Default
		}
		out = append(out, llm.WizardAnswer{ID: q.ID, Question: q.Question, Value: line})
	}
	return out
}
