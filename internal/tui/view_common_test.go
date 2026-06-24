package tui

import (
	"testing"

	"github.com/israoo/terrax/internal/stack"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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
			expectedHeight: 30 - HeaderHeight - BreadcrumbLineCount - FooterHeight,
		},
		{
			name:           "minimal height",
			height:         10,
			expectedHeight: 10 - HeaderHeight - BreadcrumbLineCount - FooterHeight,
		},
		{
			name:           "very small height returns 1",
			height:         3,
			expectedHeight: 1,
		},
		{
			name:           "zero height returns 1",
			height:         0,
			expectedHeight: 1,
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

// TestRenderPageIndicators tests pagination indicator rendering.
func TestRenderPageIndicators(t *testing.T) {
	tests := []struct {
		name        string
		currentPage int
		totalPages  int
		expectEmpty bool
		expectDots  int
	}{
		{
			name:        "single page returns empty",
			currentPage: 1,
			totalPages:  1,
			expectEmpty: true,
		},
		{
			name:        "zero pages returns empty",
			currentPage: 0,
			totalPages:  0,
			expectEmpty: true,
		},
		{
			name:        "two pages on first page",
			currentPage: 1,
			totalPages:  2,
			expectDots:  2,
		},
		{
			name:        "three pages on second page",
			currentPage: 2,
			totalPages:  3,
			expectDots:  3,
		},
		{
			name:        "five pages on last page",
			currentPage: 5,
			totalPages:  5,
			expectDots:  5,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := renderPageIndicators(tt.currentPage, tt.totalPages)

			if tt.expectEmpty {
				assert.Empty(t, result)
			} else {
				assert.NotEmpty(t, result)
				assert.Contains(t, result, "•")
			}
		})
	}
}

// TestMin tests the min helper function.
func TestMin(t *testing.T) {
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
		{
			name:     "negative values",
			a:        -5,
			b:        -10,
			expected: -10,
		},
		{
			name:     "zero and positive",
			a:        0,
			b:        5,
			expected: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := min(tt.a, tt.b)
			assert.Equal(t, tt.expected, result)
		})
	}
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
			name:     "unicode text truncation",
			text:     "héllo wörld",
			maxWidth: 8,
			expected: "héll...",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := truncateText(tt.text, tt.maxWidth)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// TestRenderer_RenderHeader tests header rendering.
func TestRenderer_RenderHeader(t *testing.T) {
	root := &stack.Node{Name: "root", Path: "/test"}
	m := NewModel(root, 1, []string{"plan"}, 3)
	m.width = 120
	m.height = 30
	m.columnWidth = 25

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
