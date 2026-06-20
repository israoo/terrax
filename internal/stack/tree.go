// Package stack provides tree building and navigation for Terragrunt stacks.
//
// This package implements the core business logic for TerraX, including filesystem
// scanning, tree construction, and hierarchical navigation operations. It is
// designed to be UI-agnostic and testable without any framework dependencies.
package stack

// Node represents a directory node in the stack tree
type Node struct {
	Name     string
	Path     string
	IsStack  bool
	Children []*Node
	Depth    int
}

func (n *Node) GetChildren() []*Node {
	if n == nil {
		return nil
	}
	return n.Children
}

func (n *Node) HasChildren() bool {
	return n != nil && len(n.Children) > 0
}

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

func (n *Node) FindChildByIndex(index int) *Node {
	if !n.HasChildren() || index < 0 || index >= len(n.Children) {
		return nil
	}
	return n.Children[index]
}
