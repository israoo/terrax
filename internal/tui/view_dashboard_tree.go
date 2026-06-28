package tui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/israoo/terrax/internal/stack"
)

// DashTreeNode is one visible entry in the flattened dashboard tree.
type DashTreeNode struct {
	Node   *stack.Node
	Depth  int
	IsLast bool
	// Prefix is the indentation string inherited from parent nodes,
	// e.g. "│  " means the parent was not the last child.
	Prefix string
}

// flattenDashTree performs a DFS over root.Children, respecting expanded.
// Root itself is not included; only its descendants are.
func flattenDashTree(root *stack.Node, expanded map[string]bool) []DashTreeNode {
	var nodes []DashTreeNode
	var walk func(node *stack.Node, depth int, prefix string, isLast bool)
	walk = func(node *stack.Node, depth int, prefix string, isLast bool) {
		nodes = append(nodes, DashTreeNode{
			Node:   node,
			Depth:  depth,
			IsLast: isLast,
			Prefix: prefix,
		})
		if node.HasChildren() && expanded[node.Path] {
			for i, child := range node.Children {
				childIsLast := i == len(node.Children)-1
				var childPrefix string
				if isLast {
					childPrefix = prefix + "   "
				} else {
					childPrefix = prefix + "│  "
				}
				walk(child, depth+1, childPrefix, childIsLast)
			}
		}
	}
	for i, child := range root.Children {
		walk(child, 0, "", i == len(root.Children)-1)
	}
	return nodes
}

var (
	//nolint:unused // Used by renderDashboardTree.
	dashTreeCursorStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(accentColor)
	//nolint:unused // Used by renderDashboardTree.
	dashTreeDirStyle = lipgloss.NewStyle().
				Foreground(secondaryColor)
	//nolint:unused // Used by renderDashboardTree.
	dashTreeStackStyle = lipgloss.NewStyle().
				Foreground(textColor)
	//nolint:unused // Used by renderDashboardTree.
	dashTreeDimStyle = lipgloss.NewStyle().
				Foreground(dimColor)
)

// renderDashboardTree renders the indented tree panel content.
// width is the inner content width (excluding borders/padding).
// height is the maximum number of lines to render.
//
//nolint:unused // Called by Task 9 renderDashboardView.
func (m Model) renderDashboardTree(width, height int) string {
	if len(m.dashTreeNodes) == 0 {
		return dashTreeDimStyle.Render("No stacks found.")
	}

	// Compute visible window (scroll to keep cursor visible).
	start := 0
	if m.dashTreeCursor >= height {
		start = m.dashTreeCursor - height + 1
	}
	end := min(start+height, len(m.dashTreeNodes))

	var sb strings.Builder
	for i := start; i < end; i++ {
		tn := m.dashTreeNodes[i]
		isCursor := i == m.dashTreeCursor

		connector := "├─ "
		if tn.IsLast {
			connector = "└─ "
		}

		var icon string
		if tn.Node.HasChildren() {
			if m.dashTreeExpanded[tn.Node.Path] {
				icon = "▼ "
			} else {
				icon = "▶ "
			}
		} else {
			icon = "  "
		}

		prefix := dashTreeDimStyle.Render(tn.Prefix + connector)
		name := tn.Node.Name
		maxNameWidth := width - len(tn.Prefix) - 3 - 2 // connector(3) + icon(2)
		if maxNameWidth < 4 {
			maxNameWidth = 4
		}
		if len(name) > maxNameWidth {
			name = name[:maxNameWidth-1] + "…"
		}

		var nameRendered string
		if isCursor {
			nameRendered = dashTreeCursorStyle.Render(icon + name)
		} else if tn.Node.HasChildren() {
			nameRendered = dashTreeDirStyle.Render(icon + name)
		} else {
			nameRendered = dashTreeStackStyle.Render(icon + name)
		}

		sb.WriteString(prefix + nameRendered + "\n")
	}
	return sb.String()
}
