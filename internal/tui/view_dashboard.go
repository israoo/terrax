package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

var (
	dashPanelFocusedStyle = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(primaryColor)
	dashPanelNormalStyle = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(dimColor)
	dashPaletteOverlayStyle = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(secondaryColor).
				Padding(1, 2)
	dashPaletteTitleStyle  = lipgloss.NewStyle().Bold(true).Foreground(secondaryColor)
	dashPaletteSelectStyle = lipgloss.NewStyle().Bold(true).Foreground(accentColor)
	dashPaletteItemStyle   = lipgloss.NewStyle().Foreground(textColor)
)

// renderDashboardView renders the complete StateDashboard layout.
func (m Model) renderDashboardView() string {
	if !m.ready || m.width == 0 || m.height == 0 {
		return Initializing
	}

	header := headerStyle.Width(m.width).Render("🌍 " + AppTitle)
	footer := m.renderDashboardFooter()

	// Layout: topH = total - header(1) - footer(1) - bottom panel.
	bottomH := m.height * 30 / 100
	if bottomH < 6 {
		bottomH = 6
	}
	topH := m.height - 1 - 1 - bottomH // minus header, footer, bottom.
	if topH < 4 {
		topH = 4
	}

	// Tree panel: 25% of width (min 20).
	treeW := m.width * 25 / 100
	if treeW < 20 {
		treeW = 20
	}
	outputW := m.width - treeW

	// Render sub-panels (inner content width = panel width - 2 for borders).
	treeContent := m.renderDashboardTree(treeW-2, topH-2)
	outputContent := m.renderDashboardOutput(outputW-2, topH-2)
	bottomContent := m.renderDashboardBottom(m.width-2, bottomH-2)

	treePanel := m.styleDashPanel(treeContent, treeW, topH, m.dashFocusedPanel == 0)
	outputPanel := m.styleDashPanel(outputContent, outputW, topH, m.dashFocusedPanel == 1)
	topRow := lipgloss.JoinHorizontal(lipgloss.Top, treePanel, outputPanel)

	bottomPanel := m.styleDashPanel(bottomContent, m.width, bottomH, m.dashFocusedPanel == 2)

	body := lipgloss.JoinVertical(lipgloss.Left, topRow, bottomPanel)

	view := lipgloss.JoinVertical(lipgloss.Left, header, body, footer)

	if m.dashPaletteOpen {
		view = m.overlayCommandPalette(view)
	}

	return view
}

// styleDashPanel wraps content in a rounded border coloured by focus state.
func (m Model) styleDashPanel(content string, width, height int, focused bool) string {
	style := dashPanelNormalStyle
	if focused {
		style = dashPanelFocusedStyle
	}
	// Width/Height are the INNER dimensions; Lipgloss adds 1 char border on each side.
	return style.Width(width - 2).Height(height - 2).Render(content)
}

// renderDashboardFooter renders the key-hint footer for StateDashboard.
func (m Model) renderDashboardFooter() string {
	if m.dashRunning {
		return footerStyle.Render("ctrl+c: cancel  tab: focus  q: quit")
	}
	return footerStyle.Render("p:plan  a:apply  i:init  v:validate  d:destroy  space:palette  tab:focus  [/]:tabs  q:quit")
}

// overlayCommandPalette renders the command palette as a centred overlay on top of base.
func (m Model) overlayCommandPalette(base string) string {
	paletteW := m.width / 2
	if paletteW < 30 {
		paletteW = 30
	}

	var selectedNode string
	if len(m.dashTreeNodes) > 0 {
		selectedNode = m.dashTreeNodes[m.dashTreeCursor].Node.Name
	}

	var sb strings.Builder
	sb.WriteString(dashPaletteTitleStyle.Render(fmt.Sprintf("Run on: %s", selectedNode)) + "\n\n")
	for i, cmd := range m.commands {
		if i == m.dashPaletteCursor {
			sb.WriteString(dashPaletteSelectStyle.Render("► "+cmd) + "\n")
		} else {
			sb.WriteString(dashPaletteItemStyle.Render("  "+cmd) + "\n")
		}
	}
	sb.WriteString("\n" + dashPaletteItemStyle.Render("↑↓: navigate  enter: run  esc: cancel"))

	palette := dashPaletteOverlayStyle.Width(paletteW).Render(sb.String())

	// Use lipgloss.Place to centre the overlay over the base view.
	return lipgloss.Place(m.width, m.height,
		lipgloss.Center, lipgloss.Center,
		palette,
		lipgloss.WithWhitespaceChars(" "),
	)
}
