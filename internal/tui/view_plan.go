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

	// Calculate explicit width for inner content to support wrapping
	masterWidth := m.width / PlanMasterWidthRatio
	detailWidth := m.width - masterWidth - PlanDetailMargin
	// planDetailStyle has Border (2 chars) + Padding(0,1) (2 chars)
	// So inner text width is roughly detailWidth - 4
	innerContentWidth := detailWidth - 4
	if innerContentWidth < 20 {
		innerContentWidth = 20 // minimal safety
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

			// Use JoinHorizontal for hanging indent effect
			// Left column: Prefix
			// Right column: Address + Type (Wrapped)
			prefixView := style.Render(prefix)

			// Right block width: innerContentWidth - len(prefix) - 1 (space)
			// prefix len is 1 or 3 ("-/+"). Let's assume average or dynamic?
			// "-/+" is 3 chars. "+" is 1.
			// Let's give right block the remaining space.
			pLen := len(prefix)
			rightWidth := innerContentWidth - pLen - 1
			if rightWidth < 10 {
				rightWidth = 10
			}

			rightContent := fmt.Sprintf("%s (%s)", rc.Address, rc.Type)
			rightView := style.Width(rightWidth).Render(rightContent)

			// Join with a space
			line := lipgloss.JoinHorizontal(lipgloss.Top, prefixView, " ", rightView)

			b.WriteString(line)
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
	// User requested "white" (neutral) for right side text of Create/Delete resource details
	// But symbols should be colored.
	createAttrStyle := lipgloss.NewStyle() // Default white/neutral
	deleteAttrStyle := lipgloss.NewStyle() // Default white/neutral

	// Prefixes
	prefixAdd := addStyle.Render("+")
	prefixDel := destroyStyle.Render("-")
	// For alignment, we might want consistent spacing.
	// Standard indent is 4 spaces.
	// If we have prefix, maybe we indent 2 spaces + prefix + space?
	// To align with "    key", maybe "  + key"?
	// Let's use standard indent for now as assumed before: "    + key"

	for _, k := range sortedKeys {
		vBefore, inBefore := before[k]
		vAfter, inAfter := after[k]
		isUnknown := false

		// Check if key is unknown/computed
		if val, ok := unknown[k]; ok {
			if bVal, isBool := val.(bool); isBool && bVal {
				isUnknown = true
			}
		}

		// Skip internal attributes or noise if needed
		if strings.HasPrefix(k, "_") {
			continue
		}

		// User asked to remove bold from keys.
		keyStr := k

		if inBefore && isUnknown {
			// Update to Unknown: value -> (known after apply)
			line := fmt.Sprintf("%s%s: %v -> (known after apply)", indent, keyStr, vBefore)
			b.WriteString(changeStyle.Render(line))
			b.WriteString("\n")
		} else if !inBefore && isUnknown {
			// Add Unknown: (known after apply)
			if rc.ChangeType == plan.ChangeTypeCreate {
				// Colored prefix, neutral text
				// use " " after prefix to separate from key
				line := fmt.Sprintf("%s%s %s: (known after apply)", indent, prefixAdd, keyStr)
				b.WriteString(createAttrStyle.Render(line))
			} else {
				// Update adding field -> Green line
				line := fmt.Sprintf("%s+ %s: (known after apply)", indent, keyStr)
				b.WriteString(addStyle.Render(line))
			}
			b.WriteString("\n")
		} else if inBefore && inAfter {
			// Update: check if value changed
			if fmt.Sprintf("%v", vBefore) != fmt.Sprintf("%v", vAfter) {
				line := fmt.Sprintf("%s%s: %v -> %v", indent, keyStr, vBefore, vAfter)
				b.WriteString(changeStyle.Render(line))
				b.WriteString("\n")
			} else {
				// Unchanged
				line := fmt.Sprintf("%s%s: %v", indent, keyStr, vBefore)
				b.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("240")).Render(line))
				b.WriteString("\n")
			}
		} else if inAfter {
			// Create (or added attribute)
			if rc.ChangeType == plan.ChangeTypeCreate {
				// Prefix is colored, text is neutral
				line := fmt.Sprintf("%s%s %s: %v", indent, prefixAdd, keyStr, vAfter)
				b.WriteString(createAttrStyle.Render(line))
			} else {
				// Update adding field -> Green line
				line := fmt.Sprintf("%s+ %s: %v", indent, keyStr, vAfter)
				b.WriteString(addStyle.Render(line))
			}
			b.WriteString("\n")
		} else if inBefore {
			// Delete (or removed attribute)
			if rc.ChangeType == plan.ChangeTypeDelete {
				// Prefix is colored, text is neutral
				line := fmt.Sprintf("%s%s %s: %v", indent, prefixDel, keyStr, vBefore)
				b.WriteString(deleteAttrStyle.Render(line))
			} else {
				// Update removing field -> Red line
				line := fmt.Sprintf("%s- %s: %v", indent, keyStr, vBefore)
				b.WriteString(destroyStyle.Render(line))
			}
			b.WriteString("\n")
		}
	}

	return b.String()
}
