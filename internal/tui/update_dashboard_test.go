package tui

import (
	"context"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/assert"

	"github.com/israoo/terrax/internal/stack"
)

func makeDashModel(t *testing.T) Model {
	t.Helper()
	root := &stack.Node{
		Name: "root", Path: "/root",
		Children: []*stack.Node{
			{
				Name: "infra", Path: "/root/infra",
				Children: []*stack.Node{
					{Name: "dev", Path: "/root/infra/dev", IsStack: true},
				},
			},
			{Name: "mgmt", Path: "/root/mgmt", IsStack: true},
		},
	}
	nav := stack.NewNavigator(root, 2)
	navState := stack.NewNavigationState(2)
	nav.PropagateSelection(navState)

	noopFilter := FilterPathsFn(func(_ context.Context, p string) (string, []string, error) {
		return "/root", []string{p}, nil
	})

	m := NewDashboardModel(root, 2, []string{"plan", "apply"}, 3, context.Background(), noopFilter, nil)
	m.width = 120
	m.height = 40
	m.ready = true
	m.dashTreeNodes = flattenDashTree(nav.GetRoot(), m.dashTreeExpanded)
	return m
}

func TestDashboard_TreeNavDown(t *testing.T) {
	m := makeDashModel(t)
	assert.Equal(t, 0, m.dashTreeCursor)

	result, _ := m.handleDashboardUpdate(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("j")})
	// "j" is not mapped; cursor stays.
	assert.Equal(t, 0, result.(Model).dashTreeCursor)

	result, _ = m.handleDashboardUpdate(tea.KeyMsg{Type: tea.KeyDown})
	assert.Equal(t, 1, result.(Model).dashTreeCursor)
}

func TestDashboard_TreeNavUp_AtTop(t *testing.T) {
	m := makeDashModel(t)
	result, _ := m.handleDashboardUpdate(tea.KeyMsg{Type: tea.KeyUp})
	assert.Equal(t, 0, result.(Model).dashTreeCursor, "cursor should not go below 0")
}

func TestDashboard_TreeExpand(t *testing.T) {
	m := makeDashModel(t)
	// cursor is on "infra" (index 0), which has children.
	result, _ := m.handleDashboardUpdate(tea.KeyMsg{Type: tea.KeyEnter})
	m2 := result.(Model)
	assert.True(t, m2.dashTreeExpanded["/root/infra"], "infra should be expanded")
	assert.Len(t, m2.dashTreeNodes, 3, "infra + dev + mgmt")
}

func TestDashboard_TreeCollapse(t *testing.T) {
	m := makeDashModel(t)
	m.dashTreeExpanded["/root/infra"] = true
	m.dashTreeNodes = flattenDashTree(m.navigator.GetRoot(), m.dashTreeExpanded)

	result, _ := m.handleDashboardUpdate(tea.KeyMsg{Type: tea.KeyEnter})
	m2 := result.(Model)
	assert.False(t, m2.dashTreeExpanded["/root/infra"], "infra should be collapsed")
	assert.Len(t, m2.dashTreeNodes, 2)
}

func TestDashboard_TabSwitchesFocus(t *testing.T) {
	m := makeDashModel(t)
	assert.Equal(t, 0, m.dashFocusedPanel)

	result, _ := m.handleDashboardUpdate(tea.KeyMsg{Type: tea.KeyTab})
	assert.Equal(t, 1, result.(Model).dashFocusedPanel)

	result, _ = result.(Model).handleDashboardUpdate(tea.KeyMsg{Type: tea.KeyTab})
	assert.Equal(t, 2, result.(Model).dashFocusedPanel)

	result, _ = result.(Model).handleDashboardUpdate(tea.KeyMsg{Type: tea.KeyTab})
	assert.Equal(t, 0, result.(Model).dashFocusedPanel)
}

func TestDashboard_PaletteOpenClose(t *testing.T) {
	m := makeDashModel(t)
	// Move cursor to a leaf (mgmt at index 1).
	m.dashTreeCursor = 1

	result, _ := m.handleDashboardUpdate(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(" ")})
	m2 := result.(Model)
	assert.True(t, m2.dashPaletteOpen)

	result, _ = m2.handleDashboardUpdate(tea.KeyMsg{Type: tea.KeyEsc})
	assert.False(t, result.(Model).dashPaletteOpen)
}

func TestDashboard_CommandIgnoredWhileRunning(t *testing.T) {
	m := makeDashModel(t)
	m.dashTreeCursor = 1 // mgmt is a leaf.
	m.dashRunning = true

	result, cmd := m.handleDashboardUpdate(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("p")})
	assert.True(t, result.(Model).dashRunning, "should still be running")
	assert.Nil(t, cmd, "no command should be dispatched")
}

func TestDashboard_RenderDoesNotPanic(t *testing.T) {
	m := makeDashModel(t)
	m.dashTreeNodes = flattenDashTree(m.navigator.GetRoot(), m.dashTreeExpanded)
	result := m.renderDashboardView()
	assert.NotEmpty(t, result)
}

func TestDashboard_QuitCancelsRunningCommand(t *testing.T) {
	m := makeDashModel(t)
	cancelled := false
	m.dashCancelCmd = func() { cancelled = true }
	m.dashRunning = true

	newM, cmd := m.handleDashboardUpdate(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("q")})
	assert.True(t, cancelled)
	assert.Nil(t, cmd, "command should be nil while cancelling")
	assert.True(t, newM.(Model).dashQuitPending, "quit should be pending")
}
