package stack

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestNewNavigator tests the Navigator constructor.
func TestNewNavigator(t *testing.T) {
	root := &Node{Name: "root", Path: "/test"}
	maxDepth := 3

	nav := NewNavigator(root, maxDepth)

	require.NotNil(t, nav)
	assert.Equal(t, root, nav.GetRoot())
	assert.Equal(t, maxDepth, nav.GetMaxDepth())
}

// TestNewNavigationState tests the NavigationState constructor.
func TestNewNavigationState(t *testing.T) {
	maxDepth := 3

	state := NewNavigationState(maxDepth)

	require.NotNil(t, state)
	assert.Len(t, state.Columns, maxDepth)
	assert.Len(t, state.SelectedIndices, maxDepth)
	assert.Len(t, state.CurrentNodes, maxDepth)
}

// TestNavigator_PropagateSelection tests the selection propagation logic.
func TestNavigator_PropagateSelection(t *testing.T) {
	tests := []struct {
		name              string
		setupTree         func() *Node
		maxDepth          int
		initialState      func(maxDepth int) *NavigationState
		expectedColumns   [][]string
		expectedIndices   []int
		expectedNodeNames []string
	}{
		{
			name: "single level - propagates children to first column",
			setupTree: func() *Node {
				return &Node{
					Name: "root",
					Path: "/root",
					Children: []*Node{
						{Name: "child1", Path: "/root/child1"},
						{Name: "child2", Path: "/root/child2"},
						{Name: "child3", Path: "/root/child3"},
					},
				}
			},
			maxDepth: 1,
			initialState: func(maxDepth int) *NavigationState {
				return NewNavigationState(maxDepth)
			},
			expectedColumns: [][]string{
				{"child1", "child2", "child3"},
			},
			expectedIndices:   []int{0},
			expectedNodeNames: []string{"child1"},
		},
		{
			name: "two levels - propagates children at both depths",
			setupTree: func() *Node {
				return &Node{
					Name: "root",
					Path: "/root",
					Children: []*Node{
						{
							Name: "env",
							Path: "/root/env",
							Children: []*Node{
								{Name: "dev", Path: "/root/env/dev"},
								{Name: "prod", Path: "/root/env/prod"},
							},
						},
						{Name: "modules", Path: "/root/modules"},
					},
				}
			},
			maxDepth: 2,
			initialState: func(maxDepth int) *NavigationState {
				return NewNavigationState(maxDepth)
			},
			expectedColumns: [][]string{
				{"env", "modules"},
				{"dev", "prod"},
			},
			expectedIndices:   []int{0, 0},
			expectedNodeNames: []string{"env", "dev"},
		},
		{
			name: "selection at depth 1 - changes second column",
			setupTree: func() *Node {
				return &Node{
					Name: "root",
					Path: "/root",
					Children: []*Node{
						{
							Name: "env",
							Path: "/root/env",
							Children: []*Node{
								{Name: "dev", Path: "/root/env/dev"},
								{Name: "staging", Path: "/root/env/staging"},
							},
						},
						{
							Name: "modules",
							Path: "/root/modules",
							Children: []*Node{
								{Name: "vpc", Path: "/root/modules/vpc"},
								{Name: "rds", Path: "/root/modules/rds"},
							},
						},
					},
				}
			},
			maxDepth: 2,
			initialState: func(maxDepth int) *NavigationState {
				state := NewNavigationState(maxDepth)
				state.SelectedIndices[0] = 1 // Select "modules"
				return state
			},
			expectedColumns: [][]string{
				{"env", "modules"},
				{"vpc", "rds"},
			},
			expectedIndices:   []int{1, 0},
			expectedNodeNames: []string{"modules", "vpc"},
		},
		{
			name: "leaf node - clears subsequent columns",
			setupTree: func() *Node {
				return &Node{
					Name: "root",
					Path: "/root",
					Children: []*Node{
						{Name: "leaf", Path: "/root/leaf"}, // No children
					},
				}
			},
			maxDepth: 3,
			initialState: func(maxDepth int) *NavigationState {
				return NewNavigationState(maxDepth)
			},
			expectedColumns: [][]string{
				{"leaf"},
				{}, // Cleared
				{}, // Cleared
			},
			expectedIndices:   []int{0, 0, 0},
			expectedNodeNames: []string{"leaf"},
		},
		{
			name: "nil root - returns nil",
			setupTree: func() *Node {
				return nil
			},
			maxDepth: 1,
			initialState: func(maxDepth int) *NavigationState {
				return NewNavigationState(maxDepth)
			},
			expectedColumns:   nil, // Don't check columns for nil root
			expectedIndices:   []int{0},
			expectedNodeNames: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			root := tt.setupTree()
			nav := NewNavigator(root, tt.maxDepth)
			state := tt.initialState(tt.maxDepth)

			resultNode := nav.PropagateSelection(state)

			// Verify columns (skip if expectedColumns is nil).
			if tt.expectedColumns != nil {
				for i, expectedCol := range tt.expectedColumns {
					if i < len(state.Columns) {
						assert.Equal(t, expectedCol, state.Columns[i],
							"column %d mismatch", i)
					}
				}
			}

			// Verify selected indices.
			assert.Equal(t, tt.expectedIndices, state.SelectedIndices)

			// Verify current nodes.
			for i, expectedName := range tt.expectedNodeNames {
				if i < len(state.CurrentNodes) && state.CurrentNodes[i] != nil {
					assert.Equal(t, expectedName, state.CurrentNodes[i].Name,
						"node name at depth %d mismatch", i)
				}
			}

			// Verify result node.
			if len(tt.expectedNodeNames) > 0 && resultNode != nil {
				lastIdx := len(tt.expectedNodeNames) - 1
				assert.Equal(t, tt.expectedNodeNames[lastIdx], resultNode.Name)
			}
		})
	}
}

// TestNavigator_GetNodeAtDepth tests retrieving nodes at specific depths.
func TestNavigator_GetNodeAtDepth(t *testing.T) {
	// Setup tree.
	root := &Node{
		Name: "root",
		Path: "/root",
		Children: []*Node{
			{
				Name: "env",
				Path: "/root/env",
				Children: []*Node{
					{Name: "dev", Path: "/root/env/dev"},
				},
			},
		},
	}

	nav := NewNavigator(root, 2)
	state := NewNavigationState(2)
	nav.PropagateSelection(state)

	tests := []struct {
		name         string
		depth        int
		expectedName string
		expectNil    bool
	}{
		{
			name:         "depth 0 - returns first node",
			depth:        0,
			expectedName: "env",
			expectNil:    false,
		},
		{
			name:         "depth 1 - returns second node",
			depth:        1,
			expectedName: "dev",
			expectNil:    false,
		},
		{
			name:      "depth -1 - returns nil",
			depth:     -1,
			expectNil: true,
		},
		{
			name:      "depth out of bounds - returns nil",
			depth:     5,
			expectNil: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			node := nav.GetNodeAtDepth(state, tt.depth)

			if tt.expectNil {
				assert.Nil(t, node)
			} else {
				require.NotNil(t, node)
				assert.Equal(t, tt.expectedName, node.Name)
			}
		})
	}
}

// TestNavigator_GetMaxVisibleDepth tests finding the deepest visible column.
func TestNavigator_GetMaxVisibleDepth(t *testing.T) {
	tests := []struct {
		name          string
		maxDepth      int
		setupState    func() *NavigationState
		expectedDepth int
	}{
		{
			name:     "all columns populated",
			maxDepth: 3,
			setupState: func() *NavigationState {
				state := NewNavigationState(3)
				state.Columns[0] = []string{"a", "b"}
				state.Columns[1] = []string{"c", "d"}
				state.Columns[2] = []string{"e", "f"}
				return state
			},
			expectedDepth: 3,
		},
		{
			name:     "only first column populated",
			maxDepth: 3,
			setupState: func() *NavigationState {
				state := NewNavigationState(3)
				state.Columns[0] = []string{"a", "b"}
				return state
			},
			expectedDepth: 1,
		},
		{
			name:     "no columns populated",
			maxDepth: 3,
			setupState: func() *NavigationState {
				return NewNavigationState(3)
			},
			expectedDepth: 0,
		},
		{
			name:     "middle column populated",
			maxDepth: 3,
			setupState: func() *NavigationState {
				state := NewNavigationState(3)
				state.Columns[0] = []string{"a"}
				state.Columns[1] = []string{"b"}
				// Column 2 is empty
				return state
			},
			expectedDepth: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			nav := NewNavigator(&Node{}, tt.maxDepth)
			state := tt.setupState()

			depth := nav.GetMaxVisibleDepth(state)

			assert.Equal(t, tt.expectedDepth, depth)
		})
	}
}

// TestNavigator_MoveUp tests upward navigation.
func TestNavigator_MoveUp(t *testing.T) {
	tests := []struct {
		name          string
		maxDepth      int
		depth         int
		columnSize    int
		initialIndex  int
		expectedMoved bool
		expectedIndex int
	}{
		{
			name:          "can move up from index 2",
			maxDepth:      1,
			depth:         0,
			columnSize:    3,
			initialIndex:  2,
			expectedMoved: true,
			expectedIndex: 1,
		},
		{
			name:          "cyclic move up from index 0 wraps to last",
			maxDepth:      1,
			depth:         0,
			columnSize:    3,
			initialIndex:  0,
			expectedMoved: true,
			expectedIndex: 2,
		},
		{
			name:          "invalid depth returns false",
			maxDepth:      1,
			depth:         5,
			columnSize:    3,
			initialIndex:  0,
			expectedMoved: false,
			expectedIndex: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			nav := NewNavigator(&Node{}, tt.maxDepth)
			state := NewNavigationState(tt.maxDepth)

			if tt.depth >= 0 && tt.depth < tt.maxDepth {
				state.SelectedIndices[tt.depth] = tt.initialIndex
				state.Columns[tt.depth] = make([]string, tt.columnSize)
			}

			moved := nav.MoveUp(state, tt.depth)

			assert.Equal(t, tt.expectedMoved, moved)
			if tt.depth >= 0 && tt.depth < tt.maxDepth {
				assert.Equal(t, tt.expectedIndex, state.SelectedIndices[tt.depth])
			}
		})
	}
}

// TestNavigator_MoveDown tests downward navigation.
func TestNavigator_MoveDown(t *testing.T) {
	tests := []struct {
		name          string
		maxDepth      int
		depth         int
		columnSize    int
		initialIndex  int
		expectedMoved bool
		expectedIndex int
	}{
		{
			name:          "can move down when not at bottom",
			maxDepth:      1,
			depth:         0,
			columnSize:    3,
			initialIndex:  0,
			expectedMoved: true,
			expectedIndex: 1,
		},
		{
			name:          "cyclic move down from bottom wraps to top",
			maxDepth:      1,
			depth:         0,
			columnSize:    3,
			initialIndex:  2,
			expectedMoved: true,
			expectedIndex: 0,
		},
		{
			name:          "invalid depth returns false",
			maxDepth:      1,
			depth:         5,
			columnSize:    3,
			initialIndex:  0,
			expectedMoved: false,
			expectedIndex: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			nav := NewNavigator(&Node{}, tt.maxDepth)
			state := NewNavigationState(tt.maxDepth)

			if tt.depth >= 0 && tt.depth < tt.maxDepth {
				state.SelectedIndices[tt.depth] = tt.initialIndex
				state.Columns[tt.depth] = make([]string, tt.columnSize)
			}

			moved := nav.MoveDown(state, tt.depth)

			assert.Equal(t, tt.expectedMoved, moved)
			if tt.depth >= 0 && tt.depth < tt.maxDepth {
				assert.Equal(t, tt.expectedIndex, state.SelectedIndices[tt.depth])
			}
		})
	}
}

// TestNavigator_CanMoveUp tests the CanMoveUp predicate.
func TestNavigator_CanMoveUp(t *testing.T) {
	tests := []struct {
		name         string
		maxDepth     int
		depth        int
		initialIndex int
		expected     bool
	}{
		{
			name:         "can move up from index 1",
			maxDepth:     1,
			depth:        0,
			initialIndex: 1,
			expected:     true,
		},
		{
			name:         "cannot move up from index 0",
			maxDepth:     1,
			depth:        0,
			initialIndex: 0,
			expected:     false,
		},
		{
			name:         "invalid depth returns false",
			maxDepth:     1,
			depth:        -1,
			initialIndex: 1,
			expected:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			nav := NewNavigator(&Node{}, tt.maxDepth)
			state := NewNavigationState(tt.maxDepth)

			if tt.depth >= 0 && tt.depth < tt.maxDepth {
				state.SelectedIndices[tt.depth] = tt.initialIndex
			}

			canMove := nav.CanMoveUp(state, tt.depth)

			assert.Equal(t, tt.expected, canMove)
		})
	}
}

// TestNavigator_CanMoveDown tests the CanMoveDown predicate.
func TestNavigator_CanMoveDown(t *testing.T) {
	tests := []struct {
		name         string
		maxDepth     int
		depth        int
		columnSize   int
		initialIndex int
		expected     bool
	}{
		{
			name:         "can move down when not at bottom",
			maxDepth:     1,
			depth:        0,
			columnSize:   3,
			initialIndex: 0,
			expected:     true,
		},
		{
			name:         "cannot move down when at bottom",
			maxDepth:     1,
			depth:        0,
			columnSize:   3,
			initialIndex: 2,
			expected:     false,
		},
		{
			name:         "invalid depth returns false",
			maxDepth:     1,
			depth:        -1,
			columnSize:   3,
			initialIndex: 0,
			expected:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			nav := NewNavigator(&Node{}, tt.maxDepth)
			state := NewNavigationState(tt.maxDepth)

			if tt.depth >= 0 && tt.depth < tt.maxDepth {
				state.SelectedIndices[tt.depth] = tt.initialIndex
				state.Columns[tt.depth] = make([]string, tt.columnSize)
			}

			canMove := nav.CanMoveDown(state, tt.depth)

			assert.Equal(t, tt.expected, canMove)
		})
	}
}

// TestNavigator_ClearColumnsFrom tests clearing columns from a starting depth.
func TestNavigator_ClearColumnsFrom(t *testing.T) {
	tests := []struct {
		name       string
		maxDepth   int
		startDepth int
		setupState func() *NavigationState
		verify     func(t *testing.T, state *NavigationState)
	}{
		{
			name:       "clear from depth 1 onwards",
			maxDepth:   3,
			startDepth: 1,
			setupState: func() *NavigationState {
				state := NewNavigationState(3)
				state.Columns[0] = []string{"a", "b"}
				state.Columns[1] = []string{"c", "d"}
				state.Columns[2] = []string{"e", "f"}
				state.SelectedIndices[0] = 1
				state.SelectedIndices[1] = 1
				state.SelectedIndices[2] = 1
				return state
			},
			verify: func(t *testing.T, state *NavigationState) {
				assert.Equal(t, []string{"a", "b"}, state.Columns[0])
				assert.Empty(t, state.Columns[1])
				assert.Empty(t, state.Columns[2])
				assert.Equal(t, 0, state.SelectedIndices[1])
				assert.Equal(t, 0, state.SelectedIndices[2])
			},
		},
		{
			name:       "negative depth does nothing",
			maxDepth:   2,
			startDepth: -1,
			setupState: func() *NavigationState {
				state := NewNavigationState(2)
				state.Columns[0] = []string{"a"}
				state.Columns[1] = []string{"b"}
				return state
			},
			verify: func(t *testing.T, state *NavigationState) {
				assert.Equal(t, []string{"a"}, state.Columns[0])
				assert.Equal(t, []string{"b"}, state.Columns[1])
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			nav := NewNavigator(&Node{}, tt.maxDepth)
			state := tt.setupState()

			nav.clearColumnsFrom(state, tt.startDepth)

			tt.verify(t, state)
		})
	}
}

// TestNavigator_PropagateSelection_EdgeCases tests edge cases and error conditions.
func TestNavigator_PropagateSelection_EdgeCases(t *testing.T) {
	tests := []struct {
		name     string
		setupNav func() (*Navigator, *NavigationState)
		verify   func(t *testing.T, state *NavigationState, result *Node)
	}{
		{
			name: "nil state returns nil",
			setupNav: func() (*Navigator, *NavigationState) {
				root := &Node{Name: "root", Path: "/root"}
				nav := NewNavigator(root, 1)
				return nav, nil
			},
			verify: func(t *testing.T, state *NavigationState, result *Node) {
				assert.Nil(t, result)
			},
		},
		{
			name: "nil navigator returns nil",
			setupNav: func() (*Navigator, *NavigationState) {
				state := NewNavigationState(1)
				return nil, state
			},
			verify: func(t *testing.T, state *NavigationState, result *Node) {
				assert.Nil(t, result)
			},
		},
		{
			name: "maxDepth 0 returns nil",
			setupNav: func() (*Navigator, *NavigationState) {
				root := &Node{Name: "root", Path: "/root"}
				nav := NewNavigator(root, 0)
				state := NewNavigationState(0)
				return nav, state
			},
			verify: func(t *testing.T, state *NavigationState, result *Node) {
				assert.Nil(t, result)
			},
		},
		{
			name: "selected index out of bounds - clamped to 0",
			setupNav: func() (*Navigator, *NavigationState) {
				root := &Node{
					Name: "root",
					Path: "/root",
					Children: []*Node{
						{Name: "child1", Path: "/root/child1"},
						{Name: "child2", Path: "/root/child2"},
					},
				}
				nav := NewNavigator(root, 1)
				state := NewNavigationState(1)
				state.SelectedIndices[0] = 99 // Out of bounds
				return nav, state
			},
			verify: func(t *testing.T, state *NavigationState, result *Node) {
				assert.NotNil(t, result)
				assert.Equal(t, "child1", result.Name)
				assert.Equal(t, 0, state.SelectedIndices[0])
				assert.Equal(t, []string{"child1", "child2"}, state.Columns[0])
			},
		},
		{
			name: "irregular tree - shallow branch clears subsequent columns",
			setupNav: func() (*Navigator, *NavigationState) {
				root := &Node{
					Name: "root",
					Path: "/root",
					Children: []*Node{
						{
							Name: "deep",
							Path: "/root/deep",
							Children: []*Node{
								{Name: "level2", Path: "/root/deep/level2"},
							},
						},
						{
							Name: "shallow",
							Path: "/root/shallow",
							// No children - selecting this should clear depth 1
						},
					},
				}
				nav := NewNavigator(root, 2)
				state := NewNavigationState(2)
				// Pre-populate with data that should be cleared
				state.Columns[1] = []string{"should", "be", "cleared"}
				state.SelectedIndices[0] = 1 // Select "shallow"
				return nav, state
			},
			verify: func(t *testing.T, state *NavigationState, result *Node) {
				assert.NotNil(t, result)
				assert.Equal(t, "shallow", result.Name)
				assert.Equal(t, []string{"deep", "shallow"}, state.Columns[0])
				assert.Empty(t, state.Columns[1], "Column 1 should be cleared")
				assert.Equal(t, 0, state.SelectedIndices[1])
			},
		},
		{
			name: "node with empty children array",
			setupNav: func() (*Navigator, *NavigationState) {
				root := &Node{
					Name:     "root",
					Path:     "/root",
					Children: []*Node{}, // Empty children slice
				}
				nav := NewNavigator(root, 1)
				state := NewNavigationState(1)
				return nav, state
			},
			verify: func(t *testing.T, state *NavigationState, result *Node) {
				// Returns root since it has no children to navigate to
				assert.NotNil(t, result)
				assert.Equal(t, "root", result.Name)
				assert.Empty(t, state.Columns[0])
			},
		},
		{
			name: "deep tree with mid-level index out of bounds",
			setupNav: func() (*Navigator, *NavigationState) {
				root := &Node{
					Name: "root",
					Path: "/root",
					Children: []*Node{
						{
							Name: "env",
							Path: "/root/env",
							Children: []*Node{
								{Name: "dev", Path: "/root/env/dev"},
							},
						},
					},
				}
				nav := NewNavigator(root, 3)
				state := NewNavigationState(3)
				state.SelectedIndices[0] = 0  // Valid
				state.SelectedIndices[1] = 50 // Out of bounds at depth 1
				state.SelectedIndices[2] = 10 // Should never be used
				return nav, state
			},
			verify: func(t *testing.T, state *NavigationState, result *Node) {
				assert.NotNil(t, result)
				assert.Equal(t, "dev", result.Name)
				assert.Equal(t, 0, state.SelectedIndices[1], "Out of bounds index should be clamped to 0")
				assert.Empty(t, state.Columns[2], "Depth 2 should be empty (dev is leaf)")
			},
		},
		{
			name: "all selected indices out of bounds",
			setupNav: func() (*Navigator, *NavigationState) {
				root := &Node{
					Name: "root",
					Path: "/root",
					Children: []*Node{
						{
							Name: "a",
							Path: "/root/a",
							Children: []*Node{
								{Name: "b", Path: "/root/a/b"},
							},
						},
					},
				}
				nav := NewNavigator(root, 2)
				state := NewNavigationState(2)
				state.SelectedIndices[0] = 100
				state.SelectedIndices[1] = 200
				return nav, state
			},
			verify: func(t *testing.T, state *NavigationState, result *Node) {
				assert.NotNil(t, result)
				assert.Equal(t, "b", result.Name)
				assert.Equal(t, 0, state.SelectedIndices[0])
				assert.Equal(t, 0, state.SelectedIndices[1])
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			nav, state := tt.setupNav()
			result := nav.PropagateSelection(state)
			tt.verify(t, state, result)
		})
	}
}

// TestNavigator_GetNavigationPath tests building navigation paths.
func TestNavigator_GetNavigationPath(t *testing.T) {
	tests := []struct {
		name     string
		setupNav func() (*Navigator, *NavigationState)
		depth    int
		expected string
	}{
		{
			name: "nil root returns tilde",
			setupNav: func() (*Navigator, *NavigationState) {
				nav := NewNavigator(nil, 1)
				state := NewNavigationState(1)
				return nav, state
			},
			depth:    0,
			expected: "~",
		},
		{
			name: "root level (depth -1) returns root path",
			setupNav: func() (*Navigator, *NavigationState) {
				root := &Node{
					Name: "root",
					Path: "/test/root",
				}
				nav := NewNavigator(root, 1)
				state := NewNavigationState(1)
				return nav, state
			},
			depth:    -1,
			expected: "/test/root",
		},
		{
			name: "max depth 0 returns root path",
			setupNav: func() (*Navigator, *NavigationState) {
				root := &Node{
					Name: "root",
					Path: "/test/root",
				}
				nav := NewNavigator(root, 0)
				state := NewNavigationState(0)
				return nav, state
			},
			depth:    0,
			expected: "/test/root",
		},
		{
			name: "single level deep path",
			setupNav: func() (*Navigator, *NavigationState) {
				root := &Node{
					Name: "root",
					Path: "/test/root",
					Children: []*Node{
						{Name: "env"},
						{Name: "modules"},
					},
				}
				nav := NewNavigator(root, 1)
				state := NewNavigationState(1)
				state.Columns[0] = []string{"env", "modules"}
				state.SelectedIndices[0] = 0 // Select "env"
				return nav, state
			},
			depth:    0,
			expected: "/test/root/env",
		},
		{
			name: "multi level deep path",
			setupNav: func() (*Navigator, *NavigationState) {
				root := &Node{
					Name: "root",
					Path: "/test/root",
					Children: []*Node{
						{Name: "env"},
					},
				}
				nav := NewNavigator(root, 3)
				state := NewNavigationState(3)
				state.Columns[0] = []string{"env"}
				state.Columns[1] = []string{"dev", "prod"}
				state.Columns[2] = []string{"us-east", "us-west"}
				state.SelectedIndices[0] = 0 // "env"
				state.SelectedIndices[1] = 1 // "prod"
				state.SelectedIndices[2] = 0 // "us-east"
				return nav, state
			},
			depth:    2,
			expected: "/test/root/env/prod/us-east",
		},
		{
			name: "path with stack marker (emoji)",
			setupNav: func() (*Navigator, *NavigationState) {
				root := &Node{
					Name: "root",
					Path: "/test/root",
				}
				nav := NewNavigator(root, 1)
				state := NewNavigationState(1)
				state.Columns[0] = []string{"env 📦", "modules"}
				state.SelectedIndices[0] = 0 // Select "env 📦"
				return nav, state
			},
			depth:    0,
			expected: "/test/root/env 📦", // Emoji is included in path as-is
		},
		{
			name: "partial path (depth less than columns)",
			setupNav: func() (*Navigator, *NavigationState) {
				root := &Node{
					Name: "root",
					Path: "/test/root",
				}
				nav := NewNavigator(root, 3)
				state := NewNavigationState(3)
				state.Columns[0] = []string{"env"}
				state.Columns[1] = []string{"dev", "prod"}
				state.Columns[2] = []string{"us-east"}
				state.SelectedIndices[0] = 0
				state.SelectedIndices[1] = 1
				state.SelectedIndices[2] = 0
				return nav, state
			},
			depth:    1, // Only go to depth 1
			expected: "/test/root/env/prod",
		},
		{
			name: "invalid selection index skipped",
			setupNav: func() (*Navigator, *NavigationState) {
				root := &Node{
					Name: "root",
					Path: "/test/root",
				}
				nav := NewNavigator(root, 2)
				state := NewNavigationState(2)
				state.Columns[0] = []string{"env"}
				state.Columns[1] = []string{"dev"}
				state.SelectedIndices[0] = 0
				state.SelectedIndices[1] = -1 // Invalid index
				return nav, state
			},
			depth:    1,
			expected: "/test/root/env",
		},
		{
			name: "selected index out of bounds skipped",
			setupNav: func() (*Navigator, *NavigationState) {
				root := &Node{
					Name: "root",
					Path: "/test/root",
				}
				nav := NewNavigator(root, 2)
				state := NewNavigationState(2)
				state.Columns[0] = []string{"env"}
				state.Columns[1] = []string{"dev"}
				state.SelectedIndices[0] = 0
				state.SelectedIndices[1] = 99 // Out of bounds
				return nav, state
			},
			depth:    1,
			expected: "/test/root/env",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			nav, state := tt.setupNav()
			path := nav.GetNavigationPath(state, tt.depth)
			assert.Equal(t, tt.expected, path)
		})
	}
}
