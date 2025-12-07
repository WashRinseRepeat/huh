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
		MarginBottom(1)

	CommandStyle = lipgloss.NewStyle().
		Foreground(secondaryColor).
		Padding(1, 2).
		Border(lipgloss.RoundedBorder()).
		BorderForeground(primaryColor)

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
)
