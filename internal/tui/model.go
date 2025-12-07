package tui

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/israoo/terrax/internal/stack"
)

// ColumnType represents the type of column being focused.
type ColumnType int

const (
	// CommandsColumn represents the commands column.
	CommandsColumn ColumnType = iota
	// NavigationColumn represents a navigation column.
	NavigationColumn
)

// Model is the main TUI model following Bubble Tea architecture.
// It maintains minimal state and delegates business logic to Navigator.
type Model struct {
	// Navigation
	navigator *stack.Navigator
	navState  *stack.NavigationState

	// Commands
	commands        []string
	selectedCommand int

	// UI State
	focusedColumn    int  // 0 = commands, 1+ = navigation columns
	navigationOffset int  // First visible navigation level (sliding window)
	confirmed        bool // Whether user confirmed selection

	// Layout
	width       int
	height      int
	columnWidth int // Pre-calculated static column width

	// State flags
	ready bool
}

// NewModel creates a new TUI model instance.
func NewModel(stackRoot *stack.Node, maxDepth int) Model {
	navigator := stack.NewNavigator(stackRoot, maxDepth)
	navState := stack.NewNavigationState(maxDepth)

	m := Model{
		navigator: navigator,
		navState:  navState,
		commands: []string{
			"plan",
			"apply",
			"destroy",
			"validate",
			"init",
			"output",
			"refresh",
			"fmt",
		},
		selectedCommand:  0,
		focusedColumn:    0,
		navigationOffset: 0,
		confirmed:        false,
		ready:            false,
	}

	// Initialize navigation state
	navigator.PropagateSelection(navState)

	return m
}

// Init initializes the model (BubbleTea interface).
func (m Model) Init() tea.Cmd {
	return nil
}

// Update handles messages and updates state (BubbleTea interface).
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		return m.handleKeyPress(msg)
	case tea.WindowSizeMsg:
		return m.handleWindowResize(msg), nil
	}
	return m, nil
}

// handleWindowResize processes window resize events.
func (m Model) handleWindowResize(msg tea.WindowSizeMsg) Model {
	m.width = msg.Width
	m.height = msg.Height
	m.columnWidth = m.calculateColumnWidth()
	m.ready = true
	return m
}

// calculateColumnWidth computes the static width for all columns.
// Fixed: 1 commands column + max 3 navigation columns in sliding window.
func (m Model) calculateColumnWidth() int {
	maxDepth := m.navigator.GetMaxDepth()
	if maxDepth == 0 {
		return MinColumnWidth
	}

	// Always calculate for 1 commands + 3 navigation columns max
	maxVisibleColumns := 4 // 1 commands + 3 navigation
	totalOverhead := ColumnOverhead * maxVisibleColumns
	availableWidth := m.width - totalOverhead
	colWidth := availableWidth / maxVisibleColumns

	if colWidth < MinColumnWidth {
		return MinColumnWidth
	}
	return colWidth
}

// handleKeyPress processes keyboard input.
func (m Model) handleKeyPress(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case KeyCtrlC, KeyQ:
		return m, tea.Quit
	case KeyEnter:
		return m.handleEnterKey()
	case KeyUp, KeyK:
		return m.handleVerticalMove(true), nil
	case KeyDown, KeyJ:
		return m.handleVerticalMove(false), nil
	case KeyLeft, KeyH:
		return m.handleHorizontalMove(true), nil
	case KeyRight, KeyL:
		return m.handleHorizontalMove(false), nil
	}
	return m, nil
}

// handleEnterKey processes the enter key with dual behavior.
func (m Model) handleEnterKey() (tea.Model, tea.Cmd) {
	var targetNode *stack.Node

	if m.isCommandsColumnFocused() {
		// Commands column: use root directory (ignore navigation selections)
		targetNode = m.navigator.GetRoot()
	} else {
		// Navigation column: use path only up to focused level
		depth := m.getNavigationDepth()
		targetNode = m.navigator.GetNodeAtDepth(m.navState, depth)
	}

	if targetNode != nil {
		m.confirmed = true
		return m, tea.Quit
	}

	return m, nil
}

// handleVerticalMove processes up/down navigation.
func (m Model) handleVerticalMove(isUp bool) Model {
	if m.isCommandsColumnFocused() {
		m.moveCommandSelection(isUp)
	} else {
		m.moveNavigationSelection(isUp)
	}
	return m
}

// handleHorizontalMove processes left/right column switching.
func (m Model) handleHorizontalMove(isLeft bool) Model {
	if isLeft {
		m.moveToPreviousColumn()
	} else {
		m.moveToNextColumn()
	}
	return m
}

// moveCommandSelection moves selection in commands column.
func (m *Model) moveCommandSelection(isUp bool) {
	if isUp && m.selectedCommand > 0 {
		m.selectedCommand--
	} else if !isUp && m.selectedCommand < len(m.commands)-1 {
		m.selectedCommand++
	}
}

// moveNavigationSelection moves selection in navigation column.
func (m *Model) moveNavigationSelection(isUp bool) {
	depth := m.getNavigationDepth()
	if depth < 0 {
		return
	}

	var moved bool
	if isUp {
		moved = m.navigator.MoveUp(m.navState, depth)
	} else {
		moved = m.navigator.MoveDown(m.navState, depth)
	}

	if moved {
		m.navigator.PropagateSelection(m.navState)
	}
}

// moveToPreviousColumn moves focus to the previous column with sliding window.
func (m *Model) moveToPreviousColumn() {
	if m.focusedColumn > 0 {
		// Move focus left
		m.focusedColumn--

		// If new focus is outside left window boundary (and not commands column)
		if m.focusedColumn > 0 && m.focusedColumn < m.navigationOffset+1 {
			// Slide window left
			if m.navigationOffset > 0 {
				m.navigationOffset--
			}
		}
	} else {
		// Wrap to last visible column
		maxVisibleDepth := m.navigator.GetMaxVisibleDepth(m.navState)
		m.focusedColumn = maxVisibleDepth

		// Adjust window to show the last column
		if maxVisibleDepth > 3 {
			m.navigationOffset = maxVisibleDepth - 3
		} else {
			m.navigationOffset = 0
		}
	}
}

// moveToNextColumn moves focus to the next column with sliding window.
func (m *Model) moveToNextColumn() {
	maxVisibleDepth := m.navigator.GetMaxVisibleDepth(m.navState)

	if m.focusedColumn < maxVisibleDepth {
		// Move focus right
		m.focusedColumn++

		// If new focus is outside right window boundary
		// Window shows levels: navigationOffset, navigationOffset+1, navigationOffset+2
		// Focus is at column index (1 + depth), so depth = focusedColumn - 1
		if m.focusedColumn > 0 {
			depth := m.focusedColumn - 1 // Convert to 0-based depth
			if depth > m.navigationOffset+2 {
				// Slide window right
				m.navigationOffset++
			}
		}
	} else {
		// Wrap to commands column
		m.focusedColumn = 0
		m.navigationOffset = 0
	}
}

// isCommandsColumnFocused returns true if the commands column is focused.
func (m Model) isCommandsColumnFocused() bool {
	return m.focusedColumn == 0
}

// getNavigationDepth returns the current navigation depth (0-indexed).
// Returns -1 if not in a navigation column.
func (m Model) getNavigationDepth() int {
	if m.isCommandsColumnFocused() {
		return -1
	}
	return m.focusedColumn - 1
}

// GetSelectedCommand returns the currently selected command name.
func (m Model) GetSelectedCommand() string {
	if m.selectedCommand >= 0 && m.selectedCommand < len(m.commands) {
		return m.commands[m.selectedCommand]
	}
	return NoItemSelected
}

// GetSelectedStackPath returns the selected stack path.
func (m Model) GetSelectedStackPath() string {
	var targetNode *stack.Node

	if m.isCommandsColumnFocused() {
		// Commands column: return root directory path
		targetNode = m.navigator.GetRoot()
	} else {
		// Navigation column: return path up to focused level
		depth := m.getNavigationDepth()
		targetNode = m.navigator.GetNodeAtDepth(m.navState, depth)
	}

	if targetNode != nil {
		return targetNode.Path
	}
	return NoItemSelected
}

// IsConfirmed returns whether the user confirmed the selection.
func (m Model) IsConfirmed() bool {
	return m.confirmed
}

// getCurrentNavigationPath returns the current navigation path as a string.
// It builds a breadcrumb path starting from root directory up to the focused column.
func (m Model) getCurrentNavigationPath() string {
	// Start with root directory path
	rootNode := m.navigator.GetRoot()
	if rootNode == nil {
		return "~"
	}

	path := rootNode.Path

	// If in commands column, return just the root path
	if m.isCommandsColumnFocused() || m.navigator.GetMaxDepth() == 0 {
		return path
	}

	depth := m.getNavigationDepth()

	// Build path from selected indices, appending subdirectories
	for i := 0; i <= depth && i < len(m.navState.Columns); i++ {
		if i >= len(m.navState.SelectedIndices) {
			break
		}

		selectedIdx := m.navState.SelectedIndices[i]
		if selectedIdx >= 0 && selectedIdx < len(m.navState.Columns[i]) {
			// Extract directory name (remove emoji marker if present)
			dirName := m.navState.Columns[i][selectedIdx]
			// Remove " 📦" marker if it exists
			if len(dirName) > 3 && dirName[len(dirName)-2:] == "📦" {
				dirName = dirName[:len(dirName)-3]
			}
			path += "/" + dirName
		}
	}

	return path
}

// hasLeftOverflow returns true if there are navigation columns to the left.
func (m Model) hasLeftOverflow() bool {
	return m.navigationOffset > 0
}

// canAdvanceFurther returns true if the currently focused node has children.
// This determines if the user can navigate deeper into the hierarchy.
func (m Model) canAdvanceFurther() bool {
	// Commands column has no children to advance to
	if m.isCommandsColumnFocused() {
		return false
	}

	depth := m.getNavigationDepth()
	if depth < 0 || depth >= len(m.navState.CurrentNodes) {
		return false
	}

	// Get the currently focused node
	currentNode := m.navState.CurrentNodes[depth]
	if currentNode == nil {
		return false
	}

	// Check if this node has children
	return currentNode.HasChildren()
}

// hasRightOverflow returns true if there are navigation columns to the right.
// Shows indicator if: 1) sliding window doesn't cover last levels AND 2) current node has children.
func (m Model) hasRightOverflow() bool {
	maxDepth := m.navigator.GetMaxDepth()

	// First check: is there space beyond the visible window?
	if m.navigationOffset+3 >= maxDepth {
		return false
	}

	// Second check: does the currently selected node have children?
	// (Don't show arrow if we're at a leaf node even if maxDepth is deeper)
	if !m.canAdvanceFurther() {
		return false
	}

	return true
}
