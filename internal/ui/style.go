package ui

import "github.com/charmbracelet/lipgloss"

var (
	// Colors
	primaryColor   = lipgloss.Color("#7D56F4") // Purple
	secondaryColor = lipgloss.Color("#04B575") // Green
	subtleColor    = lipgloss.Color("#6B6B6B") // Grey
	errorColor     = lipgloss.Color("#FF3333") // Red

	// Text Styles
	TitleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(primaryColor).
			MarginBottom(0)

	CommandStyle = lipgloss.NewStyle().
			Foreground(secondaryColor).
			Padding(0, 3).
			Margin(0, 0).
			Border(lipgloss.RoundedBorder()).
			BorderForeground(primaryColor)

	InactiveCommandStyle = lipgloss.NewStyle().
				Foreground(subtleColor).
				Padding(0, 3).
				Border(lipgloss.RoundedBorder()).
				BorderForeground(subtleColor)

	DescriptionStyle = lipgloss.NewStyle().
				Foreground(subtleColor).
				Italic(true)

	// List Styles
	SelectedItemStyle = lipgloss.NewStyle().
				PaddingLeft(2).
				Foreground(lipgloss.Color("205")).
				Bold(true)

	ItemStyle = lipgloss.NewStyle().
			PaddingLeft(2)

	DirectoryStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("33")). // Blueish
			PaddingLeft(2)
)
