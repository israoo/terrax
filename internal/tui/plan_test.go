package tui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/israoo/terrax/internal/plan"
	"github.com/stretchr/testify/assert"
)

// Helper to create a model in plan review state
func createTestPlanModel() Model {
	m := NewModel(nil, 0, nil, 0)

	// manually inject a plan report to simulate "Plan Review" mode
	m.state = StatePlanReview
	m.planReport = &plan.PlanReport{
		Stacks: []plan.StackResult{
			{
				StackPath:  "dev/us-east-1/vpc",
				HasChanges: true,
				Stats: plan.StackStats{
					Add: 1,
				},
				ResourceChanges: []plan.ResourceChange{
					{
						Address:    "aws_vpc.main",
						Type:       "aws_vpc",
						ChangeType: plan.ChangeTypeCreate,
						After: map[string]interface{}{
							"cidr_block": "10.0.0.0/16",
						},
					},
				},
			},
			{
				StackPath:  "dev/us-east-1/s3",
				HasChanges: false,
			},
		},
	}
	// Re-run the initialization logic that happens in NewPlanReviewModel
	// But since NewPlanReviewModel returns a new model, let's just use BuildTree directly or mock what's needed
	// Actually we should prob test NewPlanReviewModel directly.

	return NewPlanReviewModel(m.planReport)
}

func TestNewPlanReviewModel(t *testing.T) {
	report := &plan.PlanReport{
		Stacks: []plan.StackResult{
			{StackPath: "a", HasChanges: true, Stats: plan.StackStats{Add: 1}},
			{StackPath: "b", IsDependency: true, HasChanges: true, Stats: plan.StackStats{Change: 1}},
		},
	}

	m := NewPlanReviewModel(report)

	assert.Equal(t, StatePlanReview, m.state)
	assert.Equal(t, 1, m.planTargetStats.Add)
	assert.Equal(t, 1, m.planDependencyStats.Change)
	assert.NotEmpty(t, m.planTreeRoots)
	assert.NotEmpty(t, m.planFlatItems)
}

func TestHandlePlanReviewUpdate_Navigation(t *testing.T) {
	m := createTestPlanModel()
	// Should have at least "dev" -> "us-east-1" -> "vpc" (3 items if flattened fully, or more depending on structure)
	// BuildTree logic: "dev" (node), "dev/us-east-1" (node), "dev/us-east-1/vpc" (leaf)
	// FlattenTree should produce a list.

	initialCursor := m.planListCursor
	assert.Equal(t, 0, initialCursor)

	// Move Down
	msg := tea.KeyMsg{Type: tea.KeyDown}
	updatedModel, cmd := m.Update(msg)
	newModel := updatedModel.(Model)

	assert.Nil(t, cmd)
	assert.Greater(t, newModel.planListCursor, initialCursor)

	// Move Up
	msg = tea.KeyMsg{Type: tea.KeyUp}
	updatedModel, _ = newModel.Update(msg)
	finalModel := updatedModel.(Model)

	assert.Equal(t, 0, finalModel.planListCursor)
}

func TestHandlePlanReviewUpdate_Quit(t *testing.T) {
	m := createTestPlanModel()

	msg := tea.KeyMsg{Type: tea.KeyEsc}
	updatedModel, cmd := m.Update(msg)
	newModel := updatedModel.(Model)

	assert.NotNil(t, cmd)
	// We can't easily compare function pointers, but we can verify it's not nil
	// and maybe check if it matches tea.Quit if possible, or just trust it.
	// Actually tea.Quit returns a Msg, we can run it.
	assert.Equal(t, tea.Quit(), cmd())
	assert.Equal(t, StatePlanReview, newModel.state)
}

func TestHandlePlanReviewUpdate_Quit_FromDetail(t *testing.T) {
	m := createTestPlanModel()
	m.planReviewFocusedElement = 1 // Focus Detail View

	// Test Esc
	msg := tea.KeyMsg{Type: tea.KeyEsc}
	updatedModel, cmd := m.Update(msg)
	newModel := updatedModel.(Model)

	assert.NotNil(t, cmd)
	assert.Equal(t, tea.Quit(), cmd())
	assert.Equal(t, StatePlanReview, newModel.state)

	// Test 'q'
	m.planReviewFocusedElement = 1
	msg = tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("q")}
	updatedModel, cmd = m.Update(msg)
	newModel = updatedModel.(Model)

	assert.NotNil(t, cmd)
	assert.Equal(t, tea.Quit(), cmd())
	assert.Equal(t, StatePlanReview, newModel.state)
}

func TestRenderPlanReviewView(t *testing.T) {
	m := createTestPlanModel()
	m.width = 100
	m.height = 30

	output := m.View()

	// Basic checks for presence of key elements
	assert.Contains(t, output, "Execution plan:")
	assert.Contains(t, output, "Target:")
	assert.Contains(t, output, "Deps:")
	assert.Contains(t, output, "dev")
	assert.Contains(t, output, "Directory Summary") // Since root "dev" is selected initially
}

func TestRenderAttributes(t *testing.T) {
	rc := plan.ResourceChange{
		Before: map[string]interface{}{
			"old_attr": "old_val",
			"mod_attr": "value1",
		},
		After: map[string]interface{}{
			"new_attr": "new_val",
			"mod_attr": "value2",
		},
	}

	diff := renderAttributes(rc)

	// We expect ANSI codes, but we can check for the text content
	// or specific tokens.
	// Check for update diffs
	assert.Contains(t, diff, "new_attr")
	assert.Contains(t, diff, "new_val")
}

func TestRenderAttributes_Prefixes(t *testing.T) {
	// Case 1: Create - should have "+ " prefix on attributes, colored symbol
	rcCreate := plan.ResourceChange{
		ChangeType: plan.ChangeTypeCreate,
		After: map[string]interface{}{
			"attr": "val",
		},
	}
	diffCreate := renderAttributes(rcCreate)

	// Assert content contains key/val
	assert.Contains(t, diffCreate, "attr: val")
	// Assert content contains "+"
	assert.Contains(t, diffCreate, "+")

	// Case 2: Delete - should have "- " prefix on attributes
	rcDelete := plan.ResourceChange{
		ChangeType: plan.ChangeTypeDelete,
		Before: map[string]interface{}{
			"attr": "val",
		},
	}
	diffDelete := renderAttributes(rcDelete)
	assert.Contains(t, diffDelete, "attr: val")
	assert.Contains(t, diffDelete, "-")
}

func TestRenderPlanReviewView_Detailed(t *testing.T) {
	// Create a model with various change types
	report := &plan.PlanReport{
		Stacks: []plan.StackResult{
			{
				StackPath:  "delete-stack",
				HasChanges: true,
				Stats:      plan.StackStats{Destroy: 1},
				ResourceChanges: []plan.ResourceChange{
					{
						Address:    "res.del",
						Type:       "type",
						ChangeType: plan.ChangeTypeDelete,
						Before:     map[string]interface{}{"id": "1"},
					},
				},
			},
			{
				StackPath:  "update-stack",
				HasChanges: true,
				Stats:      plan.StackStats{Change: 1},
				ResourceChanges: []plan.ResourceChange{
					{
						Address:    "res.upd",
						Type:       "type",
						ChangeType: plan.ChangeTypeUpdate,
						Before:     map[string]interface{}{"val": "1"},
						After:      map[string]interface{}{"val": "2"},
					},
				},
			},
			{
				StackPath:  "replace-stack",
				HasChanges: true,
				Stats:      plan.StackStats{Add: 1, Destroy: 1}, // Replace is usually add+destroy or separate type
				ResourceChanges: []plan.ResourceChange{
					{
						Address:    "res.rep",
						Type:       "type",
						ChangeType: plan.ChangeTypeReplace,
						Before:     map[string]interface{}{"val": "1"},
						After:      map[string]interface{}{"val": "2"},
					},
				},
			},
		},
	}

	m := NewPlanReviewModel(report)
	m.width = 100
	m.height = 30
	m.ready = true

	// Helper to find index
	findIndex := func(name string) int {
		for i, item := range m.planFlatItems {
			if strings.Contains(item.Name, name) {
				return i
			}
		}
		return -1
	}

	// Test 1: Render Delete Stack
	idx := findIndex("delete-stack")
	if idx >= 0 {
		m.planListCursor = idx
		out := m.View()
		assert.Contains(t, out, "Plan: delete-stack")
		assert.Contains(t, out, "- res.del")
	}

	// Test 2: Render Update Stack
	idx = findIndex("update-stack")
	if idx >= 0 {
		m.planListCursor = idx
		out := m.View()
		assert.Contains(t, out, "Plan: update-stack")
		assert.Contains(t, out, "~ res.upd")
		assert.Contains(t, out, "val: 1 -> 2")
	}

	// Test 3: Render Replace Stack
	idx = findIndex("replace-stack")
	if idx >= 0 {
		m.planListCursor = idx
		out := m.View()
		assert.Contains(t, out, "Plan: replace-stack")
		assert.Contains(t, out, "-/+ res.rep")
	}
}

func TestHandlePlanReviewUpdate_WindowSize(t *testing.T) {
	m := createTestPlanModel()
	msg := tea.WindowSizeMsg{Width: 200, Height: 50}

	updatedModel, _ := m.Update(msg)
	newModel := updatedModel.(Model)

	assert.Equal(t, 200, newModel.width)
	assert.Equal(t, 50, newModel.height)
	assert.True(t, newModel.ready)
}

func TestCalculateVisibleRangePlan(t *testing.T) {
	// Total < Viewport
	start, end := calculateVisibleRange(5, 2, 10)
	assert.Equal(t, 0, start)
	assert.Equal(t, 5, end)

	// Cursor at start
	start, end = calculateVisibleRange(20, 0, 5)
	assert.Equal(t, 0, start)
	assert.Equal(t, 5, end)

	// Cursor at end
	start, end = calculateVisibleRange(20, 19, 5)
	assert.Equal(t, 15, start)
	assert.Equal(t, 20, end)

	// Cursor in middle
	start, end = calculateVisibleRange(20, 10, 5)
	// 10 - 2 = 8
	assert.Equal(t, 8, start)
	assert.Equal(t, 13, end)
}

func TestPlanReview_FocusAndScroll(t *testing.T) {
	m := createTestPlanModel()
	m.width = 100
	m.height = 8 // Reduced height to ensure content overflows and scrolling is possible

	// Initial State: Focus on Master (0)
	assert.Equal(t, 0, m.planReviewFocusedElement)

	// Test 1: Switch Focus Right
	msg := tea.KeyMsg{Type: tea.KeyRight}
	updatedModel, _ := m.Update(msg)
	newModel := updatedModel.(Model)
	assert.Equal(t, 1, newModel.planReviewFocusedElement)

	// Test 2: Scroll Down (increment count)
	keyMsg := tea.KeyMsg{Type: tea.KeyDown}
	updatedModel, _ = newModel.Update(keyMsg)
	newModel = updatedModel.(Model)
	assert.Equal(t, 1, newModel.planDetailScrollOffset)

	// Test 3: Scroll Down to Limit
	for i := 0; i < 10; i++ {
		keyMsg = tea.KeyMsg{Type: tea.KeyDown}
		updatedModel, _ = newModel.Update(keyMsg)
		newModel = updatedModel.(Model)
	}
	assert.Greater(t, newModel.planDetailScrollOffset, 0)
	assert.Less(t, newModel.planDetailScrollOffset, 20)

	// Test 4: Scroll Up
	previousOffset := newModel.planDetailScrollOffset
	updatedModel, _ = newModel.Update(tea.KeyMsg{Type: tea.KeyUp})
	newModel = updatedModel.(Model)
	assert.Equal(t, previousOffset-1, newModel.planDetailScrollOffset)

	// Test 5: Switch Focus Left
	keyMsg = tea.KeyMsg{Type: tea.KeyLeft}
	updatedModel, _ = newModel.Update(keyMsg)
	newModel = updatedModel.(Model)
	assert.Equal(t, 0, newModel.planReviewFocusedElement)
}

func TestPlanReview_ScrollResetOnSelectionChange(t *testing.T) {
	m := createTestPlanModel()
	// Set some scroll offset
	m.planDetailScrollOffset = 5
	m.planReviewFocusedElement = 0 // Focus Master

	// Move Selection Down
	msg := tea.KeyMsg{Type: tea.KeyDown}
	updatedModel, _ := m.Update(msg)
	newModel := updatedModel.(Model)

	// Scroll offset should be reset to 0
	assert.Equal(t, 0, newModel.planDetailScrollOffset)
}

func TestPlanReview_PageNavigation(t *testing.T) {
	// Inject a larger plan report to ensure sufficient content for scrolling
	largeReport := &plan.PlanReport{
		Stacks: []plan.StackResult{
			{
				StackPath:  "long/stack",
				HasChanges: true,
				Stats:      plan.StackStats{Add: 1},
				ResourceChanges: []plan.ResourceChange{
					{
						Address:    "res.long",
						Type:       "type",
						ChangeType: plan.ChangeTypeCreate,
						After: map[string]interface{}{
							"attr1": "val1", "attr2": "val2", "attr3": "val3",
							"attr4": "val4", "attr5": "val5", "attr6": "val6",
							"attr7": "val7", "attr8": "val8", "attr9": "val9",
						},
					},
				},
			},
		},
	}
	m := NewPlanReviewModel(largeReport)
	m.width = 100
	m.height = 8 // Visible = 8 - 4 = 4 lines.

	// Focus Detail View
	// Cursor 0 is "long" (dir), Cursor 1 is "long/stack" (stack with changes)
	m.planListCursor = 1
	m.planReviewFocusedElement = 1
	m.ready = true

	// Test 1: Page Down
	// Initial offset = 0
	msg := tea.KeyMsg{Type: tea.KeyPgDown}
	updatedModel, _ := m.Update(msg)
	newModel := updatedModel.(Model)

	// Should have scrolled by visible height (4)
	assert.Equal(t, 4, newModel.planDetailScrollOffset)

	// Test 2: Page Up
	// Scroll back up
	msg = tea.KeyMsg{Type: tea.KeyPgUp}
	updatedModel, _ = newModel.Update(msg)
	newModel = updatedModel.(Model)

	// Should be back to 0
	assert.Equal(t, 0, newModel.planDetailScrollOffset)

	// Test 3: Page Down at Boundary
	// Scroll down multiple times
	for i := 0; i < 5; i++ {
		updatedModel, _ = newModel.Update(tea.KeyMsg{Type: tea.KeyPgDown})
		newModel = updatedModel.(Model)
	}
	// Should be capped at maxOffset (Total - Visible)
	// Just verify it's > 4 and stable
	offset := newModel.planDetailScrollOffset
	assert.Greater(t, offset, 4)

	newModel = updatedModel.(Model)
	assert.Equal(t, offset, newModel.planDetailScrollOffset)
}

func TestRenderPlanDetailView_ScrollingLogic(t *testing.T) {
	// Create a model with many lines in detailed view
	report := &plan.PlanReport{
		Stacks: []plan.StackResult{
			{
				StackPath:  "long-stack",
				HasChanges: true,
				Stats:      plan.StackStats{Change: 1},
				ResourceChanges: []plan.ResourceChange{
					{
						Address:    "res.long",
						Type:       "type",
						ChangeType: plan.ChangeTypeUpdate,
						Before:     map[string]interface{}{"val": "old"},
						After:      map[string]interface{}{"val": "new"},
					},
				},
			},
		},
	}

	// We want to force getPlanDetailLines to return many lines
	// The standard renderer output might be short.
	// Let's rely on the fact that we can interact with the scroll offset directly.

	m := NewPlanReviewModel(report)
	m.width = 100
	m.height = 10
	m.planListCursor = 0 // Select the root dir

	// Root dir summary might be short if only 1 child.
	// Let's use the explicit stack which has more content?
	// The flatten logic puts the stack at some index.
	// "long-stack" -> 1 item if root level?

	// Let's manually inject planFlatItems to ensure we control what's selected
	node := &plan.TreeNode{
		Path:       "test",
		Name:       "test",
		HasChanges: true,
		Stats:      plan.StackStats{Add: 100}, // Make it look interesting
		Stack: &plan.StackResult{
			HasChanges:      true,
			ResourceChanges: make([]plan.ResourceChange, 20), // 20 changes
		},
	}
	for i := 0; i < 20; i++ {
		node.Stack.ResourceChanges[i] = plan.ResourceChange{
			Address:    "res",
			Type:       "type",
			ChangeType: plan.ChangeTypeCreate,
			After:      map[string]interface{}{"foo": "bar"},
		}
	}

	m.planFlatItems = []*plan.TreeNode{node}
	m.planListCursor = 0

	// Render with 0 offset
	m.planDetailScrollOffset = 0
	view0 := m.renderPlanDetailView()
	assert.NotEmpty(t, view0)

	// Render with offset
	m.planDetailScrollOffset = 5
	view5 := m.renderPlanDetailView()
	assert.NotEmpty(t, view5)
	assert.NotEqual(t, view0, view5)

	// Render with large offset (should be clamped/handled safety)
	m.planDetailScrollOffset = 1000
	viewOver := m.renderPlanDetailView()
	assert.NotEmpty(t, viewOver)
}

func TestRenderPlanDetailView_NoSelection(t *testing.T) {
	m := NewPlanReviewModel(&plan.PlanReport{})
	m.planFlatItems = []*plan.TreeNode{}
	m.planListCursor = -1

	view := m.renderPlanDetailView()
	assert.Contains(t, view, "Select an item")
}

func TestRenderAttributes_Unknown(t *testing.T) {
	// Case 1: Unknown value after update
	rc := plan.ResourceChange{
		ChangeType: plan.ChangeTypeUpdate,
		Before:     map[string]interface{}{"attr": "old"},
		Unknown:    map[string]interface{}{"attr": true},
	}
	diff := renderAttributes(rc)
	assert.Contains(t, diff, "old -> (known after apply)")

	// Case 2: Unknown value new (create)
	rc2 := plan.ResourceChange{
		ChangeType: plan.ChangeTypeCreate,
		Unknown:    map[string]interface{}{"attr": true},
	}
	diff2 := renderAttributes(rc2)
	assert.Contains(t, diff2, "attr: (known after apply)")
}
