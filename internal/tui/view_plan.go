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
			MarginRight(1).
			PaddingLeft(1)

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

	// Calculate content height properly accounting for Header (1) and Footer (1)
	// We use the full m.height, subtract 2 for header/footer.
	// PlanVerticalFrame currently subtracts for borders.
	// Let's ensure consistency.
	contentAvailableHeight := m.height - 2
	if contentAvailableHeight < 10 {
		contentAvailableHeight = 10 // Min height
	}

	// We pass a modified height to the internal views so they know how much space they have
	// The internal views use m.height - PlanVerticalFrame.
	// We need to adjust PlanVerticalFrame or temporarily shadow m.height?
	// Shadowing m.height in a copy of Model is cleaner, but let's just make sure
	// the styles use `contentAvailableHeight - PlanVerticalFrame`.

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

	// Dynamic styling based on focus
	// Subtract extra 2 lines for header/footer from the height calculation for columns
	columnHeight := contentAvailableHeight - PlanVerticalFrame
	masterStyle := planMasterStyle.Width(masterWidth).Height(columnHeight)
	detailStyle := planDetailStyle.Width(detailWidth).Height(columnHeight)

	if m.planReviewFocusedElement == 0 {
		masterStyle = masterStyle.BorderForeground(lipgloss.Color("205")) // Pink/Active
		detailStyle = detailStyle.BorderForeground(lipgloss.Color("240")) // Dim
	} else {
		masterStyle = masterStyle.BorderForeground(lipgloss.Color("240")) // Dim
		detailStyle = detailStyle.BorderForeground(lipgloss.Color("205")) // Pink/Active
	}

	mainContent := lipgloss.JoinHorizontal(
		lipgloss.Top,
		masterStyle.Render(masterView),
		detailStyle.Render(detailView),
	)

	return lipgloss.JoinVertical(
		lipgloss.Left,
		headerStyle.Width(m.width).Render("🌍 "+AppTitle+" - Plan Viewer"),
		mainContent,
		footerStyle.Render(HelpText),
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
		var itemStyle lipgloss.Style
		if node.Stack == nil {
			// Directory style
			itemStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("252")) // White/Gray
		} else {
			// Leaf style based on change type
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
			// Selected item highlight
			b.WriteString(planSelectedItemStyle.Render(lineContent))
		} else {
			// Unselected item uses semantic color
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
		innerContentWidth = 20
	}

	if node.Stack != nil {
		// Render Leaf Stack Details
		for _, rc := range node.Stack.ResourceChanges {
			var prefix string
			var style lipgloss.Style
			var prefixColorStyle lipgloss.Style

			switch rc.ChangeType {
			case plan.ChangeTypeCreate:
				prefix = "+"
				style = addStyle
				prefixColorStyle = addStyle
			case plan.ChangeTypeDelete:
				prefix = "-"
				style = destroyStyle
				prefixColorStyle = destroyStyle
			case plan.ChangeTypeUpdate:
				prefix = "~"
				style = changeStyle
				prefixColorStyle = changeStyle
			case plan.ChangeTypeReplace:
				prefix = "-/+"
				style = changeStyle
				prefixColorStyle = changeStyle
			}

			// Use JoinHorizontal for hanging indent effect:
			// Left column: Prefix
			// Right column: Address + Type (Wrapped)
			prefixView := prefixColorStyle.Render(prefix)

			// Right block width: innerContentWidth - len(prefix) - 1 (space)
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
				// Add extra newline
				b.WriteString("\n")
				b.WriteString(attrDiff)
				b.WriteString("\n")
			}
		}
	} else {
		// Render Directory Summary
		b.WriteString("Directory Summary:\n\n")

		// Find children with changes
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

	// Handle Vertical Scrolling with Strict Wrapping
	fullContent := b.String()

	// Crucial Fix: Wrap content strictly to width BEFORE splitting lines for viewport
	wrappedContent := lipgloss.NewStyle().Width(innerContentWidth).Render(fullContent)
	lines := strings.Split(wrappedContent, "\n")

	// Calculate visible height
	// height - PlanVerticalFrame (borders/margin) - 2 (detail border)
	visibleHeight := m.height - PlanVerticalFrame - 2
	// Adjust for global header (1) + footer (1) if they are not part of PlanVerticalFrame calculation updates
	// PlanVerticalFrame was likely just borders. We need to account for the new layout.
	// We will update PlanVerticalFrame or handle it in renderPlanReviewView.
	// For now, let's assume the height passed to renderPlanDetailView's style is the height of the container,
	// so we just subtract the container's borders (2).

	if visibleHeight < 1 {
		visibleHeight = 1
	}

	totalLines := len(lines)
	if totalLines <= visibleHeight {
		return wrappedContent
	}

	start := m.planDetailScrollOffset
	if start > totalLines-visibleHeight {
		start = totalLines - visibleHeight
	}
	if start < 0 {
		start = 0
	}

	end := start + visibleHeight
	if end > totalLines {
		end = totalLines
	}

	visibleContent := strings.Join(lines[start:end], "\n")
	return visibleContent
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
	createAttrStyle := lipgloss.NewStyle() // Default white/neutral
	deleteAttrStyle := lipgloss.NewStyle() // Default white/neutral

	// Prefixes
	prefixAdd := addStyle.Render("+")
	prefixDel := destroyStyle.Render("-")

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

		// Skip internal attributes
		if strings.HasPrefix(k, "_") {
			continue
		}

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
