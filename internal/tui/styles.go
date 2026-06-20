package tui

import "github.com/charmbracelet/lipgloss"

// Styles using lipgloss
var (
	// Colors
	primaryColor   = lipgloss.Color("#7D56F4")
	secondaryColor = lipgloss.Color("#00D9FF")
	accentColor    = lipgloss.Color("#FF6B9D")
	textColor      = lipgloss.Color("#FFFFFF")
	dimColor       = lipgloss.Color("#888888")

	// Column styles
	focusedBorder = lipgloss.RoundedBorder()

	// Header style
	headerStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(textColor).
			Background(primaryColor).
			Padding(0, 1).
			Align(lipgloss.Center)

	// Footer style
	footerStyle = lipgloss.NewStyle().
			Foreground(dimColor).
			Padding(0, 1).
			Italic(true)

	// Column title style
	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(secondaryColor).
			Padding(0, 1)

	// Normal item style
	itemStyle = lipgloss.NewStyle().
			Foreground(textColor).
			Padding(0, 1)

	// Selected item style
	selectedItemStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(accentColor).
				Padding(0, 1)

	// Arrow indicator style
	arrowStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(secondaryColor).
			Padding(0, 1)

	// Breadcrumb bar style (prominent top bar below header)
	breadcrumbBarStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(textColor).
				Background(lipgloss.Color("#2E2E2E")).
				Padding(0, 2).
				Margin(0, 0)

	// Page indicator styles
	pageIndicatorStyle = lipgloss.NewStyle().
				Foreground(dimColor).
				Padding(0, 1)

	activePageIndicatorStyle = lipgloss.NewStyle().
					Foreground(secondaryColor).
					Bold(true).
					Padding(0, 1)
)
