package tui

import "github.com/israoo/terrax/internal/stack"

// DashTreeNode is a flattened row in the dashboard tree panel.
// Task 3 implements flattenDashTree and renderDashboardTree in this file.
type DashTreeNode struct {
	Node  *stack.Node // underlying stack node
	Depth int         // display indent depth
}

// flattenDashTree converts the node tree into a flat list for linear navigation.
// Nodes whose ancestors are not in expanded are hidden. Root children are always shown.
func flattenDashTree(root *stack.Node, expanded map[string]bool) []DashTreeNode {
	if root == nil {
		return nil
	}
	var result []DashTreeNode
	for _, child := range root.Children {
		result = appendDashTreeNode(result, child, 0, expanded)
	}
	return result
}

// appendDashTreeNode recursively appends node and its visible children to result.
func appendDashTreeNode(result []DashTreeNode, node *stack.Node, depth int, expanded map[string]bool) []DashTreeNode {
	result = append(result, DashTreeNode{Node: node, Depth: depth})
	if expanded[node.Path] {
		for _, child := range node.Children {
			result = appendDashTreeNode(result, child, depth+1, expanded)
		}
	}
	return result
}
