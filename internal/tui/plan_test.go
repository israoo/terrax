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
	_, cmd := m.Update(msg)

	assert.Equal(t, tea.Quit(), cmd())
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
	// or specific tokens. Since lipgloss adds style, exact string match is hard.
	// But let's check basic logic:
	assert.Contains(t, diff, "new_attr")
	assert.Contains(t, diff, "new_val")
	assert.Contains(t, diff, "old_attr")
	assert.Contains(t, diff, "old_val")
	assert.Contains(t, diff, "mod_attr")
	assert.Contains(t, diff, "value1")
	assert.Contains(t, diff, "value2")
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
