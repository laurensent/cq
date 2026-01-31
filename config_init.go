package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type configStep int

const (
	stepMode configStep = iota
	stepProvider
	stepAPIKey
	stepBaseURL
	stepModel
	stepRawOutput
	stepTheme
	stepThinking
	stepWebSearch
	stepConfirm
)

const wizardTotalSteps = 10

var (
	wizardAccent    = lipgloss.Color("#E36C38")
	wizardTitle     = lipgloss.NewStyle().Bold(true).Foreground(wizardAccent)
	wizardLabel     = lipgloss.NewStyle().Bold(true)
	wizardSelected  = lipgloss.NewStyle().Foreground(wizardAccent).Bold(true)
	wizardDim       = lipgloss.NewStyle().Faint(true)
	wizardSummaryKV = lipgloss.NewStyle().PaddingLeft(2)
)

type configWizard struct {
	step     configStep
	cursor   int
	input    textinput.Model
	config   appConfig
	width    int
	height   int
	done     bool
	canceled bool
}

func newConfigWizard() configWizard {
	ti := textinput.New()
	ti.Placeholder = "sk-ant-..."
	ti.EchoMode = textinput.EchoPassword
	ti.EchoCharacter = '*'
	ti.Focus()

	return configWizard{
		step:  stepMode,
		input: ti,
		config: appConfig{
			Mode:     "cli",
			Theme:    "auto",
			Thinking: true,
		},
	}
}

func (m configWizard) Init() tea.Cmd {
	return textinput.Blink
}

func (m configWizard) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil
	case tea.KeyMsg:
		if m.step == stepAPIKey || m.step == stepBaseURL {
			return m.updateTextInput(msg)
		}
		return m.updateSelection(msg)
	}

	if m.step == stepAPIKey || m.step == stepBaseURL {
		var cmd tea.Cmd
		m.input, cmd = m.input.Update(msg)
		return m, cmd
	}
	return m, nil
}

func (m configWizard) updateTextInput(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.Type {
	case tea.KeyEnter:
		if m.step == stepAPIKey {
			m.config.APIKey = m.input.Value()
			m.step = m.nextAfterAPIKey()
			m.cursor = 0
			m.prepareBaseURLInput()
		} else if m.step == stepBaseURL {
			m.config.BaseURL = m.input.Value()
			m.step = stepModel
			m.cursor = 0
		}
		return m, nil
	case tea.KeyEsc, tea.KeyCtrlC:
		m.canceled = true
		return m, tea.Quit
	}
	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	return m, cmd
}

// nextAfterAPIKey determines the next step after API key entry.
func (m configWizard) nextAfterAPIKey() configStep {
	prov := m.config.resolvedProvider()
	// Anthropic and Gemini have fixed base URLs, skip base URL step
	if prov == "anthropic" || prov == "gemini" {
		return stepModel
	}
	return stepBaseURL
}

func (m *configWizard) prepareBaseURLInput() {
	m.input.Placeholder = ""
	m.input.EchoMode = textinput.EchoNormal
	m.input.EchoCharacter = 0
	m.input.SetValue("")

	// Pre-fill with provider default URL
	if p, err := getProvider(m.config.resolvedProvider()); err == nil {
		if ocp, ok := p.(openaiCompatProvider); ok {
			m.input.SetValue(ocp.defaultURL)
		}
	}
}

func (m configWizard) updateSelection(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	opts := m.currentOptions()
	switch msg.Type {
	case tea.KeyUp:
		if m.cursor > 0 {
			m.cursor--
		}
	case tea.KeyDown:
		if m.cursor < len(opts)-1 {
			m.cursor++
		}
	case tea.KeyEnter:
		return m.confirmSelection()
	case tea.KeyEsc, tea.KeyCtrlC:
		m.canceled = true
		return m, tea.Quit
	default:
		switch msg.String() {
		case "k":
			if m.cursor > 0 {
				m.cursor--
			}
		case "j":
			if m.cursor < len(opts)-1 {
				m.cursor++
			}
		}
	}
	return m, nil
}

func (m configWizard) confirmSelection() (tea.Model, tea.Cmd) {
	opts := m.currentOptions()
	switch m.step {
	case stepMode:
		m.config.Mode = opts[m.cursor]
		if m.config.Mode == "api" {
			m.step = stepProvider
		} else {
			m.config.Provider = ""
			m.config.APIKey = ""
			m.config.BaseURL = ""
			m.step = stepModel
		}
		m.cursor = 0
	case stepProvider:
		m.config.Provider = opts[m.cursor]
		if m.config.Provider == "ollama" {
			m.config.APIKey = ""
			m.step = stepBaseURL
			m.cursor = 0
			m.prepareBaseURLInput()
		} else {
			m.step = stepAPIKey
			m.input.SetValue("")
			m.input.EchoMode = textinput.EchoPassword
			m.input.EchoCharacter = '*'
			m.input.Placeholder = m.apiKeyPlaceholder()
		}
		m.cursor = 0
	case stepModel:
		m.config.DefaultModel = opts[m.cursor]
		m.step = stepRawOutput
		m.cursor = 0
	case stepRawOutput:
		m.config.RawOutput = opts[m.cursor] == "true"
		m.step = stepTheme
		m.cursor = 0
	case stepTheme:
		m.config.Theme = opts[m.cursor]
		if m.config.Mode == "api" {
			m.step = stepThinking
		} else {
			m.step = stepConfirm
		}
		m.cursor = 0
	case stepThinking:
		m.config.Thinking = opts[m.cursor] == "true"
		m.step = stepWebSearch
		m.cursor = 0
	case stepWebSearch:
		m.config.WebSearch = opts[m.cursor] == "true"
		m.step = stepConfirm
		m.cursor = 0
	case stepConfirm:
		if opts[m.cursor] == "Cancel" {
			m.canceled = true
			return m, tea.Quit
		}
		m.done = true
		return m, tea.Quit
	}
	return m, nil
}

func (m configWizard) apiKeyPlaceholder() string {
	switch m.config.resolvedProvider() {
	case "anthropic":
		return "sk-ant-..."
	case "openai":
		return "sk-..."
	case "gemini":
		return "AI..."
	case "xai":
		return "xai-..."
	default:
		return ""
	}
}

func (m configWizard) currentOptions() []string {
	switch m.step {
	case stepMode:
		return []string{"cli", "api"}
	case stepProvider:
		return providerNames()
	case stepModel:
		return m.modelOptions()
	case stepRawOutput:
		return []string{"false", "true"}
	case stepTheme:
		return []string{"auto", "dark", "light", "dracula", "pink", "ascii", "notty"}
	case stepThinking:
		return []string{"true", "false"}
	case stepWebSearch:
		return []string{"false", "true"}
	case stepConfirm:
		return []string{"Save", "Cancel"}
	default:
		return nil
	}
}

func (m configWizard) modelOptions() []string {
	if m.config.Mode == "api" {
		if p, err := getProvider(m.config.resolvedProvider()); err == nil {
			return p.ModelAliases()
		}
	}
	// CLI mode: show Anthropic aliases as default
	return []string{"sonnet", "opus", "haiku"}
}

func (m configWizard) stepTitle() string {
	switch m.step {
	case stepMode:
		return "Mode"
	case stepProvider:
		return "Provider"
	case stepAPIKey:
		return "API Key"
	case stepBaseURL:
		return "Base URL"
	case stepModel:
		return "Default Model"
	case stepRawOutput:
		return "Raw Output"
	case stepTheme:
		return "Theme"
	case stepThinking:
		return "Extended Thinking"
	case stepWebSearch:
		return "Web Search"
	case stepConfirm:
		return "Confirm"
	default:
		return ""
	}
}

func (m configWizard) stepDescription() string {
	switch m.step {
	case stepMode:
		return "cli = use claude CLI, api = call LLM provider API directly"
	case stepProvider:
		return "Choose the LLM provider for API mode"
	case stepAPIKey:
		return fmt.Sprintf("Enter your %s API key", m.config.resolvedProvider())
	case stepBaseURL:
		return "Custom API base URL (edit or press Enter to accept default)"
	case stepModel:
		return "Choose the default model for queries"
	case stepRawOutput:
		return "Skip markdown rendering and output raw text?"
	case stepTheme:
		return "Choose the markdown rendering theme"
	case stepThinking:
		return "Enable extended thinking for deeper reasoning?"
	case stepWebSearch:
		return "Enable web search for up-to-date information?"
	case stepConfirm:
		return "Review your configuration"
	default:
		return ""
	}
}

func (m configWizard) View() string {
	if m.canceled {
		return ""
	}

	var b strings.Builder

	// Header
	header := wizardTitle.Render("ask config")
	stepInfo := wizardDim.Render(fmt.Sprintf("  [%d/%d]", int(m.step)+1, wizardTotalSteps))
	b.WriteString(header + stepInfo + "\n\n")

	// Step title and description
	b.WriteString(wizardLabel.Render(m.stepTitle()) + "\n")
	b.WriteString(wizardDim.Render(m.stepDescription()) + "\n\n")

	if m.step == stepAPIKey || m.step == stepBaseURL {
		b.WriteString(m.input.View() + "\n\n")
		b.WriteString(wizardDim.Render("Enter = confirm  Esc = cancel"))
		return b.String()
	}

	if m.step == stepConfirm {
		b.WriteString(m.renderSummary())
		b.WriteString("\n")
	}

	// Render options
	opts := m.currentOptions()
	for i, opt := range opts {
		if i == m.cursor {
			b.WriteString(wizardSelected.Render("> " + opt))
		} else {
			b.WriteString("  " + opt)
		}
		b.WriteString("\n")
	}

	b.WriteString("\n" + wizardDim.Render("Up/K Down/J = navigate  Enter = confirm  Esc = cancel"))
	return b.String()
}

func (m configWizard) renderSummary() string {
	var b strings.Builder
	kv := func(k, v string) {
		b.WriteString(wizardSummaryKV.Render(
			wizardLabel.Render(k+": ") + v,
		) + "\n")
	}

	kv("Mode", m.config.Mode)
	if m.config.Mode == "api" {
		kv("Provider", m.config.resolvedProvider())
		masked := m.config.APIKey
		if len(masked) > 8 {
			masked = masked[:4] + strings.Repeat("*", len(masked)-8) + masked[len(masked)-4:]
		} else if len(masked) > 0 {
			masked = strings.Repeat("*", len(masked))
		}
		if masked != "" {
			kv("API Key", masked)
		}
		if m.config.BaseURL != "" {
			kv("Base URL", m.config.BaseURL)
		}
	}
	kv("Default Model", m.config.DefaultModel)
	kv("Raw Output", fmt.Sprintf("%v", m.config.RawOutput))
	kv("Theme", m.config.Theme)
	if m.config.Mode == "api" {
		kv("Thinking", fmt.Sprintf("%v", m.config.Thinking))
		kv("Web Search", fmt.Sprintf("%v", m.config.WebSearch))
	}
	return b.String()
}

func saveConfig(cfg appConfig) error {
	if err := os.MkdirAll(configDir(), 0755); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}
	data = append(data, '\n')
	if err := os.WriteFile(configPath(), data, 0644); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}
	return nil
}

func runConfigInit() error {
	m := newConfigWizard()
	p := tea.NewProgram(m, tea.WithAltScreen(), tea.WithOutput(os.Stderr))
	result, err := p.Run()
	if err != nil {
		return err
	}

	final := result.(configWizard)
	if final.canceled {
		fmt.Fprintln(os.Stderr, "Configuration canceled.")
		return nil
	}
	if !final.done {
		return nil
	}

	if err := saveConfig(final.config); err != nil {
		return err
	}
	fmt.Fprintf(os.Stderr, "Configuration saved to %s\n", configPath())
	return nil
}
