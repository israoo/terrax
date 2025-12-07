package tui

import (
	"github.com/israoo/terrax/internal/stack"
)

// NewTestModel creates a Model instance for testing with configurable state.
// This function allows tests to bypass the normal initialization and set specific states.
func NewTestModel(stackRoot *stack.Node, maxDepth int, commands []string, maxNavigationColumns int, confirmed bool, selectedCommand, selectedStackPath string) Model {
	navigator := stack.NewNavigator(stackRoot, maxDepth)
	navState := stack.NewNavigationState(maxDepth)

	// Default maxNavigationColumns if invalid
	if maxNavigationColumns < 1 {
		maxNavigationColumns = 3
	}

	// Find the command index.
	selectedCommandIdx := 0
	for i, cmd := range commands {
		if cmd == selectedCommand {
			selectedCommandIdx = i
			break
		}
	}

	m := Model{
		navigator:            navigator,
		navState:             navState,
		commands:             commands,
		selectedCommand:      selectedCommandIdx,
		focusedColumn:        0,
		navigationOffset:     0,
		confirmed:            confirmed,
		ready:                true,
		maxNavigationColumns: maxNavigationColumns,
		width:                120,
		height:               30,
		columnWidth:          25,
	}

	// Initialize navigation state.
	navigator.PropagateSelection(navState)

	return m
}
