package ui

import (
	"fmt"
	"math/rand"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"

	"time"

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
	StateSuccessAnim
	StateSuggestion
	StateExplained
	StateError
	StatePermissionDenied
	StateCopied
)

type CommandLayout struct {
	Y      int
	Height int
}

type Model struct {
	State              State
	PreviousState      State // To return after file prompt
	Question           string
	Input              textinput.Model
	ContextInfo        string // Display string (e.g. "Attached: foo.txt")
	ContextContent     string // Actual content
	PermissionPath     string // Path that failed permission check
	Suggestion         string
	PendingSuggestion  string   // Holds suggestion during success animation
	RunnableCommands   []string // Extracted commands for execution/copy
	ActiveCommandIndex int      // Which command is currently selected
	Explanation        string
	Err                error

	// Animation
	AnimationFrame int

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

	// Scroll Tracking
	CommandLayouts []CommandLayout

	// Viewport
	viewport  viewport.Model
	maxHeight int
	ready     bool
}

func NewModel(question string, contextInfo string, contextContent string, queryFunc func(string, string) (string, error), explainFunc func(string, string) (string, error), refineFunc func(string, string, string) (string, error)) Model {
	initialState := StateLoading
	ti := textinput.New()
	ti.Width = 50

	if question == "" {
		initialState = StateInput
		// Random Placeholders to guide the user
		placeholders := []string{
			"how do I check disk space?",
			"how do I undo the last git commit?",
			"find all large files in /var",
			"convert video.mp4 to gif",
			"check my public IP address",
			"list all running docker containers",
			"compress this directory into a tarball",
			"show me the weather in Tokyo",
			"delete all files older than 7 days",
			"replace 'foo' with 'bar' in all files",
		}
		ti.Placeholder = "e.g. " + placeholders[rand.Intn(len(placeholders))]
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
	var cmds []tea.Cmd
	if m.State == StateInput {
		cmds = append(cmds, textinput.Blink)
	}
	// Always perform query if in loading state (initial state might be loading)
	if m.State == StateLoading {
		cmds = append(cmds,
			func() tea.Msg {
				res, err := m.QueryFunc(m.Question, m.ContextContent)
				if err != nil {
					return ErrorMsg(err)
				}
				return SuggestionMsg(res)
			},
			tick(),
		)
	}
	return tea.Batch(cmds...)
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
			m.viewport.Width = msg.Width
			m.maxHeight = msg.Height - verticalMarginHeight
		}

		m.Input.Width = msg.Width - 4

		// Re-render content with new width
		m.updateViewportContent()

	case TickMsg:
		if m.State == StateLoading {
			m.AnimationFrame++
			return m, tick()
		}

	case CopiedTimeoutMsg:
		return m, tea.Quit

	case SuggestionMsg:
		// Transition to Success Animation
		m.PendingSuggestion = string(msg)
		m.State = StateSuccessAnim
		return m, waitForSuccess()

	case SuccessTimeoutMsg:
		m.Suggestion = m.PendingSuggestion
		m.PendingSuggestion = ""
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
					return m, tea.Batch(
						func() tea.Msg {
							res, err := m.QueryFunc(m.Question, m.ContextContent)
							if err != nil {
								return ErrorMsg(err)
							}
							return SuggestionMsg(res)
						},
						tick(),
					)
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
					return m, tea.Batch(
						func() tea.Msg {
							res, err := m.RefineFunc(currentSuggestion, refinement, m.ContextContent)
							if err != nil {
								return ErrorMsg(err)
							}
							return SuggestionMsg(res)
						},
						tick(),
					)
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
			case "up", "down":
				// Cycle matches
				if len(m.Matches) > 0 {
					direction := 1
					if msg.String() == "up" {
						direction = -1
					}

					m.MatchIndex += direction
					// Wrap around
					if m.MatchIndex < 0 {
						m.MatchIndex = len(m.Matches) - 1
					} else if m.MatchIndex >= len(m.Matches) {
						m.MatchIndex = 0
					}

					current := m.Matches[m.MatchIndex]
					m.Input.SetValue(current)
					m.Input.SetCursor(len(m.Input.Value()))
				}
				return m, nil

			case "tab":
				// If we have suggestions, select the current one first (Drill down)
				if len(m.Matches) > 0 {
					m.Input.SetValue(m.Matches[m.MatchIndex])
					m.Input.SetCursor(len(m.Input.Value()))
				}

				// Refresh / Drill down
				m.Matches = nil // Force refresh
				input := m.Input.Value()
				if input == "" {
					input = ""
				}

				matches, err := getMatches(input)
				if err == nil && len(matches) > 0 {
					m.Matches = matches
					m.MatchIndex = 0

					// Only auto-complete if single match
					if len(matches) == 1 {
						m.Input.SetValue(m.Matches[0])
						m.Input.SetCursor(len(m.Input.Value()))
					}
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
						if os.IsPermission(err) {
							m.PermissionPath = path
							m.State = StatePermissionDenied
							return m, nil
						}
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

					// Restore placeholder
					if m.State == StateRefining {
						m.Input.Placeholder = "Your follow-up question here..."
					} else {
						m.Input.Placeholder = "e.g. how do I check disk space?"
					}

					m.FocusIndex = 0 // Reset focus to input
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
				// Live update of matches
				newMatches, err := getMatches(m.Input.Value())
				// Only update if no error (e.g. invalid path)
				if err == nil {
					m.Matches = newMatches
					// Reset index if matches changed
					m.MatchIndex = 0
				} else {
					m.Matches = nil
				}
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
					m.ensureVisible(m.ActiveCommandIndex)
				}
				return m, nil
			case "shift+tab":
				if len(m.RunnableCommands) > 1 {
					m.ActiveCommandIndex--
					if m.ActiveCommandIndex < 0 {
						m.ActiveCommandIndex = len(m.RunnableCommands) - 1
					}
					m.updateViewportContent()
					m.ensureVisible(m.ActiveCommandIndex)
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

		case StatePermissionDenied:
			switch msg.String() {
			case "y", "Y":
				// Run sudo cat
				return m, sudoRead(m.PermissionPath)
			case "n", "N", "esc":
				m.State = StateFilePrompt
				m.PermissionPath = ""
				return m, nil
			case "ctrl+c":
				return m, tea.Quit
			}
		}
	}

	switch msg := msg.(type) {
	case SudoReadMsg:
		if msg.Err != nil {
			m.Err = fmt.Errorf("sudo failed: %v", msg.Err)
			m.State = StateError
			return m, nil
		}

		// Read temp file
		b, err := os.ReadFile(msg.ContentPath)
		// Clean up
		os.Remove(msg.ContentPath)

		if err != nil {
			m.Err = fmt.Errorf("read temp error: %v", err)
			m.State = StateError
			return m, nil
		}

		// Success - append content
		m.ContextContent += fmt.Sprintf("\n--- File: %s ---\n%s\n", m.PermissionPath, string(b))
		if m.ContextInfo == "" {
			m.ContextInfo = m.PermissionPath
		} else {
			m.ContextInfo += ", " + m.PermissionPath
		}

		// Return to previous state
		m.State = m.PreviousState
		m.Input.SetValue("")

		// Restore placeholder
		if m.State == StateRefining {
			m.Input.Placeholder = "Your follow-up question here..."
		} else {
			m.Input.Placeholder = "e.g. how do I check disk space?"
		}

		m.FocusIndex = 0
		m.Input.Focus()
		return m, nil
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
		m.State = StateCopied
		return m, waitForCopy()
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
		m.Input.Placeholder = "Your follow-up question here..."
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

	// Clear positions
	m.CommandLayouts = nil

	if m.State == StateSuggestion {
		if len(m.RunnableCommands) > 0 && strings.Contains(m.Suggestion, "```") {
			// Split by code blocks
			parts := strings.Split(m.Suggestion, "```")
			cmdIndex := 0
			// Track current line count
			currentLine := 0

			for i, part := range parts {
				if i%2 == 1 { // Inside code block
					if cmdIndex < len(m.RunnableCommands) {
						cmdVal := m.RunnableCommands[cmdIndex]
						style := InactiveCommandStyle
						if cmdIndex == m.ActiveCommandIndex {
							style = CommandStyle
						}

						// Render command
						renderedCmd := style.Render(cmdVal)
						content.WriteString(renderedCmd)

						// Record position
						h := lipgloss.Height(renderedCmd)
						m.CommandLayouts = append(m.CommandLayouts, CommandLayout{Y: currentLine, Height: h})

						// Advance line count
						currentLine += h

						cmdIndex++
					} else {
						// Fallback if mismatched
						rendered := wordwrap.String(InactiveCommandStyle.Render(part), m.viewport.Width)
						content.WriteString(rendered)
						currentLine += lipgloss.Height(rendered)
					}
				} else { // Outside code block
					rendered := wordwrap.String(part, m.viewport.Width)
					content.WriteString(rendered)
					currentLine += lipgloss.Height(rendered)
				}
			}
		} else {
			rendered := wordwrap.String(m.Suggestion, m.viewport.Width)
			content.WriteString(rendered)
			// Assuming single command if any (but usually logic above handles it)
			// If purely text, no commands positions to track unless we parse implicit logic?
			// The original logic just dumps string.
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

func (m *Model) ensureVisible(index int) {
	if index < 0 || index >= len(m.CommandLayouts) {
		return
	}

	layout := m.CommandLayouts[index]

	// Check if top is above viewport
	if layout.Y < m.viewport.YOffset {
		m.viewport.SetYOffset(layout.Y)
	} else if layout.Y+layout.Height > m.viewport.YOffset+m.viewport.Height {
		// Check if bottom is below viewport
		// Try to align bottom of command with bottom of viewport
		targetY := layout.Y + layout.Height - m.viewport.Height
		// But don't scroll past top if command is taller than viewport?
		// If command is taller than viewport, align top.
		if layout.Height > m.viewport.Height {
			m.viewport.SetYOffset(layout.Y)
		} else {
			m.viewport.SetYOffset(targetY)
		}
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
				// Show relative path or just basename? Full path is clearer for now.
				cursorStyle := ItemStyle
				textStyle := ItemStyle

				if strings.HasSuffix(match, "/") {
					textStyle = DirectoryStyle
				}

				if i == m.MatchIndex {
					cursor = ">"
					cursorStyle = SelectedItemStyle
					textStyle = SelectedItemStyle // Selected overrides style? Or maybe keep blue but bold?
					// Let's keep selected style for text to ensure visibility, maybe bold blue?
					if strings.HasSuffix(match, "/") {
						textStyle = SelectedItemStyle.Copy().Foreground(lipgloss.Color("33"))
					}
				}

				s.WriteString(cursorStyle.Render(cursor) + " " + textStyle.Render(match) + "\n")
			}

			if len(m.Matches) > 5 {
				s.WriteString(ItemStyle.Render("..."))
			}
		}

	case StateLoading:
		// Improved Robot Animation
		// Frames: Center, Left, Right, Blink
		eyeColor := primaryColor
		if m.Explanation != "" {
			eyeColor = secondaryColor // Green eyes when explaining/found
		}

		eyeStyle := lipgloss.NewStyle().Foreground(eyeColor).Bold(true)
		bodyStyle := lipgloss.NewStyle().Foreground(subtleColor)

		// Base parts
		top := bodyStyle.Render("      /----\\")
		bot := bodyStyle.Render("      \\____/")

		// Dynamic parts
		var eyes string

		// 4-frame cycle
		step := m.AnimationFrame % 4
		switch step {
		case 0: // Center
			eyes = fmt.Sprintf("|%s  %s|", eyeStyle.Render("O"), eyeStyle.Render("O"))
		case 1: // Look Left
			eyes = fmt.Sprintf("|%s   |", eyeStyle.Render("O"))
		case 2: // Look Right
			eyes = fmt.Sprintf("|   %s|", eyeStyle.Render("O"))
		case 3: // Blink
			eyes = fmt.Sprintf("|%s  %s|", eyeStyle.Render("-"), eyeStyle.Render("-"))
		}

		// Add antenna bobbing
		antenna := " "
		if step%2 == 0 {
			antenna = bodyStyle.Render("        |")
		} else {
			antenna = bodyStyle.Render("       \\|/") // Wiggle
		}

		robot := fmt.Sprintf("%s\n%s\n      %s\n%s", antenna, top, eyes, bot)

		if m.Explanation == "" && m.Suggestion == "" {
			s.WriteString(fmt.Sprintf("Thinking about: %s...", m.Question))
			if m.ContextInfo != "" {
				s.WriteString(fmt.Sprintf("\n(Context: %s)", m.ContextInfo))
			}
			s.WriteString("\n")
			s.WriteString(robot)
		} else {
			s.WriteString("Explaining...\n")
			s.WriteString(robot)
		}

	case StateSuccessAnim:
		// Success Robot
		eyeStyle := lipgloss.NewStyle().Foreground(secondaryColor).Bold(true)
		bodyStyle := lipgloss.NewStyle().Foreground(subtleColor)

		top := bodyStyle.Render("       /----\\")
		bot := bodyStyle.Render("       \\____/")
		eyes := fmt.Sprintf("|%s  %s|", eyeStyle.Render("^"), eyeStyle.Render("^"))
		sparkles := lipgloss.NewStyle().Foreground(lipgloss.Color("220")).Render("    !!        !!")

		robot := fmt.Sprintf("%s\n%s \n       %s\n%s", sparkles, top, eyes, bot)

		s.WriteString(fmt.Sprintf("Thinking about: %s...\n", m.Question))
		s.WriteString(robot)

	case StateSuggestion:
		s.WriteString(TitleStyle.Render("Suggestion:"))
		s.WriteString("\n")

		s.WriteString(m.viewport.View())

		// Scroll Indicators
		var hints []string
		if !m.viewport.AtTop() {
			hints = append(hints, "↑ More above")
		}
		if !m.viewport.AtBottom() {
			hints = append(hints, "↓ More below")
		}

		if len(hints) > 0 {
			s.WriteString(lipgloss.NewStyle().Foreground(subtleColor).Render("\n(" + strings.Join(hints, " | ") + ")"))
		} else {
			s.WriteString("\n")
		}

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
		s.WriteString(lipgloss.NewStyle().Foreground(subtleColor).Render("  (<-/-> select, Enter confirm, Arrows scroll)"))

	case StateExplained:
		s.WriteString(TitleStyle.Render("Explanation:"))
		s.WriteString("\n")

		s.WriteString(m.viewport.View())

	case StateError:
		s.WriteString(TitleStyle.Foreground(errorColor).Render("Error:"))
		s.WriteString("\n")
		s.WriteString(fmt.Sprintf("%v", m.Err))
		s.WriteString("\n\n(Press q to quit)")

	case StatePermissionDenied:
		s.WriteString(TitleStyle.Foreground(errorColor).Render("Permission Denied"))
		s.WriteString("\n\n")
		s.WriteString(fmt.Sprintf("Could not read '%s'.\nTry reading with sudo? (y/n)", m.PermissionPath))

	case StateCopied:
		s.WriteString("\n")
		s.WriteString(TitleStyle.Copy().Foreground(secondaryColor).Render("  ✓ Copied to clipboard!"))
		s.WriteString("\n\n")
		s.WriteString(lipgloss.NewStyle().Foreground(subtleColor).Render("  (Quitting...)"))
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
type TickMsg time.Time
type SuccessTimeoutMsg time.Time
type CopiedTimeoutMsg time.Time

type SudoReadMsg struct {
	Err         error
	ContentPath string
}

func tick() tea.Cmd {
	return tea.Tick(time.Millisecond*200, func(t time.Time) tea.Msg {
		return TickMsg(t)
	})
}

func waitForSuccess() tea.Cmd {
	return tea.Tick(time.Millisecond*250, func(t time.Time) tea.Msg {
		return SuccessTimeoutMsg(t)
	})
}

func waitForCopy() tea.Cmd {
	return tea.Tick(time.Millisecond*800, func(t time.Time) tea.Msg {
		return CopiedTimeoutMsg(t)
	})
}

func getMatches(pattern string) ([]string, error) {
	// Expand ~
	if strings.HasPrefix(pattern, "~") {
		home, err := os.UserHomeDir()
		if err == nil {
			pattern = filepath.Join(home, pattern[1:])
		}
	}

	dir, file := filepath.Split(pattern)
	if dir == "" {
		dir = "."
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}

	var matches []string
	for _, e := range entries {
		name := e.Name()
		// Skip hidden files unless typed explicitly?
		// User said "looks for typed string anywhere".
		// Usually hidden files are skipped unless pattern starts with .
		if strings.HasPrefix(name, ".") && !strings.HasPrefix(file, ".") {
			continue
		}

		if strings.Contains(strings.ToLower(name), strings.ToLower(file)) {
			fullPath := filepath.Join(dir, name)
			if e.IsDir() {
				fullPath += string(os.PathSeparator)
			}
			matches = append(matches, fullPath)
		}
	}

	return matches, nil
}

func sudoRead(path string) tea.Cmd {
	return func() tea.Msg {
		// Create temp file
		f, err := os.CreateTemp("", "huh-sudo-*")
		if err != nil {
			return SudoReadMsg{Err: err}
		}
		f.Close()

		// Use sh to capture output
		// sudo cat <path> > <temp>
		// We use sh -c to allow IO redirection
		cmdStr := fmt.Sprintf("sudo cat %q > %q", path, f.Name())
		c := exec.Command("sh", "-c", cmdStr)

		// Connect stderr to capture sudo prompt
		c.Stderr = os.Stderr
		c.Stdout = os.Stderr // Just in case, but usually cat output goes to file
		c.Stdin = os.Stdin

		return tea.ExecProcess(c, func(err error) tea.Msg {
			return SudoReadMsg{Err: err, ContentPath: f.Name()}
		})()
	}
}
