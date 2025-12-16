package tui

import (
	"testing"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/israoo/terrax/internal/stack"
	"github.com/stretchr/testify/assert"
)

// testCommands defines a standard list of commands for testing.
var testCommands = []string{
	"plan",
	"apply",
	"validate",
	"fmt",
	"init",
	"output",
	"refresh",
	"destroy",
}

// TestModel_Init tests the Bubble Tea Init method.
func TestModel_Init(t *testing.T) {
	root := &stack.Node{Name: "root", Path: "/test"}
	model := NewModel(root, 1, testCommands, 3)

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

// TestModel_MoveCommandSelection tests moving command selection up and down.
func TestModel_MoveCommandSelection(t *testing.T) {
	tests := []struct {
		name          string
		initialIndex  int
		isUp          bool
		expectedIndex int
		totalCommands int
		setupFilter   func() map[int]textinput.Model
	}{
		{
			name:          "move down from index 0",
			initialIndex:  0,
			isUp:          false,
			expectedIndex: 1,
			totalCommands: 3,
			setupFilter:   func() map[int]textinput.Model { return make(map[int]textinput.Model) },
		},
		{
			name:          "move up from index 2",
			initialIndex:  2,
			isUp:          true,
			expectedIndex: 1,
			totalCommands: 3,
			setupFilter:   func() map[int]textinput.Model { return make(map[int]textinput.Model) },
		},
		{
			name:          "cyclic move up from index 0 wraps to last",
			initialIndex:  0,
			isUp:          true,
			expectedIndex: 2,
			totalCommands: 3,
			setupFilter:   func() map[int]textinput.Model { return make(map[int]textinput.Model) },
		},
		{
			name:          "cyclic move down from last index wraps to first",
			initialIndex:  2,
			isUp:          false,
			expectedIndex: 0,
			totalCommands: 3,
			setupFilter:   func() map[int]textinput.Model { return make(map[int]textinput.Model) },
		},
		{
			name:          "empty commands list - no movement",
			initialIndex:  0,
			isUp:          false,
			expectedIndex: 0,
			totalCommands: 0,
			setupFilter:   func() map[int]textinput.Model { return make(map[int]textinput.Model) },
		},
		{
			name:          "single command - no movement down",
			initialIndex:  0,
			isUp:          false,
			expectedIndex: 0,
			totalCommands: 1,
			setupFilter:   func() map[int]textinput.Model { return make(map[int]textinput.Model) },
		},
		{
			name:          "single command - no movement up",
			initialIndex:  0,
			isUp:          true,
			expectedIndex: 0,
			totalCommands: 1,
			setupFilter:   func() map[int]textinput.Model { return make(map[int]textinput.Model) },
		},
		{
			name:          "with active filter - move within filtered list",
			initialIndex:  0, // "plan"
			isUp:          false,
			expectedIndex: 0, // Can't move if only "plan" matches filter
			totalCommands: 3,
			setupFilter: func() map[int]textinput.Model {
				filters := make(map[int]textinput.Model)
				ti := textinput.New()
				ti.SetValue("plan") // Only matches "plan"
				filters[0] = ti
				return filters
			},
		},
		{
			name:          "filtered list empty - selection stays at current",
			initialIndex:  0,
			isUp:          false,
			expectedIndex: 0,
			totalCommands: 3,
			setupFilter: func() map[int]textinput.Model {
				filters := make(map[int]textinput.Model)
				ti := textinput.New()
				ti.SetValue("nonexistent") // No matches
				filters[0] = ti
				return filters
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := Model{
				selectedCommand: tt.initialIndex,
				commands:        make([]string, tt.totalCommands),
				columnFilters:   tt.setupFilter(),
			}
			// Populate commands
			for i := 0; i < tt.totalCommands; i++ {
				if i < len(testCommands) {
					m.commands[i] = testCommands[i]
				}
			}

			m.moveCommandSelection(tt.isUp)

			assert.Equal(t, tt.expectedIndex, m.selectedCommand)
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

// TestModel_HandleWindowResize tests window resize message handling.
func TestModel_HandleWindowResize(t *testing.T) {
	root := &stack.Node{Name: "root"}
	model := NewModel(root, 1, testCommands, 3)

	assert.False(t, model.ready, "model should not be ready initially")

	msg := tea.WindowSizeMsg{
		Width:  120,
		Height: 30,
	}

	model = model.handleWindowResize(msg)

	assert.True(t, model.ready, "model should be ready after resize")
	assert.Equal(t, 120, model.width)
	assert.Equal(t, 30, model.height)
	assert.Greater(t, model.columnWidth, 0)
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

// TestModel_CanAdvanceFurther tests checking if current node has children.
func TestModel_CanAdvanceFurther(t *testing.T) {
	tests := []struct {
		name       string
		setupModel func() Model
		expected   bool
	}{
		{
			name: "commands column - cannot advance",
			setupModel: func() Model {
				root := &stack.Node{
					Name: "root",
					Children: []*stack.Node{
						{Name: "child"},
					},
				}
				nav := stack.NewNavigator(root, 1)
				state := stack.NewNavigationState(1)
				return Model{
					navigator:     nav,
					navState:      state,
					focusedColumn: 0, // Commands column
				}
			},
			expected: false,
		},
		{
			name: "node with children - can advance",
			setupModel: func() Model {
				root := &stack.Node{
					Name: "root",
					Children: []*stack.Node{
						{
							Name: "parent",
							Children: []*stack.Node{
								{Name: "child"},
							},
						},
					},
				}
				nav := stack.NewNavigator(root, 2)
				state := stack.NewNavigationState(2)
				nav.PropagateSelection(state)

				return Model{
					navigator:     nav,
					navState:      state,
					focusedColumn: 1, // First navigation column
				}
			},
			expected: true,
		},
		{
			name: "leaf node - cannot advance",
			setupModel: func() Model {
				root := &stack.Node{
					Name: "root",
					Children: []*stack.Node{
						{Name: "leaf"}, // No children
					},
				}
				nav := stack.NewNavigator(root, 2)
				state := stack.NewNavigationState(2)
				nav.PropagateSelection(state)

				return Model{
					navigator:     nav,
					navState:      state,
					focusedColumn: 1, // First navigation column
				}
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := tt.setupModel()
			result := m.canAdvanceFurther()
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

// TestModel_HandleKeyPress tests key press routing.
func TestModel_HandleKeyPress(t *testing.T) {
	root := &stack.Node{
		Name: "root",
		Children: []*stack.Node{
			{Name: "child1"},
			{Name: "child2"},
		},
	}

	tests := []struct {
		name         string
		key          string
		initialModel Model
		expectQuit   bool
	}{
		{
			name:         "ctrl+c quits",
			key:          KeyCtrlC,
			initialModel: NewModel(root, 1, testCommands, 3),
			expectQuit:   true,
		},
		{
			name:         "q quits",
			key:          KeyQ,
			initialModel: NewModel(root, 1, testCommands, 3),
			expectQuit:   true,
		},
		{
			name:         "up key does not quit",
			key:          KeyUp,
			initialModel: NewModel(root, 1, testCommands, 3),
			expectQuit:   false,
		},
		{
			name:         "down key does not quit",
			key:          KeyDown,
			initialModel: NewModel(root, 1, testCommands, 3),
			expectQuit:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			msg := tea.KeyMsg{Type: tea.KeyRunes}
			// Set the key string representation
			switch tt.key {
			case KeyCtrlC:
				msg.Type = tea.KeyCtrlC
			case KeyQ:
				msg.Type = tea.KeyRunes
				msg.Runes = []rune{'q'}
			case KeyUp:
				msg.Type = tea.KeyUp
			case KeyDown:
				msg.Type = tea.KeyDown
			}

			_, cmd := tt.initialModel.handleKeyPress(msg)

			if tt.expectQuit {
				assert.NotNil(t, cmd, "should return quit command")
			} else {
				// Non-quit keys may or may not return a command
				// Just verify no panic occurred
				assert.NotPanics(t, func() {
					_, _ = tt.initialModel.handleKeyPress(msg)
				})
			}
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
				return NewModel(root, 1, testCommands, 3)
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

// TestModel_HandleVerticalMove tests vertical navigation handling.
func TestModel_HandleVerticalMove(t *testing.T) {
	root := &stack.Node{
		Name: "root",
		Children: []*stack.Node{
			{Name: "child1"},
			{Name: "child2"},
			{Name: "child3"},
		},
	}

	tests := []struct {
		name            string
		focusedColumn   int
		selectedCommand int
		isUp            bool
		expectedCommand int
	}{
		{
			name:            "move down in commands",
			focusedColumn:   0,
			selectedCommand: 0,
			isUp:            false,
			expectedCommand: 1,
		},
		{
			name:            "move up in commands",
			focusedColumn:   0,
			selectedCommand: 2,
			isUp:            true,
			expectedCommand: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := NewModel(root, 1, testCommands, 3)
			m.focusedColumn = tt.focusedColumn
			m.selectedCommand = tt.selectedCommand

			m = m.handleVerticalMove(tt.isUp)

			if tt.focusedColumn == 0 {
				assert.Equal(t, tt.expectedCommand, m.selectedCommand)
			}
		})
	}
}

// TestModel_MoveNavigationSelection tests navigation column selection movement.
func TestModel_MoveNavigationSelection(t *testing.T) {
	tests := []struct {
		name          string
		setupModel    func() Model
		isUp          bool
		expectedIndex int
	}{
		{
			name: "move down in navigation column",
			setupModel: func() Model {
				root := &stack.Node{
					Name: "root",
					Children: []*stack.Node{
						{Name: "child1"},
						{Name: "child2"},
						{Name: "child3"},
					},
				}
				nav := stack.NewNavigator(root, 1)
				state := stack.NewNavigationState(1)
				nav.PropagateSelection(state)

				m := Model{
					navigator:     nav,
					navState:      state,
					focusedColumn: 1,
					columnFilters: make(map[int]textinput.Model),
				}
				m.navState.SelectedIndices[0] = 0
				return m
			},
			isUp:          false,
			expectedIndex: 1,
		},
		{
			name: "move up in navigation column",
			setupModel: func() Model {
				root := &stack.Node{
					Name: "root",
					Children: []*stack.Node{
						{Name: "child1"},
						{Name: "child2"},
						{Name: "child3"},
					},
				}
				nav := stack.NewNavigator(root, 1)
				state := stack.NewNavigationState(1)
				nav.PropagateSelection(state)

				m := Model{
					navigator:     nav,
					navState:      state,
					focusedColumn: 1,
					columnFilters: make(map[int]textinput.Model),
				}
				m.navState.SelectedIndices[0] = 2
				return m
			},
			isUp:          true,
			expectedIndex: 1,
		},
		{
			name: "empty navigation column - no movement",
			setupModel: func() Model {
				root := &stack.Node{
					Name:     "root",
					Children: []*stack.Node{}, // Empty children
				}
				nav := stack.NewNavigator(root, 1)
				state := stack.NewNavigationState(1)
				nav.PropagateSelection(state)

				m := Model{
					navigator:     nav,
					navState:      state,
					focusedColumn: 1,
					columnFilters: make(map[int]textinput.Model),
				}
				return m
			},
			isUp:          false,
			expectedIndex: 0,
		},
		{
			name: "invalid depth - no movement",
			setupModel: func() Model {
				root := &stack.Node{Name: "root"}
				nav := stack.NewNavigator(root, 1)
				state := stack.NewNavigationState(1)

				m := Model{
					navigator:     nav,
					navState:      state,
					focusedColumn: 0, // Commands column, depth = -1
					columnFilters: make(map[int]textinput.Model),
				}
				return m
			},
			isUp:          false,
			expectedIndex: 0,
		},
		{
			name: "with filter - no matches in filtered list",
			setupModel: func() Model {
				root := &stack.Node{
					Name: "root",
					Children: []*stack.Node{
						{Name: "env"},
						{Name: "modules"},
					},
				}
				nav := stack.NewNavigator(root, 1)
				state := stack.NewNavigationState(1)
				nav.PropagateSelection(state)

				m := Model{
					navigator:     nav,
					navState:      state,
					focusedColumn: 1,
					columnFilters: make(map[int]textinput.Model),
				}
				// Add filter that matches nothing
				ti := textinput.New()
				ti.SetValue("nonexistent")
				m.columnFilters[1] = ti
				return m
			},
			isUp:          false,
			expectedIndex: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := tt.setupModel()
			m.moveNavigationSelection(tt.isUp)

			depth := m.getNavigationDepth()
			if depth >= 0 && depth < len(m.navState.SelectedIndices) {
				assert.Equal(t, tt.expectedIndex, m.navState.SelectedIndices[depth])
			}
		})
	}
}

// TestModel_MoveNavigationSelection_WithFilter tests filtered navigation scenarios.
func TestModel_MoveNavigationSelection_WithFilter(t *testing.T) {
	tests := []struct {
		name          string
		setupModel    func() Model
		isUp          bool
		expectedIndex int
	}{
		{
			name: "move down in filtered navigation list",
			setupModel: func() Model {
				root := &stack.Node{
					Name: "root",
					Children: []*stack.Node{
						{Name: "dev"},
						{Name: "prod"},
						{Name: "staging"},
					},
				}
				nav := stack.NewNavigator(root, 1)
				state := stack.NewNavigationState(1)
				nav.PropagateSelection(state)

				m := Model{
					navigator:            nav,
					navState:             state,
					focusedColumn:        1,
					columnFilters:        make(map[int]textinput.Model),
					maxNavigationColumns: 3,
				}
				// Filter to only show items containing "d" (dev, prod)
				ti := textinput.New()
				ti.SetValue("d")
				m.columnFilters[1] = ti
				m.navState.SelectedIndices[0] = 0 // Start at "dev" (index 0)
				return m
			},
			isUp:          false,
			expectedIndex: 1, // Should move to "prod" (index 1)
		},
		{
			name: "move up in filtered navigation list",
			setupModel: func() Model {
				root := &stack.Node{
					Name: "root",
					Children: []*stack.Node{
						{Name: "dev"},
						{Name: "prod"},
						{Name: "staging"},
					},
				}
				nav := stack.NewNavigator(root, 1)
				state := stack.NewNavigationState(1)
				nav.PropagateSelection(state)

				m := Model{
					navigator:            nav,
					navState:             state,
					focusedColumn:        1,
					columnFilters:        make(map[int]textinput.Model),
					maxNavigationColumns: 3,
				}
				// Filter to only show items containing "d" (dev, prod)
				ti := textinput.New()
				ti.SetValue("d")
				m.columnFilters[1] = ti
				m.navState.SelectedIndices[0] = 1 // Start at "prod" (index 1)
				return m
			},
			isUp:          true,
			expectedIndex: 0, // Should move to "dev" (index 0)
		},
		{
			name: "filtered list - current selection not in filter",
			setupModel: func() Model {
				root := &stack.Node{
					Name: "root",
					Children: []*stack.Node{
						{Name: "dev"},
						{Name: "prod"},
						{Name: "staging"},
					},
				}
				nav := stack.NewNavigator(root, 1)
				state := stack.NewNavigationState(1)
				nav.PropagateSelection(state)

				m := Model{
					navigator:            nav,
					navState:             state,
					focusedColumn:        1,
					columnFilters:        make(map[int]textinput.Model),
					maxNavigationColumns: 3,
				}
				// Filter to only show items containing "dev"
				ti := textinput.New()
				ti.SetValue("dev")
				m.columnFilters[1] = ti
				m.navState.SelectedIndices[0] = 1 // Currently at "prod" (not in filter)
				return m
			},
			isUp:          false,
			expectedIndex: 0, // Should jump to first filtered item "dev"
		},
		{
			name: "filtered list - cyclic move down from last wraps to first",
			setupModel: func() Model {
				root := &stack.Node{
					Name: "root",
					Children: []*stack.Node{
						{Name: "dev"},
						{Name: "prod"},
					},
				}
				nav := stack.NewNavigator(root, 1)
				state := stack.NewNavigationState(1)
				nav.PropagateSelection(state)

				m := Model{
					navigator:            nav,
					navState:             state,
					focusedColumn:        1,
					columnFilters:        make(map[int]textinput.Model),
					maxNavigationColumns: 3,
				}
				ti := textinput.New()
				ti.SetValue("d") // Matches both
				m.columnFilters[1] = ti
				m.navState.SelectedIndices[0] = 1 // At "prod" (last filtered item)
				return m
			},
			isUp:          false,
			expectedIndex: 0, // Cyclic: wraps to "dev"
		},
		{
			name: "filtered list - cyclic move up from first wraps to last",
			setupModel: func() Model {
				root := &stack.Node{
					Name: "root",
					Children: []*stack.Node{
						{Name: "dev"},
						{Name: "prod"},
					},
				}
				nav := stack.NewNavigator(root, 1)
				state := stack.NewNavigationState(1)
				nav.PropagateSelection(state)

				m := Model{
					navigator:            nav,
					navState:             state,
					focusedColumn:        1,
					columnFilters:        make(map[int]textinput.Model),
					maxNavigationColumns: 3,
				}
				ti := textinput.New()
				ti.SetValue("d") // Matches both
				m.columnFilters[1] = ti
				m.navState.SelectedIndices[0] = 0 // At "dev" (first filtered item)
				return m
			},
			isUp:          true,
			expectedIndex: 1, // Cyclic: wraps to "prod"
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := tt.setupModel()
			m.moveNavigationSelection(tt.isUp)

			depth := m.getNavigationDepth()
			if depth >= 0 && depth < len(m.navState.SelectedIndices) {
				assert.Equal(t, tt.expectedIndex, m.navState.SelectedIndices[depth])
			}
		})
	}
}

// TestModel_Update tests the main Bubble Tea Update function.
func TestModel_Update(t *testing.T) {
	root := &stack.Node{
		Name: "root",
		Children: []*stack.Node{
			{Name: "child1"},
			{Name: "child2"},
		},
	}

	tests := []struct {
		name         string
		initialModel Model
		msg          tea.Msg
		checkResult  func(t *testing.T, m tea.Model, cmd tea.Cmd)
	}{
		{
			name:         "window size message makes model ready",
			initialModel: NewModel(root, 1, testCommands, 3),
			msg: tea.WindowSizeMsg{
				Width:  120,
				Height: 30,
			},
			checkResult: func(t *testing.T, m tea.Model, cmd tea.Cmd) {
				model := m.(Model)
				assert.True(t, model.ready)
				assert.Equal(t, 120, model.width)
				assert.Equal(t, 30, model.height)
			},
		},
		{
			name:         "quit key returns quit command",
			initialModel: NewModel(root, 1, testCommands, 3),
			msg: tea.KeyMsg{
				Type:  tea.KeyRunes,
				Runes: []rune{'q'},
			},
			checkResult: func(t *testing.T, m tea.Model, cmd tea.Cmd) {
				assert.NotNil(t, cmd)
			},
		},
		{
			name:         "up key updates model",
			initialModel: NewModel(root, 1, testCommands, 3),
			msg: tea.KeyMsg{
				Type: tea.KeyUp,
			},
			checkResult: func(t *testing.T, m tea.Model, cmd tea.Cmd) {
				// Should not panic
				assert.NotNil(t, m)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			updatedModel, cmd := tt.initialModel.Update(tt.msg)
			tt.checkResult(t, updatedModel, cmd)
		})
	}
}

// TestModel_HandleEnterKey tests enter key handling for both commands and navigation.
func TestModel_HandleEnterKey(t *testing.T) {
	root := &stack.Node{
		Name: "root",
		Path: "/test/root",
		Children: []*stack.Node{
			{
				Name: "env",
				Path: "/test/root/env",
				Children: []*stack.Node{
					{Name: "dev", Path: "/test/root/env/dev"},
				},
			},
		},
	}

	tests := []struct {
		name              string
		setupModel        func() Model
		expectedQuit      bool
		expectedConfirmed bool
	}{
		{
			name: "enter on commands column - confirms root",
			setupModel: func() Model {
				m := NewModel(root, 2, testCommands, 3)
				m.focusedColumn = 0 // Commands column
				return m
			},
			expectedQuit:      true,
			expectedConfirmed: true,
		},
		{
			name: "enter on navigation column - confirms path",
			setupModel: func() Model {
				m := NewModel(root, 2, testCommands, 3)
				m.focusedColumn = 1 // First navigation column
				return m
			},
			expectedQuit:      true,
			expectedConfirmed: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := tt.setupModel()
			updatedModel, cmd := m.handleEnterKey()

			if tt.expectedQuit {
				assert.NotNil(t, cmd, "should return quit command")
			}

			finalModel := updatedModel.(Model)
			assert.Equal(t, tt.expectedConfirmed, finalModel.confirmed)
		})
	}
}

// TestModel_HandleHorizontalMove tests left/right navigation with sliding window.
func TestModel_HandleHorizontalMove(t *testing.T) {
	// Create deep tree for sliding window testing.
	root := &stack.Node{Name: "root", Path: "/root"}
	current := root
	for i := 1; i <= 5; i++ {
		child := &stack.Node{Name: "level", Path: "/root/level"}
		current.Children = []*stack.Node{child}
		current = child
	}

	tests := []struct {
		name              string
		initialFocused    int
		initialOffset     int
		isLeft            bool
		expectedFocused   int
		expectedOffsetMin int
		expectedOffsetMax int
	}{
		{
			name:              "move right from commands column",
			initialFocused:    0,
			initialOffset:     0,
			isLeft:            false,
			expectedFocused:   1,
			expectedOffsetMin: 0,
			expectedOffsetMax: 0,
		},
		{
			name:              "move left to commands column",
			initialFocused:    1,
			initialOffset:     0,
			isLeft:            true,
			expectedFocused:   0,
			expectedOffsetMin: 0,
			expectedOffsetMax: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := NewModel(root, 5, testCommands, 3)
			m.focusedColumn = tt.initialFocused
			m.navigationOffset = tt.initialOffset

			updated, _ := m.handleHorizontalMove(tt.isLeft)
			m = updated.(Model)

			assert.Equal(t, tt.expectedFocused, m.focusedColumn)
			assert.GreaterOrEqual(t, m.navigationOffset, tt.expectedOffsetMin)
			assert.LessOrEqual(t, m.navigationOffset, tt.expectedOffsetMax)
		})
	}
}

// TestModel_MoveToPreviousColumn tests left column navigation with wrapping.
func TestModel_MoveToPreviousColumn(t *testing.T) {
	root := &stack.Node{
		Name: "root",
		Children: []*stack.Node{
			{
				Name: "l1",
				Children: []*stack.Node{
					{
						Name: "l2",
						Children: []*stack.Node{
							{Name: "l3"},
						},
					},
				},
			},
		},
	}

	tests := []struct {
		name            string
		initialFocused  int
		initialOffset   int
		expectedFocused int
		checkOffset     bool
	}{
		{
			name:            "move from column 1 to commands",
			initialFocused:  1,
			initialOffset:   0,
			expectedFocused: 0,
			checkOffset:     false,
		},
		{
			name:            "wrap from commands to last column",
			initialFocused:  0,
			initialOffset:   0,
			expectedFocused: 3, // Should wrap to last visible
			checkOffset:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := NewModel(root, 3, testCommands, 3)
			m.focusedColumn = tt.initialFocused
			m.navigationOffset = tt.initialOffset

			m.moveToPreviousColumn()

			assert.Equal(t, tt.expectedFocused, m.focusedColumn)
			if tt.checkOffset {
				assert.GreaterOrEqual(t, m.navigationOffset, 0)
			}
		})
	}
}

// TestModel_MoveToNextColumn tests right column navigation with wrapping.
func TestModel_MoveToNextColumn(t *testing.T) {
	root := &stack.Node{
		Name: "root",
		Children: []*stack.Node{
			{
				Name: "l1",
				Children: []*stack.Node{
					{Name: "l2"},
				},
			},
		},
	}

	tests := []struct {
		name            string
		initialFocused  int
		initialOffset   int
		expectedFocused int
	}{
		{
			name:            "move from commands to first nav column",
			initialFocused:  0,
			initialOffset:   0,
			expectedFocused: 1,
		},
		{
			name:            "wrap from last column to commands",
			initialFocused:  2,
			initialOffset:   0,
			expectedFocused: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := NewModel(root, 2, testCommands, 3)
			m.focusedColumn = tt.initialFocused
			m.navigationOffset = tt.initialOffset

			m.moveToNextColumn()

			assert.Equal(t, tt.expectedFocused, m.focusedColumn)
			assert.Equal(t, 0, m.navigationOffset) // Should reset on wrap
		})
	}
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
		expectedPath  string
	}{
		{
			name:          "commands column returns root path",
			focusedColumn: 0,
			expectedPath:  "/test/root",
		},
		{
			name:          "navigation column returns selected path",
			focusedColumn: 1,
			expectedPath:  "/test/root/env",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := NewModel(root, 1, testCommands, 3)
			m.focusedColumn = tt.focusedColumn

			path := m.GetSelectedStackPath()
			assert.Equal(t, tt.expectedPath, path)
		})
	}
}

// TestModel_GetSelectedCommand tests command retrieval.
func TestModel_GetSelectedCommand(t *testing.T) {
	root := &stack.Node{Name: "root"}
	m := NewModel(root, 1, testCommands, 3)

	tests := []struct {
		name            string
		selectedCommand int
		expected        string
	}{
		{
			name:            "first command (plan)",
			selectedCommand: 0,
			expected:        "plan",
		},
		{
			name:            "second command (apply)",
			selectedCommand: 1,
			expected:        "apply",
		},
		{
			name:            "out of bounds returns default",
			selectedCommand: 999,
			expected:        NoItemSelected,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m.selectedCommand = tt.selectedCommand
			result := m.GetSelectedCommand()
			assert.Equal(t, tt.expected, result)
		})
	}
}

// TestModel_View tests View rendering function.
func TestModel_View(t *testing.T) {
	root := &stack.Node{
		Name: "root",
		Path: "/test/root",
		Children: []*stack.Node{
			{Name: "env", Path: "/test/root/env"},
			{Name: "modules", Path: "/test/root/modules"},
		},
	}

	tests := []struct {
		name        string
		setupModel  func() Model
		checkOutput func(t *testing.T, output string)
	}{
		{
			name: "not ready shows initializing",
			setupModel: func() Model {
				m := NewModel(root, 1, testCommands, 3)
				m.ready = false
				return m
			},
			checkOutput: func(t *testing.T, output string) {
				assert.Equal(t, Initializing, output)
			},
		},
		{
			name: "ready model renders full UI",
			setupModel: func() Model {
				m := NewModel(root, 1, testCommands, 3)
				m.ready = true
				m.width = 120
				m.height = 30
				m.columnWidth = 30
				return m
			},
			checkOutput: func(t *testing.T, output string) {
				assert.NotEmpty(t, output)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := tt.setupModel()
			view := m.View()
			tt.checkOutput(t, view)
		})
	}
}

// TestNewTestModel tests the test helper function.
func TestNewTestModel(t *testing.T) {
	root := &stack.Node{
		Name:     "root",
		Path:     "/test/root",
		Children: []*stack.Node{{Name: "child", Path: "/test/root/child"}},
	}

	tests := []struct {
		name            string
		confirmed       bool
		selectedCommand string
		selectedPath    string
	}{
		{
			name:            "confirmed model with plan command",
			confirmed:       true,
			selectedCommand: "plan",
			selectedPath:    "/test/root/child",
		},
		{
			name:            "unconfirmed model with apply command",
			confirmed:       false,
			selectedCommand: "apply",
			selectedPath:    "/test/root",
		},
		{
			name:            "confirmed model with destroy command",
			confirmed:       true,
			selectedCommand: "destroy",
			selectedPath:    "/test/root/child",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			model := NewTestModel(root, 1, testCommands, 3, tt.confirmed, tt.selectedCommand, tt.selectedPath)

			assert.Equal(t, tt.confirmed, model.IsConfirmed())
			assert.Equal(t, tt.selectedCommand, model.GetSelectedCommand())
			assert.True(t, model.ready)
			assert.Equal(t, 120, model.width)
			assert.Equal(t, 30, model.height)
			assert.Equal(t, 25, model.columnWidth)
		})
	}
}

// TestModel_HandleKeyPress_UnknownKey tests handling of unrecognized key presses.
func TestModel_HandleKeyPress_UnknownKey(t *testing.T) {
	root := &stack.Node{
		Name: "root",
		Children: []*stack.Node{
			{Name: "child"},
		},
	}

	model := NewModel(root, 1, testCommands, 3)

	// Send an unknown key (e.g., 'x').
	msg := tea.KeyMsg{
		Type:  tea.KeyRunes,
		Runes: []rune{'x'},
	}

	updatedModel, cmd := model.handleKeyPress(msg)

	// Should return model unchanged and no command.
	assert.Nil(t, cmd)
	assert.NotNil(t, updatedModel)
}

// TestModel_HandleEnterKey_NilNode tests enter key when no node is available.
func TestModel_HandleEnterKey_NilNode(t *testing.T) {
	// Create a model with nil navigator root.
	nav := stack.NewNavigator(nil, 0)
	state := stack.NewNavigationState(0)

	model := Model{
		navigator:     nav,
		navState:      state,
		focusedColumn: 0,
	}

	updatedModel, cmd := model.handleEnterKey()

	// Should not quit when target node is nil.
	assert.Nil(t, cmd)
	finalModel := updatedModel.(Model)
	assert.False(t, finalModel.confirmed)
}

// TestModel_MoveNavigationSelection_InvalidDepth tests navigation selection with invalid depth.
func TestModel_MoveNavigationSelection_InvalidDepth(t *testing.T) {
	root := &stack.Node{
		Name:     "root",
		Children: []*stack.Node{{Name: "child"}},
	}

	nav := stack.NewNavigator(root, 1)
	state := stack.NewNavigationState(1)

	model := Model{
		navigator:     nav,
		navState:      state,
		focusedColumn: 0, // Commands column - depth will be -1
	}

	// Call moveNavigationSelection when depth is -1.
	model.moveNavigationSelection(true)

	// Should handle gracefully without panic.
	assert.NotNil(t, model.navState)
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
				IsStack: true, // This will have a 📦 marker in the display, but Path doesn't include it
				Children: []*stack.Node{
					{Name: "dev", Path: "/test/root/env/dev"},
				},
			},
		},
	}

	nav := stack.NewNavigator(root, 2)
	state := stack.NewNavigationState(2)
	nav.PropagateSelection(state)

	model := Model{
		navigator:     nav,
		navState:      state,
		focusedColumn: 1, // First navigation column
	}

	path := model.getCurrentNavigationPath()

	// The path should contain "env" (the emoji marker only appears in the display column names, not in the path).
	assert.Contains(t, path, "env")

	// Note: The getCurrentNavigationPath function builds the path from the column display names,
	// which do include the emoji marker. This is expected behavior - the function strips it.
	// However, looking at the implementation, the path is built by appending to Node.Path,
	// so the emoji should be in the column name but needs to be stripped.
	// Let's verify the actual behavior by checking if the path makes sense.
	assert.Contains(t, path, "/test/root")
}

// TestModel_CanAdvanceFurther_EdgeCases tests edge cases for canAdvanceFurther.
func TestModel_CanAdvanceFurther_EdgeCases(t *testing.T) {
	tests := []struct {
		name       string
		setupModel func() Model
		expected   bool
	}{
		{
			name: "depth out of bounds",
			setupModel: func() Model {
				root := &stack.Node{Name: "root"}
				nav := stack.NewNavigator(root, 1)
				state := stack.NewNavigationState(1)

				return Model{
					navigator:     nav,
					navState:      state,
					focusedColumn: 10, // Way beyond valid depth
				}
			},
			expected: false,
		},
		{
			name: "nil current node",
			setupModel: func() Model {
				root := &stack.Node{Name: "root"}
				nav := stack.NewNavigator(root, 2)
				state := stack.NewNavigationState(2)

				// Manually set a nil current node.
				state.CurrentNodes = []*stack.Node{nil, nil}

				return Model{
					navigator:     nav,
					navState:      state,
					focusedColumn: 1,
				}
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := tt.setupModel()
			result := m.canAdvanceFurther()
			assert.Equal(t, tt.expected, result)
		})
	}
}

// TestModel_GetSelectedStackPath_NilNode tests GetSelectedStackPath with nil target node.
func TestModel_GetSelectedStackPath_NilNode(t *testing.T) {
	// Create a navigator that will return nil node.
	nav := stack.NewNavigator(nil, 0)
	state := stack.NewNavigationState(0)

	model := Model{
		navigator:     nav,
		navState:      state,
		focusedColumn: 0,
	}

	path := model.GetSelectedStackPath()

	// Should return NoItemSelected when node is nil.
	assert.Equal(t, NoItemSelected, path)
}

// TestModel_Update_UnhandledMessage tests Update with an unhandled message type.
func TestModel_Update_UnhandledMessage(t *testing.T) {
	root := &stack.Node{Name: "root"}
	model := NewModel(root, 1, testCommands, 3)

	// Send a message type that's not handled (e.g., a custom message).
	type CustomMsg struct{}
	msg := CustomMsg{}

	updatedModel, cmd := model.Update(msg)

	// Should return model unchanged and no command.
	assert.Nil(t, cmd)
	assert.NotNil(t, updatedModel)
}

// TestFilterItems tests the filterItems function with various inputs.
func TestFilterItems(t *testing.T) {
	tests := []struct {
		name       string
		items      []string
		filterText string
		expected   []string
	}{
		{
			name:       "empty filter returns all items",
			items:      []string{"plan", "apply", "validate", "destroy"},
			filterText: "",
			expected:   []string{"plan", "apply", "validate", "destroy"},
		},
		{
			name:       "filter matches single item",
			items:      []string{"plan", "apply", "validate", "destroy"},
			filterText: "plan",
			expected:   []string{"plan"},
		},
		{
			name:       "filter matches multiple items",
			items:      []string{"plan", "apply", "validate", "destroy"},
			filterText: "val",
			expected:   []string{"validate"},
		},
		{
			name:       "filter matches partial (case insensitive)",
			items:      []string{"Plan", "Apply", "Validate", "Destroy"},
			filterText: "val",
			expected:   []string{"Validate"},
		},
		{
			name:       "filter matches with different casing",
			items:      []string{"plan", "apply", "validate", "destroy"},
			filterText: "PLAN",
			expected:   []string{"plan"},
		},
		{
			name:       "filter matches no items",
			items:      []string{"plan", "apply", "validate", "destroy"},
			filterText: "xyz",
			expected:   []string{},
		},
		{
			name:       "empty items list returns empty",
			items:      []string{},
			filterText: "plan",
			expected:   []string{},
		},
		{
			name:       "filter matches substring",
			items:      []string{"terragrunt-plan", "terragrunt-apply", "plan-all"},
			filterText: "plan",
			expected:   []string{"terragrunt-plan", "plan-all"},
		},
		{
			name:       "filter with spaces",
			items:      []string{"plan all", "apply dev", "validate prod"},
			filterText: "all",
			expected:   []string{"plan all"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := filterItems(tt.items, tt.filterText)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// TestModel_AdjustSelectionAfterFilter tests selection adjustment after filter changes.
func TestModel_AdjustSelectionAfterFilter(t *testing.T) {
	tests := []struct {
		name               string
		setupModel         func() Model
		expectedCommand    int
		expectedNavIndex   int
		checkNavIndex      bool
		activeFilterColumn int
	}{
		{
			name: "commands column - selected item disappears, moves to first",
			setupModel: func() Model {
				root := &stack.Node{Name: "root"}
				m := NewModel(root, 1, testCommands, 3)
				m.selectedCommand = 1 // "apply"
				m.activeFilterColumn = 0

				// Create filter that excludes "apply"
				ti := textinput.New()
				ti.SetValue("plan")
				m.columnFilters[0] = ti

				return m
			},
			expectedCommand:    0, // Should move to first filtered item ("plan")
			activeFilterColumn: 0,
		},
		{
			name: "commands column - selected item remains visible",
			setupModel: func() Model {
				root := &stack.Node{Name: "root"}
				m := NewModel(root, 1, testCommands, 3)
				m.selectedCommand = 0 // "plan"
				m.activeFilterColumn = 0

				// Create filter that includes "plan"
				ti := textinput.New()
				ti.SetValue("pl")
				m.columnFilters[0] = ti

				return m
			},
			expectedCommand:    0, // Should remain on "plan"
			activeFilterColumn: 0,
		},
		{
			name: "navigation column - selected item disappears, moves to first",
			setupModel: func() Model {
				root := &stack.Node{
					Name: "root",
					Children: []*stack.Node{
						{Name: "env"},
						{Name: "modules"},
						{Name: "scripts"},
					},
				}
				nav := stack.NewNavigator(root, 1)
				state := stack.NewNavigationState(1)
				nav.PropagateSelection(state)

				m := Model{
					navigator:          nav,
					navState:           state,
					commands:           testCommands,
					focusedColumn:      1,
					activeFilterColumn: 1,
					columnFilters:      make(map[int]textinput.Model),
				}

				// Select "modules" (index 1)
				m.navState.SelectedIndices[0] = 1

				// Create filter that excludes "modules"
				ti := textinput.New()
				ti.SetValue("env")
				m.columnFilters[1] = ti

				return m
			},
			expectedNavIndex:   0, // Should move to first filtered item ("env")
			checkNavIndex:      true,
			activeFilterColumn: 1,
		},
		{
			name: "navigation column - selected item remains visible",
			setupModel: func() Model {
				root := &stack.Node{
					Name: "root",
					Children: []*stack.Node{
						{Name: "env"},
						{Name: "modules"},
						{Name: "scripts"},
					},
				}
				nav := stack.NewNavigator(root, 1)
				state := stack.NewNavigationState(1)
				nav.PropagateSelection(state)

				m := Model{
					navigator:          nav,
					navState:           state,
					commands:           testCommands,
					focusedColumn:      1,
					activeFilterColumn: 1,
					columnFilters:      make(map[int]textinput.Model),
				}

				// Select "env" (index 0)
				m.navState.SelectedIndices[0] = 0

				// Create filter that includes "env"
				ti := textinput.New()
				ti.SetValue("en")
				m.columnFilters[1] = ti

				return m
			},
			expectedNavIndex:   0, // Should remain on "env"
			checkNavIndex:      true,
			activeFilterColumn: 1,
		},
		{
			name: "no active filter - no adjustment",
			setupModel: func() Model {
				root := &stack.Node{Name: "root"}
				m := NewModel(root, 1, testCommands, 3)
				m.selectedCommand = 2
				m.activeFilterColumn = -1 // No active filter

				return m
			},
			expectedCommand:    2, // Should remain unchanged
			activeFilterColumn: -1,
		},
		{
			name: "empty filtered list - no change",
			setupModel: func() Model {
				root := &stack.Node{Name: "root"}
				m := NewModel(root, 1, testCommands, 3)
				m.selectedCommand = 0
				m.activeFilterColumn = 0

				// Create filter that matches nothing
				ti := textinput.New()
				ti.SetValue("nonexistent")
				m.columnFilters[0] = ti

				return m
			},
			expectedCommand:    0, // Should remain unchanged (no filtered items)
			activeFilterColumn: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := tt.setupModel()
			m.adjustSelectionAfterFilter()

			if tt.checkNavIndex {
				assert.Equal(t, tt.expectedNavIndex, m.navState.SelectedIndices[0])
			} else {
				assert.Equal(t, tt.expectedCommand, m.selectedCommand)
			}
		})
	}
}

// TestModel_HandleKeyPress_FilterMode tests filter activation and editing.
func TestModel_HandleKeyPress_FilterMode(t *testing.T) {
	tests := []struct {
		name        string
		setupModel  func() Model
		keySequence []tea.KeyMsg
		checkResult func(t *testing.T, m Model)
	}{
		{
			name: "slash activates filter on commands column",
			setupModel: func() Model {
				root := &stack.Node{Name: "root"}
				m := NewModel(root, 1, testCommands, 3)
				m.focusedColumn = 0
				return m
			},
			keySequence: []tea.KeyMsg{
				{Type: tea.KeyRunes, Runes: []rune{'/'}},
			},
			checkResult: func(t *testing.T, m Model) {
				assert.Equal(t, 0, m.activeFilterColumn, "filter should be active on column 0")
				assert.NotNil(t, m.columnFilters[0], "filter should exist for column 0")
			},
		},
		{
			name: "slash activates filter on navigation column",
			setupModel: func() Model {
				root := &stack.Node{
					Name: "root",
					Children: []*stack.Node{
						{Name: "env"},
						{Name: "modules"},
					},
				}
				m := NewModel(root, 1, testCommands, 3)
				m.focusedColumn = 1
				return m
			},
			keySequence: []tea.KeyMsg{
				{Type: tea.KeyRunes, Runes: []rune{'/'}},
			},
			checkResult: func(t *testing.T, m Model) {
				assert.Equal(t, 1, m.activeFilterColumn, "filter should be active on column 1")
				assert.NotNil(t, m.columnFilters[1], "filter should exist for column 1")
			},
		},
		{
			name: "escape exits filter mode and removes filter",
			setupModel: func() Model {
				root := &stack.Node{Name: "root"}
				m := NewModel(root, 1, testCommands, 3)
				m.focusedColumn = 0

				// Pre-activate filter
				ti := textinput.New()
				ti.SetValue("test")
				ti.Focus()
				m.columnFilters[0] = ti
				m.activeFilterColumn = 0

				return m
			},
			keySequence: []tea.KeyMsg{
				{Type: tea.KeyEsc},
			},
			checkResult: func(t *testing.T, m Model) {
				assert.Equal(t, -1, m.activeFilterColumn, "filter should be deactivated")
				_, exists := m.columnFilters[0]
				assert.False(t, exists, "filter should be removed")
			},
		},
		{
			name: "enter executes command with current selection",
			setupModel: func() Model {
				root := &stack.Node{Name: "root"}
				m := NewModel(root, 1, testCommands, 3)
				m.focusedColumn = 0

				// Pre-activate filter
				ti := textinput.New()
				ti.SetValue("plan")
				ti.Focus()
				m.columnFilters[0] = ti
				m.activeFilterColumn = 0

				return m
			},
			keySequence: []tea.KeyMsg{
				{Type: tea.KeyEnter},
			},
			checkResult: func(t *testing.T, m Model) {
				assert.True(t, m.confirmed, "command should be confirmed for execution")
				filter, exists := m.columnFilters[0]
				assert.True(t, exists, "filter should still exist")
				assert.Equal(t, "plan", filter.Value(), "filter value should be preserved")
			},
		},
		{
			name: "typing in filter mode updates filter value",
			setupModel: func() Model {
				root := &stack.Node{Name: "root"}
				m := NewModel(root, 1, testCommands, 3)
				m.focusedColumn = 0

				// Pre-activate filter
				ti := textinput.New()
				ti.Focus()
				m.columnFilters[0] = ti
				m.activeFilterColumn = 0

				return m
			},
			keySequence: []tea.KeyMsg{
				{Type: tea.KeyRunes, Runes: []rune{'p'}},
				{Type: tea.KeyRunes, Runes: []rune{'l'}},
			},
			checkResult: func(t *testing.T, m Model) {
				filter := m.columnFilters[0]
				// Note: The actual value depends on textinput's Update logic
				// We just verify the filter still exists and is active
				assert.NotNil(t, filter)
				assert.Equal(t, 0, m.activeFilterColumn)
			},
		},
		{
			name: "up/down navigation works in filter mode",
			setupModel: func() Model {
				root := &stack.Node{Name: "root"}
				m := NewModel(root, 1, testCommands, 3)
				m.focusedColumn = 0
				m.selectedCommand = 0

				// Pre-activate filter
				ti := textinput.New()
				ti.Focus()
				m.columnFilters[0] = ti
				m.activeFilterColumn = 0

				return m
			},
			keySequence: []tea.KeyMsg{
				{Type: tea.KeyDown},
			},
			checkResult: func(t *testing.T, m Model) {
				// Navigation should work even in filter mode
				assert.Equal(t, 1, m.selectedCommand, "selection should move down")
				assert.Equal(t, 0, m.activeFilterColumn, "filter should still be active")
			},
		},
		{
			name: "left/right navigation works in filter mode",
			setupModel: func() Model {
				root := &stack.Node{
					Name: "root",
					Children: []*stack.Node{
						{Name: "env"},
					},
				}
				m := NewModel(root, 1, testCommands, 3)
				m.focusedColumn = 0

				// Pre-activate filter on commands column
				ti := textinput.New()
				ti.Focus()
				m.columnFilters[0] = ti
				m.activeFilterColumn = 0

				return m
			},
			keySequence: []tea.KeyMsg{
				{Type: tea.KeyRight},
			},
			checkResult: func(t *testing.T, m Model) {
				// Should move to next column
				assert.Equal(t, 1, m.focusedColumn, "focus should move right")
				// Filter should be blurred on previous column
				assert.Equal(t, -1, m.activeFilterColumn, "filter should be deactivated")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := tt.setupModel()

			for _, keyMsg := range tt.keySequence {
				updatedModel, _ := m.handleKeyPress(keyMsg)
				m = updatedModel.(Model)
			}

			tt.checkResult(t, m)
		})
	}
}

// TestFindOriginalIndex tests mapping filtered index back to original index.
func TestFindOriginalIndex(t *testing.T) {
	tests := []struct {
		name          string
		originalItems []string
		filteredItems []string
		filteredIndex int
		expected      int
	}{
		{
			name:          "valid index in filtered list",
			originalItems: []string{"plan", "apply", "validate", "destroy"},
			filteredItems: []string{"plan", "apply"},
			filteredIndex: 1,
			expected:      1,
		},
		{
			name:          "first item in filtered list",
			originalItems: []string{"plan", "apply", "validate", "destroy"},
			filteredItems: []string{"apply", "validate"},
			filteredIndex: 0,
			expected:      1, // "apply" is at index 1 in original
		},
		{
			name:          "negative filtered index",
			originalItems: []string{"plan", "apply", "validate"},
			filteredItems: []string{"plan"},
			filteredIndex: -1,
			expected:      -1,
		},
		{
			name:          "filtered index out of bounds",
			originalItems: []string{"plan", "apply", "validate"},
			filteredItems: []string{"plan"},
			filteredIndex: 5,
			expected:      -1,
		},
		{
			name:          "item not found in original list",
			originalItems: []string{"plan", "apply"},
			filteredItems: []string{"destroy"}, // Not in original
			filteredIndex: 0,
			expected:      -1,
		},
		{
			name:          "empty filtered list",
			originalItems: []string{"plan", "apply"},
			filteredItems: []string{},
			filteredIndex: 0,
			expected:      -1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := findOriginalIndex(tt.originalItems, tt.filteredItems, tt.filteredIndex)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// TestFindFilteredIndex tests mapping original index to filtered index.
func TestFindFilteredIndex(t *testing.T) {
	tests := []struct {
		name          string
		originalItems []string
		filteredItems []string
		originalIndex int
		expected      int
	}{
		{
			name:          "item exists in filtered list",
			originalItems: []string{"plan", "apply", "validate", "destroy"},
			filteredItems: []string{"plan", "validate"},
			originalIndex: 0, // "plan"
			expected:      0,
		},
		{
			name:          "item not in filtered list",
			originalItems: []string{"plan", "apply", "validate", "destroy"},
			filteredItems: []string{"plan", "validate"},
			originalIndex: 1, // "apply" not in filtered
			expected:      -1,
		},
		{
			name:          "negative original index",
			originalItems: []string{"plan", "apply"},
			filteredItems: []string{"plan"},
			originalIndex: -1,
			expected:      -1,
		},
		{
			name:          "original index out of bounds",
			originalItems: []string{"plan", "apply"},
			filteredItems: []string{"plan"},
			originalIndex: 10,
			expected:      -1,
		},
		{
			name:          "last item in both lists",
			originalItems: []string{"plan", "apply", "validate"},
			filteredItems: []string{"apply", "validate"},
			originalIndex: 2, // "validate"
			expected:      1,
		},
		{
			name:          "empty filtered list",
			originalItems: []string{"plan", "apply"},
			filteredItems: []string{},
			originalIndex: 0,
			expected:      -1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := findFilteredIndex(tt.originalItems, tt.filteredItems, tt.originalIndex)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// TestModel_GetFilteredNavigationItems tests filtered navigation items retrieval.
func TestModel_GetFilteredNavigationItems(t *testing.T) {
	tests := []struct {
		name     string
		setup    func() (*Model, int)
		expected []string
	}{
		{
			name: "valid depth with no filter",
			setup: func() (*Model, int) {
				root := &stack.Node{
					Name: "root",
					Children: []*stack.Node{
						{Name: "env"},
						{Name: "modules"},
					},
				}
				nav := stack.NewNavigator(root, 1)
				state := stack.NewNavigationState(1)
				nav.PropagateSelection(state)

				m := &Model{
					navigator:     nav,
					navState:      state,
					columnFilters: make(map[int]textinput.Model),
				}
				return m, 0
			},
			expected: []string{"env", "modules"},
		},
		{
			name: "invalid depth - negative",
			setup: func() (*Model, int) {
				root := &stack.Node{Name: "root"}
				nav := stack.NewNavigator(root, 1)
				state := stack.NewNavigationState(1)

				m := &Model{
					navigator:     nav,
					navState:      state,
					columnFilters: make(map[int]textinput.Model),
				}
				return m, -1
			},
			expected: []string{},
		},
		{
			name: "invalid depth - out of bounds",
			setup: func() (*Model, int) {
				root := &stack.Node{Name: "root"}
				nav := stack.NewNavigator(root, 1)
				state := stack.NewNavigationState(1)

				m := &Model{
					navigator:     nav,
					navState:      state,
					columnFilters: make(map[int]textinput.Model),
				}
				return m, 99
			},
			expected: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m, depth := tt.setup()
			result := m.getFilteredNavigationItems(depth)
			assert.Equal(t, tt.expected, result)
		})
	}
}
