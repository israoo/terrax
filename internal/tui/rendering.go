package tui

import (
	"fmt"

	"github.com/charmbracelet/lipgloss"
)

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
	normalBorder  = lipgloss.NormalBorder()

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
)

// columnStyle returns the appropriate style for a column based on whether it is focused
func columnStyle(focused bool) lipgloss.Style {
	style := lipgloss.NewStyle().
		Padding(1, 2).
		Margin(0, 1)

	if focused {
		return style.
			Border(focusedBorder).
			BorderForeground(primaryColor)
	}

	return style.
		Border(normalBorder).
		BorderForeground(dimColor)
}

// View renders the interface (required by BubbleTea)
func (m Model) View() string {
	if m.width == 0 {
		return "Initializing..."
	}

	// Render the two columns
	commandView := m.renderCommandColumn()
	stackView := m.renderStackColumn()

	// Calculate column widths accounting for borders and margins
	availableWidth := m.width - ColumnOverhead
	colWidth := availableWidth / NumberOfColumns

	// Apply styles and dimensions
	styledCommand := columnStyle(m.focused == CommandColumn).
		Width(colWidth).
		Height(m.height - 4).
		Render(commandView)

	styledStack := columnStyle(m.focused == StackColumn).
		Width(colWidth).
		Height(m.height - 4).
		Render(stackView)

	// Combine columns horizontally
	content := lipgloss.JoinHorizontal(
		lipgloss.Top,
		styledCommand,
		styledStack,
	)

	// Add header and footer
	header := headerStyle.Width(m.width).Render("🌍 " + AppTitle)
	footer := footerStyle.Render(HelpText)

	return lipgloss.JoinVertical(
		lipgloss.Left,
		header,
		content,
		footer,
	)
}

// renderStackColumn renders the stacks column
func (m Model) renderStackColumn() string {
	title := titleStyle.Render("📦 " + StacksTitle)
	content := ""

	for i, stack := range m.stacks {
		cursor := " "
		if i == m.selectedStack {
			cursor = "►"
			stack = selectedItemStyle.Render(stack)
		} else {
			stack = itemStyle.Render(stack)
		}
		content += fmt.Sprintf("%s %s\n", cursor, stack)
	}

	return lipgloss.JoinVertical(lipgloss.Left, title, "", content)
}

// renderCommandColumn renders the commands column
func (m Model) renderCommandColumn() string {
	title := titleStyle.Render("⚡ " + CommandsTitle)
	content := ""

	for i, cmd := range m.commands {
		cursor := " "
		if i == m.selectedCommand {
			cursor = "►"
			cmd = selectedItemStyle.Render(cmd)
		} else {
			cmd = itemStyle.Render(cmd)
		}
		content += fmt.Sprintf("%s %s\n", cursor, cmd)
	}

	return lipgloss.JoinVertical(lipgloss.Left, title, "", content)
}
