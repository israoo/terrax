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
	focusedColumn int  // 0 = commands, 1+ = navigation columns
	confirmed     bool // Whether user confirmed selection

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
		selectedCommand: 0,
		focusedColumn:   0,
		confirmed:       false,
		ready:           false,
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
func (m Model) calculateColumnWidth() int {
	maxDepth := m.navigator.GetMaxDepth()
	if maxDepth == 0 {
		return MinColumnWidth
	}

	maxTotalColumns := 1 + maxDepth
	totalOverhead := ColumnOverhead * maxTotalColumns
	availableWidth := m.width - totalOverhead
	colWidth := availableWidth / maxTotalColumns

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
		// Commands column: use full propagated path
		targetNode = m.navigator.PropagateSelection(m.navState)
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

// moveToPreviousColumn moves focus to the previous column (with wrap-around).
func (m *Model) moveToPreviousColumn() {
	if m.focusedColumn > 0 {
		m.focusedColumn--
	} else {
		// Wrap to last visible column
		m.focusedColumn = m.navigator.GetMaxVisibleDepth(m.navState)
	}
}

// moveToNextColumn moves focus to the next column (with wrap-around).
func (m *Model) moveToNextColumn() {
	maxVisibleDepth := m.navigator.GetMaxVisibleDepth(m.navState)
	if m.focusedColumn < maxVisibleDepth {
		m.focusedColumn++
	} else {
		// Wrap to commands column
		m.focusedColumn = 0
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
		targetNode = m.navigator.PropagateSelection(m.navState)
	} else {
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
