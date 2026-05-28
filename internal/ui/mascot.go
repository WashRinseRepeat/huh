package ui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// MascotState is the persistent corner mascot's emotional/activity state.
type MascotState int

const (
	MascotIdle      MascotState = iota // resting (history menus)
	MascotAttentive                    // watching the user type (input states)
	MascotThinking                     // working (loading)
	MascotExcited                      // just succeeded (success anim, just-copied)
	MascotProud                        // answer ready (suggestion / explained)
	MascotConfused                     // something went wrong (error / permission)
)

// MascotWidth is the column footprint of the rendered mascot, including
// the small left gap. Callers use it to budget the rest of the row.
const MascotWidth = 8

// RenderMascot returns the 4-line styled ASCII mascot for the given state.
// frame is a monotonically increasing tick counter; the mascot uses it for
// blink and look-around cycles.
func RenderMascot(state MascotState, frame int) string {
	bodyStyle := lipgloss.NewStyle().Foreground(subtleColor)
	sparkStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("220")).Bold(true)

	// Default color (overridden per state below)
	eyeStyle := lipgloss.NewStyle().Foreground(primaryColor).Bold(true)

	antenna := bodyStyle.Render("  |  ") // pipe centered over the 6-wide head
	top := bodyStyle.Render(" /----\\")
	bot := bodyStyle.Render(" \\____/")

	leftEye := "o"
	rightEye := "o"
	skipEyes := false // when true, render eyes line entirely from `eyesLine`
	var eyesLine string

	switch state {
	case MascotIdle:
		// Slow blink every ~3 s (15 frames at 200 ms tick)
		if frame%15 == 0 {
			leftEye, rightEye = "-", "-"
		}

	case MascotAttentive:
		// Open eyes, occasional blink
		if frame%20 == 0 {
			leftEye, rightEye = "-", "-"
		}

	case MascotThinking:
		// Slow deliberate "searching" sweep — each pose holds ~800ms so it
		// reads as the mascot looking for something, not nervously twitching.
		// No blink during thinking; blinks here suggest tiredness, not focus.
		pose := (frame / 4) % 4
		switch pose {
		case 0: // forward
			// defaults
		case 1: // looking left
			eyesLine = " |" + eyeStyle.Render("o") + "   |"
			skipEyes = true
		case 2: // forward again
			// defaults
		case 3: // looking right
			eyesLine = " |   " + eyeStyle.Render("o") + "|"
			skipEyes = true
		}
		// Antenna wiggles only when the head is turned — feels like effort.
		if pose == 1 || pose == 3 {
			antenna = bodyStyle.Render(" \\|/ ")
		}

	case MascotExcited:
		eyeStyle = lipgloss.NewStyle().Foreground(secondaryColor).Bold(true)
		// Sparkles bobbing on alternate frames
		if frame%2 == 0 {
			antenna = sparkStyle.Render(" !  ! ")
		} else {
			antenna = sparkStyle.Render("  !!  ")
		}
		leftEye, rightEye = "^", "^"

	case MascotProud:
		eyeStyle = lipgloss.NewStyle().Foreground(secondaryColor).Bold(true)
		leftEye, rightEye = "^", "^"
		// Subtle blink every ~3.6 s
		if frame%18 == 0 {
			leftEye, rightEye = "-", "-"
		}

	case MascotConfused:
		eyeStyle = lipgloss.NewStyle().Foreground(errorColor).Bold(true)
		leftEye, rightEye = "x", "x"
		// Single-character wobble: alternate eye shape so it doesn't feel dead
		if frame%3 == 0 {
			leftEye, rightEye = ">", "<"
		}
	}

	if !skipEyes {
		eyesLine = bodyStyle.Render(" |") +
			eyeStyle.Render(leftEye) +
			bodyStyle.Render("  ") +
			eyeStyle.Render(rightEye) +
			bodyStyle.Render("|")
	}

	return strings.Join([]string{antenna, top, eyesLine, bot}, "\n")
}

// mascotFor maps the app state (and a few sub-state hints) to a mascot state.
func mascotFor(m Model) MascotState {
	switch m.State {
	case StateInput, StateRefining, StateFilePrompt:
		return MascotAttentive
	case StateLoading:
		return MascotThinking
	case StateSuccessAnim:
		return MascotExcited
	case StateSuggestion:
		if m.JustCopiedIndex >= 0 {
			return MascotExcited // wink during the 300ms post-copy window
		}
		return MascotProud
	case StateExplained:
		return MascotProud
	case StateError, StatePermissionDenied:
		return MascotConfused
	case StateHistoryMenu, StateHistoryList:
		return MascotIdle
	}
	return MascotIdle
}

// renderHeader returns a top-of-view block: title (and optional subline)
// on the left, mascot on the right. Falls back to just the title text when
// the terminal is too narrow to fit the mascot.
//
// styledTitle is a pre-styled string (caller applies TitleStyle / colors).
// subline is a raw string rendered in the subtle color (used for inline
// context info like "(Context: foo.txt)" under a title).
func (m Model) renderHeader(styledTitle, subline string) string {
	titleBlock := styledTitle
	if subline != "" {
		titleBlock += "\n" + lipgloss.NewStyle().Foreground(subtleColor).Render(subline)
	}

	// Hide mascot when there isn't enough room or layout isn't ready yet.
	const minWidthForMascot = 60
	if !m.ready || m.viewport.Width < minWidthForMascot {
		return titleBlock
	}

	mascot := RenderMascot(mascotFor(m), m.AnimationFrame)
	// Mascot lives on the LEFT so it stays in the user's gaze on wide
	// terminals. The title is vertically centered against the mascot's
	// 4-line height (Center alignment) so a short title appears at the
	// mascot's eye level — feels like he's looking at the title.
	gap := lipgloss.NewStyle().Width(2).Render("")
	titleWidth := m.viewport.Width - MascotWidth - 2
	if titleWidth < 10 {
		return titleBlock
	}
	paddedTitle := lipgloss.NewStyle().Width(titleWidth).Render(titleBlock)
	return lipgloss.JoinHorizontal(lipgloss.Center, mascot, gap, paddedTitle)
}

