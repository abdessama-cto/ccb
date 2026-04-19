package cmd

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/abdessama-cto/ccb/internal/config"
	"github.com/abdessama-cto/ccb/internal/llm"
)

// ─── Steps ───────────────────────────────────────────────────────────────────

type settingsStep int

const (
	stepProvider settingsStep = iota
	stepCredential // API key (openai/gemini) OR Ollama URL
	stepModel
	stepBudget
	stepProfile
	stepLanguage
	stepConfirm
)

const totalSteps = 7

var stepTitles = map[settingsStep]string{
	stepProvider:   "Choose your AI provider",
	stepCredential: "Credentials",
	stepModel:      "Choose the default model",
	stepBudget:     "Monthly spend budget (USD)",
	stepProfile:    "Default profile",
	stepLanguage:   "Communication language",
	stepConfirm:    "Review & save",
}

var stepSubtitles = map[settingsStep]string{
	stepProvider:   "Which LLM should ccb use by default? You can change this anytime.",
	stepCredential: "Needed so ccb can call the API on your behalf.",
	stepModel:      "Pick the model that matches your speed/quality/cost tradeoff.",
	stepBudget:     "Informational only — ccb will warn (not block) past this monthly figure.",
	stepProfile:    "Shapes the hooks ccb installs when you bootstrap a project.",
	stepLanguage:   "Language the AI uses to phrase wizard questions and generated docs. 'auto' = English.",
	stepConfirm:    "Press ENTER to save, or go back to tweak.",
}

// ─── Styles (reuse tui_select styles where possible) ─────────────────────────

var (
	wsStyleHeader  = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("212"))
	wsStyleTitle   = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("39"))
	wsStyleSub     = lipgloss.NewStyle().Foreground(lipgloss.Color("244"))
	wsStyleDim     = lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
	wsStyleHint    = lipgloss.NewStyle().Foreground(lipgloss.Color("238"))
	wsStyleOption  = lipgloss.NewStyle().Foreground(lipgloss.Color("39"))
	wsStyleCursor  = lipgloss.NewStyle().Foreground(lipgloss.Color("212")).Bold(true)
	wsStyleOK      = lipgloss.NewStyle().Foreground(lipgloss.Color("120"))
	wsStyleWarn    = lipgloss.NewStyle().Foreground(lipgloss.Color("220"))
	wsStyleErr     = lipgloss.NewStyle().Foreground(lipgloss.Color("196"))
	wsStyleCurrent = lipgloss.NewStyle().Foreground(lipgloss.Color("245")).Italic(true)
	wsStyleBox     = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("62")).
			Padding(0, 1)
	wsStyleProg = lipgloss.NewStyle().Foreground(lipgloss.Color("62"))
)

// ─── Model ───────────────────────────────────────────────────────────────────

type settingsWizard struct {
	cfg *config.Config

	step    settingsStep
	cursor  int
	text    textinput.Model
	err     string
	dirty   bool
	saved   bool
	aborted bool

	width  int
	height int
}

func newSettingsWizard(cfg *config.Config) settingsWizard {
	ti := textinput.New()
	ti.CharLimit = 200
	ti.Width = 50
	ti.PromptStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("212"))
	ti.TextStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("15"))

	w := settingsWizard{
		cfg:    cfg,
		step:   stepProvider,
		text:   ti,
		width:  90,
		height: 24,
	}
	w.enterStep(stepProvider)
	return w
}

// ─── Option helpers for each step ────────────────────────────────────────────

func (w settingsWizard) providerOptions() []string {
	return []string{"openai", "gemini", "ollama"}
}

func (w settingsWizard) modelOptions() []string {
	switch w.cfg.AI.Provider {
	case "gemini":
		return llm.GeminiModels
	case "ollama":
		return llm.OllamaModels
	default:
		return llm.OpenAIModels
	}
}

func (w settingsWizard) profileOptions() []string {
	return []string{"balanced", "strict", "lightweight"}
}

// languageOption pairs an ISO code (stored in config) with a human label
// shown in the wizard.
type languageOption struct {
	Code  string
	Label string
}

func languageOptionsList() []languageOption {
	return []languageOption{
		{"auto", "Auto (follow system locale)"},
		{"en", "English"},
		{"fr", "Français"},
		{"es", "Español"},
		{"ar", "العربية (Arabic)"},
	}
}

func (w settingsWizard) languageOptions() []string {
	opts := languageOptionsList()
	codes := make([]string, len(opts))
	for i, o := range opts {
		codes[i] = o.Code
	}
	return codes
}

// languageLabelFor returns the human-readable label for an ISO code.
func languageLabelFor(code string) string {
	for _, o := range languageOptionsList() {
		if o.Code == code {
			return o.Label
		}
	}
	return code
}

// currentValueForStep returns the existing value of the setting being edited
// (used to pre-position the cursor and pre-fill text inputs).
func (w settingsWizard) currentValueForStep(step settingsStep) string {
	switch step {
	case stepProvider:
		return w.cfg.AI.Provider
	case stepCredential:
		if w.cfg.AI.Provider == "ollama" {
			return w.cfg.AI.OllamaURL
		}
		if w.cfg.AI.Provider == "gemini" {
			return w.cfg.AI.GeminiKey
		}
		return w.cfg.AI.OpenAIKey
	case stepModel:
		return w.cfg.AI.ActiveModel()
	case stepBudget:
		return fmt.Sprintf("%.2f", w.cfg.AI.MonthlyBudgetUSD)
	case stepProfile:
		return w.cfg.Defaults.Profile
	case stepLanguage:
		return w.cfg.UI.Language
	}
	return ""
}

// ─── Navigation ──────────────────────────────────────────────────────────────

// nextStep returns the step that follows s, or stepConfirm if s is already the last.
func nextStep(s settingsStep) settingsStep {
	if s >= stepConfirm {
		return stepConfirm
	}
	return s + 1
}

// prevStep returns the step that precedes s, or stepProvider if s is the first.
func prevStep(s settingsStep) settingsStep {
	if s <= stepProvider {
		return stepProvider
	}
	return s - 1
}

// enterStep prepares the wizard state when we transition to a new step.
func (w *settingsWizard) enterStep(s settingsStep) {
	w.step = s
	w.err = ""
	current := w.currentValueForStep(s)

	switch s {
	case stepProvider:
		w.cursor = indexOrZero(w.providerOptions(), current)
	case stepCredential:
		w.text.SetValue(current)
		w.text.Focus()
		if w.cfg.AI.Provider == "ollama" {
			w.text.Placeholder = "http://localhost:11434"
		} else {
			w.text.Placeholder = "paste your API key (will be stored in ~/.ccb/config.yaml)"
		}
	case stepModel:
		w.cursor = indexOrZero(w.modelOptions(), current)
	case stepBudget:
		w.text.SetValue(current)
		w.text.Focus()
		w.text.Placeholder = "5.00"
	case stepProfile:
		w.cursor = indexOrZero(w.profileOptions(), current)
	case stepLanguage:
		w.cursor = indexOrZero(w.languageOptions(), current)
	case stepConfirm:
		w.text.Blur()
	}
}

func indexOrZero(options []string, current string) int {
	for i, o := range options {
		if o == current {
			return i
		}
	}
	return 0
}

// ─── Commit handlers — called when user presses ENTER on a step ──────────────

func (w *settingsWizard) commitStep() error {
	switch w.step {
	case stepProvider:
		selected := w.providerOptions()[w.cursor]
		if selected != w.cfg.AI.Provider {
			w.cfg.AI.Provider = selected
			w.dirty = true
		}
	case stepCredential:
		val := strings.TrimSpace(w.text.Value())
		switch w.cfg.AI.Provider {
		case "ollama":
			if val == "" {
				val = "http://localhost:11434"
			}
			if val != w.cfg.AI.OllamaURL {
				w.cfg.AI.OllamaURL = val
				w.dirty = true
			}
		case "gemini":
			if val != w.cfg.AI.GeminiKey {
				w.cfg.AI.GeminiKey = val
				w.dirty = true
			}
		default:
			if val != w.cfg.AI.OpenAIKey {
				w.cfg.AI.OpenAIKey = val
				w.dirty = true
			}
		}
	case stepModel:
		selected := w.modelOptions()[w.cursor]
		switch w.cfg.AI.Provider {
		case "gemini":
			if selected != w.cfg.AI.GeminiModel {
				w.cfg.AI.GeminiModel = selected
				w.dirty = true
			}
		case "ollama":
			if selected != w.cfg.AI.OllamaModel {
				w.cfg.AI.OllamaModel = selected
				w.dirty = true
			}
		default:
			if selected != w.cfg.AI.OpenAIModel {
				w.cfg.AI.OpenAIModel = selected
				w.dirty = true
			}
		}
	case stepBudget:
		val := strings.TrimSpace(w.text.Value())
		if val == "" {
			return nil
		}
		n, err := strconv.ParseFloat(val, 64)
		if err != nil || n < 0 {
			return fmt.Errorf("enter a positive number (e.g. 5.00)")
		}
		if n != w.cfg.AI.MonthlyBudgetUSD {
			w.cfg.AI.MonthlyBudgetUSD = n
			w.dirty = true
		}
	case stepProfile:
		selected := w.profileOptions()[w.cursor]
		if selected != w.cfg.Defaults.Profile {
			w.cfg.Defaults.Profile = selected
			w.dirty = true
		}
	case stepLanguage:
		selected := w.languageOptions()[w.cursor]
		if selected != w.cfg.UI.Language {
			w.cfg.UI.Language = selected
			w.dirty = true
		}
	}
	return nil
}

// ─── Bubbletea ──────────────────────────────────────────────────────────────

func (w settingsWizard) Init() tea.Cmd { return textinput.Blink }

func (w settingsWizard) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {

	case tea.WindowSizeMsg:
		w.width = msg.Width
		w.height = msg.Height
		return w, nil

	case tea.KeyMsg:
		key := msg.String()

		// Global keys (active on every step)
		switch key {
		case "ctrl+c":
			w.aborted = true
			return w, tea.Quit
		case "esc":
			// Quit without saving
			w.aborted = true
			return w, tea.Quit
		}

		// Step-specific behaviour
		if w.step == stepConfirm {
			switch key {
			case "enter":
				w.saved = true
				return w, tea.Quit
			case "left", "shift+tab", "backspace":
				w.enterStep(prevStep(w.step))
				return w, nil
			}
			return w, nil
		}

		if w.isTextStep() {
			switch key {
			case "enter":
				if err := w.commitStep(); err != nil {
					w.err = err.Error()
					return w, nil
				}
				w.enterStep(nextStep(w.step))
				return w, nil
			case "tab", "right":
				// Skip: don't commit, just move on with current value untouched.
				w.enterStep(nextStep(w.step))
				return w, nil
			case "shift+tab", "left":
				w.enterStep(prevStep(w.step))
				return w, nil
			}
			var cmd tea.Cmd
			w.text, cmd = w.text.Update(msg)
			return w, cmd
		}

		// Choice step navigation
		opts := w.currentOptions()
		switch key {
		case "up", "k":
			if w.cursor > 0 {
				w.cursor--
			}
		case "down", "j":
			if w.cursor < len(opts)-1 {
				w.cursor++
			}
		case "enter", " ":
			if err := w.commitStep(); err != nil {
				w.err = err.Error()
				return w, nil
			}
			w.enterStep(nextStep(w.step))
		case "tab", "right":
			w.enterStep(nextStep(w.step))
		case "shift+tab", "left", "backspace":
			if w.step == stepProvider {
				// Backing out of the first step = exit without saving
				w.aborted = true
				return w, tea.Quit
			}
			w.enterStep(prevStep(w.step))
		}
	}
	return w, nil
}

func (w settingsWizard) isTextStep() bool {
	return w.step == stepCredential || w.step == stepBudget
}

func (w settingsWizard) currentOptions() []string {
	switch w.step {
	case stepProvider:
		return w.providerOptions()
	case stepModel:
		return w.modelOptions()
	case stepProfile:
		return w.profileOptions()
	case stepLanguage:
		return w.languageOptions()
	}
	return nil
}

// ─── View ────────────────────────────────────────────────────────────────────

func (w settingsWizard) View() string {
	if w.saved || w.aborted {
		return ""
	}

	var sb strings.Builder

	// Header with progress
	sb.WriteString("\n")
	sb.WriteString(wsStyleHeader.Render("  ⚙  ccb settings"))
	sb.WriteString(wsStyleDim.Render(fmt.Sprintf("   ·   step %d of %d\n", int(w.step)+1, totalSteps)))

	// Progress bar
	sb.WriteString("  " + w.progressBar() + "\n\n")

	// Title + subtitle
	sb.WriteString("  " + wsStyleTitle.Render(stepTitles[w.step]) + "\n")
	if sub := stepSubtitles[w.step]; sub != "" {
		for _, ln := range wordWrap(sub, w.width-6) {
			sb.WriteString("  " + wsStyleSub.Render(ln) + "\n")
		}
	}
	sb.WriteString("\n")

	// Step body
	switch w.step {
	case stepConfirm:
		sb.WriteString(w.confirmSummary())
	default:
		if w.isTextStep() {
			sb.WriteString(w.textStepView())
		} else {
			sb.WriteString(w.choiceStepView())
		}
	}

	// Error
	if w.err != "" {
		sb.WriteString("\n  " + wsStyleErr.Render("✗ "+w.err) + "\n")
	}

	// Footer hints
	sb.WriteString("\n  " + wsStyleHint.Render(w.footerHints()) + "\n")

	return sb.String()
}

func (w settingsWizard) progressBar() string {
	const slots = totalSteps
	var b strings.Builder
	b.WriteString("[")
	for i := 0; i < slots; i++ {
		if i < int(w.step) {
			b.WriteString(wsStyleOK.Render("●"))
		} else if i == int(w.step) {
			b.WriteString(wsStyleProg.Render("●"))
		} else {
			b.WriteString(wsStyleDim.Render("○"))
		}
	}
	b.WriteString("]")
	return b.String()
}

func (w settingsWizard) choiceStepView() string {
	var sb strings.Builder
	current := w.currentValueForStep(w.step)
	opts := w.currentOptions()

	currentDisplay := valueOrNone(current)
	if w.step == stepLanguage {
		currentDisplay = languageLabelFor(current)
	}
	sb.WriteString("  " + wsStyleDim.Render("Current: ") + wsStyleCurrent.Render(currentDisplay) + "\n\n")

	for i, o := range opts {
		display := o
		if w.step == stepLanguage {
			display = languageLabelFor(o)
		}
		prefix := "   "
		label := wsStyleOption.Render(display)
		if i == w.cursor {
			prefix = wsStyleCursor.Render(" ▶ ")
			label = wsStyleCursor.Render(display)
		}
		trailing := ""
		if o == current {
			trailing = "  " + wsStyleDim.Render("(current)")
		}
		sb.WriteString("  " + prefix + label + trailing + "\n")
	}
	return sb.String()
}

func (w settingsWizard) textStepView() string {
	var sb strings.Builder
	current := w.currentValueForStep(w.step)

	label := ""
	switch w.step {
	case stepCredential:
		if w.cfg.AI.Provider == "ollama" {
			label = "Ollama base URL:"
		} else {
			label = fmt.Sprintf("%s API key:", strings.Title(w.cfg.AI.Provider))
			// Mask the current value for display
			if current != "" {
				current = maskKey(current)
			}
		}
	case stepBudget:
		label = "Monthly budget (USD):"
	}

	sb.WriteString("  " + wsStyleDim.Render("Current: ") + wsStyleCurrent.Render(valueOrNone(current)) + "\n\n")
	sb.WriteString("  " + wsStyleOption.Render(label) + "\n")
	sb.WriteString("  " + wsStyleBox.Render(w.text.View()) + "\n")
	return sb.String()
}

func (w settingsWizard) confirmSummary() string {
	var sb strings.Builder
	line := func(label, value string) {
		sb.WriteString(fmt.Sprintf("    %s %s\n",
			wsStyleDim.Render(fmt.Sprintf("%-18s", label)),
			wsStyleOption.Render(value)))
	}

	sb.WriteString("  " + wsStyleTitle.Render("Summary") + "\n\n")
	line("Provider", w.cfg.AI.Provider)
	switch w.cfg.AI.Provider {
	case "openai":
		line("API key", maskKey(w.cfg.AI.OpenAIKey))
		line("Model", w.cfg.AI.OpenAIModel)
	case "gemini":
		line("API key", maskKey(w.cfg.AI.GeminiKey))
		line("Model", w.cfg.AI.GeminiModel)
	case "ollama":
		line("Ollama URL", w.cfg.AI.OllamaURL)
		line("Model", w.cfg.AI.OllamaModel)
	}
	line("Monthly budget", fmt.Sprintf("$%.2f", w.cfg.AI.MonthlyBudgetUSD))
	line("Profile", w.cfg.Defaults.Profile)
	line("Language", w.cfg.UI.Language)

	if !w.dirty {
		sb.WriteString("\n  " + wsStyleWarn.Render("No changes to save.") + "\n")
	}
	return sb.String()
}

func (w settingsWizard) footerHints() string {
	switch w.step {
	case stepConfirm:
		return "ENTER save  ·  ←/backspace back  ·  ESC quit without saving"
	default:
		base := "↑/↓ navigate  ·  ENTER confirm & next  ·  TAB skip  ·  ←/backspace back  ·  ESC quit"
		if w.isTextStep() {
			base = "ENTER confirm & next  ·  TAB skip  ·  ←/shift+tab back  ·  ESC quit"
		}
		return base
	}
}

// ─── Helpers ────────────────────────────────────────────────────────────────

func valueOrNone(s string) string {
	if strings.TrimSpace(s) == "" {
		return "(not set)"
	}
	return s
}

func maskKey(key string) string {
	if key == "" {
		return "(not set)"
	}
	if len(key) <= 8 {
		return strings.Repeat("•", len(key))
	}
	return key[:8] + strings.Repeat("•", 8)
}

// ─── Entry point ────────────────────────────────────────────────────────────

// RunSettingsWizard opens the interactive Bubbletea wizard and saves the
// updated config when the user confirms on the final step.
func RunSettingsWizard(cfg *config.Config) (saved bool, err error) {
	m := newSettingsWizard(cfg)
	p := tea.NewProgram(m, tea.WithAltScreen())
	final, runErr := p.Run()
	if runErr != nil {
		return false, runErr
	}
	fm, ok := final.(settingsWizard)
	if !ok {
		return false, fmt.Errorf("unexpected wizard state")
	}
	if fm.aborted || !fm.saved {
		return false, nil
	}
	if err := config.Save(fm.cfg); err != nil {
		return false, fmt.Errorf("save config: %w", err)
	}
	return true, nil
}
