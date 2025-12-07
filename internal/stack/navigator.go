package stack

// Navigator provides methods for navigating the stack tree hierarchy.
// It encapsulates the business logic for tree traversal, path resolution,
// and selection management, keeping the TUI layer clean and focused on presentation.
type Navigator struct {
	root     *Node
	maxDepth int
}

// NewNavigator creates a new Navigator instance for the given stack tree.
func NewNavigator(root *Node, maxDepth int) *Navigator {
	return &Navigator{
		root:     root,
		maxDepth: maxDepth,
	}
}

// NavigationState represents the current navigation state in the tree.
type NavigationState struct {
	Columns         [][]string // Column content at each depth level
	SelectedIndices []int      // Selected index at each depth
	CurrentNodes    []*Node    // Current node at each depth
}

// NewNavigationState creates a new empty navigation state.
func NewNavigationState(maxDepth int) *NavigationState {
	return &NavigationState{
		Columns:         make([][]string, maxDepth),
		SelectedIndices: make([]int, maxDepth),
		CurrentNodes:    make([]*Node, maxDepth),
	}
}

// PropagateSelection recalculates all navigation columns based on selected indices.
// It walks the tree from root following the selection path and populates columns.
// Returns the deepest selected node or nil if navigation ends early.
func (nav *Navigator) PropagateSelection(state *NavigationState) *Node {
	if nav == nil || state == nil {
		return nil
	}

	if nav.root == nil || nav.maxDepth == 0 {
		return nil
	}

	currentNode := nav.root

	for depth := 0; depth < nav.maxDepth; depth++ {
		if currentNode == nil || !currentNode.HasChildren() {
			// No children - clear this and all subsequent columns
			nav.clearColumnsFrom(state, depth)
			return currentNode
		}

		// Populate column with children names
		state.Columns[depth] = currentNode.GetChildNames()

		// Validate and clamp selected index
		if state.SelectedIndices[depth] >= len(currentNode.Children) {
			state.SelectedIndices[depth] = 0
		}

		// Navigate to selected child
		if len(currentNode.Children) > 0 {
			selectedIndex := state.SelectedIndices[depth]
			currentNode = currentNode.Children[selectedIndex]
			state.CurrentNodes[depth] = currentNode
		} else {
			currentNode = nil
			state.CurrentNodes[depth] = nil
		}
	}

	return currentNode
}

// clearColumnsFrom clears all columns starting from the given depth.
func (nav *Navigator) clearColumnsFrom(state *NavigationState, startDepth int) {
	if state == nil || startDepth < 0 {
		return
	}

	for d := startDepth; d < nav.maxDepth; d++ {
		state.Columns[d] = []string{}
		state.SelectedIndices[d] = 0
		state.CurrentNodes[d] = nil
	}
}

// GetNodeAtDepth returns the selected node at a specific depth level.
// Returns nil if the depth is invalid or no node exists at that level.
func (nav *Navigator) GetNodeAtDepth(state *NavigationState, depth int) *Node {
	if depth < 0 || depth >= nav.maxDepth {
		return nil
	}
	return state.CurrentNodes[depth]
}

// GetMaxVisibleDepth returns the deepest depth level that has content.
func (nav *Navigator) GetMaxVisibleDepth(state *NavigationState) int {
	for depth := nav.maxDepth - 1; depth >= 0; depth-- {
		if len(state.Columns[depth]) > 0 {
			return depth + 1
		}
	}
	return 0
}

// CanMoveUp checks if moving up is possible in the given depth column.
func (nav *Navigator) CanMoveUp(state *NavigationState, depth int) bool {
	if depth < 0 || depth >= nav.maxDepth {
		return false
	}
	return state.SelectedIndices[depth] > 0
}

// CanMoveDown checks if moving down is possible in the given depth column.
func (nav *Navigator) CanMoveDown(state *NavigationState, depth int) bool {
	if depth < 0 || depth >= nav.maxDepth {
		return false
	}
	maxIndex := len(state.Columns[depth]) - 1
	return state.SelectedIndices[depth] < maxIndex
}

// MoveUp moves the selection up in the specified depth column.
// Returns true if the move was successful.
func (nav *Navigator) MoveUp(state *NavigationState, depth int) bool {
	if !nav.CanMoveUp(state, depth) {
		return false
	}
	state.SelectedIndices[depth]--
	return true
}

// MoveDown moves the selection down in the specified depth column.
// Returns true if the move was successful.
func (nav *Navigator) MoveDown(state *NavigationState, depth int) bool {
	if !nav.CanMoveDown(state, depth) {
		return false
	}
	state.SelectedIndices[depth]++
	return true
}

// GetRoot returns the root node of the tree.
func (nav *Navigator) GetRoot() *Node {
	return nav.root
}

// GetMaxDepth returns the maximum depth of the tree.
func (nav *Navigator) GetMaxDepth() int {
	return nav.maxDepth
}

// GetNavigationPath builds the full navigation path up to the specified depth.
// It constructs a filesystem path from the root through the selected nodes.
// Returns "~" if root is nil, otherwise returns the full path.
func (nav *Navigator) GetNavigationPath(state *NavigationState, depth int) string {
	// Start with root directory path
	if nav.root == nil {
		return "~"
	}

	path := nav.root.Path

	// If at root level (depth < 0), return just the root path
	if depth < 0 || nav.maxDepth == 0 {
		return path
	}

	// Build path from selected indices, appending subdirectories
	for i := 0; i <= depth && i < len(state.Columns); i++ {
		if i >= len(state.SelectedIndices) {
			break
		}

		selectedIdx := state.SelectedIndices[i]
		if selectedIdx >= 0 && selectedIdx < len(state.Columns[i]) {
			// Extract directory name (remove emoji marker if present)
			dirName := state.Columns[i][selectedIdx]
			// Remove " 📦" marker if it exists
			if len(dirName) > 3 && dirName[len(dirName)-2:] == "📦" {
				dirName = dirName[:len(dirName)-3]
			}
			path += "/" + dirName
		}
	}

	return path
}
