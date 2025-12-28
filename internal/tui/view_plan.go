package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/israoo/terrax/internal/plan"
)

// Styling constants for plan review
var (
	planMasterStyle = lipgloss.NewStyle().
			Border(lipgloss.NormalBorder()).
			BorderForeground(lipgloss.Color("63")).
			MarginRight(1)

	planDetailStyle = lipgloss.NewStyle().
			Border(lipgloss.NormalBorder()).
			BorderForeground(lipgloss.Color("63")).
			Padding(0, 1)

	planHeaderStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("205")).
			Bold(true).
			Padding(0, 1)

	planSelectedItemStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("229")).
				Background(lipgloss.Color("57")).
				Bold(true)

	planUnselectedItemStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("252"))

	addStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("42"))  // Green
	changeStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("214")) // Orange/Yellow
	destroyStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("196")) // Red
)

func (m Model) renderPlanReviewView() string {
	if m.planReport == nil || len(m.planReport.Stacks) == 0 {
		return "No plan results found."
	}

	masterView := m.renderPlanMasterView()
	detailView := m.renderPlanDetailView()

	// Calculate widths
	masterWidth := m.width / 3
	detailWidth := m.width - masterWidth - 4 // border/margin adjustment

	return lipgloss.JoinHorizontal(
		lipgloss.Top,
		planMasterStyle.Width(masterWidth).Height(m.height-2).Render(masterView),
		planDetailStyle.Width(detailWidth).Height(m.height-2).Render(detailView),
	)
}

func (m Model) renderPlanMasterView() string {
	var b strings.Builder

	b.WriteString(planHeaderStyle.Render("Stacks"))
	b.WriteString("\n\n")

	// Calculate visible range for scrolling
	start, end := m.calculateVisibleRange(len(m.planReport.Stacks), m.planListCursor, m.height-6)

	for i := start; i < end; i++ {
		stack := m.planReport.Stacks[i]

		// Icon based on changes
		icon := " "
		if stack.HasChanges {
			if stack.Stats.Destroy > 0 && stack.Stats.Add > 0 {
				icon = "~" // Replace/Mix
			} else if stack.Stats.Destroy > 0 {
				icon = "-"
			} else if stack.Stats.Add > 0 {
				icon = "+"
			} else if stack.Stats.Change > 0 {
				icon = "~"
			}
		}

		// Line content
		line := fmt.Sprintf("%s %s", icon, stack.StackPath)
		if stack.IsDependency {
			line += " [dep]"
		}

		// Styling
		var styledLine string
		if i == m.planListCursor {
			styledLine = planSelectedItemStyle.Render(line)
		} else {
			styledLine = planUnselectedItemStyle.Render(line)
		}

		b.WriteString(styledLine)
		b.WriteString("\n")
	}

	return b.String()
}

func (m Model) renderPlanDetailView() string {
	if m.planListCursor < 0 || m.planListCursor >= len(m.planReport.Stacks) {
		return "Select a stack to view details"
	}

	stack := m.planReport.Stacks[m.planListCursor]
	var b strings.Builder

	b.WriteString(planHeaderStyle.Render(fmt.Sprintf("Plan: %s", stack.StackPath)))
	b.WriteString("\n")
	b.WriteString(fmt.Sprintf("Add: %d, Change: %d, Destroy: %d\n\n",
		stack.Stats.Add, stack.Stats.Change, stack.Stats.Destroy))

	if !stack.HasChanges {
		b.WriteString("No changes.")
		return b.String()
	}

	for _, rc := range stack.ResourceChanges {
		var prefix string
		var style lipgloss.Style

		switch rc.ChangeType {
		case plan.ChangeTypeCreate:
			prefix = "+"
			style = addStyle
		case plan.ChangeTypeDelete:
			prefix = "-"
			style = destroyStyle
		case plan.ChangeTypeUpdate:
			prefix = "~"
			style = changeStyle
		case plan.ChangeTypeReplace:
			prefix = "-/+"
			style = changeStyle
		}

		line := fmt.Sprintf("%s %s (%s)\n", prefix, rc.Address, rc.Type)
		b.WriteString(style.Render(line))
	}

	return b.String()
}

// Helper for generic list scrolling (could be moved to shared util)
func (m Model) calculateVisibleRange(totalItems, cursor, visibleHeight int) (int, int) {
	if totalItems == 0 {
		return 0, 0
	}

	// Simple scrolling logic: keep cursor in middle-ish or ensure visible
	start := 0
	end := totalItems

	if totalItems > visibleHeight {
		if cursor < visibleHeight/2 {
			start = 0
			end = visibleHeight
		} else if cursor >= totalItems-visibleHeight/2 {
			start = totalItems - visibleHeight
			end = totalItems
		} else {
			start = cursor - visibleHeight/2
			end = start + visibleHeight
		}
	}

	return start, end
}
