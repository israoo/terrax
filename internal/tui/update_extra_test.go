package tui

import (
	"testing"

	"github.com/charmbracelet/bubbles/textinput"
	"github.com/stretchr/testify/assert"
)

// Test wrapping behavior in command selection
func TestMoveCommandSelection_wrapping(t *testing.T) {
	// Setup model with 3 commands
	cmds := []string{"cmd1", "cmd2", "cmd3"}
	// Height large enough so all visible (no pagination)
	m := NewModel(nil, 0, cmds, 0)
	m.height = 100
	m.focusedColumn = 0

	// Initial state: selected index 0
	assert.Equal(t, 0, m.selectedCommand)

	// Move Up from 0 -> Should wrap to last (2)
	m.moveCommandSelection(true) // Up
	assert.Equal(t, 2, m.selectedCommand)

	// Move Down from 2 -> Should wrap to first (0)
	m.moveCommandSelection(false) // Down
	assert.Equal(t, 0, m.selectedCommand)
}

// Test pagination jump behavior
func TestMoveCommandSelection_pagination(t *testing.T) {
	// Create many commands to force pagination
	// Each page shows "maxVisible" items.
	// maxVisible = availableHeight - 1.
	// Let's force a small height.
	// Header(1) + Bread(1) + Title(1) + Space(1) + Footer(1) + Pad(4) = 9 lines reserved.
	// If Height = 13. Available = 4. MaxVisible = 3.

	cmds := []string{"c1", "c2", "c3", "c4", "c5", "c6", "c7"}
	m := NewModel(nil, 0, cmds, 0)
	m.height = 13

	// Reserved = 9. Avail = 4. Max = 3.
	// Page 1: c1, c2, c3
	// Page 2: c4, c5, c6
	// Page 3: c7

	// Start at 0 (c1)
	m.selectedCommand = 0

	// Move Down -> 1 (c2)
	m.moveCommandSelection(false)
	assert.Equal(t, 1, m.selectedCommand)

	// Move Down -> 2 (c3) -> End of Page 1
	m.moveCommandSelection(false)
	assert.Equal(t, 2, m.selectedCommand)

	// Move Down -> 3 (c4) -> Start of Page 2 (Jump)
	m.moveCommandSelection(false)
	assert.Equal(t, 3, m.selectedCommand)

	// Move Up -> 2 (c3) -> End of Page 1 (Jump Back)
	m.moveCommandSelection(true)
	assert.Equal(t, 2, m.selectedCommand)
}

// Test column navigation wrapping and sliding
// Test column navigation wrapping and sliding
func TestMoveNavigationSelection_wrapping(t *testing.T) {
	// Setup model with navigation columns
	// We need to import stack but it's already imported in update.go so available in package.
	// We rely on tui's import of stack.

	m := NewTestModel(nil, 3, nil, 3, false, "", "")

	// Manually populate columns (assuming stack struct fields are public)
	// We need to verify if stack.NavigationState fields are settable.
	// From reading update.go: m.navState.Columns[depth]
	// So yes.

	// Create dummy columns:
	// Col 0: 3 items
	col0 := []string{"item1", "item2", "item3"}

	// We can't easily append if we don't know the internal structure init,
	// but NewTestModel inits navState with maxDepth.
	// navState.Columns is likely [][]string with size maxDepth? or slice?
	// NewNavigationState logic: Columns = make([][]string, maxDepth) usually.

	// Let's safe append or set.
	if len(m.navState.Columns) > 0 {
		m.navState.Columns[0] = col0
		m.navState.SelectedIndices[0] = 0 // Select first

		m.focusedColumn = 1 // Navigation column 0 corresponds to focusedCol 1 (0 is commands)

		// Move Up from 0 -> Wrap to last (2)
		m.moveNavigationSelection(true)
		assert.Equal(t, 2, m.navState.SelectedIndices[0])

		// Move Down from 2 -> Wrap to first (0)
		m.moveNavigationSelection(false)
		assert.Equal(t, 0, m.navState.SelectedIndices[0])
	}
}

func TestColumnNavigation_wrapping(t *testing.T) {
	// Setup model with 3 navigation columns (commands + 3 nav cols = 4 total cols)
	// indices: 0 (cmds), 1 (root), 2 (child), 3 (grandchild)
	m := NewTestModel(nil, 3, nil, 3, false, "", "")

	// Set mock max visible depth for the navigator
	// Since we mock, we can set it via focusedColumn logic which calls GetMaxVisibleDepth
	// But GetMaxVisibleDepth relies on navState.
	// We need navState to have proper depth.

	m.navState.Columns = make([][]string, 3)
	m.navState.SelectedIndices = make([]int, 3)
	// This implies depth 3. Max visible depth will be 3.

	m.focusedColumn = 0

	// Move Left from 0 -> Wrap to last (3)
	m.moveToPreviousColumn()
	// GetMaxVisibleDepth depends on whether next col has selected index pointing to valid item?
	// The implementation checks:
	// if depth < len(ns.Columns) && len(ns.Columns[depth]) > 0
	// So we need columns to be populated.
	m.navState.Columns[0] = []string{"Root"}
	m.navState.Columns[1] = []string{"Child"}
	m.navState.Columns[2] = []string{"GrandChild"}

	// Now try again
	m.moveToPreviousColumn()

	// It should wrap to 3 (max visible depth)
	// If standard behavior is 0 -> Max.
	// But let's check implementation.
	// "if m.focusedColumn > 0 ... else Wrap to last visible column."

	// Note: default NewTestModel sets maxNavigationColumns=3.

	// If it wrapped:
	assert.Equal(t, 3, m.focusedColumn)

	// Move Right from 3 -> Wrap to 0
	m.moveToNextColumn()
	assert.Equal(t, 0, m.focusedColumn)

	// Test Normal Move and Window Slide
	// Reset focus to 1 (Root)
	m.focusedColumn = 1
	m.navigationOffset = 0

	// Move Right -> 2 (Child)
	m.moveToNextColumn()
	assert.Equal(t, 2, m.focusedColumn)

	// Move Right -> 3 (GrandChild). Should Slide Window because maxNavigationColumns=3.
	// Visible cols: 0 (CMD), 1 (Root), 2 (Child). Focus 3 needs window to slide?
	// Window shows: navOffset to navOffset + maxCols - 1.
	// 0 to 2.
	// Focus 3 (Depth 2).
	// Depth 2 > 0 + 2 = 2? No.
	// Wait. focusedColumn 3 = Depth 2.
	// Window: 0, 1, 2 (depths).
	// So focusedColumn 3 is visible.

	m.moveToNextColumn()
	assert.Equal(t, 3, m.focusedColumn)
	// Check slide?

	// Let's force slide. Reduce maxNavigationColumns to 2.
	m.maxNavigationColumns = 2
	m.navigationOffset = 0
	// Window: Depths 0, 1. (Cols 1, 2).
	// Focus: 2.

	// Move Focus to 3 (Depth 2).
	// Depth 2 > 0 + 1 = 1. Yes. Slide.
	m.moveToNextColumn()
	// (Note: m.focusedColumn was 3 already from previous step, let's reset)
	m.focusedColumn = 2  // Depth 1.
	m.moveToNextColumn() // -> 3 (Depth 2).

	assert.Equal(t, 3, m.focusedColumn)
	assert.Equal(t, 1, m.navigationOffset) // Slide right.

	// Move Left
	m.moveToPreviousColumn()
	assert.Equal(t, 2, m.focusedColumn)

	// Move Left -> 1 (Depth 0). Slide Left.
	// Depth 0 < 1. Yes.
	m.moveToPreviousColumn()
	assert.Equal(t, 1, m.focusedColumn)
	assert.Equal(t, 0, m.navigationOffset)
}

func TestHelper_NewTestModel(t *testing.T) {
	// Simple invocation to cover the helper
	cmds := []string{"foo", "bar"}
	m := NewTestModel(nil, 5, cmds, 3, true, "bar", "")

	assert.Equal(t, 1, m.selectedCommand)
	assert.True(t, m.confirmed)
	assert.Equal(t, 3, m.maxNavigationColumns)
}

// Test filter navigation
// Test filter navigation
func TestMoveCommandSelection_filtered(t *testing.T) {
	cmds := []string{"apple", "banana", "apricot", "cherry"}
	m := NewModel(nil, 0, cmds, 0)
	m.height = 100

	// Setup text input filter
	ti := textinput.New()
	ti.SetValue("ap") // Matches apple, apricot
	m.columnFilters[0] = ti

	// Initial selection: 0 ("apple")
	assert.Equal(t, 0, m.selectedCommand)

	// Move Down
	// Filtered list: [apple (0), apricot (2)]
	// Index in filtered list: 0.
	// Move Down -> Index 1 in filtered list (apricot).

	m.moveCommandSelection(false)

	// Should be "apricot" (Original Index 2)
	assert.Equal(t, 2, m.selectedCommand)

	// Move Down again -> End of filtered list -> Wrap to top (apple)
	m.moveCommandSelection(false)
	assert.Equal(t, 0, m.selectedCommand)

	// Move Up -> Wrap to bottom (apricot)
	m.moveCommandSelection(true)
	assert.Equal(t, 2, m.selectedCommand)
}
