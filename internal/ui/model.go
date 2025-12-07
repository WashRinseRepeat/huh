package ui

import (
	"fmt"
	"strings"

	"github.com/atotto/clipboard"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type State int

const (
	StateLoading State = iota
	StateSuggestion
	StateExplained
	StateError
)

type Model struct {
	State        State
	Question     string
	Suggestion   string
	Explanation  string
	Err          error
	
	// Query
	QueryFunc func() (string, error)

	// Menu
	Options        []string
	SelectedOption int
}

func NewModel(question string, queryFunc func() (string, error)) Model {
	return Model{
		State:          StateLoading,
		Question:       question,
		Options:        []string{"Copy", "Explain", "Edit", "Cancel"},
		SelectedOption: 0,
		QueryFunc:      queryFunc,
	}
}

func (m Model) Init() tea.Cmd {
	return func() tea.Msg {
		res, err := m.QueryFunc()
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
	case ErrorMsg:
		m.Err = msg
		m.State = StateError
		return m, nil
	case tea.KeyMsg:
		switch m.State {
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
		clipboard.WriteAll(m.Suggestion)
		return m, tea.Quit
	case "Explain":
		m.State = StateExplained
		// In real app, trigger explain query here if not already cached
	case "Edit":
		// TODO: Implement Edit mode
		return m, tea.Quit
	case "Cancel":
		return m, tea.Quit
	}
	return m, nil
}

func (m Model) View() string {
	var s strings.Builder

	switch m.State {
	case StateLoading:
		s.WriteString(fmt.Sprintf("Thinking about: %s...", m.Question))
	
	case StateSuggestion:
		s.WriteString(TitleStyle.Render("Suggestion:"))
		s.WriteString("\n")
		s.WriteString(CommandStyle.Render(m.Suggestion))
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
