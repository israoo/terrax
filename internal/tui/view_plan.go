package tui

import (
	"fmt"
	"path/filepath"
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

	// Render Header
	renderStats := func(stats plan.StackStats) string {
		var addStr, changeStr, destroyStr string

		if stats.Add > 0 {
			addStr = addStyle.Render(fmt.Sprintf("+%d", stats.Add))
		} else {
			addStr = fmt.Sprintf("+%d", stats.Add)
		}

		if stats.Change > 0 {
			changeStr = changeStyle.Render(fmt.Sprintf("~%d", stats.Change))
		} else {
			changeStr = fmt.Sprintf("~%d", stats.Change)
		}

		if stats.Destroy > 0 {
			destroyStr = destroyStyle.Render(fmt.Sprintf("-%d", stats.Destroy))
		} else {
			destroyStr = fmt.Sprintf("-%d", stats.Destroy)
		}

		return fmt.Sprintf("%s %s %s", addStr, changeStr, destroyStr)
	}

	targetStr := renderStats(m.planTargetStats)
	depStr := renderStats(m.planDependencyStats)

	b.WriteString(planHeaderStyle.Render("Execution plan:"))
	b.WriteString("\n")
	b.WriteString(fmt.Sprintf("Target: %s\n", targetStr))
	b.WriteString(fmt.Sprintf("Deps:   %s\n\n", depStr))

	// Render Tree
	if len(m.planFlatItems) == 0 {
		return "No changes to display."
	}

	start, end := m.calculateVisibleRange(len(m.planFlatItems), m.planListCursor, m.height-6)

	for i := start; i < end; i++ {
		node := m.planFlatItems[i]

		// Calculate indentation level based on path depth
		// A simple heuristic: count separators in relative path, or use pre-calculated depth?
		// Since we have the flat items but not explicit depth in TreeNode struct, we can infer it.
		// Or strictly, we should have stored depth in flattenTree.
		// For now, let's count separators in Path.
		// Note from BuildTree: Path is relative to project root (or as built).
		// Name is just the segment.

		// To properly draw the tree lines (└, ├), we need context of siblings, which flat view loses.
		// For a simple first version, indentation by depth is sufficient.
		// Depth = number of separators in node.Path?
		// Actually, node.Path is built accumulatively.
		depth := strings.Count(node.Path, string(filepath.Separator))

		indent := strings.Repeat("  ", depth)

		// Symbol based on node stats
		icon := " "
		if node.HasChanges {
			if node.Stats.Add > 0 {
				icon = "+"
			} else if node.Stats.Destroy > 0 {
				icon = "-"
			} else if node.Stats.Change > 0 {
				icon = "~"
			}
			// If mix, prioritize destructive/transformative?
			if node.Stats.Add > 0 && node.Stats.Destroy > 0 {
				icon = "~"
			}
		}

		// Styling based on change type
		// If it's a directory (no Stack), maybe different color?
		var itemStyle lipgloss.Style
		if node.Stack == nil {
			// Directory
			itemStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("252")) // White/Gray
		} else {
			// Leaf
			if node.Stats.Add > 0 {
				itemStyle = addStyle
			} else if node.Stats.Destroy > 0 {
				itemStyle = destroyStyle
			} else {
				itemStyle = changeStyle
			}
		}

		// Apply selection style
		lineContent := fmt.Sprintf("%s%s %s", indent, icon, node.Name)
		if node.Stack != nil && node.Stack.IsDependency {
			lineContent += " [dep]"
		}

		if i == m.planListCursor {
			// Selected item gets reversed colors or highlight
			b.WriteString(planSelectedItemStyle.Render(lineContent))
		} else {
			// Unselected item uses its semantic color
			// We need to apply the color to the text, but not background
			// Actually planSelectedItemStyle sets background.
			// planUnselectedItemStyle sets basic foreground.
			// We want semantic color if unselected.
			b.WriteString(itemStyle.Render(lineContent))
		}
		b.WriteString("\n")
	}

	return b.String()
}

func (m Model) renderPlanDetailView() string {
	if m.planListCursor < 0 || m.planListCursor >= len(m.planFlatItems) {
		return "Select an item to view details"
	}

	node := m.planFlatItems[m.planListCursor]
	var b strings.Builder

	b.WriteString(planHeaderStyle.Render(fmt.Sprintf("Plan: %s", node.Path)))
	b.WriteString("\n")
	b.WriteString(fmt.Sprintf("Add: %d, Change: %d, Destroy: %d\n\n",
		node.Stats.Add, node.Stats.Change, node.Stats.Destroy))

	if !node.HasChanges {
		b.WriteString("No changes.")
		return b.String()
	}

	if node.Stack != nil {
		// Render Leaf Stack Details
		for _, rc := range node.Stack.ResourceChanges {
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

			line := fmt.Sprintf("%s %s (%s)", prefix, rc.Address, rc.Type)
			b.WriteString(style.Render(line))
			b.WriteString("\n")
		}
	} else {
		// Render Directory Summary
		b.WriteString("Directory Summary:\n\n")

		// Find children with changes
		// Since we don't have direct children pointer easily in flat view, we rely on the node structure which IS preserved.
		// node is a *TreeNode, so it has .Children
		for _, child := range node.Children {
			if child.HasChanges {
				var style lipgloss.Style
				if child.Stats.Add > 0 {
					style = addStyle
				} else if child.Stats.Destroy > 0 {
					style = destroyStyle
				} else {
					style = changeStyle
				}

				line := fmt.Sprintf("- %s (Add: %d, Change: %d, Destroy: %d)",
					child.Name, child.Stats.Add, child.Stats.Change, child.Stats.Destroy)
				b.WriteString(style.Render(line))
				b.WriteString("\n")
			}
		}
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
