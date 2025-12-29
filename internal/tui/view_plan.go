package tui

import (
	"fmt"
	"path/filepath"
	"sort"
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
			Bold(true)

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
	// Total width = width
	// Master Block = masterWidth + 2 (border)
	// Margin = 1
	// Detail Block = detailWidth + 2 (padding) + 2 (border)
	// Constraint: (masterWidth + 2) + 1 + (detailWidth + 4) <= width
	// detailWidth <= width - masterWidth - PlanDetailMargin
	masterWidth := m.width / PlanMasterWidthRatio
	detailWidth := m.width - masterWidth - PlanDetailMargin

	if detailWidth < PlanMinDetailWidth {
		detailWidth = PlanMinDetailWidth // Safety floor
	}

	return lipgloss.JoinHorizontal(
		lipgloss.Top,
		planMasterStyle.Width(masterWidth).Height(m.height-PlanVerticalFrame).Render(masterView),
		planDetailStyle.Width(detailWidth).Height(m.height-PlanVerticalFrame).Render(detailView),
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
	b.WriteString(fmt.Sprintf("Target: %s | Deps: %s\n\n", targetStr, depStr))

	// Render Tree
	if len(m.planFlatItems) == 0 {
		return "No changes to display."
	}

	start, end := calculateVisibleRange(len(m.planFlatItems), m.planListCursor, m.height-PlanContentFrame)

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

			line := fmt.Sprintf("%s\u00A0%s (%s)", prefix, rc.Address, rc.Type)
			b.WriteString(style.Render(line))
			b.WriteString("\n")

			// Render attribute changes
			attrDiff := renderAttributes(rc)
			if attrDiff != "" {
				// Add extra newline as requested
				b.WriteString("\n")
				b.WriteString(attrDiff)
				b.WriteString("\n")
			}
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

// renderAttributes generates a diff string for resource attributes
func renderAttributes(rc plan.ResourceChange) string {
	var b strings.Builder
	indent := "    "

	// Helper to safely cast interface{} to map[string]interface{}
	toMap := func(i interface{}) map[string]interface{} {
		if m, ok := i.(map[string]interface{}); ok {
			return m
		}
		return make(map[string]interface{})
	}

	before := toMap(rc.Before)
	after := toMap(rc.After)
	unknown := toMap(rc.Unknown)

	// Collect all keys
	keys := make(map[string]bool)
	for k := range before {
		keys[k] = true
	}
	for k := range after {
		keys[k] = true
	}
	for k := range unknown {
		keys[k] = true
	}

	// Sort keys for deterministic output
	var sortedKeys []string
	for k := range keys {
		sortedKeys = append(sortedKeys, k)
	}
	sort.Strings(sortedKeys)

	// Styles for attributes depend on the change type context
	// User requested "white" (neutral) for Create/Delete resource details
	createAttrStyle := addStyle
	deleteAttrStyle := destroyStyle

	if rc.ChangeType == plan.ChangeTypeCreate {
		createAttrStyle = lipgloss.NewStyle() // Default white
	}
	if rc.ChangeType == plan.ChangeTypeDelete {
		deleteAttrStyle = lipgloss.NewStyle() // Default white
	}

	for _, k := range sortedKeys {
		vBefore, inBefore := before[k]
		vAfter, inAfter := after[k]
		isUnknown := false

		// Check if key is unknown/computed
		// In Terraform JSON, unknown values are represented in 'after_unknown' (mapped to Unknown here)
		// as Boolean true (or sometimes other structures for nested types, but simple fields are true).
		if val, ok := unknown[k]; ok {
			if bVal, isBool := val.(bool); isBool && bVal {
				isUnknown = true
			}
		}

		// Skip internal attributes or noise if needed
		if strings.HasPrefix(k, "_") {
			continue // Optional: skip internal fields if not relevant
		}

		if inBefore && isUnknown {
			// Update to Unknown: value -> (known after apply)
			line := fmt.Sprintf("%s%s: %v -> (known after apply)", indent, k, vBefore)
			b.WriteString(changeStyle.Render(line))
			b.WriteString("\n")
		} else if !inBefore && isUnknown {
			// Add Unknown: (known after apply)
			// For Create, this should also be neutral? "known after apply" is yellow usually.
			// But user said "detalles ... en color blanco".
			// Let's keep "known after apply" highlighted as it's special state?
			// Or should it be white too?
			// Usually "known after apply" is arguably a value.
			// Let's keep addStyle logic (which uses createAttrStyle now).

			// Actually, let's stick to yellow for "known after apply" as it implies "computed".
			// The user specific complaint was about add/delete green/red.
			// Re-reading user request: "deberían mostrarse en color blanco (o sea sin color verde o rojo)"
			// This specifically targets the green/red aspect.
			// "known after apply" is handled by my previous fix to be yellow (ChangeStyle) or addStyle.

			// If I use addStyle for the known-after-apply line in a Create resource, it's green.
			// Should I make it white?
			// Standard behavior for `(known after apply)` is usually distinctive.
			// Let's assume keep distinct logic for Unknown.

			// If I use createAttrStyle (white), it loses the distinction?
			// But previous task I made it ChangeStyle (Yellow) or AddStyle (Green).
			// The change "add unknown" used `addStyle`.
			// If I change `addStyle` to `createAttrStyle` here, it becomes white.
			// But known-after-apply might deserve yellow?
			// The previous task specifically asked to make known-after-apply yellow for *Updates* where it looked like a deletion.
			// For a pure Create, valid values are green, known are ...?
			// I'll use createAttrStyle (White) generally, but maybe "known after apply" text itself could be styled?
			// For now, let's use the createAttrStyle to satisfy "no green".

			var prefix string
			if rc.ChangeType != plan.ChangeTypeCreate {
				prefix = "+ "
			}
			line := fmt.Sprintf("%s%s%s: (known after apply)", indent, prefix, k)
			b.WriteString(createAttrStyle.Render(line))
			b.WriteString("\n")
		} else if inBefore && inAfter {
			// Update: check if value changed
			if fmt.Sprintf("%v", vBefore) != fmt.Sprintf("%v", vAfter) {
				line := fmt.Sprintf("%s%s: %v -> %v", indent, k, vBefore, vAfter)
				b.WriteString(changeStyle.Render(line))
				b.WriteString("\n")
			} else {
				// Unchanged
				line := fmt.Sprintf("%s%s: %v", indent, k, vBefore)
				// Neutral style (no color) -> Gray/Dim
				b.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("240")).Render(line))
				b.WriteString("\n")
			}
		} else if inAfter {
			// Create (or added attribute)
			var prefix string
			if rc.ChangeType != plan.ChangeTypeCreate {
				prefix = "+ "
			}
			line := fmt.Sprintf("%s%s%s: %v", indent, prefix, k, vAfter)
			b.WriteString(createAttrStyle.Render(line))
			b.WriteString("\n")
		} else if inBefore {
			// Delete (or removed attribute)
			var prefix string
			if rc.ChangeType != plan.ChangeTypeDelete {
				prefix = "- "
			}
			line := fmt.Sprintf("%s%s%s: %v", indent, prefix, k, vBefore)
			b.WriteString(deleteAttrStyle.Render(line))
			b.WriteString("\n")
		}
	}

	return b.String()
}
