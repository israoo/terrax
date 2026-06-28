package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
)

var (
	dashTabActiveStyle   = lipgloss.NewStyle().Bold(true).Foreground(secondaryColor).Underline(true)
	dashTabInactiveStyle = lipgloss.NewStyle().Foreground(dimColor)
	dashBottomLineStyle  = lipgloss.NewStyle().Foreground(textColor)
)

// renderDashboardBottom renders the tabbed bottom panel. //nolint:unused
func (m Model) renderDashboardBottom(width, height int) string {
	tabBar := m.renderBottomTabBar()
	sep := strings.Repeat("─", width)

	var content string
	switch m.dashActiveTab {
	case 0:
		content = m.renderDepsTab(width, height-2)
	case 1:
		content = m.renderLastRunTab(width, height-2)
	case 2:
		content = m.renderPlanDiffTab(width, height-2)
	}

	return tabBar + "\n" + sep + "\n" + content
}

// renderBottomTabBar renders the tab selector row.
func (m Model) renderBottomTabBar() string {
	tabs := []string{"Dependencies", "Last Run", "Plan Diff"}
	parts := make([]string, len(tabs))
	for i, t := range tabs {
		if i == m.dashActiveTab {
			parts[i] = dashTabActiveStyle.Render(fmt.Sprintf("[%s]", t))
		} else {
			parts[i] = dashTabInactiveStyle.Render(fmt.Sprintf(" %s ", t))
		}
	}
	return strings.Join(parts, "  ")
}

// renderDepsTab renders the Dependencies tab content.
func (m Model) renderDepsTab(width, height int) string {
	if len(m.dashTreeNodes) == 0 {
		return dashTabInactiveStyle.Render("No stack selected.")
	}
	node := m.dashTreeNodes[m.dashTreeCursor].Node

	var lines []string
	if len(node.Dependencies) == 0 && len(node.Dependents) == 0 {
		lines = append(lines, dashTabInactiveStyle.Render("No dependencies."))
	}
	for _, dep := range node.Dependencies {
		lines = append(lines, dashBottomLineStyle.Render("↑ "+truncateText(dep, width-4)))
	}
	for _, dep := range node.Dependents {
		lines = append(lines, dashBottomLineStyle.Render("← "+truncateText(dep, width-4)))
	}

	start := m.dashBottomOffset
	end := min(start+height, len(lines))
	if start >= len(lines) {
		start = 0
		end = min(height, len(lines))
	}
	return strings.Join(lines[start:end], "\n")
}

// renderLastRunTab renders the Last Run tab content.
func (m Model) renderLastRunTab(width, height int) string {
	if m.dashLastRun == nil {
		return dashTabInactiveStyle.Render("No executions yet.")
	}
	e := m.dashLastRun
	dur := time.Duration(e.DurationS * float64(time.Second)).Round(time.Millisecond)
	status := "✓"
	if e.ExitCode != 0 {
		status = fmt.Sprintf("✗ (exit %d)", e.ExitCode)
	}
	lines := []string{
		dashBottomLineStyle.Render(fmt.Sprintf("Command  : %s", e.Command)),
		dashBottomLineStyle.Render(fmt.Sprintf("Status   : %s", status)),
		dashBottomLineStyle.Render(fmt.Sprintf("Duration : %s", dur)),
		dashBottomLineStyle.Render(fmt.Sprintf("Time     : %s", e.Timestamp.Format("2006-01-02 15:04:05"))),
		dashBottomLineStyle.Render(fmt.Sprintf("Stack    : %s", truncateText(e.AbsolutePath, width-12))),
	}
	start := m.dashBottomOffset
	end := min(start+height, len(lines))
	if start >= len(lines) {
		start = 0
		end = min(height, len(lines))
	}
	return strings.Join(lines[start:end], "\n")
}

// renderPlanDiffTab renders the Plan Diff tab content.
func (m Model) renderPlanDiffTab(width, height int) string {
	if m.dashPlanReport == nil {
		return dashTabInactiveStyle.Render("No plan available — run 'plan' first.")
	}

	var lines []string
	for _, result := range m.dashPlanReport.Stacks {
		if result.Stats.Add+result.Stats.Change+result.Stats.Destroy == 0 {
			continue
		}
		summary := fmt.Sprintf("+%d ~%d -%d  %s",
			result.Stats.Add, result.Stats.Change, result.Stats.Destroy,
			truncateText(result.StackPath, width-20))
		lines = append(lines, dashBottomLineStyle.Render(summary))
	}
	if len(lines) == 0 {
		lines = []string{dashTabInactiveStyle.Render("No changes in plan.")}
	}

	start := m.dashBottomOffset
	end := min(start+height, len(lines))
	if start >= len(lines) {
		start = 0
		end = min(height, len(lines))
	}
	return strings.Join(lines[start:end], "\n")
}
