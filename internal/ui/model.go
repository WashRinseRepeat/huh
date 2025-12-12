package ui

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/atotto/clipboard"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/reflow/wordwrap"
)

type State int

const (
	StateInput State = iota
	StateRefining
	StateFilePrompt
	StateLoading
	StateSuggestion
	StateExplained
	StateError
)

type Model struct {
	State              State
	PreviousState      State // To return after file prompt
	Question           string
	Input              textinput.Model
	ContextInfo        string // Display string (e.g. "Attached: foo.txt")
	ContextContent     string // Actual content
	Suggestion         string
	RunnableCommands   []string // Extracted commands for execution/copy
	ActiveCommandIndex int      // Which command is currently selected
	Explanation        string
	Err                error

	// Query
	QueryFunc   func(string, string) (string, error)
	ExplainFunc func(string, string) (string, error)
	RefineFunc  func(string, string, string) (string, error)

	// Menu
	Options        []string
	SelectedOption int

	// Focus
	FocusIndex int // 0: Input, 1: AttachButton

	// Completion
	Matches    []string
	MatchIndex int

	// Viewport
	viewport  viewport.Model
	maxHeight int
	ready     bool
}

func NewModel(question string, contextInfo string, contextContent string, queryFunc func(string, string) (string, error), explainFunc func(string, string) (string, error), refineFunc func(string, string, string) (string, error)) Model {
	initialState := StateLoading
	ti := textinput.New()

	if question == "" {
		initialState = StateInput
		ti.Placeholder = "example: how do I check disk space?"
		ti.Focus()
	}

	return Model{
		State:          initialState,
		Question:       question,
		Input:          ti,
		ContextInfo:    contextInfo,
		ContextContent: contextContent,
		Options:        []string{"Copy", "Explain", "Refine", "Cancel"},
		SelectedOption: 0,
		QueryFunc:      queryFunc,
		ExplainFunc:    explainFunc,
		RefineFunc:     refineFunc,
	}
}

func (m Model) Init() tea.Cmd {
	if m.State == StateInput {
		return textinput.Blink
	}
	return func() tea.Msg {
		res, err := m.QueryFunc(m.Question, m.ContextContent)
		if err != nil {
			return ErrorMsg(err)
		}
		return SuggestionMsg(res)
	}
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var (
		cmd  tea.Cmd
		cmds []tea.Cmd
	)

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		headerHeight := lipgloss.Height(TitleStyle.Render("Header"))
		footerHeight := lipgloss.Height("Footer")
		verticalMarginHeight := headerHeight + footerHeight

		if !m.ready {
			m.viewport = viewport.New(msg.Width, msg.Height-verticalMarginHeight)
			m.viewport.YPosition = headerHeight
			m.maxHeight = msg.Height - verticalMarginHeight
			m.ready = true
		} else {
			m.viewport.Width = msg.Width
			m.maxHeight = msg.Height - verticalMarginHeight
		}

		// Re-render content with new width
		m.updateViewportContent()

	case SuggestionMsg:
		m.Suggestion = string(msg)
		m.Explanation = "" // Clear previous if any
		m.State = StateSuggestion

		// Parse Markdown Code Blocks
		// Use regex to find all code blocks
		re := regexp.MustCompile("(?s)```(.*?)```")
		matches := re.FindAllStringSubmatch(m.Suggestion, -1)

		m.RunnableCommands = nil
		if len(matches) > 0 {
			for _, match := range matches {
				raw := match[1]
				// Clean content
				raw = strings.TrimSpace(raw)
				if idx := strings.Index(raw, "\n"); idx != -1 {
					firstLine := raw[:idx]
					if !strings.Contains(firstLine, " ") {
						raw = strings.TrimSpace(raw[idx+1:])
					}
				}
				m.RunnableCommands = append(m.RunnableCommands, raw)
			}
			// Default to last command as active
			m.ActiveCommandIndex = len(m.RunnableCommands) - 1
		} else {
			m.ActiveCommandIndex = -1
		}
		m.updateViewportContent()

	case ExplanationMsg:
		m.Explanation = string(msg)
		m.State = StateExplained
		m.updateViewportContent()

	case ErrorMsg:
		m.Err = msg
		m.State = StateError
		return m, nil
	case tea.KeyMsg:
		switch m.State {
		case StateInput:
			switch msg.String() {
			case "tab", "shift+tab":
				m.FocusIndex = 1 - m.FocusIndex // Toggle 0/1
				if m.FocusIndex == 0 {
					m.Input.Focus()
				} else {
					m.Input.Blur()
				}
				return m, nil

			case "enter":
				if m.FocusIndex == 1 {
					// Attach File Button Clicked
					m.PreviousState = m.State
					m.State = StateFilePrompt
					m.Input.SetValue("")
					m.Input.Placeholder = "/path/to/file"
					m.Input.Focus()
					return m, textinput.Blink
				}

				m.Question = m.Input.Value()
				if m.Question != "" {
					m.State = StateLoading
					return m, func() tea.Msg {
						res, err := m.QueryFunc(m.Question, m.ContextContent)
						if err != nil {
							return ErrorMsg(err)
						}
						return SuggestionMsg(res)
					}
				}
			case "ctrl+c", "esc":
				return m, tea.Quit
			}
			var cmd tea.Cmd
			m.Input, cmd = m.Input.Update(msg)
			return m, cmd

		case StateRefining:
			switch msg.String() {
			case "tab", "shift+tab":
				m.FocusIndex = 1 - m.FocusIndex // Toggle 0/1
				if m.FocusIndex == 0 {
					m.Input.Focus()
				} else {
					m.Input.Blur()
				}
				return m, nil

			case "enter":
				if m.FocusIndex == 1 {
					// Attach File Button Clicked
					m.PreviousState = m.State
					m.State = StateFilePrompt
					m.Input.SetValue("")
					m.Input.Placeholder = "/path/to/file"
					m.Input.Focus()
					return m, textinput.Blink
				}

				// Submit Input
				refinement := m.Input.Value()
				if refinement != "" {
					m.Question = refinement // Update question to reflect new query in UI

					// Capture current suggestion for refinement logic
					currentSuggestion := m.Suggestion
					if len(m.RunnableCommands) > 0 {
						currentSuggestion = m.RunnableCommands[m.ActiveCommandIndex]
					}
					// Clear suggestion in model so View() shows "Thinking about..." instead of "Explaining..."
					m.Suggestion = ""

					m.State = StateLoading
					return m, func() tea.Msg {
						res, err := m.RefineFunc(currentSuggestion, refinement, m.ContextContent)
						if err != nil {
							return ErrorMsg(err)
						}
						return SuggestionMsg(res)
					}
				}
			case "esc":
				m.State = StateSuggestion
				return m, nil
			case "ctrl+c":
				return m, tea.Quit
			}
			var cmd tea.Cmd
			m.Input, cmd = m.Input.Update(msg)
			return m, cmd

		case StateFilePrompt:
			switch msg.String() {
			case "tab":
				// Auto-complete logic
				input := m.Input.Value()
				if input == "" {
					input = "" // Keep empty to match * in getMatches
				}

				// New completion query
				if m.Matches == nil {
					matches, err := getMatches(input)
					if err == nil && len(matches) > 0 {
						m.Matches = matches
						m.MatchIndex = 0
						// Set initial match immediately
						m.Input.SetValue(m.Matches[0])
						m.Input.SetCursor(len(m.Input.Value()))
						return m, nil
					}
				}

				// Cycle matches
				if len(m.Matches) > 0 {
					// Increment FIRST, then set
					m.MatchIndex = (m.MatchIndex + 1) % len(m.Matches)

					current := m.Matches[m.MatchIndex]
					m.Input.SetValue(current)
					m.Input.SetCursor(len(m.Input.Value()))
				}
				return m, nil

			case "enter":
				path := m.Input.Value()
				if path != "" {
					// Check if directory
					info, err := os.Stat(path)
					if err == nil && info.IsDir() {
						// Do nothing on directories (require explicit / to drill)
						return m, nil
					}

					// Read file
					b, err := os.ReadFile(path)
					if err != nil {
						m.Err = fmt.Errorf("read error: %v", err)
						m.State = StateError
						return m, nil
					}

					// Append context
					m.ContextContent += fmt.Sprintf("\n--- File: %s ---\n%s\n", path, string(b))
					if m.ContextInfo == "" {
						m.ContextInfo = path
					} else {
						m.ContextInfo += ", " + path
					}

					// Return to previous state
					m.State = m.PreviousState
					m.Input.SetValue("") // Clear input for question/refinement
					m.FocusIndex = 0     // Reset focus to input
					m.Input.Focus()
					return m, nil
				}
			case "esc":
				m.State = m.PreviousState
				return m, nil
			case "ctrl+c":
				return m, tea.Quit
			}
			var cmd tea.Cmd
			// Reset completion if user types
			if msg.String() != "tab" {
				m.Matches = nil
			}
			m.Input, cmd = m.Input.Update(msg)
			return m, cmd

		case StateSuggestion:
			switch msg.String() {
			case "q", "ctrl+c":
				return m, tea.Quit
			case "tab":
				if len(m.RunnableCommands) > 1 {
					m.ActiveCommandIndex = (m.ActiveCommandIndex + 1) % len(m.RunnableCommands)
					m.updateViewportContent()
				}
				return m, nil
			case "shift+tab":
				if len(m.RunnableCommands) > 1 {
					m.ActiveCommandIndex--
					if m.ActiveCommandIndex < 0 {
						m.ActiveCommandIndex = len(m.RunnableCommands) - 1
					}
					m.updateViewportContent()
				}
				return m, nil
			case "left", "h":
				if m.SelectedOption > 0 {
					m.SelectedOption--
				}
			case "right", "l":
				if m.SelectedOption < len(m.Options)-1 {
					m.SelectedOption++
				}
			case "up", "k":
				m.viewport.ScrollUp(1)
			case "down", "j":
				m.viewport.ScrollDown(1)
			case "pgup", "ctrl+u":
				m.viewport.ScrollUp(m.viewport.Height / 2)
			case "pgdown", "ctrl+d":
				m.viewport.ScrollDown(m.viewport.Height / 2)
			case "enter":
				// If at bottom and scrolling down, maybe Enter on simple text does nothing?
				// But Enter triggers selection.
				return m.handleSelection()
			}
		case StateExplained:
			switch msg.String() {
			case "esc", "q":
				m.State = StateSuggestion // Go back
			case "up", "k":
				m.viewport.ScrollUp(1)
			case "down", "j":
				m.viewport.ScrollDown(1)
			case "pgup", "ctrl+u", "shift+up":
				m.viewport.ScrollUp(m.viewport.Height / 2)
			case "pgdown", "ctrl+d", "shift+down":
				m.viewport.ScrollDown(m.viewport.Height / 2)
			}
		case StateError:
			if msg.String() == "q" || msg.String() == "ctrl+c" {
				return m, tea.Quit
			}
		}
	}

	// Handle viewport updates
	m.viewport, cmd = m.viewport.Update(msg)
	cmds = append(cmds, cmd)

	return m, tea.Batch(cmds...)
}

func (m Model) handleSelection() (tea.Model, tea.Cmd) {
	selected := m.Options[m.SelectedOption]
	switch selected {
	case "Copy":
		if len(m.RunnableCommands) == 0 {
			m.Err = fmt.Errorf("no executable command found to copy")
			m.State = StateError
			return m, nil
		}
		// Copy Active Command
		cmd := m.RunnableCommands[m.ActiveCommandIndex]
		if err := clipboard.WriteAll(cmd); err != nil {
			m.Err = fmt.Errorf("failed to copy: %v (install wl-clipboard or xclip)", err)
			m.State = StateError
			return m, nil
		}
		return m, tea.Quit
	case "Explain":
		m.State = StateLoading // Show loading while explaining
		return m, func() tea.Msg {
			target := m.Suggestion
			if len(m.RunnableCommands) > 0 {
				target = m.RunnableCommands[m.ActiveCommandIndex]
			}
			exp, err := m.ExplainFunc(target, m.ContextContent)
			if err != nil {
				return ErrorMsg(err)
			}
			return ExplanationMsg(exp)
		}
	case "Refine":
		if len(m.RunnableCommands) == 0 {
			m.Err = fmt.Errorf("no command to edit")
			m.State = StateError
			return m, nil
		}
		m.State = StateRefining
		m.Input.SetValue("") // Should we pre-fill? Probably not per previous logic.
		m.Input.Placeholder = "example: how do I check disk space?"
		m.Input.Focus()
		return m, textinput.Blink

	case "Cancel":
		return m, tea.Quit
	}
	return m, nil
}

func (m *Model) updateViewportContent() {
	var content strings.Builder

	// Create a wrapper style for wrapping text
	// Ensure we reserve some padding if needed, but viewport width is usually sufficient
	// wrapper := lipgloss.NewStyle().Width(m.viewport.Width)

	if m.State == StateSuggestion {
		if len(m.RunnableCommands) > 0 && strings.Contains(m.Suggestion, "```") {
			// Split by code blocks
			parts := strings.Split(m.Suggestion, "```")
			cmdIndex := 0
			for i, part := range parts {
				if i%2 == 1 { // Inside code block
					if cmdIndex < len(m.RunnableCommands) {
						cmdVal := m.RunnableCommands[cmdIndex]
						style := InactiveCommandStyle
						if cmdIndex == m.ActiveCommandIndex {
							style = CommandStyle
						}
						// Commands are usually short, but let's wrap just in case or truncation?
						// Code blocks usually scroll horizontally or wrap poorly.
						// Let's assume commands fit or soft wrap naturally.
						content.WriteString(style.Render(cmdVal))
						cmdIndex++
					} else {
						content.WriteString(wordwrap.String(InactiveCommandStyle.Render(part), m.viewport.Width))
					}
				} else { // Outside code block
					content.WriteString(wordwrap.String(part, m.viewport.Width))
				}
			}
		} else {
			content.WriteString(wordwrap.String(m.Suggestion, m.viewport.Width))
		}

		content.WriteString("\n")
		if len(m.RunnableCommands) > 1 {
			content.WriteString(wordwrap.String(lipgloss.NewStyle().Foreground(subtleColor).Render("(Tab to cycle commands)"), m.viewport.Width))
			content.WriteString("\n")
		}
		content.WriteString("\n")
	} else if m.State == StateExplained {
		content.WriteString(wordwrap.String(DescriptionStyle.Render(m.Explanation), m.viewport.Width))
		content.WriteString(wordwrap.String("\n(Press Esc to back)", m.viewport.Width))
	}

	str := content.String()
	m.viewport.SetContent(str)

	// Dynamic Height Adjustment
	// Count lines in the rendered content
	lineCount := lipgloss.Height(str)

	if lineCount < m.maxHeight {
		m.viewport.Height = lineCount
	} else {
		m.viewport.Height = m.maxHeight
	}
}

func (m Model) View() string {
	if !m.ready {
		return "\n  Initializing..."
	}
	var s strings.Builder

	switch m.State {
	case StateInput:
		s.WriteString(TitleStyle.Render("What would you like to do?"))
		if m.ContextInfo != "" {
			s.WriteString(lipgloss.NewStyle().Foreground(subtleColor).Render(fmt.Sprintf("\n(Context: %s)", m.ContextInfo)))
		}
		s.WriteString("\n\n")

		// Input View
		s.WriteString(m.Input.View())
		s.WriteString("\n\n")

		// Attach Button Logic
		btnStyle := ItemStyle
		if m.FocusIndex == 1 {
			btnStyle = SelectedItemStyle
		}
		s.WriteString(btnStyle.Render("[ Attach File ]"))

		s.WriteString("\n\n(Tab to select, Enter to confirm, Esc to quit)")

	case StateRefining:
		s.WriteString(TitleStyle.Render("How should the command be changed?"))
		s.WriteString("\n\n")
		s.WriteString(lipgloss.NewStyle().Foreground(secondaryColor).Render(m.Suggestion))
		s.WriteString("\n\n")

		// Input View
		s.WriteString(m.Input.View())
		s.WriteString("\n\n")

		// Attach Button Logic
		btnStyle := ItemStyle
		if m.FocusIndex == 1 {
			btnStyle = SelectedItemStyle
		}
		s.WriteString(btnStyle.Render("[Attach File]"))

		s.WriteString("\n\n(Tab to select, Enter to confirm, Esc to cancel)")

	case StateFilePrompt:
		s.WriteString(TitleStyle.Render("File to attach:"))
		s.WriteString("\n\n")
		s.WriteString(m.Input.View())
		s.WriteString("\n\n(Press Enter to attach, Esc to cancel)")

		// Render Matches
		if len(m.Matches) > 0 {
			s.WriteString("\n\n")
			s.WriteString(TitleStyle.Render("Suggestions:"))
			s.WriteString("\n")

			// Limit to 5 matches for cleaner UI
			start := 0
			end := len(m.Matches)
			if end > 5 {
				// Simple windowing logic could be added here, for now just slice
				if m.MatchIndex >= 5 {
					start = m.MatchIndex - 4
				}
				if start+5 < end {
					end = start + 5
				}
			}

			for i := start; i < end; i++ {
				match := m.Matches[i]
				cursor := " "
				style := ItemStyle
				if i == m.MatchIndex {
					cursor = ">"
					style = SelectedItemStyle
				}
				// Show relative path or just basename? Full path is clearer for now.
				s.WriteString(style.Render(fmt.Sprintf("%s %s", cursor, match)) + "\n")
			}

			if len(m.Matches) > 5 {
				s.WriteString(ItemStyle.Render("..."))
			}
		}

	case StateLoading:
		if m.Explanation == "" && m.Suggestion != "" {
			s.WriteString("Explaining...")
		} else {
			s.WriteString(fmt.Sprintf("Thinking about: %s...", m.Question))
			if m.ContextInfo != "" {
				s.WriteString(fmt.Sprintf("\n(Context: %s)", m.ContextInfo))
			}
		}

	case StateSuggestion:
		s.WriteString(TitleStyle.Render("Suggestion:"))
		s.WriteString("\n")

		s.WriteString(m.viewport.View())

		s.WriteString("\n")
		// Render Options Horizontally
		var options []string
		for i, opt := range m.Options {
			style := ItemStyle
			if m.SelectedOption == i {
				style = SelectedItemStyle
			}
			options = append(options, style.Render(opt))
		}
		s.WriteString(lipgloss.JoinHorizontal(lipgloss.Top, options...))
		s.WriteString(lipgloss.NewStyle().Foreground(subtleColor).Render("  (<-/-> to select, Enter to confirm)"))

	case StateExplained:
		s.WriteString(TitleStyle.Render("Explanation:"))
		s.WriteString("\n")

		s.WriteString(m.viewport.View())

	case StateError:
		s.WriteString(TitleStyle.Foreground(errorColor).Render("Error:"))
		s.WriteString("\n")
		s.WriteString(fmt.Sprintf("%v", m.Err))
		s.WriteString("\n\n(Press q to quit)")
	}

	return lipgloss.NewStyle().Margin(1, 1).Render(s.String())
}

// Commands
func SetSuggestion(cmd string) tea.Msg {
	return SuggestionMsg(cmd)
}

type SuggestionMsg string
type ExplanationMsg string
type ErrorMsg error

func getMatches(pattern string) ([]string, error) {
	// Expand ~
	if strings.HasPrefix(pattern, "~") {
		home, err := os.UserHomeDir()
		if err == nil {
			pattern = filepath.Join(home, pattern[1:])
		}
	}

	// Add * for prefix matching
	search := pattern + "*"
	matches, err := filepath.Glob(search)
	if err != nil {
		return nil, err
	}

	// Filter out hidden files unless pattern starts with .
	// Actually Glob handles standard logic.
	// But let's support directory traversal hints (add / if dir)

	var processed []string
	processed = append(processed, matches...)

	return processed, nil
}
