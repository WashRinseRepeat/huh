package setup

import (
	"fmt"
	"os"
	"strings"

	"github.com/WashRinseRepeat/huh/internal/config"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// Colors matching internal/ui/style.go
var (
	wizPrimary   = lipgloss.Color("#7D56F4")
	wizSecondary = lipgloss.Color("#04B575")
	wizSubtle    = lipgloss.Color("#6B6B6B")
	wizError     = lipgloss.Color("#FF3333")

	wizTitleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(wizPrimary)

	wizSuccessStyle = lipgloss.NewStyle().
			Foreground(wizSecondary)

	wizSubtleStyle = lipgloss.NewStyle().
			Foreground(wizSubtle).
			Italic(true)

	wizErrorStyle = lipgloss.NewStyle().
			Foreground(wizError)

	wizBoxStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(wizPrimary).
			Padding(1, 4)

	wizSelectedStyle = lipgloss.NewStyle().
				PaddingLeft(2).
				Foreground(lipgloss.Color("205")).
				Bold(true)

	wizItemStyle = lipgloss.NewStyle().
			PaddingLeft(2)

	backHint = wizSubtleStyle.Render("  (Esc to go back)")
)

type step int

const (
	stepWelcome step = iota
	stepProvider
	stepOllamaDetect
	stepOllamaHost
	stepOllamaModel
	stepOpenRouterGuide
	stepOpenRouterKey
	stepOpenRouterModelGuide
	stepOpenRouterModel
	stepCustomURL
	stepCustomKey
	stepCustomModel
	stepDone
)

type provider int

const (
	providerOllama provider = iota
	providerOpenRouter
	providerCustom
)

type wizardModel struct {
	step     step
	quitting bool

	// Selection state
	cursor   int
	selected provider

	// Collected values
	providerType string
	host         string
	apiKey       string
	model        string
	baseURL      string

	// Ollama detection
	ollamaModels []string
	detectErr    error

	// Input components
	textInput textinput.Model
	spinner   spinner.Model

	// List items for current step
	listItems []string

	// Result config
	result *config.Config
}

func newWizardModel() wizardModel {
	ti := textinput.New()
	ti.CharLimit = 256

	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(wizPrimary)

	return wizardModel{
		step:      stepWelcome,
		textInput: ti,
		spinner:   s,
		host:      "http://localhost:11434",
	}
}

func (m wizardModel) Init() tea.Cmd {
	return nil
}

func (m wizardModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if msg.String() == "ctrl+c" {
			m.quitting = true
			return m, tea.Quit
		}
	}

	switch m.step {
	case stepWelcome:
		return m.updateWelcome(msg)
	case stepProvider:
		return m.updateProvider(msg)
	case stepOllamaDetect:
		return m.updateOllamaDetect(msg)
	case stepOllamaHost:
		return m.updateOllamaHost(msg)
	case stepOllamaModel:
		return m.updateOllamaModel(msg)
	case stepOpenRouterGuide:
		return m.updateOpenRouterGuide(msg)
	case stepOpenRouterKey:
		return m.updateOpenRouterKey(msg)
	case stepOpenRouterModelGuide:
		return m.updateOpenRouterModelGuide(msg)
	case stepOpenRouterModel:
		return m.updateOpenRouterModel(msg)
	case stepCustomURL:
		return m.updateCustomURL(msg)
	case stepCustomKey:
		return m.updateCustomKey(msg)
	case stepCustomModel:
		return m.updateCustomModel(msg)
	case stepDone:
		return m.updateDone(msg)
	}

	return m, nil
}

// goToProvider resets state and returns to the provider selection step.
func (m wizardModel) goToProvider() (tea.Model, tea.Cmd) {
	m.step = stepProvider
	m.cursor = 0
	m.detectErr = nil
	m.textInput.Blur()
	m.listItems = []string{
		"Ollama (local, private, no API key needed)",
		"OpenRouter (100+ models, cloud-based)",
		"OpenAI-compatible (custom endpoint)",
	}
	return m, nil
}

// --- Step update handlers ---

func (m wizardModel) updateWelcome(msg tea.Msg) (tea.Model, tea.Cmd) {
	if key, ok := msg.(tea.KeyMsg); ok {
		switch key.String() {
		case "esc":
			m.quitting = true
			return m, tea.Quit
		case "enter":
			m.step = stepProvider
			m.cursor = 0
			m.listItems = []string{
				"Ollama (local, private, no API key needed)",
				"OpenRouter (100+ models, cloud-based)",
				"OpenAI-compatible (custom endpoint)",
			}
		}
	}
	return m, nil
}

func (m wizardModel) updateProvider(msg tea.Msg) (tea.Model, tea.Cmd) {
	if key, ok := msg.(tea.KeyMsg); ok {
		switch key.String() {
		case "esc":
			m.step = stepWelcome
			return m, nil
		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
			}
		case "down", "j":
			if m.cursor < len(m.listItems)-1 {
				m.cursor++
			}
		case "enter":
			switch m.cursor {
			case 0:
				m.selected = providerOllama
				m.providerType = "ollama"
				m.step = stepOllamaDetect
				return m, tea.Batch(m.spinner.Tick, detectOllama(m.host))
			case 1:
				m.selected = providerOpenRouter
				m.providerType = "openrouter"
				m.step = stepOpenRouterGuide
			case 2:
				m.selected = providerCustom
				m.providerType = "openai"
				m.step = stepCustomURL
				m.textInput.Placeholder = "http://localhost:1234/v1"
				m.textInput.SetValue("")
				m.textInput.Focus()
				return m, m.textInput.Cursor.BlinkCmd()
			}
		}
	}
	return m, nil
}

func (m wizardModel) updateOllamaDetect(msg tea.Msg) (tea.Model, tea.Cmd) {
	if key, ok := msg.(tea.KeyMsg); ok && key.String() == "esc" {
		return m.goToProvider()
	}
	switch msg := msg.(type) {
	case ollamaDetectMsg:
		if msg.err != nil {
			m.detectErr = msg.err
			m.step = stepOllamaHost
			m.textInput.Placeholder = "http://localhost:11434"
			m.textInput.SetValue(m.host)
			m.textInput.Focus()
			return m, m.textInput.Cursor.BlinkCmd()
		}
		m.ollamaModels = msg.models
		m.listItems = msg.models
		m.cursor = 0
		m.step = stepOllamaModel
		return m, nil
	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd
	}
	return m, nil
}

func (m wizardModel) updateOllamaHost(msg tea.Msg) (tea.Model, tea.Cmd) {
	if key, ok := msg.(tea.KeyMsg); ok && key.String() == "esc" {
		m.textInput.Blur()
		return m.goToProvider()
	}
	if key, ok := msg.(tea.KeyMsg); ok && key.String() == "enter" {
		val := strings.TrimSpace(m.textInput.Value())
		if val == "" {
			val = "http://localhost:11434"
		}
		m.host = val
		m.detectErr = nil
		m.step = stepOllamaDetect
		return m, tea.Batch(m.spinner.Tick, detectOllama(m.host))
	}

	var cmd tea.Cmd
	m.textInput, cmd = m.textInput.Update(msg)
	return m, cmd
}

func (m wizardModel) updateOllamaModel(msg tea.Msg) (tea.Model, tea.Cmd) {
	if key, ok := msg.(tea.KeyMsg); ok {
		switch key.String() {
		case "esc":
			return m.goToProvider()
		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
			}
		case "down", "j":
			if m.cursor < len(m.listItems)-1 {
				m.cursor++
			}
		case "enter":
			m.model = m.listItems[m.cursor]
			return m.finish()
		}
	}
	return m, nil
}

func (m wizardModel) updateOpenRouterGuide(msg tea.Msg) (tea.Model, tea.Cmd) {
	if key, ok := msg.(tea.KeyMsg); ok && key.String() == "esc" {
		return m.goToProvider()
	}
	if key, ok := msg.(tea.KeyMsg); ok && key.String() == "enter" {
		m.step = stepOpenRouterKey
		m.textInput.Placeholder = "sk-or-v1-..."
		m.textInput.SetValue("")
		m.textInput.EchoMode = textinput.EchoNormal
		m.textInput.Focus()
		return m, m.textInput.Cursor.BlinkCmd()
	}
	return m, nil
}

func (m wizardModel) updateOpenRouterKey(msg tea.Msg) (tea.Model, tea.Cmd) {
	if key, ok := msg.(tea.KeyMsg); ok && key.String() == "esc" {
		m.textInput.Blur()
		m.textInput.EchoMode = textinput.EchoNormal
		m.step = stepOpenRouterGuide
		return m, nil
	}
	if key, ok := msg.(tea.KeyMsg); ok && key.String() == "enter" {
		val := strings.TrimSpace(m.textInput.Value())
		if val == "" {
			return m, nil
		}
		m.apiKey = val
		m.textInput.EchoMode = textinput.EchoNormal
		m.step = stepOpenRouterModelGuide
	}

	var cmd tea.Cmd
	m.textInput, cmd = m.textInput.Update(msg)
	return m, cmd
}

func (m wizardModel) updateOpenRouterModelGuide(msg tea.Msg) (tea.Model, tea.Cmd) {
	if key, ok := msg.(tea.KeyMsg); ok && key.String() == "esc" {
		m.step = stepOpenRouterKey
		m.textInput.Placeholder = "sk-or-v1-..."
		m.textInput.SetValue("")
		m.textInput.EchoMode = textinput.EchoNormal
		m.textInput.Focus()
		return m, m.textInput.Cursor.BlinkCmd()
	}
	if key, ok := msg.(tea.KeyMsg); ok && key.String() == "enter" {
		m.step = stepOpenRouterModel
		m.textInput.Placeholder = "e.g. deepseek/deepseek-chat-v3-0324"
		m.textInput.SetValue("")
		m.textInput.Focus()
		return m, m.textInput.Cursor.BlinkCmd()
	}
	return m, nil
}

func (m wizardModel) updateOpenRouterModel(msg tea.Msg) (tea.Model, tea.Cmd) {
	if key, ok := msg.(tea.KeyMsg); ok && key.String() == "esc" {
		m.textInput.Blur()
		m.step = stepOpenRouterModelGuide
		return m, nil
	}
	if key, ok := msg.(tea.KeyMsg); ok && key.String() == "enter" {
		val := strings.TrimSpace(m.textInput.Value())
		if val == "" {
			return m, nil
		}
		m.model = val
		return m.finish()
	}

	var cmd tea.Cmd
	m.textInput, cmd = m.textInput.Update(msg)
	return m, cmd
}

func (m wizardModel) updateCustomURL(msg tea.Msg) (tea.Model, tea.Cmd) {
	if key, ok := msg.(tea.KeyMsg); ok && key.String() == "esc" {
		m.textInput.Blur()
		return m.goToProvider()
	}
	if key, ok := msg.(tea.KeyMsg); ok && key.String() == "enter" {
		val := strings.TrimSpace(m.textInput.Value())
		if val == "" {
			return m, nil
		}
		m.baseURL = val
		m.step = stepCustomKey
		m.textInput.Placeholder = "(optional, press Enter to skip)"
		m.textInput.SetValue("")
		m.textInput.EchoMode = textinput.EchoNormal
		return m, nil
	}

	var cmd tea.Cmd
	m.textInput, cmd = m.textInput.Update(msg)
	return m, cmd
}

func (m wizardModel) updateCustomKey(msg tea.Msg) (tea.Model, tea.Cmd) {
	if key, ok := msg.(tea.KeyMsg); ok && key.String() == "esc" {
		m.textInput.Blur()
		m.textInput.EchoMode = textinput.EchoNormal
		m.step = stepCustomURL
		m.textInput.Placeholder = "http://localhost:1234/v1"
		m.textInput.SetValue("")
		m.textInput.Focus()
		return m, m.textInput.Cursor.BlinkCmd()
	}
	if key, ok := msg.(tea.KeyMsg); ok && key.String() == "enter" {
		m.apiKey = strings.TrimSpace(m.textInput.Value())
		m.textInput.EchoMode = textinput.EchoNormal
		m.step = stepCustomModel
		m.textInput.Placeholder = "model-name"
		m.textInput.SetValue("")
		m.textInput.Focus()
		return m, m.textInput.Cursor.BlinkCmd()
	}

	var cmd tea.Cmd
	m.textInput, cmd = m.textInput.Update(msg)
	return m, cmd
}

func (m wizardModel) updateCustomModel(msg tea.Msg) (tea.Model, tea.Cmd) {
	if key, ok := msg.(tea.KeyMsg); ok && key.String() == "esc" {
		m.textInput.Blur()
		m.detectErr = nil
		m.step = stepCustomKey
		m.textInput.Placeholder = "(optional, press Enter to skip)"
		m.textInput.SetValue("")
		m.textInput.EchoMode = textinput.EchoNormal
		m.textInput.Focus()
		return m, m.textInput.Cursor.BlinkCmd()
	}
	if key, ok := msg.(tea.KeyMsg); ok && key.String() == "enter" {
		val := strings.TrimSpace(m.textInput.Value())
		if val == "" {
			return m, nil
		}
		m.model = val
		return m.finish()
	}

	var cmd tea.Cmd
	m.textInput, cmd = m.textInput.Update(msg)
	return m, cmd
}

func (m wizardModel) updateDone(msg tea.Msg) (tea.Model, tea.Cmd) {
	if key, ok := msg.(tea.KeyMsg); ok && key.String() == "enter" {
		return m, tea.Quit
	}
	return m, nil
}

// finish builds the config, writes it, and transitions to stepDone.
func (m wizardModel) finish() (tea.Model, tea.Cmd) {
	cfg := m.buildConfig()
	m.result = &cfg

	if err := config.WriteConfig(cfg); err != nil {
		m.detectErr = err
	}

	m.step = stepDone
	return m, nil
}

func (m wizardModel) buildConfig() config.Config {
	cfg := config.Config{
		SystemPrompt: "You are a helpful CLI assistant.\n" +
			"Always explain the command briefly before showing the code block.\n" +
			"If a sequence of commands is suggested, explain each command briefly before showing the code block containing the commands in sequence.\n" +
			"If the user asks for a command, provide it inside a markdown code block, like:\n" +
			"```bash\ncommand here\n```\n" +
			"If the user asks a question, answer it normally.\n",
		Context: map[string]string{
			"level": "basic",
		},
		Providers: make(map[string]config.ProviderConfig),
	}

	switch m.selected {
	case providerOllama:
		cfg.DefaultProvider = "ollama"
		cfg.Providers["ollama"] = config.ProviderConfig{
			Type: "ollama",
			Params: map[string]string{
				"host":  m.host,
				"model": m.model,
			},
		}
	case providerOpenRouter:
		cfg.DefaultProvider = "openrouter"
		cfg.Providers["openrouter"] = config.ProviderConfig{
			Type: "openrouter",
			Params: map[string]string{
				"api_key": m.apiKey,
				"model":   m.model,
			},
		}
	case providerCustom:
		cfg.DefaultProvider = "openai"
		params := map[string]string{
			"model": m.model,
		}
		if m.apiKey != "" {
			params["api_key"] = m.apiKey
		}
		if m.baseURL != "" {
			params["base_url"] = m.baseURL
		}
		cfg.Providers["openai"] = config.ProviderConfig{
			Type:   "openai",
			Params: params,
		}
	}

	return cfg
}

// --- View ---

func (m wizardModel) View() string {
	if m.quitting {
		return ""
	}

	var s strings.Builder

	switch m.step {
	case stepWelcome:
		s.WriteString(m.viewWelcome())
	case stepProvider:
		s.WriteString(m.viewProvider())
	case stepOllamaDetect:
		s.WriteString(m.viewOllamaDetect())
	case stepOllamaHost:
		s.WriteString(m.viewOllamaHost())
	case stepOllamaModel:
		s.WriteString(m.viewOllamaModel())
	case stepOpenRouterGuide:
		s.WriteString(m.viewOpenRouterGuide())
	case stepOpenRouterKey:
		s.WriteString(m.viewOpenRouterKey())
	case stepOpenRouterModelGuide:
		s.WriteString(m.viewOpenRouterModelGuide())
	case stepOpenRouterModel:
		s.WriteString(m.viewOpenRouterModel())
	case stepCustomURL:
		s.WriteString(m.viewCustomURL())
	case stepCustomKey:
		s.WriteString(m.viewCustomKey())
	case stepCustomModel:
		s.WriteString(m.viewCustomModel())
	case stepDone:
		s.WriteString(m.viewDone())
	}

	s.WriteString("\n")
	return s.String()
}

func (m wizardModel) viewWelcome() string {
	var s strings.Builder
	s.WriteString("\n")

	box := wizBoxStyle.Render(
		wizTitleStyle.Render("Welcome to huh!") + "\n\n" +
			"Your terminal AI assistant",
	)
	s.WriteString(box)
	s.WriteString("\n\n")
	s.WriteString("  Let's set up your AI provider.\n\n")
	s.WriteString(wizSubtleStyle.Render("  Press Enter to continue..."))
	return s.String()
}

func (m wizardModel) viewProvider() string {
	var s strings.Builder
	s.WriteString("\n")
	s.WriteString(wizTitleStyle.Render("  Choose your AI provider:"))
	s.WriteString("\n\n")

	descriptions := []string{
		"    Runs models locally on your machine. Completely private, no API\n" +
			"    key needed, and free to use. Requires Ollama to be installed.\n" +
			"    Best if you have a decent GPU and want full control over your data.",
		"    Cloud API that gives access to 100+ models from many providers\n" +
			"    through a single API key. Pay-per-token pricing, most models are\n" +
			"    very cheap. Best balance of cost, quality, and convenience.",
		"    Connect to any server that speaks the OpenAI chat completions\n" +
			"    protocol. Works with LM Studio, vLLM, llama.cpp server, etc.\n" +
			"    Best if you already have a local or self-hosted inference setup.",
	}

	for i, item := range m.listItems {
		if i == m.cursor {
			s.WriteString(wizSelectedStyle.Render("> " + item))
		} else {
			s.WriteString(wizItemStyle.Render("  " + item))
		}
		s.WriteString("\n")
		s.WriteString(wizSubtleStyle.Render(descriptions[i]))
		s.WriteString("\n\n")
	}

	s.WriteString(wizSubtleStyle.Render("  (↑/↓ to select, Enter to confirm)") + "\n")
	s.WriteString(backHint)
	return s.String()
}

func (m wizardModel) viewOllamaDetect() string {
	var s strings.Builder
	s.WriteString("\n")
	s.WriteString(fmt.Sprintf("  %s Checking for Ollama at %s...\n\n", m.spinner.View(), m.host))
	s.WriteString(backHint)
	return s.String()
}

func (m wizardModel) viewOllamaHost() string {
	var s strings.Builder
	s.WriteString("\n")
	s.WriteString(wizErrorStyle.Render("  ✗ "+m.detectErr.Error()) + "\n\n")
	s.WriteString("  Enter your Ollama host URL:\n\n")
	s.WriteString("  " + m.textInput.View() + "\n\n")
	s.WriteString(wizSubtleStyle.Render("  (Enter to try again, or type a different URL)") + "\n")
	s.WriteString(backHint)
	return s.String()
}

func (m wizardModel) viewOllamaModel() string {
	var s strings.Builder
	s.WriteString("\n")
	s.WriteString(wizSuccessStyle.Render(fmt.Sprintf("  ✓ Ollama detected! Found %d model(s):", len(m.ollamaModels))))
	s.WriteString("\n\n")

	for i, item := range m.listItems {
		if i == m.cursor {
			s.WriteString(wizSelectedStyle.Render("> " + item))
		} else {
			s.WriteString(wizItemStyle.Render("  " + item))
		}
		s.WriteString("\n")
	}

	s.WriteString("\n")
	s.WriteString(wizSubtleStyle.Render("  (↑/↓ to select, Enter to confirm)") + "\n")
	s.WriteString(backHint)
	return s.String()
}

func (m wizardModel) viewOpenRouterGuide() string {
	var s strings.Builder
	s.WriteString("\n")
	s.WriteString(wizTitleStyle.Render("  To use OpenRouter, you need an API key:"))
	s.WriteString("\n\n")
	s.WriteString("  1. Go to  https://openrouter.ai\n")
	s.WriteString("  2. Sign up or log in\n")
	s.WriteString("  3. Navigate to \"Keys\" in your dashboard\n")
	s.WriteString("  4. Click \"Create Key\"\n")
	s.WriteString("  5. Copy the key and paste it below\n\n")
	s.WriteString(wizSubtleStyle.Render("  Press Enter when ready...") + "\n")
	s.WriteString(backHint)
	return s.String()
}

func (m wizardModel) viewOpenRouterKey() string {
	var s strings.Builder
	s.WriteString("\n")
	s.WriteString(wizTitleStyle.Render("  Enter your OpenRouter API key:"))
	s.WriteString("\n\n")
	s.WriteString("  " + m.textInput.View() + "\n\n")
	s.WriteString(wizSubtleStyle.Render("  (Paste your key and press Enter)") + "\n")
	s.WriteString(backHint)
	return s.String()
}

func (m wizardModel) viewOpenRouterModelGuide() string {
	var s strings.Builder
	s.WriteString("\n")
	s.WriteString(wizTitleStyle.Render("  Now pick a model."))
	s.WriteString("\n\n")
	s.WriteString("  Browse models at:\n\n")
	s.WriteString("    " + wizTitleStyle.Render("https://openrouter.ai/models?categories=programming&max_price=1&order=most-popular") + "\n\n")
	s.WriteString("  This link filters for cheap, popular programming models — likely the\n")
	s.WriteString("  best fit for a terminal assistant. Click the copy button next to any\n")
	s.WriteString("  model name to copy its ID, then paste it in the next step.\n\n")
	s.WriteString("  You can always change the model later in your config file.\n\n")
	s.WriteString(wizSubtleStyle.Render("  Press Enter when ready to paste a model ID...") + "\n")
	s.WriteString(backHint)
	return s.String()
}

func (m wizardModel) viewOpenRouterModel() string {
	var s strings.Builder
	s.WriteString("\n")
	s.WriteString(wizTitleStyle.Render("  Paste the model ID:"))
	s.WriteString("\n\n")
	s.WriteString("  " + m.textInput.View() + "\n\n")
	s.WriteString(wizSubtleStyle.Render("  (Paste the model ID from OpenRouter and press Enter)") + "\n")
	s.WriteString(backHint)
	return s.String()
}

func (m wizardModel) viewCustomURL() string {
	var s strings.Builder
	s.WriteString("\n")
	s.WriteString(wizTitleStyle.Render("  Enter your API base URL:"))
	s.WriteString("\n\n")
	s.WriteString("  " + m.textInput.View() + "\n\n")
	s.WriteString(wizSubtleStyle.Render("  (The URL should point to an OpenAI-compatible chat completions endpoint)") + "\n")
	s.WriteString(backHint)
	return s.String()
}

func (m wizardModel) viewCustomKey() string {
	var s strings.Builder
	s.WriteString("\n")
	s.WriteString(wizTitleStyle.Render("  Enter your API key:"))
	s.WriteString("\n\n")
	s.WriteString("  " + m.textInput.View() + "\n\n")
	s.WriteString(wizSubtleStyle.Render("  (Optional — press Enter to skip)") + "\n")
	s.WriteString(backHint)
	return s.String()
}

func (m wizardModel) viewCustomModel() string {
	var s strings.Builder
	s.WriteString("\n")
	if m.detectErr != nil {
		s.WriteString(wizErrorStyle.Render("  ✗ "+m.detectErr.Error()) + "\n\n")
	}
	s.WriteString(wizTitleStyle.Render("  Enter the model name:"))
	s.WriteString("\n\n")
	s.WriteString("  " + m.textInput.View() + "\n\n")
	s.WriteString(wizSubtleStyle.Render("  (e.g. gpt-4o, llama3:8b, etc.)") + "\n")
	s.WriteString(backHint)
	return s.String()
}

func (m wizardModel) viewDone() string {
	var s strings.Builder
	s.WriteString("\n")

	if m.detectErr != nil {
		s.WriteString(wizErrorStyle.Render("  ✗ Error saving config: "+m.detectErr.Error()) + "\n")
		return s.String()
	}

	s.WriteString(wizSuccessStyle.Render("  ✓ Configuration saved!") + "\n\n")

	providerName := m.providerType
	switch m.selected {
	case providerOllama:
		providerName = "Ollama"
	case providerOpenRouter:
		providerName = "OpenRouter"
	case providerCustom:
		providerName = "OpenAI-compatible"
	}

	s.WriteString(fmt.Sprintf("    Provider: %s\n", providerName))
	s.WriteString(fmt.Sprintf("    Model:    %s\n\n", m.model))

	configPath, err := config.GetConfigLocation()
	if err == nil {
		s.WriteString(wizSubtleStyle.Render(fmt.Sprintf("  Config written to %s", configPath)) + "\n")
	}
	s.WriteString(wizSubtleStyle.Render("  You can change settings anytime by editing this file.") + "\n\n")
	s.WriteString("  Press Enter to start using huh...")
	return s.String()
}

// Run launches the setup wizard and returns true if it completed successfully.
func Run() bool {
	p := tea.NewProgram(newWizardModel(), tea.WithOutput(os.Stderr))
	finalModel, err := p.Run()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error running setup wizard: %v\n", err)
		return false
	}

	wm, ok := finalModel.(wizardModel)
	if !ok {
		return false
	}

	return wm.result != nil && !wm.quitting
}
