package ui

import (
	"fmt"
	"math/rand"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/WashRinseRepeat/huh/internal/history"
	"github.com/WashRinseRepeat/huh/internal/llm"

	"time"

	"github.com/atotto/clipboard"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/glamour"
	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/reflow/wordwrap"
)

// renderMarkdown renders explanation text via glamour so headers, lists,
// tables, code blocks etc. look like rendered markdown instead of one big
// italic block of "comment". Width is clamped because glamour doesn't
// handle width < 20 gracefully; on render error we fall back to the raw
// text (better something readable than nothing).
func renderMarkdown(src string, width int) string {
	if width < 40 {
		width = 40
	}
	r, err := glamour.NewTermRenderer(
		glamour.WithAutoStyle(),
		glamour.WithWordWrap(width),
	)
	if err != nil {
		return src
	}
	out, err := r.Render(src)
	if err != nil {
		return src
	}
	// Glamour pads with a leading blank line; trim so the title sits flush.
	return strings.TrimLeft(out, "\n")
}

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
	StateHistoryMenu
	StateHistoryList
)

type CommandLayout struct {
	Y      int
	Height int
}

type Model struct {
	State              State
	PreviousState      State // To return after file prompt
	Question           string
	OriginalQuestion   string // First question asked this session; preserved across refines so the refine prompt always knows the user's actual goal
	Input              textinput.Model
	CurrentPlaceholder string
	ContextInfo        string // Display string (e.g. "Attached: foo.txt")
	ContextContent     string // Actual content
	PermissionPath     string // Path that failed permission check
	Suggestion         string
	PendingSuggestion  string   // Holds suggestion during success animation
	RunnableCommands   []string // Extracted commands for execution/copy
	ActiveCommandIndex int      // Which command is currently selected
	JustCopiedIndex    int      // Index of command just copied (-1 = none); shown as ✓ in final render
	SavedInput         string   // Input.Value() preserved across the file-prompt sub-state so the user's question/refinement text isn't lost when they Esc out
	Explanation        string
	Err                error

	// Animation
	AnimationFrame int

	// Query
	QueryFunc   func(string, string) (string, llm.TokenUsage, error)
	ExplainFunc func(string, string) (string, llm.TokenUsage, error)
	// RefineFunc(originalQuestion, original, refinement, dynamicContext) — the
	// originalQuestion is the user's first/anchor question (not the refinement
	// text), so the LLM gets the real goal even after several refine cycles.
	RefineFunc func(string, string, string, string) (string, llm.TokenUsage, error)
	CopyFunc   func(string) error

	// Token usage tracking
	TotalPromptTokens     int
	TotalCompletionTokens int

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

	// History
	HistoryItems     []history.Item
	HistoryListIndex int
}

func NewModel(question string, contextInfo string, contextContent string, queryFunc func(string, string) (string, llm.TokenUsage, error), explainFunc func(string, string) (string, llm.TokenUsage, error), refineFunc func(string, string, string, string) (string, llm.TokenUsage, error)) Model {
	initialState := StateLoading
	ti := textinput.New()
	ti.Width = 50

	placeholders := []string{
		"how do I check disk space?",
		"how do I delete a git branch?",
		"how do I docker compose up?",
		"how do I find files larger than 100MB?",
		"how do I tar a directory?",
		"how do I kill a process on port 3000?",
		"how do I revert the last commit?",
		"how do I watch the logs of a systemd service?",
		"how do I unzip a file?",
		"how do I check my IP address?",
	}
	currentPlaceholder := "e.g. " + placeholders[rand.Intn(len(placeholders))]

	// History logic
	var historyItems []history.Item
	// Only load history if we are starting fresh (no question provided)
	// We might load it anyway just in case? No, saving requires loading, but model doesn't need it unless viewing.
	// But if we want to show the menu, we must check if history exists.
	loadedHistory, _ := history.Load()
	historyItems = loadedHistory

	if question == "" {
		if len(historyItems) > 0 {
			initialState = StateHistoryMenu
		} else {
			initialState = StateInput
			ti.Placeholder = currentPlaceholder
			ti.Focus()
		}
	}

	return Model{
		State:              initialState,
		Question:           question,
		OriginalQuestion:   question, // empty when launched interactively; captured later on first input submit
		Input:              ti,
		HistoryItems:       historyItems,
		CurrentPlaceholder: currentPlaceholder,
		JustCopiedIndex:    -1,
		ContextInfo:    contextInfo,
		ContextContent: contextContent,
		Options:        []string{"Copy", "Explain", "Refine", "Quit"},
		SelectedOption: 0,
		QueryFunc:      queryFunc,
		ExplainFunc:    explainFunc,
		RefineFunc:     refineFunc,
		CopyFunc:       clipboard.WriteAll,
	}
}

func (m Model) Init() tea.Cmd {
	// Always run the mascot animation tick so the corner mascot can blink
	// / look around regardless of which state we land in.
	cmds := []tea.Cmd{tick()}
	if m.State == StateInput {
		cmds = append(cmds, textinput.Blink)
	}
	// Kick off the initial query when launched with a question on the CLI.
	if m.State == StateLoading {
		cmds = append(cmds, func() tea.Msg {
			res, usage, err := m.QueryFunc(m.Question, m.ContextContent)
			if err != nil {
				return ErrorMsg(err)
			}
			return SuggestionMsg{Content: res, Usage: usage}
		})
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
		// Drive the persistent mascot animation regardless of state.
		// Re-schedule the next tick from here so it runs forever (until quit).
		m.AnimationFrame++
		return m, tick()

	case CopiedTimeoutMsg:
		// Only quit if we're still showing the post-copy suggestion view.
		// If the user navigated away (e.g. hit 'r' to refine in the 300ms
		// window), drop the timer and stay where they went.
		if m.State != StateSuggestion || m.JustCopiedIndex < 0 {
			return m, nil
		}
		m.saveToHistory()
		return m, tea.Quit

	case SuggestionMsg:
		// Accumulate token usage
		m.TotalPromptTokens += msg.Usage.PromptTokens
		m.TotalCompletionTokens += msg.Usage.CompletionTokens
		// Transition to Success Animation
		m.PendingSuggestion = msg.Content
		m.State = StateSuccessAnim
		return m, waitForSuccess()

	case SuccessTimeoutMsg:
		m.Suggestion = m.PendingSuggestion
		m.PendingSuggestion = ""
		m.Explanation = "" // Clear previous if any
		m.State = StateSuggestion

		m.RunnableCommands = extractRunnableCommands(m.Suggestion)
		if len(m.RunnableCommands) > 0 {
			m.ActiveCommandIndex = 0
		} else {
			m.ActiveCommandIndex = -1
		}
		m.JustCopiedIndex = -1
		m.updateViewportContent()

	case ExplanationMsg:
		// Accumulate token usage
		m.TotalPromptTokens += msg.Usage.PromptTokens
		m.TotalCompletionTokens += msg.Usage.CompletionTokens
		m.Explanation = msg.Content
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
					m.SavedInput = m.Input.Value() // preserve question across file prompt
					m.State = StateFilePrompt
					m.Input.SetValue("")
					m.Input.Placeholder = "/path/to/file"
					m.Input.Focus()
					return m, textinput.Blink
				}

				m.Question = m.Input.Value()
				if m.Question != "" {
					// Capture the original question on first submit so refine
					// can pass it to the LLM even if the user refines later
					// (which overwrites m.Question with the refinement text).
					if m.OriginalQuestion == "" {
						m.OriginalQuestion = m.Question
					}
					m.PreviousState = StateInput
					m.State = StateLoading
					return m, tea.Batch(
						func() tea.Msg {
							res, usage, err := m.QueryFunc(m.Question, m.ContextContent)
							if err != nil {
								return ErrorMsg(err)
							}
							return SuggestionMsg{Content: res, Usage: usage}
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
					m.SavedInput = m.Input.Value() // preserve refinement text across file prompt
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

					// Pick the refinement scope based on whether the answer
					// is single-command or multi-command:
					//   • 1 command  → send just the bare command (current
					//     fast path, keeps the LLM focused on that string)
					//   • 2+ commands → send the full original answer (prose
					//     and all code blocks) so the LLM can update the
					//     whole sequence in one shot
					//   • 0 commands → fall back to full text (refine should
					//     be blocked here, but be safe)
					var currentSuggestion string
					switch {
					case len(m.RunnableCommands) > 1:
						currentSuggestion = m.Suggestion
					case len(m.RunnableCommands) == 1:
						currentSuggestion = m.RunnableCommands[0]
					default:
						currentSuggestion = m.Suggestion
					}
					// Clear suggestion in model so View() shows "Thinking about..." instead of "Explaining..."
					m.Suggestion = ""

					m.PreviousState = StateRefining
					m.State = StateLoading
					originalQuestion := m.OriginalQuestion
					return m, tea.Batch(
						func() tea.Msg {
							res, usage, err := m.RefineFunc(originalQuestion, currentSuggestion, refinement, m.ContextContent)
							if err != nil {
								return ErrorMsg(err)
							}
							return SuggestionMsg{Content: res, Usage: usage}
						},
						tick(),
					)
				}
			case "esc":
				m.State = StateSuggestion
				return m, nil
			case "ctrl+c":
				m.saveToHistory()
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

					// Return to previous state, restoring the question text
					// the user was typing before they opened the file prompt.
					m.State = m.PreviousState
					m.Input.SetValue(m.SavedInput)
					m.Input.SetCursor(len(m.SavedInput))
					m.SavedInput = ""

					// Restore placeholder
					if m.State == StateRefining {
						m.Input.Placeholder = "Your follow-up question here..."
					} else {
						m.Input.Placeholder = m.CurrentPlaceholder
					}

					m.FocusIndex = 0 // Reset focus to input
					m.Input.Focus()
					return m, nil
				}
			case "esc":
				// Cancel: restore the question text and placeholder, drop
				// the half-typed file path.
				m.State = m.PreviousState
				m.Input.SetValue(m.SavedInput)
				m.Input.SetCursor(len(m.SavedInput))
				m.SavedInput = ""
				if m.State == StateRefining {
					m.Input.Placeholder = "Your follow-up question here..."
				} else {
					m.Input.Placeholder = m.CurrentPlaceholder
				}
				m.FocusIndex = 0
				m.Input.Focus()
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
				m.saveToHistory()
				return m, tea.Quit
			case "tab", "down", "j":
				// Cycle commands forward when there are multiple; otherwise
				// fall back to scrolling so single-command answers still
				// scroll long preamble text with ↓.
				if len(m.RunnableCommands) > 1 {
					m.ActiveCommandIndex = (m.ActiveCommandIndex + 1) % len(m.RunnableCommands)
					m.updateViewportContent()
					m.ensureVisible(m.ActiveCommandIndex)
				} else {
					m.viewport.ScrollDown(1)
				}
				return m, nil
			case "shift+tab", "up", "k":
				if len(m.RunnableCommands) > 1 {
					m.ActiveCommandIndex--
					if m.ActiveCommandIndex < 0 {
						m.ActiveCommandIndex = len(m.RunnableCommands) - 1
					}
					m.updateViewportContent()
					m.ensureVisible(m.ActiveCommandIndex)
				} else {
					m.viewport.ScrollUp(1)
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
			case "pgup", "ctrl+u":
				m.viewport.ScrollUp(m.viewport.Height / 2)
			case "pgdown", "ctrl+d":
				m.viewport.ScrollDown(m.viewport.Height / 2)
			case "enter":
				// If at bottom and scrolling down, maybe Enter on simple text does nothing?
				// But Enter triggers selection.
				return m.handleSelection()
			case "c":
				return m.performCopy()
			case "e":
				// Explain
				m.SelectedOption = 1 // Update selection to match
				m.State = StateLoading
				return m, func() tea.Msg {
					target := m.Suggestion
					if len(m.RunnableCommands) > 0 {
						target = m.RunnableCommands[m.ActiveCommandIndex]
					}
					exp, usage, err := m.ExplainFunc(target, m.ContextContent)
					if err != nil {
						return ErrorMsg(err)
					}
					return ExplanationMsg{Content: exp, Usage: usage}
				}
			case "r":
				// Refine
				if len(m.RunnableCommands) == 0 {
					m.Err = fmt.Errorf("no command to edit")
					m.State = StateError
					return m, nil
				}
				m.SelectedOption = 2 // Update selection to match
				m.State = StateRefining
				m.Input.SetValue("")
				m.Input.Placeholder = "Your follow-up question here..."
				m.Input.Focus()
				return m, textinput.Blink
			}
		case StateExplained:
			switch msg.String() {
			case "esc", "q":
				// Go back to the original answer. Re-render the viewport
				// with the suggestion content (it currently holds the
				// explanation) and reset scroll to the top so the user
				// lands on the answer, not mid-page.
				m.State = StateSuggestion
				m.updateViewportContent()
				m.viewport.GotoTop()
				return m, nil
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
			if msg.String() == "esc" {
				m.State = m.PreviousState
				m.Err = nil
				return m, nil
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

		case StateHistoryMenu:
			switch msg.String() {
			case "up", "k":
				if m.SelectedOption > 0 {
					m.SelectedOption--
				}
				return m, nil
			case "down", "j":
				if m.SelectedOption < 2 {
					m.SelectedOption++
				}
				return m, nil
			case "enter":
				switch m.SelectedOption {
				case 0: // Start new chat
					m.State = StateInput
					m.Input.Focus()
					return m, textinput.Blink
				case 1: // View last answer
					if len(m.HistoryItems) > 0 {
						last := m.HistoryItems[len(m.HistoryItems)-1]
						m.Question = last.Question
						m.OriginalQuestion = last.Question
						m.Suggestion = last.Answer
						m.ContextContent = last.ContextContent
						m.ContextInfo = last.ContextInfo
						m.RunnableCommands = extractRunnableCommands(m.Suggestion)
						if len(m.RunnableCommands) > 0 {
							m.ActiveCommandIndex = 0
						} else {
							m.ActiveCommandIndex = -1
						}
						m.JustCopiedIndex = -1
						m.State = StateSuggestion
						m.SelectedOption = 0 // Reset action bar selection (Copy)
						m.updateViewportContent()
					}
					return m, nil
				case 2: // View history
					m.State = StateHistoryList
					m.HistoryListIndex = 0 // Start at top (newest)
					return m, nil
				}
			case "q", "esc", "ctrl+c":
				return m, tea.Quit
			default:
				// Type-to-start: any printable text (single keystroke or a
				// paste) jumps straight into a new chat with that text as
				// the start of input. The j/k/q vim-style bindings handled
				// above take precedence.
				if len(msg.Runes) >= 1 && msg.Runes[0] >= 0x20 && msg.Runes[0] != 0x7f {
					text := string(msg.Runes)
					m.State = StateInput
					m.Input.SetValue(text)
					m.Input.SetCursor(len(text))
					m.Input.Focus()
					return m, textinput.Blink
				}
			}

		case StateHistoryList:
			switch msg.String() {
			case "up", "k":
				if m.HistoryListIndex > 0 {
					m.HistoryListIndex--
				}
				return m, nil
			case "down", "j":
				if m.HistoryListIndex < len(m.HistoryItems)-1 {
					m.HistoryListIndex++
				}
				return m, nil
			case "enter":
				// Load selected item
				// List is displayed newest first?
				// If index 0 is newest, then item is HistoryItems[len-1-index]
				if len(m.HistoryItems) > 0 {
					idx := len(m.HistoryItems) - 1 - m.HistoryListIndex
					if idx >= 0 && idx < len(m.HistoryItems) {
						item := m.HistoryItems[idx]
						m.Question = item.Question
						m.OriginalQuestion = item.Question
						m.Suggestion = item.Answer
						m.ContextContent = item.ContextContent
						m.ContextInfo = item.ContextInfo
						m.RunnableCommands = extractRunnableCommands(m.Suggestion)
						if len(m.RunnableCommands) > 0 {
							m.ActiveCommandIndex = 0
						} else {
							m.ActiveCommandIndex = -1
						}
						m.JustCopiedIndex = -1
						m.State = StateSuggestion
						m.SelectedOption = 0 // Reset action bar selection
						m.updateViewportContent()
					}
				}
				return m, nil
			case "esc":
				m.State = StateHistoryMenu
				return m, nil
			case "q", "ctrl+c":
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

		// Return to previous state, restoring the user's question/refinement
		// text that we stashed when entering the file prompt.
		m.State = m.PreviousState
		m.Input.SetValue(m.SavedInput)
		m.Input.SetCursor(len(m.SavedInput))
		m.SavedInput = ""

		// Restore placeholder
		if m.State == StateRefining {
			m.Input.Placeholder = "Your follow-up question here..."
		} else {
			m.Input.Placeholder = m.CurrentPlaceholder
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
		return m.performCopy()
	case "Explain":
		m.PreviousState = StateSuggestion
		m.State = StateLoading // Show loading while explaining
		return m, func() tea.Msg {
			target := m.Suggestion
			if len(m.RunnableCommands) > 0 {
				target = m.RunnableCommands[m.ActiveCommandIndex]
			}
			exp, usage, err := m.ExplainFunc(target, m.ContextContent)
			if err != nil {
				return ErrorMsg(err)
			}
			return ExplanationMsg{Content: exp, Usage: usage}
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

	case "Quit":
		m.saveToHistory()
		return m, tea.Quit
	}
	return m, nil
}

func (m Model) saveToHistory() {
	if m.Suggestion == "" {
		return
	}
	item := history.Item{
		Question:       m.Question,
		Answer:         m.Suggestion,
		ContextContent: m.ContextContent,
		ContextInfo:    m.ContextInfo,
		Timestamp:      time.Now(),
	}
	history.Add(item)
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
						if cmdIndex == m.JustCopiedIndex {
							copiedLabel := lipgloss.NewStyle().
								Foreground(secondaryColor).
								Bold(true).
								Render("  ✓ copied")
							renderedCmd = lipgloss.JoinHorizontal(lipgloss.Center, renderedCmd, copiedLabel)
						}
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
			hint := fmt.Sprintf("(Tab to cycle commands - %d/%d)", m.ActiveCommandIndex+1, len(m.RunnableCommands))
			content.WriteString(wordwrap.String(lipgloss.NewStyle().Foreground(subtleColor).Render(hint), m.viewport.Width))
			content.WriteString("\n")
		}
		content.WriteString("\n")
	} else if m.State == StateExplained {
		content.WriteString(renderMarkdown(m.Explanation, m.viewport.Width))
		content.WriteString("\n(↑/↓ scroll · Esc back to answer)")
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
		subline := ""
		if m.ContextInfo != "" {
			subline = fmt.Sprintf("(Context: %s)", m.ContextInfo)
		}
		s.WriteString(m.renderHeader(TitleStyle.Render("What would you like to do?"), subline))
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

		s.WriteString("\n\n(Tab attach · Enter ask · Esc quit)")

	case StateRefining:
		// Title adapts to the scope of refinement: single-command answers
		// edit one command; multi-command answers refine the full sequence.
		title := "How should the command be changed?"
		if len(m.RunnableCommands) > 1 {
			title = "How should the answer be changed?"
		}
		s.WriteString(m.renderHeader(TitleStyle.Render(title), ""))
		s.WriteString("\n\n")

		// Anchor:
		//   • Single command  → box the actual command so it's clear what's
		//     being edited.
		//   • Multi-command   → plain-text scope label only. We don't re-box
		//     all the commands here because the user is coming straight from
		//     the suggestion view and just saw them.
		subtle := lipgloss.NewStyle().Foreground(subtleColor)
		if len(m.RunnableCommands) > 1 {
			question := m.Question
			if len(question) > 60 {
				question = question[:57] + "..."
			}
			s.WriteString(subtle.Render(fmt.Sprintf("Refining all %d commands of your answer to: \"%s\"", len(m.RunnableCommands), question)))
			s.WriteString("\n\n")
		} else {
			anchor := m.Suggestion
			if len(m.RunnableCommands) == 1 {
				anchor = m.RunnableCommands[0]
			}
			s.WriteString(subtle.Render("Editing:"))
			s.WriteString("\n")
			s.WriteString(CommandStyle.Render(anchor))
			s.WriteString("\n\n")
		}

		// Input View
		s.WriteString(m.Input.View())
		s.WriteString("\n\n")

		// Attach Button Logic
		btnStyle := ItemStyle
		if m.FocusIndex == 1 {
			btnStyle = SelectedItemStyle
		}
		s.WriteString(btnStyle.Render("[Attach File]"))

		s.WriteString("\n\n(Tab attach · Enter submit · Esc back)")

	case StateFilePrompt:
		s.WriteString(m.renderHeader(TitleStyle.Render("File to attach:"), ""))
		s.WriteString("\n\n")
		s.WriteString(m.Input.View())
		s.WriteString("\n\n(Tab complete · Enter attach · Esc back)")

		// Render Matches
		if len(m.Matches) > 0 {
			s.WriteString("\n\n")
			s.WriteString(TitleStyle.Render("Suggestions"))
			s.WriteString(lipgloss.NewStyle().Foreground(subtleColor).Render(fmt.Sprintf(" (%d/%d)", m.MatchIndex+1, len(m.Matches))))
			s.WriteString(TitleStyle.Render(":"))
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
		// Loading text plus the corner mascot (which is in MascotThinking).
		// The previously-centered big robot is gone; the persistent mascot
		// now carries the "I'm working" signal.
		var title, subline string
		if m.Explanation == "" && m.Suggestion == "" {
			title = fmt.Sprintf("Thinking about: %s...", m.Question)
			if m.ContextInfo != "" {
				subline = fmt.Sprintf("(Context: %s)", m.ContextInfo)
			}
		} else {
			title = "Explaining..."
		}
		s.WriteString(m.renderHeader(TitleStyle.Render(title), subline))

	case StateSuccessAnim:
		s.WriteString(m.renderHeader(TitleStyle.Render(fmt.Sprintf("Thinking about: %s...", m.Question)), ""))

	case StateSuggestion:
		s.WriteString(m.renderHeader(TitleStyle.Render("Suggestion:"), ""))
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
		// Render Options Horizontally as [C]opy [E]xplain [R]efine [Q]uit.
		// Brackets make the keyboard shortcut visible across all color schemes
		// (underline alone was hard to spot).
		var options []string
		for i, opt := range m.Options {
			style := ItemStyle
			if m.SelectedOption == i {
				style = SelectedItemStyle
			}
			label := "[" + opt[:1] + "]" + opt[1:]
			options = append(options, style.Render(label))
		}
		s.WriteString(lipgloss.JoinHorizontal(lipgloss.Top, options...))
		// Hint varies by whether there are multiple commands to cycle through.
		var hint string
		if len(m.RunnableCommands) > 1 {
			hint = "  (↑/↓ or Tab pick command · ←/→ action · Enter confirm · PgUp/Dn scroll · q quit)"
		} else {
			hint = "  (↑/↓ scroll · ←/→ action · Enter confirm · q quit)"
		}
		s.WriteString(lipgloss.NewStyle().Foreground(subtleColor).Render(hint))

	case StateExplained:
		s.WriteString(m.renderHeader(TitleStyle.Render("Explanation:"), ""))
		s.WriteString("\n")

		s.WriteString(m.viewport.View())

	case StateError:
		s.WriteString(m.renderHeader(TitleStyle.Foreground(errorColor).Render("Error:"), ""))
		s.WriteString("\n")
		s.WriteString(fmt.Sprintf("%v", m.Err))
		s.WriteString("\n\n(Esc back · q quit)")

	case StatePermissionDenied:
		s.WriteString(m.renderHeader(TitleStyle.Foreground(errorColor).Render("Permission Denied"), ""))
		s.WriteString("\n\n")
		s.WriteString(fmt.Sprintf("Could not read '%s'.\nTry reading with sudo? (y/n)", m.PermissionPath))

	case StateHistoryMenu:
		s.WriteString(m.renderHeader(TitleStyle.Render("Welcome back to huh"), ""))
		s.WriteString("\n\n")

		opts := []string{"Start new chat", "View last answer", "View history"}
		for i, opt := range opts {
			cursor := " "
			style := ItemStyle
			if m.SelectedOption == i {
				cursor = ">"
				style = SelectedItemStyle
			}
			s.WriteString(style.Render(fmt.Sprintf("%s %s", cursor, opt)) + "\n")
		}

		s.WriteString("\n(Enter to select · type to start a new chat · Esc to quit)")

	case StateHistoryList:
		s.WriteString(m.renderHeader(TitleStyle.Render("History (Last 10)"), ""))
		s.WriteString("\n\n")

		// Display in reverse order (newest first)
		for i := 0; i < len(m.HistoryItems); i++ {
			// Virtual index i maps to real index len-1-i
			realIdx := len(m.HistoryItems) - 1 - i
			item := m.HistoryItems[realIdx]

			cursor := " "
			style := ItemStyle
			if m.HistoryListIndex == i {
				cursor = ">"
				style = SelectedItemStyle
			}

			// Maybe truncate long questions?
			q := item.Question
			if len(q) > 60 {
				q = q[:57] + "..."
			}

			relTime := relativeTime(item.Timestamp)
			timeStr := lipgloss.NewStyle().Foreground(subtleColor).Render(relTime)
			s.WriteString(style.Render(fmt.Sprintf("%s %s", cursor, q)) + "  " + timeStr + "\n")
		}

		s.WriteString("\n(Enter view · Esc back · q quit)")
	}

	// Token usage footer
	totalTokens := m.TotalPromptTokens + m.TotalCompletionTokens
	if totalTokens > 0 {
		s.WriteString("\n")
		tokenInfo := fmt.Sprintf("tokens: %d in / %d out / %d total", m.TotalPromptTokens, m.TotalCompletionTokens, totalTokens)
		s.WriteString(lipgloss.NewStyle().Foreground(subtleColor).Render(tokenInfo))
	}

	return lipgloss.NewStyle().Margin(1, 1).Render(s.String())
}

// Commands
func SetSuggestion(cmd string) tea.Msg {
	return SuggestionMsg{Content: cmd}
}

type SuggestionMsg struct {
	Content string
	Usage   llm.TokenUsage
}
type ExplanationMsg struct {
	Content string
	Usage   llm.TokenUsage
}
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
	return tea.Tick(time.Millisecond*300, func(t time.Time) tea.Msg {
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
		// Create temp file and keep the handle open so we can wire it
		// directly as the child's stdout — no shell, no quoting concerns.
		f, err := os.CreateTemp("", "huh-sudo-*")
		if err != nil {
			return SudoReadMsg{Err: err}
		}

		c := exec.Command("sudo", "cat", path)
		c.Stdout = f          // cat output goes straight into the temp file
		c.Stderr = os.Stderr  // sudo password prompt
		c.Stdin = os.Stdin    // sudo reads the password from here

		return tea.ExecProcess(c, func(err error) tea.Msg {
			f.Close() // flush before the caller reads it
			return SudoReadMsg{Err: err, ContentPath: f.Name()}
		})()
	}
}

func (m Model) performCopy() (tea.Model, tea.Cmd) {
	if len(m.RunnableCommands) == 0 {
		m.Err = fmt.Errorf("no executable command found to copy")
		m.State = StateError
		return m, nil
	}
	// Copy Active Command
	cmd := m.RunnableCommands[m.ActiveCommandIndex]
	if err := m.CopyFunc(cmd); err != nil {
		m.Err = fmt.Errorf("failed to copy: %v (install wl-clipboard or xclip)", err)
		m.State = StateError
		return m, nil
	}

	// Mark which command was copied and re-render. We stay in StateSuggestion
	// so the final frame left in the terminal shows the full suggestion (with
	// a ✓ next to the copied command), which the user can mouse-select to
	// grab the other commands.
	m.JustCopiedIndex = m.ActiveCommandIndex
	m.updateViewportContent()
	return m, waitForCopy()
}

// relativeTime renders a time.Time as a short, human-readable interval
// like "just now", "5 min ago", "yesterday", or "Jan 2" for older items.
func relativeTime(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	d := time.Since(t)
	switch {
	case d < 0:
		return t.Format("Jan 2")
	case d < time.Minute:
		return "just now"
	case d < time.Hour:
		m := int(d.Minutes())
		if m == 1 {
			return "1 min ago"
		}
		return fmt.Sprintf("%d min ago", m)
	case d < 24*time.Hour:
		h := int(d.Hours())
		if h == 1 {
			return "1 hour ago"
		}
		return fmt.Sprintf("%d hours ago", h)
	case d < 48*time.Hour:
		return "yesterday"
	case d < 7*24*time.Hour:
		return fmt.Sprintf("%d days ago", int(d.Hours()/24))
	default:
		return t.Format("Jan 2")
	}
}

func extractRunnableCommands(suggestion string) []string {
	// Parse Markdown Code Blocks
	// Use regex to find all code blocks
	re := regexp.MustCompile("(?s)```(.*?)```")
	matches := re.FindAllStringSubmatch(suggestion, -1)

	var runnableCommands []string
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
			runnableCommands = append(runnableCommands, raw)
		}
	}
	return runnableCommands
}
