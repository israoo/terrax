package tui

import (
	"strings"
	"testing"

	"github.com/israoo/terrax/internal/stack"
	"github.com/stretchr/testify/assert"
)

// TestRenderColumnsWithArrows tests sliding window column rendering.
func TestRenderColumnsWithArrows(t *testing.T) {
	tests := []struct {
		name              string
		setupModel        func() Model
		expectLeftArrow   bool
		expectRightArrow  bool
		expectColumnCount int
	}{
		{
			name: "no overflow - single level",
			setupModel: func() Model {
				root := &stack.Node{
					Name: "root",
					Path: "/test",
					Children: []*stack.Node{
						{Name: "env", Path: "/test/env"},
					},
				}
				nav := stack.NewNavigator(root, 1)
				navState := stack.NewNavigationState(1)
				nav.PropagateSelection(navState)

				m := NewModel(root, 1, []string{"plan"}, 3)
				m.ready = true
				m.width = 120
				m.height = 30
				m.columnWidth = 25
				return m
			},
			expectLeftArrow:   false,
			expectRightArrow:  false,
			expectColumnCount: 2, // commands + 1 nav column
		},
		{
			name: "left overflow - navigationOffset > 0",
			setupModel: func() Model {
				root := &stack.Node{
					Name: "root",
					Path: "/test",
					Children: []*stack.Node{
						{
							Name: "env",
							Path: "/test/env",
							Children: []*stack.Node{
								{
									Name: "dev",
									Path: "/test/env/dev",
									Children: []*stack.Node{
										{Name: "vpc", Path: "/test/env/dev/vpc"},
									},
								},
							},
						},
					},
				}
				nav := stack.NewNavigator(root, 3)
				navState := stack.NewNavigationState(3)
				nav.PropagateSelection(navState)

				m := NewModel(root, 3, []string{"plan"}, 3)
				m.ready = true
				m.width = 120
				m.height = 30
				m.columnWidth = 25
				m.navigationOffset = 1 // Offset = 1 means we've scrolled right
				return m
			},
			expectLeftArrow:  true,
			expectRightArrow: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := tt.setupModel()
			layout := NewLayoutCalculator(120, 30, 25)
			renderer := NewRenderer(m, layout)

			columns := renderer.renderColumnsWithArrows()

			assert.NotEmpty(t, columns)

			hasLeftArrow := false
			hasRightArrow := false
			for _, col := range columns {
				if strings.Contains(col, "«") {
					hasLeftArrow = true
				}
				if strings.Contains(col, "»") {
					hasRightArrow = true
				}
			}

			assert.Equal(t, tt.expectLeftArrow, hasLeftArrow, "left arrow expectation failed")
			assert.Equal(t, tt.expectRightArrow, hasRightArrow, "right arrow expectation failed")
		})
	}
}

// TestRenderCommandsColumn tests commands column rendering.
func TestRenderCommandsColumn(t *testing.T) {
	root := &stack.Node{Name: "root", Path: "/test"}
	m := NewModel(root, 1, []string{"plan", "apply", "destroy"}, 3)
	m.ready = true
	m.width = 120
	m.height = 30
	m.columnWidth = 25

	layout := NewLayoutCalculator(120, 30, 25)
	renderer := NewRenderer(m, layout)

	column := renderer.renderCommandsColumn()

	assert.Contains(t, column, CommandsTitle)
	assert.Contains(t, column, "plan")
	assert.Contains(t, column, "apply")
	assert.Contains(t, column, "destroy")
}

// TestBuildCommandList tests command list building with selection.
func TestBuildCommandList(t *testing.T) {
	tests := []struct {
		name            string
		commands        []string
		selectedCommand int
		expectCursor    bool
	}{
		{
			name:            "first command selected",
			commands:        []string{"plan", "apply"},
			selectedCommand: 0,
			expectCursor:    true,
		},
		{
			name:            "second command selected",
			commands:        []string{"plan", "apply", "destroy"},
			selectedCommand: 1,
			expectCursor:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			root := &stack.Node{Name: "root"}
			m := NewModel(root, 1, tt.commands, 3)
			m.selectedCommand = tt.selectedCommand
			m.height = 30
			m.columnWidth = 25

			layout := NewLayoutCalculator(120, 30, 25)
			renderer := NewRenderer(m, layout)

			list := renderer.buildCommandList()

			for _, cmd := range tt.commands {
				assert.Contains(t, list, cmd)
			}
			if tt.expectCursor {
				assert.Contains(t, list, "►")
			}
		})
	}
}

// TestRenderNavigationColumn tests navigation column rendering.
func TestRenderNavigationColumn(t *testing.T) {
	root := &stack.Node{
		Name: "root",
		Path: "/test",
		Children: []*stack.Node{
			{Name: "env", Path: "/test/env"},
			{Name: "modules", Path: "/test/modules"},
		},
	}

	nav := stack.NewNavigator(root, 1)
	navState := stack.NewNavigationState(1)
	nav.PropagateSelection(navState)

	m := Model{
		navigator:     nav,
		navState:      navState,
		height:        30,
		columnWidth:   25,
		scrollOffsets: make(map[int]int),
	}

	layout := NewLayoutCalculator(120, 30, 25)
	renderer := NewRenderer(m, layout)

	column := renderer.renderNavigationColumn(0)

	assert.Contains(t, column, "Level 1")
	assert.Contains(t, column, "env")
	assert.Contains(t, column, "modules")
}

// TestBuildNavigationList tests navigation list building.
func TestBuildNavigationList(t *testing.T) {
	root := &stack.Node{
		Name: "root",
		Path: "/test",
		Children: []*stack.Node{
			{Name: "dev", Path: "/test/dev"},
			{Name: "staging", Path: "/test/staging"},
			{Name: "prod", Path: "/test/prod"},
		},
	}

	nav := stack.NewNavigator(root, 1)
	navState := stack.NewNavigationState(1)
	nav.PropagateSelection(navState)

	m := Model{
		navigator:     nav,
		navState:      navState,
		focusedColumn: 1,
		height:        30,
		columnWidth:   25,
		scrollOffsets: make(map[int]int),
	}

	layout := NewLayoutCalculator(120, 30, 25)
	renderer := NewRenderer(m, layout)

	list := renderer.buildNavigationList(0)

	assert.Contains(t, list, "dev")
	assert.Contains(t, list, "staging")
	assert.Contains(t, list, "prod")
	assert.Contains(t, list, "►") // Cursor should be present
}

// TestStyleColumn tests column styling for focused and unfocused states.
func TestStyleColumn(t *testing.T) {
	tests := []struct {
		name      string
		isFocused bool
	}{
		{
			name:      "focused column",
			isFocused: true,
		},
		{
			name:      "unfocused column",
			isFocused: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := Model{columnWidth: 25, height: 30}
			layout := NewLayoutCalculator(120, 30, 25)
			renderer := NewRenderer(m, layout)

			content := "Test Content"
			styled := renderer.styleColumn(content, tt.isFocused)

			assert.NotEmpty(t, styled)
			assert.Contains(t, styled, content)
		})
	}
}

// TestGetLevelTitle tests level title generation.
func TestGetLevelTitle(t *testing.T) {
	tests := []struct {
		name          string
		depth         int
		expectedTitle string
	}{
		{
			name:          "level 1",
			depth:         0,
			expectedTitle: "Level 1",
		},
		{
			name:          "level 2",
			depth:         1,
			expectedTitle: "Level 2",
		},
		{
			name:          "level 5",
			depth:         4,
			expectedTitle: "Level 5",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := Model{}
			layout := NewLayoutCalculator(120, 30, 25)
			renderer := NewRenderer(m, layout)

			title := renderer.getLevelTitle(tt.depth)

			assert.Equal(t, tt.expectedTitle, title)
		})
	}
}

// TestColumnStyle tests the columnStyle function.
func TestColumnStyle(t *testing.T) {
	tests := []struct {
		name    string
		focused bool
	}{
		{
			name:    "focused style",
			focused: true,
		},
		{
			name:    "unfocused style",
			focused: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			style := columnStyle(tt.focused)
			assert.NotNil(t, style)
		})
	}
}

// TestCalculatePaginatedRange tests the paginated range calculation function.
func TestCalculatePaginatedRange(t *testing.T) {
	tests := []struct {
		name            string
		scrollOffset    int
		maxVisibleItems int
		totalItems      int
		expectedStart   int
		expectedEnd     int
	}{
		{
			name:            "all items visible",
			scrollOffset:    0,
			maxVisibleItems: 10,
			totalItems:      5,
			expectedStart:   0,
			expectedEnd:     5,
		},
		{
			name:            "scrolled down",
			scrollOffset:    5,
			maxVisibleItems: 10,
			totalItems:      20,
			expectedStart:   5,
			expectedEnd:     15,
		},
		{
			name:            "at end of list",
			scrollOffset:    15,
			maxVisibleItems: 10,
			totalItems:      20,
			expectedStart:   15,
			expectedEnd:     20,
		},
		{
			name:            "zero total items",
			scrollOffset:    0,
			maxVisibleItems: 10,
			totalItems:      0,
			expectedStart:   0,
			expectedEnd:     0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			start, end := calculatePaginatedRange(tt.scrollOffset, tt.maxVisibleItems, tt.totalItems)

			assert.Equal(t, tt.expectedStart, start)
			assert.Equal(t, tt.expectedEnd, end)
		})
	}
}

// TestGetMaxItemTextWidth tests item text width calculation.
func TestGetMaxItemTextWidth(t *testing.T) {
	m := Model{columnWidth: 30}
	layout := NewLayoutCalculator(120, 30, 30)
	renderer := NewRenderer(m, layout)

	width := renderer.getMaxItemTextWidth()

	// Should be positive and less than column width
	assert.Greater(t, width, 0)
	assert.Less(t, width, 30)
}

// TestNewRenderer tests the Renderer constructor.
func TestNewRenderer(t *testing.T) {
	root := &stack.Node{Name: "root", Path: "/test"}
	m := NewModel(root, 1, []string{"plan"}, 3)
	m.width = 120
	m.height = 30
	m.columnWidth = 25

	layout := NewLayoutCalculator(120, 30, 25)
	renderer := NewRenderer(m, layout)

	assert.NotNil(t, renderer)
	assert.Equal(t, layout, renderer.layout)
}

// TestRenderer_Render tests the main Render method.
func TestRenderer_Render(t *testing.T) {
	root := &stack.Node{
		Name: "root",
		Path: "/test/root",
		Children: []*stack.Node{
			{Name: "env", Path: "/test/root/env"},
		},
	}

	nav := stack.NewNavigator(root, 1)
	navState := stack.NewNavigationState(1)
	nav.PropagateSelection(navState)

	m := Model{
		ready:           true,
		width:           120,
		height:          30,
		columnWidth:     25,
		navigator:       nav,
		navState:        navState,
		commands:        []string{"plan", "apply"},
		focusedColumn:   0,
		selectedCommand: 0,
		scrollOffsets:   make(map[int]int),
	}

	layout := NewLayoutCalculator(120, 30, 25)
	renderer := NewRenderer(m, layout)

	output := renderer.Render()

	// Verify output contains key elements
	assert.Contains(t, output, AppTitle)
	assert.Contains(t, output, "plan")
	assert.Contains(t, output, "apply")
	assert.NotEmpty(t, output)
}

// TestView_NavigationState tests View in navigation state.
func TestView_NavigationState(t *testing.T) {
	root := &stack.Node{
		Name: "root",
		Path: "/test",
		Children: []*stack.Node{
			{Name: "env", Path: "/test/env"},
		},
	}

	m := NewModel(root, 1, []string{"plan"}, 3)
	m.ready = true
	m.width = 120
	m.height = 30
	m.columnWidth = 25
	m.state = StateNavigation

	view := m.View()

	assert.Contains(t, view, AppTitle)
	assert.Contains(t, view, "plan")
}

// TestView_NotReady tests View when model is not ready.
func TestView_NotReady(t *testing.T) {
	m := Model{
		ready: false,
		state: StateNavigation,
	}

	view := m.View()
	assert.Equal(t, Initializing, view)
}

// TestView_ZeroWidth tests View when width is zero.
func TestView_ZeroWidth(t *testing.T) {
	m := Model{
		ready: true,
		width: 0,
		state: StateNavigation,
	}

	view := m.View()
	assert.Equal(t, Initializing, view)
}

// TestView_ScanningStacks tests View when no stacks detected.
func TestView_ScanningStacks(t *testing.T) {
	root := &stack.Node{Name: "root", Path: "/test"}
	nav := stack.NewNavigator(root, 0)

	m := Model{
		ready:       true,
		width:       120,
		height:      30,
		navigator:   nav,
		columnWidth: 0,
		state:       StateNavigation,
	}

	view := m.View()
	assert.Equal(t, ScanningStacks, view)
}
