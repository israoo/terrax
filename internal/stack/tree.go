// Package stack provides tree building and navigation for Terragrunt stacks.
//
// This package implements the core business logic for TerraX, including filesystem
// scanning, tree construction, and hierarchical navigation operations. It is
// designed to be UI-agnostic and testable without any framework dependencies.
package stack

// Node represents a directory node in the stack tree
type Node struct {
	Name     string  // Directory name
	Path     string  // Full path
	IsStack  bool    // True if contains terragrunt.hcl
	Children []*Node // Child directories
	Depth    int     // Depth level in the tree
}

// GetChildren returns the child nodes of this node
func (n *Node) GetChildren() []*Node {
	if n == nil {
		return nil
	}
	return n.Children
}

// HasChildren returns true if the node has children
func (n *Node) HasChildren() bool {
	return n != nil && len(n.Children) > 0
}

// GetChildNames returns a slice of child node names
func (n *Node) GetChildNames() []string {
	if !n.HasChildren() {
		return []string{}
	}

	names := make([]string, len(n.Children))
	for i, child := range n.Children {
		marker := ""
		if child.IsStack {
			marker = " 📦"
		}
		names[i] = child.Name + marker
	}
	return names
}

// FindChildByIndex returns the child node at the given index
func (n *Node) FindChildByIndex(index int) *Node {
	if !n.HasChildren() || index < 0 || index >= len(n.Children) {
		return nil
	}
	return n.Children[index]
}
