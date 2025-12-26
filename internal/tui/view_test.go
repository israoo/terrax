package tui

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/israoo/terrax/internal/stack"
)

// TestNewLayoutCalculator tests the LayoutCalculator constructor.
func TestNewLayoutCalculator(t *testing.T) {
	tests := []struct {
		name        string
		width       int
		height      int
		columnWidth int
	}{
		{
			name:        "standard terminal size",
			width:       120,
			height:      30,
			columnWidth: 25,
		},
		{
			name:        "small terminal",
			width:       80,
			height:      20,
			columnWidth: 18,
		},
		{
			name:        "large terminal",
			width:       200,
			height:      50,
			columnWidth: 40,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			lc := NewLayoutCalculator(tt.width, tt.height, tt.columnWidth)

			require.NotNil(t, lc)
			assert.Equal(t, tt.width, lc.width)
			assert.Equal(t, tt.height, lc.height)
			assert.Equal(t, tt.columnWidth, lc.columnWidth)
		})
	}
}

// TestLayoutCalculator_GetContentHeight tests content height calculation.
func TestLayoutCalculator_GetContentHeight(t *testing.T) {
	tests := []struct {
		name           string
		height         int
		expectedHeight int
	}{
		{
			name:           "standard height",
			height:         30,
			expectedHeight: 30 - HeaderHeight - 1 - FooterHeight, // Header + Breadcrumb + Footer
		},
		{
			name:           "minimal height",
			height:         10,
			expectedHeight: 10 - HeaderHeight - 1 - FooterHeight, // Header + Breadcrumb + Footer
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			lc := NewLayoutCalculator(100, tt.height, 20)
			contentHeight := lc.GetContentHeight()

			assert.Equal(t, tt.expectedHeight, contentHeight)
		})
	}
}

// TestLayoutCalculator_GetColumnWidth tests column width retrieval.
func TestLayoutCalculator_GetColumnWidth(t *testing.T) {
	lc := NewLayoutCalculator(120, 30, 25)
	assert.Equal(t, 25, lc.GetColumnWidth())
}

// TestModel_View_NotReady tests View when model is not ready.
func TestModel_View_NotReady(t *testing.T) {
	m := Model{
		ready: false,
		width: 120,
	}

	view := m.View()
	assert.Equal(t, Initializing, view)
}

// TestModel_View_ZeroWidth tests View when width is zero.
func TestModel_View_ZeroWidth(t *testing.T) {
	m := Model{
		ready: true,
		width: 0,
	}

	view := m.View()
	assert.Equal(t, Initializing, view)
}

// TestModel_View_ScanningStacks tests View when no stacks detected.
func TestModel_View_ScanningStacks(t *testing.T) {
	root := &stack.Node{
		Name:     "root",
		Path:     "/test",
		Children: []*stack.Node{},
	}

	nav := stack.NewNavigator(root, 0)

	m := Model{
		ready:       true,
		width:       120,
		height:      30,
		navigator:   nav,
		columnWidth: 0,
	}

	view := m.View()
	assert.Equal(t, ScanningStacks, view)
}

// TestModel_View_ScanningStacks_ZeroDepth tests View when maxDepth is zero.
func TestModel_View_ScanningStacks_ZeroDepth(t *testing.T) {
	root := &stack.Node{
		Name:     "root",
		Path:     "/test",
		Children: []*stack.Node{{Name: "child"}},
	}

	nav := stack.NewNavigator(root, 0)

	m := Model{
		ready:       true,
		width:       120,
		height:      30,
		navigator:   nav,
		columnWidth: 25,
	}

	view := m.View()
	assert.Equal(t, ScanningStacks, view)
}

// TestNewRenderer tests the Renderer constructor.
func TestNewRenderer(t *testing.T) {
	root := &stack.Node{
		Name: "root",
		Path: "/test",
	}

	nav := stack.NewNavigator(root, 1)
	navState := stack.NewNavigationState(1)

	m := Model{
		ready:       true,
		width:       120,
		height:      30,
		columnWidth: 25,
		navigator:   nav,
		navState:    navState,
		commands:    []string{"plan", "apply"},
	}

	layout := NewLayoutCalculator(120, 30, 25)
	renderer := NewRenderer(m, layout)

	require.NotNil(t, renderer)
	assert.Equal(t, m, renderer.model)
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

// TestRenderer_RenderHeader tests header rendering.
func TestRenderer_RenderHeader(t *testing.T) {
	m := Model{width: 120}
	layout := NewLayoutCalculator(120, 30, 25)
	renderer := NewRenderer(m, layout)

	header := renderer.renderHeader()

	assert.Contains(t, header, AppTitle)
	assert.NotEmpty(t, header)
}

// TestRenderer_RenderBreadcrumbBar tests breadcrumb bar rendering.
func TestRenderer_RenderBreadcrumbBar(t *testing.T) {
	root := &stack.Node{
		Name: "root",
		Path: "/test/project",
		Children: []*stack.Node{
			{Name: "env", Path: "/test/project/env"},
		},
	}

	nav := stack.NewNavigator(root, 1)
	navState := stack.NewNavigationState(1)
	nav.PropagateSelection(navState)

	m := Model{
		width:         120,
		navigator:     nav,
		navState:      navState,
		focusedColumn: 0,
	}

	layout := NewLayoutCalculator(120, 30, 25)
	renderer := NewRenderer(m, layout)

	breadcrumb := renderer.renderBreadcrumbBar()

	assert.Contains(t, breadcrumb, "/test/project")
	assert.Contains(t, breadcrumb, "📁")
}

// TestRenderer_RenderFooter tests footer rendering.
func TestRenderer_RenderFooter(t *testing.T) {
	m := Model{}
	layout := NewLayoutCalculator(120, 30, 25)
	renderer := NewRenderer(m, layout)

	footer := renderer.renderFooter()

	assert.Contains(t, footer, HelpText)
}

// TestRenderer_RenderCommandsColumn tests commands column rendering.
func TestRenderer_RenderCommandsColumn(t *testing.T) {
	m := Model{
		commands:      []string{"plan", "apply", "destroy"},
		height:        30, // Ensure sufficient height for all items
		scrollOffsets: make(map[int]int),
	}

	layout := NewLayoutCalculator(120, 30, 25)
	renderer := NewRenderer(m, layout)

	column := renderer.renderCommandsColumn()

	assert.Contains(t, column, CommandsTitle)
	assert.Contains(t, column, "plan")
	assert.Contains(t, column, "apply")
	assert.Contains(t, column, "destroy")
}

// TestRenderer_BuildCommandList tests command list building.
func TestRenderer_BuildCommandList(t *testing.T) {
	tests := []struct {
		name            string
		commands        []string
		selectedCommand int
		expectCursor    string
	}{
		{
			name:            "first command selected",
			commands:        []string{"plan", "apply"},
			selectedCommand: 0,
			expectCursor:    "►",
		},
		{
			name:            "second command selected",
			commands:        []string{"plan", "apply", "destroy"},
			selectedCommand: 1,
			expectCursor:    "►",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := Model{
				commands:        tt.commands,
				selectedCommand: tt.selectedCommand,
				height:          30, // Ensure sufficient height for all items
				scrollOffsets:   make(map[int]int),
			}

			layout := NewLayoutCalculator(120, 30, 25)
			renderer := NewRenderer(m, layout)

			list := renderer.buildCommandList()

			// Verify all commands appear
			for _, cmd := range tt.commands {
				assert.Contains(t, list, cmd)
			}

			// Verify cursor appears
			assert.Contains(t, list, tt.expectCursor)
		})
	}
}

// TestRenderer_RenderNavigationColumn tests navigation column rendering.
func TestRenderer_RenderNavigationColumn(t *testing.T) {
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
		height:        30, // Ensure sufficient height for all items
		scrollOffsets: make(map[int]int),
	}

	layout := NewLayoutCalculator(120, 30, 25)
	renderer := NewRenderer(m, layout)

	column := renderer.renderNavigationColumn(0)

	assert.Contains(t, column, "Level 1")
	assert.Contains(t, column, "env")
	assert.Contains(t, column, "modules")
}

// TestRenderer_BuildNavigationList tests navigation list building.
func TestRenderer_BuildNavigationList(t *testing.T) {
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

	tests := []struct {
		name          string
		selectedIndex int
	}{
		{
			name:          "first item selected",
			selectedIndex: 0,
		},
		{
			name:          "second item selected",
			selectedIndex: 1,
		},
		{
			name:          "third item selected",
			selectedIndex: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			navState.SelectedIndices[0] = tt.selectedIndex

			m := Model{
				navigator:     nav,
				navState:      navState,
				height:        30, // Ensure sufficient height for all items
				scrollOffsets: make(map[int]int),
			}

			layout := NewLayoutCalculator(120, 30, 25)
			renderer := NewRenderer(m, layout)

			list := renderer.buildNavigationList(0)

			// Verify all items appear
			assert.Contains(t, list, "dev")
			assert.Contains(t, list, "staging")
			assert.Contains(t, list, "prod")

			// Verify cursor appears
			assert.Contains(t, list, "►")
		})
	}
}

// TestRenderer_StyleColumn tests column styling.
func TestRenderer_StyleColumn(t *testing.T) {
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
			m := Model{}
			layout := NewLayoutCalculator(120, 30, 25)
			renderer := NewRenderer(m, layout)

			content := "Test Content"
			styled := renderer.styleColumn(content, tt.isFocused)

			assert.NotEmpty(t, styled)
			// Content should be present (may be wrapped in styling)
			assert.Contains(t, styled, content)
		})
	}
}

// TestRenderer_RenderArrowIndicator tests arrow indicator rendering.
func TestRenderer_RenderArrowIndicator(t *testing.T) {
	tests := []struct {
		name  string
		arrow string
	}{
		{
			name:  "left arrow",
			arrow: "«",
		},
		{
			name:  "right arrow",
			arrow: "»",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := Model{}
			layout := NewLayoutCalculator(120, 30, 25)
			renderer := NewRenderer(m, layout)

			indicator := renderer.renderArrowIndicator(tt.arrow)

			assert.Contains(t, indicator, tt.arrow)
			assert.NotEmpty(t, indicator)
		})
	}
}

// TestRenderer_GetLevelTitle tests level title generation.
func TestRenderer_GetLevelTitle(t *testing.T) {
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

// TestMinFunction tests the min helper function.
func TestMinFunction(t *testing.T) {
	tests := []struct {
		name     string
		a        int
		b        int
		expected int
	}{
		{
			name:     "a is smaller",
			a:        5,
			b:        10,
			expected: 5,
		},
		{
			name:     "b is smaller",
			a:        10,
			b:        5,
			expected: 5,
		},
		{
			name:     "equal values",
			a:        7,
			b:        7,
			expected: 7,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := min(tt.a, tt.b)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// TestRenderer_RenderColumnsWithArrows tests sliding window column rendering.
func TestRenderer_RenderColumnsWithArrows(t *testing.T) {
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

				return Model{
					navigator:            nav,
					navState:             navState,
					commands:             []string{"plan"},
					focusedColumn:        0,
					navigationOffset:     0,
					maxNavigationColumns: 3,
				}
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

				return Model{
					navigator:            nav,
					navState:             navState,
					commands:             []string{"plan"},
					focusedColumn:        2,
					navigationOffset:     1, // Offset = 1 means we've scrolled right
					maxNavigationColumns: 3,
				}
			},
			expectLeftArrow:   true,
			expectRightArrow:  false,
			expectColumnCount: 4, // commands + left arrow + 2 nav columns
		},
		{
			name: "deep tree with children - shows right arrow",
			setupModel: func() Model {
				// Build a 5-level deep tree
				level4 := &stack.Node{Name: "level4", Path: "/test/l1/l2/l3/l4", Children: []*stack.Node{}}
				level3 := &stack.Node{Name: "level3", Path: "/test/l1/l2/l3", Children: []*stack.Node{level4}}
				level2 := &stack.Node{Name: "level2", Path: "/test/l1/l2", Children: []*stack.Node{level3}}
				level1 := &stack.Node{Name: "level1", Path: "/test/l1", Children: []*stack.Node{level2}}
				root := &stack.Node{Name: "root", Path: "/test", Children: []*stack.Node{level1}}

				nav := stack.NewNavigator(root, 5)
				navState := stack.NewNavigationState(5)
				nav.PropagateSelection(navState)

				return Model{
					navigator:            nav,
					navState:             navState,
					commands:             []string{"plan"},
					focusedColumn:        1,
					navigationOffset:     0,
					maxNavigationColumns: 3,
				}
			},
			expectLeftArrow:   false,
			expectRightArrow:  true,
			expectColumnCount: 5, // commands + 3 nav columns + right arrow
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := tt.setupModel()
			layout := NewLayoutCalculator(120, 30, 25)
			renderer := NewRenderer(m, layout)

			columns := renderer.renderColumnsWithArrows()

			assert.Len(t, columns, tt.expectColumnCount)

			// Check for left arrow
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

// TestModel_View_IntegrationWithRenderer tests the full View rendering pipeline.
func TestModel_View_IntegrationWithRenderer(t *testing.T) {
	root := &stack.Node{
		Name: "terraform",
		Path: "/projects/terraform",
		Children: []*stack.Node{
			{
				Name: "env",
				Path: "/projects/terraform/env",
				Children: []*stack.Node{
					{Name: "dev", Path: "/projects/terraform/env/dev"},
					{Name: "prod", Path: "/projects/terraform/env/prod"},
				},
			},
			{
				Name: "modules",
				Path: "/projects/terraform/modules",
			},
		},
	}

	nav := stack.NewNavigator(root, 2)
	navState := stack.NewNavigationState(2)
	nav.PropagateSelection(navState)

	m := Model{
		ready:                true,
		width:                120,
		height:               30,
		columnWidth:          25,
		navigator:            nav,
		navState:             navState,
		commands:             []string{"plan", "apply", "destroy"},
		focusedColumn:        1,
		selectedCommand:      0,
		maxNavigationColumns: 3,
	}

	view := m.View()

	// Verify complete integration
	assert.NotEmpty(t, view)
	assert.Contains(t, view, AppTitle)
	assert.Contains(t, view, "/projects/terraform")
	assert.Contains(t, view, "plan")
	assert.Contains(t, view, "apply")
	assert.Contains(t, view, "destroy")
	assert.Contains(t, view, "env")
	assert.Contains(t, view, "modules")
	assert.Contains(t, view, HelpText)
}

// TestTruncateText tests the text truncation function.
func TestTruncateText(t *testing.T) {
	tests := []struct {
		name     string
		text     string
		maxWidth int
		expected string
	}{
		{
			name:     "text fits exactly",
			text:     "hello",
			maxWidth: 5,
			expected: "hello",
		},
		{
			name:     "text shorter than max",
			text:     "hi",
			maxWidth: 10,
			expected: "hi",
		},
		{
			name:     "text needs truncation",
			text:     "this is a very long text",
			maxWidth: 10,
			expected: "this is...",
		},
		{
			name:     "maxWidth equals ellipsis width",
			text:     "hello",
			maxWidth: EllipsisWidth,
			expected: "hel",
		},
		{
			name:     "maxWidth less than ellipsis width",
			text:     "hello",
			maxWidth: 2,
			expected: "he",
		},
		{
			name:     "maxWidth is zero",
			text:     "hello",
			maxWidth: 0,
			expected: "",
		},
		{
			name:     "maxWidth is negative",
			text:     "hello",
			maxWidth: -1,
			expected: "",
		},
		{
			name:     "empty text",
			text:     "",
			maxWidth: 10,
			expected: "",
		},
		{
			name:     "truncate at exact ellipsis boundary",
			text:     "hello world",
			maxWidth: 8,
			expected: "hello...",
		},
		{
			name:     "very long text with small width",
			text:     "this is a really really really long piece of text that needs significant truncation",
			maxWidth: 15,
			expected: "this is a re...",
		},
		{
			name:     "unicode characters - byte slicing may break",
			text:     "hello world",
			maxWidth: 8,
			expected: "hello...",
		},
		{
			name:     "maxWidth exactly 4 (ellipsis width + 1)",
			text:     "hello",
			maxWidth: 4,
			expected: "h...",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := truncateText(tt.text, tt.maxWidth)
			assert.Equal(t, tt.expected, result)

			// Verify result doesn't exceed maxWidth
			if tt.maxWidth > 0 {
				assert.LessOrEqual(t, len(result), tt.maxWidth,
					"truncated text should not exceed maxWidth")
			}
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
			name:            "all items fit on one page",
			scrollOffset:    0,
			maxVisibleItems: 10,
			totalItems:      5,
			expectedStart:   0,
			expectedEnd:     5,
		},
		{
			name:            "first page of multiple pages",
			scrollOffset:    0,
			maxVisibleItems: 5,
			totalItems:      20,
			expectedStart:   0,
			expectedEnd:     5,
		},
		{
			name:            "middle page",
			scrollOffset:    5,
			maxVisibleItems: 5,
			totalItems:      20,
			expectedStart:   5,
			expectedEnd:     10,
		},
		{
			name:            "last page - partial",
			scrollOffset:    15,
			maxVisibleItems: 5,
			totalItems:      18,
			expectedStart:   15,
			expectedEnd:     18,
		},
		{
			name:            "last page - exact fit",
			scrollOffset:    15,
			maxVisibleItems: 5,
			totalItems:      20,
			expectedStart:   15,
			expectedEnd:     20,
		},
		{
			name:            "scrollOffset beyond items",
			scrollOffset:    25,
			maxVisibleItems: 5,
			totalItems:      20,
			expectedStart:   0,
			expectedEnd:     20, // Note: endIdx is capped at totalItems, not recalculated from reset startIdx
		},
		{
			name:            "empty list",
			scrollOffset:    0,
			maxVisibleItems: 5,
			totalItems:      0,
			expectedStart:   0,
			expectedEnd:     0,
		},
		{
			name:            "single item",
			scrollOffset:    0,
			maxVisibleItems: 5,
			totalItems:      1,
			expectedStart:   0,
			expectedEnd:     1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			start, end := calculatePaginatedRange(tt.scrollOffset, tt.maxVisibleItems, tt.totalItems)
			assert.Equal(t, tt.expectedStart, start, "start index mismatch")
			assert.Equal(t, tt.expectedEnd, end, "end index mismatch")

			// Verify invariants
			assert.LessOrEqual(t, start, end, "start should be <= end")
			assert.GreaterOrEqual(t, start, 0, "start should be >= 0")
			assert.LessOrEqual(t, end, tt.totalItems, "end should be <= totalItems")
		})
	}
}

// TestRenderPageIndicators tests the page indicator rendering function.
func TestRenderPageIndicators(t *testing.T) {
	tests := []struct {
		name        string
		currentPage int
		totalPages  int
		expectEmpty bool
		expectDots  int
	}{
		{
			name:        "single page - no indicators",
			currentPage: 1,
			totalPages:  1,
			expectEmpty: true,
			expectDots:  0,
		},
		{
			name:        "two pages - first page",
			currentPage: 1,
			totalPages:  2,
			expectEmpty: false,
			expectDots:  2,
		},
		{
			name:        "two pages - second page",
			currentPage: 2,
			totalPages:  2,
			expectEmpty: false,
			expectDots:  2,
		},
		{
			name:        "five pages - first page",
			currentPage: 1,
			totalPages:  5,
			expectEmpty: false,
			expectDots:  5,
		},
		{
			name:        "five pages - middle page",
			currentPage: 3,
			totalPages:  5,
			expectEmpty: false,
			expectDots:  5,
		},
		{
			name:        "five pages - last page",
			currentPage: 5,
			totalPages:  5,
			expectEmpty: false,
			expectDots:  5,
		},
		{
			name:        "zero pages",
			currentPage: 0,
			totalPages:  0,
			expectEmpty: true,
			expectDots:  0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := renderPageIndicators(tt.currentPage, tt.totalPages)

			if tt.expectEmpty {
				assert.Empty(t, result, "expected empty result")
			} else {
				assert.NotEmpty(t, result, "expected non-empty result")
				// Count dots in the result (both active "•" and inactive "•")
				dotCount := strings.Count(result, "•")
				assert.Equal(t, tt.expectDots, dotCount,
					"expected %d dots but got %d", tt.expectDots, dotCount)
			}
		})
	}
}
