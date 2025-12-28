package tui

import (
	"testing"

	"github.com/charmbracelet/bubbles/textinput"
	"github.com/israoo/terrax/internal/stack"
	"github.com/stretchr/testify/assert"
)

// TestModel_Init tests the Bubble Tea Init method.
func TestModel_Init(t *testing.T) {
	root := &stack.Node{Name: "root", Path: "/test"}
	model := NewModel(root, 1, []string{"plan", "apply"}, 3)

	cmd := model.Init()

	assert.Nil(t, cmd, "Init should return nil command")
}

// TestModel_IsCommandsColumnFocused tests checking if commands column is focused.
func TestModel_IsCommandsColumnFocused(t *testing.T) {
	tests := []struct {
		name          string
		focusedColumn int
		expected      bool
	}{
		{
			name:          "focused on commands column",
			focusedColumn: 0,
			expected:      true,
		},
		{
			name:          "focused on first navigation column",
			focusedColumn: 1,
			expected:      false,
		},
		{
			name:          "focused on second navigation column",
			focusedColumn: 2,
			expected:      false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := Model{focusedColumn: tt.focusedColumn}
			result := m.isCommandsColumnFocused()
			assert.Equal(t, tt.expected, result)
		})
	}
}

// TestModel_GetNavigationDepth tests getting navigation depth from focused column.
func TestModel_GetNavigationDepth(t *testing.T) {
	tests := []struct {
		name          string
		focusedColumn int
		expected      int
	}{
		{
			name:          "commands column returns -1",
			focusedColumn: 0,
			expected:      -1,
		},
		{
			name:          "first navigation column returns 0",
			focusedColumn: 1,
			expected:      0,
		},
		{
			name:          "second navigation column returns 1",
			focusedColumn: 2,
			expected:      1,
		},
		{
			name:          "third navigation column returns 2",
			focusedColumn: 3,
			expected:      2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := Model{focusedColumn: tt.focusedColumn}
			depth := m.getNavigationDepth()
			assert.Equal(t, tt.expected, depth)
		})
	}
}

// TestModel_CalculateColumnWidth tests column width calculation.
func TestModel_CalculateColumnWidth(t *testing.T) {
	tests := []struct {
		name        string
		width       int
		maxDepth    int
		expectedMin int
		expectedMax int
	}{
		{
			name:        "wide terminal - should calculate proper width",
			width:       200,
			maxDepth:    3,
			expectedMin: 30,
			expectedMax: 100,
		},
		{
			name:        "narrow terminal - should return minimum",
			width:       60,
			maxDepth:    3,
			expectedMin: MinColumnWidth,
			expectedMax: MinColumnWidth,
		},
		{
			name:        "zero depth - should return minimum",
			width:       200,
			maxDepth:    0,
			expectedMin: MinColumnWidth,
			expectedMax: MinColumnWidth,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			root := &stack.Node{Name: "root"}
			nav := stack.NewNavigator(root, tt.maxDepth)

			m := Model{
				navigator:            nav,
				width:                tt.width,
				maxNavigationColumns: 3,
				columnFilters:        make(map[int]textinput.Model),
			}

			result := m.calculateColumnWidth()

			assert.GreaterOrEqual(t, result, tt.expectedMin)
			assert.LessOrEqual(t, result, tt.expectedMax)
		})
	}
}

// TestModel_HasLeftOverflow tests left overflow detection.
func TestModel_HasLeftOverflow(t *testing.T) {
	tests := []struct {
		name             string
		navigationOffset int
		expected         bool
	}{
		{
			name:             "no left overflow at offset 0",
			navigationOffset: 0,
			expected:         false,
		},
		{
			name:             "has left overflow at offset 1",
			navigationOffset: 1,
			expected:         true,
		},
		{
			name:             "has left overflow at offset 3",
			navigationOffset: 3,
			expected:         true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := Model{navigationOffset: tt.navigationOffset}
			result := m.hasLeftOverflow()
			assert.Equal(t, tt.expected, result)
		})
	}
}

// TestModel_HasRightOverflow tests right overflow detection.
func TestModel_HasRightOverflow(t *testing.T) {
	tests := []struct {
		name       string
		setupModel func() Model
		expected   bool
	}{
		{
			name: "no right overflow - window covers all levels",
			setupModel: func() Model {
				root := &stack.Node{
					Name: "root",
					Children: []*stack.Node{
						{Name: "child"},
					},
				}
				nav := stack.NewNavigator(root, 2)
				state := stack.NewNavigationState(2)

				return Model{
					navigator:        nav,
					navState:         state,
					navigationOffset: 0,
					focusedColumn:    0,
				}
			},
			expected: false,
		},
		{
			name: "has right overflow - deep tree with children",
			setupModel: func() Model {
				// Create deep tree: root -> l1 -> l2 -> l3 -> l4 -> l5
				root := &stack.Node{Name: "root"}
				current := root
				for i := 1; i <= 5; i++ {
					child := &stack.Node{Name: "level"}
					current.Children = []*stack.Node{child}
					current = child
				}

				nav := stack.NewNavigator(root, 5)
				state := stack.NewNavigationState(5)
				nav.PropagateSelection(state)

				return Model{
					navigator:        nav,
					navState:         state,
					navigationOffset: 0,
					focusedColumn:    1, // Focus on first nav column
				}
			},
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := tt.setupModel()
			result := m.hasRightOverflow()
			assert.Equal(t, tt.expected, result)
		})
	}
}

// TestModel_GetCurrentNavigationPath tests breadcrumb path generation.
func TestModel_GetCurrentNavigationPath(t *testing.T) {
	tests := []struct {
		name       string
		setupModel func() Model
		expected   string
	}{
		{
			name: "commands column - returns root path",
			setupModel: func() Model {
				root := &stack.Node{
					Name: "root",
					Path: "/test/root",
				}
				return NewModel(root, 1, []string{"plan"}, 3)
			},
			expected: "/test/root",
		},
		{
			name: "nil root - returns tilde",
			setupModel: func() Model {
				nav := stack.NewNavigator(nil, 1)
				state := stack.NewNavigationState(1)
				return Model{
					navigator:     nav,
					navState:      state,
					focusedColumn: 0,
				}
			},
			expected: "~",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := tt.setupModel()
			path := m.getCurrentNavigationPath()
			assert.Equal(t, tt.expected, path)
		})
	}
}

// TestModel_GetCurrentNavigationPath_WithStackMarker tests path generation with emoji marker.
func TestModel_GetCurrentNavigationPath_WithStackMarker(t *testing.T) {
	root := &stack.Node{
		Name: "root",
		Path: "/test/root",
		Children: []*stack.Node{
			{
				Name:    "env",
				Path:    "/test/root/env",
				IsStack: true,
			},
		},
	}

	nav := stack.NewNavigator(root, 1)
	state := stack.NewNavigationState(1)
	nav.PropagateSelection(state)

	m := Model{
		navigator:     nav,
		navState:      state,
		focusedColumn: 1, // First navigation column
	}

	path := m.getCurrentNavigationPath()
	assert.Contains(t, path, "/test/root")
}

// TestModel_GetSelectedStackPath tests path retrieval for different focus states.
func TestModel_GetSelectedStackPath(t *testing.T) {
	root := &stack.Node{
		Name: "root",
		Path: "/test/root",
		Children: []*stack.Node{
			{Name: "env", Path: "/test/root/env"},
		},
	}

	tests := []struct {
		name          string
		focusedColumn int
		expected      string
	}{
		{
			name:          "commands column - returns root path",
			focusedColumn: 0,
			expected:      "/test/root",
		},
		{
			name:          "navigation column - returns selected path",
			focusedColumn: 1,
			expected:      "/test/root/env",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := NewModel(root, 1, []string{"plan"}, 3)
			m.focusedColumn = tt.focusedColumn

			path := m.GetSelectedStackPath()
			assert.Equal(t, tt.expected, path)
		})
	}
}

// TestModel_GetSelectedStackPath_NilNode tests GetSelectedStackPath with nil target node.
func TestModel_GetSelectedStackPath_NilNode(t *testing.T) {
	nav := stack.NewNavigator(nil, 0)
	state := stack.NewNavigationState(0)

	m := Model{
		navigator:     nav,
		navState:      state,
		focusedColumn: 0,
	}

	path := m.GetSelectedStackPath()
	assert.Equal(t, NoItemSelected, path)
}

// TestModel_GetSelectedCommand tests command retrieval.
func TestModel_GetSelectedCommand(t *testing.T) {
	commands := []string{"plan", "apply", "destroy"}

	tests := []struct {
		name            string
		selectedCommand int
		expected        string
	}{
		{
			name:            "first command",
			selectedCommand: 0,
			expected:        "plan",
		},
		{
			name:            "second command",
			selectedCommand: 1,
			expected:        "apply",
		},
		{
			name:            "third command",
			selectedCommand: 2,
			expected:        "destroy",
		},
		{
			name:            "negative index returns no item selected",
			selectedCommand: -1,
			expected:        NoItemSelected,
		},
		{
			name:            "out of bounds index returns no item selected",
			selectedCommand: 10,
			expected:        NoItemSelected,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			root := &stack.Node{Name: "root"}
			m := NewModel(root, 1, commands, 3)
			m.selectedCommand = tt.selectedCommand

			cmd := m.GetSelectedCommand()
			assert.Equal(t, tt.expected, cmd)
		})
	}
}

// TestModel_IsConfirmed tests the IsConfirmed getter.
func TestModel_IsConfirmed(t *testing.T) {
	tests := []struct {
		name      string
		confirmed bool
		expected  bool
	}{
		{
			name:      "not confirmed",
			confirmed: false,
			expected:  false,
		},
		{
			name:      "confirmed",
			confirmed: true,
			expected:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := Model{confirmed: tt.confirmed}
			assert.Equal(t, tt.expected, m.IsConfirmed())
		})
	}
}
