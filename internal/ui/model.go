package ui

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/atotto/clipboard"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
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
	State          State
	PreviousState  State // To return after file prompt
	Question       string
	Input          textinput.Model
	ContextInfo    string // Display string (e.g. "Attached: foo.txt")
	ContextContent string // Actual content
	Suggestion     string
	RunnableCommand string // Extracted command for execution/copy
	Explanation    string
	Err          error
	
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
}

func NewModel(question string, contextInfo string, contextContent string, queryFunc func(string, string) (string, error), explainFunc func(string, string) (string, error), refineFunc func(string, string, string) (string, error)) Model {
	initialState := StateLoading
	ti := textinput.New()
	
	if question == "" {
		initialState = StateInput
		ti.Placeholder = "e.g. how do I check disk space?"
		ti.Focus()
	}

	return Model{
		State:          initialState,
		Question:       question,
		Input:          ti,
		ContextInfo:    contextInfo,
		ContextContent: contextContent,
		Options:        []string{"Copy", "Explain", "Edit", "Cancel"},
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
	switch msg := msg.(type) {
	case SuggestionMsg:
		m.Suggestion = string(msg)
		m.Explanation = "" // Clear previous if any
		m.State = StateSuggestion
		
		// Parse Markdown Code Blocks
		// Use regex to find all code blocks and take the last one
		// Regex: (?s)```(.*?)``` matches balanced backticks, dot matches newlines
		re := regexp.MustCompile("(?s)```(.*?)```")
		matches := re.FindAllStringSubmatch(m.Suggestion, -1)
		
		if len(matches) > 0 {
			// Take the content of the last match (capture group 1)
			lastMatch := matches[len(matches)-1]
			raw := lastMatch[1]
			
			// Trim first line if it's a language identifier
			// Also trim surrounding whitespace
			raw = strings.TrimSpace(raw)
			if idx := strings.Index(raw, "\n"); idx != -1 {
				// Check if the top line is a single word (lang identifier)
				firstLine := raw[:idx]
				if !strings.Contains(firstLine, " ") {
					raw = strings.TrimSpace(raw[idx+1:])
				}
			}
			m.RunnableCommand = raw
		} else {
			m.RunnableCommand = ""
		}
	case ExplanationMsg:
		m.Explanation = string(msg)
		m.State = StateExplained
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
					m.State = StateLoading
					return m, func() tea.Msg {
						res, err := m.RefineFunc(m.Suggestion, refinement, m.ContextContent)
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
			case "up", "k":
				if m.SelectedOption > 0 {
					m.SelectedOption--
				}
			case "down", "j":
				if m.SelectedOption < len(m.Options)-1 {
					m.SelectedOption++
				}
			case "enter":
				return m.handleSelection()
			}
		case StateExplained:
			if msg.String() == "esc" || msg.String() == "q" {
				m.State = StateSuggestion // Go back
			}
		case StateError:
			if msg.String() == "q" || msg.String() == "ctrl+c" {
				return m, tea.Quit
			}
		}
	}
	return m, nil
}

func (m Model) handleSelection() (tea.Model, tea.Cmd) {
	selected := m.Options[m.SelectedOption]
	switch selected {
	case "Copy":
		if m.RunnableCommand == "" {
			m.Err = fmt.Errorf("no executable command found to copy")
			m.State = StateError
			return m, nil
		}
		if err := clipboard.WriteAll(m.RunnableCommand); err != nil {
			m.Err = fmt.Errorf("failed to copy: %v (install wl-clipboard or xclip)", err)
			m.State = StateError
			return m, nil
		}
		return m, tea.Quit
	case "Explain":
		m.State = StateLoading // Show loading while explaining
		return m, func() tea.Msg {
			// Explain either the full text or just the command?
			// Probably the user wants to understand the command if there is one.
			target := m.Suggestion
			if m.RunnableCommand != "" {
				target = m.RunnableCommand
			}
			exp, err := m.ExplainFunc(target, m.ContextContent)
			if err != nil {
				return ErrorMsg(err)
			}
			return ExplanationMsg(exp)
		}
	case "Edit":
		if m.RunnableCommand == "" {
			m.Err = fmt.Errorf("no command to edit")
			m.State = StateError
			return m, nil
		}
		m.State = StateRefining
		m.Input.SetValue("")
		m.Input.Placeholder = "e.g. add a recursive flag"
		m.Input.Focus()
		return m, textinput.Blink
	case "Cancel":
		return m, tea.Quit
	}
	return m, nil
}

func (m Model) View() string {
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
		s.WriteString(btnStyle.Render("[ Attach File ]"))

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
				if start + 5 < end {
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
		
		// Render mixed content
		if m.RunnableCommand != "" && strings.Contains(m.Suggestion, "```") {
			// Split by code blocks
			parts := strings.Split(m.Suggestion, "```")
			for i, part := range parts {
				if i%2 == 1 { // Inside code block
					// Remove valid language identifier if present on first line
					content := part
					if idx := strings.Index(content, "\n"); idx != -1 {
						// e.g. "bash\nls -la" -> "ls -la"
						// But wait, our splitting kept the newlines.
						// Simple heuristic: if first line is single word, drop it.
						firstLine := content[:idx]
						if !strings.Contains(firstLine, " ") {
							content = content[idx+1:]
						}
					}
					s.WriteString(CommandStyle.Render(strings.TrimSpace(content)))
				} else { // Outside code block
					s.WriteString(part)
				}
			}
		} else {
			// No code block or plain text
			if m.RunnableCommand != "" {
				// Fallback if parsing weirdness, just render all as command? 
				// No, if Runnable exists, we probably parsed it.
				// If we are here, likely just plain text response.
				s.WriteString(m.Suggestion)
			} else {
				// Pure text response
				s.WriteString(m.Suggestion)
			}
		}
		
		s.WriteString("\n\n")
		s.WriteString("What next?\n")
		
		for i, opt := range m.Options {
			cursor := " "
			style := ItemStyle
			if m.SelectedOption == i {
				cursor = ">"
				style = SelectedItemStyle
			}
			s.WriteString(style.Render(fmt.Sprintf("%s %s", cursor, opt)) + "\n")
		}

	case StateExplained:
		s.WriteString(TitleStyle.Render("Explanation:"))
		s.WriteString("\n")
		s.WriteString(DescriptionStyle.Render(m.Explanation))
		s.WriteString("\n\n(Press Esc to back)")

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
	for _, m := range matches {
		processed = append(processed, m)
	}
	
	return processed, nil
}
