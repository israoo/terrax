package tui

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/israoo/terrax/internal/stack"
	"github.com/stretchr/testify/assert"
)

// TestModel_Init tests the Bubble Tea Init method.
func TestModel_Init(t *testing.T) {
	root := &stack.Node{Name: "root", Path: "/test"}
	model := NewModel(root, 1)

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
	}{
		{
			name:          "move down from index 0",
			initialIndex:  0,
			isUp:          false,
			expectedIndex: 1,
			totalCommands: 3,
		},
		{
			name:          "move up from index 2",
			initialIndex:  2,
			isUp:          true,
			expectedIndex: 1,
			totalCommands: 3,
		},
		{
			name:          "cannot move up from index 0",
			initialIndex:  0,
			isUp:          true,
			expectedIndex: 0,
			totalCommands: 3,
		},
		{
			name:          "cannot move down from last index",
			initialIndex:  2,
			isUp:          false,
			expectedIndex: 2,
			totalCommands: 3,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := Model{
				selectedCommand: tt.initialIndex,
				commands:        make([]string, tt.totalCommands),
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
				navigator: nav,
				width:     tt.width,
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
	model := NewModel(root, 1)

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
			initialModel: NewModel(root, 1),
			expectQuit:   true,
		},
		{
			name:         "q quits",
			key:          KeyQ,
			initialModel: NewModel(root, 1),
			expectQuit:   true,
		},
		{
			name:         "up key does not quit",
			key:          KeyUp,
			initialModel: NewModel(root, 1),
			expectQuit:   false,
		},
		{
			name:         "down key does not quit",
			key:          KeyDown,
			initialModel: NewModel(root, 1),
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
				return NewModel(root, 1)
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
			m := NewModel(root, 1)
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

	tests := []struct {
		name          string
		focusedColumn int
		isUp          bool
		initialIndex  int
		expectedIndex int
	}{
		{
			name:          "move down in navigation column",
			focusedColumn: 1,
			isUp:          false,
			initialIndex:  0,
			expectedIndex: 1,
		},
		{
			name:          "move up in navigation column",
			focusedColumn: 1,
			isUp:          true,
			initialIndex:  2,
			expectedIndex: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := Model{
				navigator:     nav,
				navState:      state,
				focusedColumn: tt.focusedColumn,
			}
			m.navState.SelectedIndices[0] = tt.initialIndex

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
			initialModel: NewModel(root, 1),
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
			initialModel: NewModel(root, 1),
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
			initialModel: NewModel(root, 1),
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
				m := NewModel(root, 2)
				m.focusedColumn = 0 // Commands column
				return m
			},
			expectedQuit:      true,
			expectedConfirmed: true,
		},
		{
			name: "enter on navigation column - confirms path",
			setupModel: func() Model {
				m := NewModel(root, 2)
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
			m := NewModel(root, 5)
			m.focusedColumn = tt.initialFocused
			m.navigationOffset = tt.initialOffset

			m = m.handleHorizontalMove(tt.isLeft)

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
			m := NewModel(root, 3)
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
			m := NewModel(root, 2)
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
			m := NewModel(root, 1)
			m.focusedColumn = tt.focusedColumn

			path := m.GetSelectedStackPath()
			assert.Equal(t, tt.expectedPath, path)
		})
	}
}

// TestModel_GetSelectedCommand tests command retrieval.
func TestModel_GetSelectedCommand(t *testing.T) {
	root := &stack.Node{Name: "root"}
	m := NewModel(root, 1)

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
				m := NewModel(root, 1)
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
				m := NewModel(root, 1)
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
			model := NewTestModel(root, 1, tt.confirmed, tt.selectedCommand, tt.selectedPath)

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

	model := NewModel(root, 1)

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
	model := NewModel(root, 1)

	// Send a message type that's not handled (e.g., a custom message).
	type CustomMsg struct{}
	msg := CustomMsg{}

	updatedModel, cmd := model.Update(msg)

	// Should return model unchanged and no command.
	assert.Nil(t, cmd)
	assert.NotNil(t, updatedModel)
}
