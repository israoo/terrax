package plan

import (
	"path/filepath"
	"sort"
	"strings"
)

// BuildTree converts a list of StackResults into a slice of root TreeNodes.
// It assumes that the input stacks are already filtered (if needed) or it can handle all.
// Here we assume we process all stacks that satisfy the collector (e.g. valid plans).
func BuildTree(stacks []StackResult) []*TreeNode {
	rootMap := make(map[string]*TreeNode)
	var roots []*TreeNode

	// Helper to find or create a node for a given path components
	// We build the tree by iterating through path components of each stack
	for i := range stacks {
		// safe to take address since we loop by index
		stack := &stacks[i]

		// Normalize path
		path := stack.StackPath
		parts := strings.Split(path, string(filepath.Separator))

		var currentParent *TreeNode
		currentPath := ""

		for j, part := range parts {
			if part == "" {
				continue
			}

			if currentPath == "" {
				currentPath = part
			} else {
				currentPath = filepath.Join(currentPath, part)
			}

			// Check if this node already exists at this level under the current parent
			var node *TreeNode
			if currentParent == nil {
				// We are at root level
				if existing, ok := rootMap[part]; ok {
					node = existing
				}
			} else {
				// Search in parent's children
				for _, child := range currentParent.Children {
					if child.Name == part {
						node = child
						break
					}
				}
			}

			// Create if not exists
			if node == nil {
				node = &TreeNode{
					Name: part,
					Path: currentPath,
				}
				if currentParent == nil {
					rootMap[part] = node
					roots = append(roots, node)
				} else {
					currentParent.Children = append(currentParent.Children, node)
					currentParent.Children = sortNodes(currentParent.Children)
				}
			}

			// If this is the last part, attach the stack result
			if j == len(parts)-1 {
				node.Stack = stack
			}

			currentParent = node
		}
	}

	// Post-processing: Aggregate stats and propagate HasChanges
	for _, root := range roots {
		aggregateStats(root)
	}

	roots = sortNodes(roots)
	return roots
}

func sortNodes(nodes []*TreeNode) []*TreeNode {
	sort.Slice(nodes, func(i, j int) bool {
		return nodes[i].Name < nodes[j].Name
	})
	return nodes
}

// aggregateStats recursively calculates stats for directory nodes
// based on their children (leaf stacks or other directories).
func aggregateStats(node *TreeNode) {
	// If it's a leaf node (has a stack), use its stats
	if node.Stack != nil {
		node.Stats = node.Stack.Stats
		node.HasChanges = node.Stack.HasChanges
	}

	// Recursively process children
	for _, child := range node.Children {
		aggregateStats(child)

		// Aggregate children stats into current node
		// Note: A node can be both a stack (have tfplan) AND have children (sub-directories with other stacks)?
		// Terragrunt usually separates them, but theoretically possible.
		// If node.Stack is set, we added its stats above. Now add children stats.
		// Wait, if node.Stack is set, it represents a specific unit.
		// If it also has children, do we sum them up?
		// Ideally, the "Directory" view sums up everything under it.
		// If a node is ALSO a stack, its own stats are part of the sum.
		// So simple addition is correct.

		node.Stats.Add += child.Stats.Add
		node.Stats.Change += child.Stats.Change
		node.Stats.Destroy += child.Stats.Destroy

		if child.HasChanges {
			node.HasChanges = true
		}
	}
}
