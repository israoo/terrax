package tui

import (
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/israoo/terrax/internal/history"
	"github.com/israoo/terrax/internal/stack"
)

// AppState represents the current state of the application.
type AppState int

const (
	// StateNavigation is the default state for navigating stacks and commands.
	StateNavigation AppState = iota
	// StateHistory is the state for viewing execution history.
	StateHistory
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
	// Application State
	state AppState

	// Navigation
	navigator *stack.Navigator
	navState  *stack.NavigationState

	// Commands
	commands        []string
	selectedCommand int

	// History
	history              []history.ExecutionLogEntry
	historyCursor        int
	selectedHistoryEntry *history.ExecutionLogEntry // Entry selected for re-execution
	reExecuteFromHistory bool                       // Flag to indicate re-execution from history

	// UI State
	focusedColumn    int  // 0 = commands, 1+ = navigation columns
	navigationOffset int  // First visible navigation level (sliding window)
	confirmed        bool // Whether user confirmed selection

	// Layout
	width                int
	height               int
	columnWidth          int // Pre-calculated static column width
	maxNavigationColumns int // Maximum navigation columns visible (sliding window)

	// Filtering (per-column)
	columnFilters      map[int]textinput.Model // Filter inputs per column (0=commands, 1+=navigation)
	activeFilterColumn int                     // Which column's filter is currently being edited (-1 = none)

	// Scrolling (per-column vertical viewport)
	scrollOffsets map[int]int // Scroll offset per column (0=commands, 1+=navigation)

	// State flags
	ready bool
}

// NewModel creates a new TUI model instance.
// commands: List of available Terragrunt commands to display.
// maxNavigationColumns: Maximum number of navigation columns visible simultaneously (must be validated before calling).
func NewModel(stackRoot *stack.Node, maxDepth int, commands []string, maxNavigationColumns int) Model {
	navigator := stack.NewNavigator(stackRoot, maxDepth)
	navState := stack.NewNavigationState(maxDepth)

	m := Model{
		state:                StateNavigation,
		navigator:            navigator,
		navState:             navState,
		commands:             commands,
		selectedCommand:      0,
		focusedColumn:        0,
		navigationOffset:     0,
		confirmed:            false,
		ready:                false,
		maxNavigationColumns: maxNavigationColumns,
		columnFilters:        make(map[int]textinput.Model),
		activeFilterColumn:   -1,
		scrollOffsets:        make(map[int]int),
		history:              nil,
		historyCursor:        0,
		selectedHistoryEntry: nil,
		reExecuteFromHistory: false,
	}

	// Initialize navigation state
	navigator.PropagateSelection(navState)

	return m
}

// NewHistoryModel creates a model initialized in history viewing mode.
func NewHistoryModel(historyEntries []history.ExecutionLogEntry) Model {
	m := Model{
		state:                StateHistory,
		history:              historyEntries,
		historyCursor:        0,
		ready:                false,
		selectedHistoryEntry: nil,
		reExecuteFromHistory: false,
	}
	return m
}

// Init initializes the model (BubbleTea interface).
func (m Model) Init() tea.Cmd {
	return nil
}

// Update handles messages and updates state (BubbleTea interface).
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	// Delegate to appropriate handler based on state
	if m.state == StateHistory {
		return m.handleHistoryUpdate(msg)
	}

	// Default: StateNavigation
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
// Uses configured maxNavigationColumns for sliding window.
func (m Model) calculateColumnWidth() int {
	maxDepth := m.navigator.GetMaxDepth()
	if maxDepth == 0 {
		return MinColumnWidth
	}

	// Calculate for 1 commands + N navigation columns max (configurable)
	maxVisibleColumns := 1 + m.maxNavigationColumns
	totalOverhead := ColumnOverhead * maxVisibleColumns
	availableWidth := m.width - totalOverhead
	colWidth := availableWidth / maxVisibleColumns

	if colWidth < MinColumnWidth {
		return MinColumnWidth
	}
	return colWidth
}

// handleHistoryUpdate handles updates when in StateHistory mode.
func (m Model) handleHistoryUpdate(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.ready = true
		return m, nil

	case tea.KeyMsg:
		switch msg.String() {
		case KeyQ, KeyEsc:
			// Exit the history viewer
			return m, tea.Quit

		case KeyUp:
			if len(m.history) > 0 {
				m.historyCursor--
				if m.historyCursor < 0 {
					// Cyclic wrap to last item
					m.historyCursor = len(m.history) - 1
				}
			}
			return m, nil

		case KeyDown:
			if len(m.history) > 0 {
				m.historyCursor++
				if m.historyCursor >= len(m.history) {
					// Cyclic wrap to first item
					m.historyCursor = 0
				}
			}
			return m, nil

		case KeyEnter:
			// Re-execute the selected history entry
			if len(m.history) > 0 && m.historyCursor >= 0 && m.historyCursor < len(m.history) {
				m.selectedHistoryEntry = &m.history[m.historyCursor]
				m.reExecuteFromHistory = true
			}
			return m, tea.Quit
		}
	}

	return m, nil
}

// handleKeyPress processes keyboard input.
func (m Model) handleKeyPress(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// Handle filter input editing mode
	if m.activeFilterColumn >= 0 {
		switch msg.String() {
		case KeyEsc:
			// Exit filter input mode and remove the filter completely
			delete(m.columnFilters, m.activeFilterColumn)
			m.activeFilterColumn = -1
			return m, nil
		case KeyEnter:
			// Execute command with current selection
			return m.handleEnterKey()
		case KeyUp:
			// Allow navigation while filtering
			return m.handleVerticalMove(true), nil
		case KeyDown:
			// Allow navigation while filtering
			return m.handleVerticalMove(false), nil
		case KeyLeft:
			// Allow navigation while filtering
			return m.handleHorizontalMove(true)
		case KeyRight:
			// Allow navigation while filtering
			return m.handleHorizontalMove(false)
		default:
			// Delegate to the active filter's text input
			if filter, exists := m.columnFilters[m.activeFilterColumn]; exists {
				oldValue := filter.Value()
				var cmd tea.Cmd
				filter, cmd = filter.Update(msg)
				m.columnFilters[m.activeFilterColumn] = filter

				// If filter value changed, adjust selection if needed
				if filter.Value() != oldValue {
					m.adjustSelectionAfterFilter()
				}

				return m, cmd
			}
		}
	}

	// Normal navigation mode (always available)
	switch msg.String() {
	case KeyCtrlC, KeyQ:
		return m, tea.Quit
	case KeySlash:
		// Activate filter for current focused column
		columnID := m.focusedColumn
		if _, exists := m.columnFilters[columnID]; !exists {
			// Create new filter for this column
			ti := textinput.New()
			ti.Placeholder = "Filter..."
			ti.CharLimit = 50
			ti.Width = 20
			m.columnFilters[columnID] = ti
		}
		filter := m.columnFilters[columnID]
		filter.Focus()
		m.columnFilters[columnID] = filter
		m.activeFilterColumn = columnID
		return m, textinput.Blink
	case KeyEnter:
		return m.handleEnterKey()
	case KeyUp:
		return m.handleVerticalMove(true), nil
	case KeyDown:
		return m.handleVerticalMove(false), nil
	case KeyLeft:
		return m.handleHorizontalMove(true)
	case KeyRight:
		return m.handleHorizontalMove(false)
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
func (m Model) handleHorizontalMove(isLeft bool) (tea.Model, tea.Cmd) {
	// If we're editing a filter, blur it when moving to another column
	if m.activeFilterColumn >= 0 {
		if filter, exists := m.columnFilters[m.activeFilterColumn]; exists {
			filter.Blur()
			m.columnFilters[m.activeFilterColumn] = filter
		}
		m.activeFilterColumn = -1
	}

	if isLeft {
		m.moveToPreviousColumn()
	} else {
		m.moveToNextColumn()
	}

	// After moving to a new column, check if that column has a filter
	// If it does, automatically activate it for editing
	if filter, exists := m.columnFilters[m.focusedColumn]; exists {
		filter.Focus()
		m.columnFilters[m.focusedColumn] = filter
		m.activeFilterColumn = m.focusedColumn
		return m, textinput.Blink
	}

	return m, nil
}

// moveCommandSelection moves selection in commands column with page-based navigation.
func (m *Model) moveCommandSelection(isUp bool) {
	filteredCommands := m.getFilteredCommands()
	if len(filteredCommands) == 0 {
		return
	}

	// Initialize scrollOffsets map if it doesn't exist (for tests and edge cases)
	if m.scrollOffsets == nil {
		m.scrollOffsets = make(map[int]int)
	}

	// Check if filter is active
	hasFilter := false
	if filter, exists := m.columnFilters[0]; exists && filter.Value() != "" {
		hasFilter = true
	}

	maxVisibleItems := m.getMaxVisibleItems()
	totalPages := m.getTotalPages(len(filteredCommands))
	currentPage := m.getCurrentPage(0) // columnID = 0 for commands

	if !hasFilter {
		// No filter: page-based navigation with the original commands list
		if isUp {
			if m.selectedCommand > 0 {
				// Check if we're at the first item of current page
				pageStart := m.getPageStartIndex(currentPage)
				if m.selectedCommand == pageStart && currentPage > 1 {
					// Jump to last item of previous page
					prevPage := currentPage - 1
					prevPageStart := m.getPageStartIndex(prevPage)
					prevPageEnd := min(prevPageStart+maxVisibleItems-1, len(m.commands)-1)
					m.selectedCommand = prevPageEnd
					m.scrollOffsets[0] = prevPageStart
				} else {
					m.selectedCommand--
				}
			} else {
				// Wrap to bottom (last item of last page)
				m.selectedCommand = len(m.commands) - 1
				lastPage := m.getTotalPages(len(m.commands))
				m.scrollOffsets[0] = m.getPageStartIndex(lastPage)
			}
		} else {
			if m.selectedCommand < len(m.commands)-1 {
				// Check if we're at the last item of current page
				pageStart := m.getPageStartIndex(currentPage)
				pageEnd := min(pageStart+maxVisibleItems-1, len(m.commands)-1)
				if m.selectedCommand == pageEnd && currentPage < totalPages {
					// Jump to first item of next page
					nextPage := currentPage + 1
					nextPageStart := m.getPageStartIndex(nextPage)
					m.selectedCommand = nextPageStart
					m.scrollOffsets[0] = nextPageStart
				} else {
					m.selectedCommand++
				}
			} else {
				// Wrap to top (first item of first page)
				m.selectedCommand = 0
				m.scrollOffsets[0] = 0
			}
		}
		return
	}

	// Filter is active: page-based navigation within filtered list
	filteredIndex := findFilteredIndex(m.commands, filteredCommands, m.selectedCommand)
	if filteredIndex < 0 {
		// Current selection not in filtered list, select first filtered item
		m.selectedCommand = findOriginalIndex(m.commands, filteredCommands, 0)
		m.scrollOffsets[0] = 0
		return
	}

	// Move within filtered list with page-based navigation
	if isUp {
		if filteredIndex > 0 {
			// Check if we're at the first item of current page
			pageStart := m.getPageStartIndex(currentPage)
			if filteredIndex == pageStart && currentPage > 1 {
				// Jump to last item of previous page
				prevPage := currentPage - 1
				prevPageStart := m.getPageStartIndex(prevPage)
				prevPageEnd := min(prevPageStart+maxVisibleItems-1, len(filteredCommands)-1)
				filteredIndex = prevPageEnd
				m.scrollOffsets[0] = prevPageStart
			} else {
				filteredIndex--
			}
		} else {
			// Wrap to bottom
			filteredIndex = len(filteredCommands) - 1
			lastPage := m.getTotalPages(len(filteredCommands))
			m.scrollOffsets[0] = m.getPageStartIndex(lastPage)
		}
	} else {
		if filteredIndex < len(filteredCommands)-1 {
			// Check if we're at the last item of current page
			pageStart := m.getPageStartIndex(currentPage)
			pageEnd := min(pageStart+maxVisibleItems-1, len(filteredCommands)-1)
			if filteredIndex == pageEnd && currentPage < totalPages {
				// Jump to first item of next page
				nextPage := currentPage + 1
				nextPageStart := m.getPageStartIndex(nextPage)
				filteredIndex = nextPageStart
				m.scrollOffsets[0] = nextPageStart
			} else {
				filteredIndex++
			}
		} else {
			// Wrap to top
			filteredIndex = 0
			m.scrollOffsets[0] = 0
		}
	}

	// Map back to original index
	m.selectedCommand = findOriginalIndex(m.commands, filteredCommands, filteredIndex)
}

// moveNavigationSelection moves selection in navigation column with page-based navigation.
func (m *Model) moveNavigationSelection(isUp bool) {
	depth := m.getNavigationDepth()
	if depth < 0 {
		return
	}

	filteredItems := m.getFilteredNavigationItems(depth)
	if len(filteredItems) == 0 {
		return
	}

	// Initialize scrollOffsets map if it doesn't exist (for tests and edge cases)
	if m.scrollOffsets == nil {
		m.scrollOffsets = make(map[int]int)
	}

	originalItems := m.navState.Columns[depth]
	currentIndex := m.navState.SelectedIndices[depth]

	// Check if filter is active for this column
	columnID := depth + 1
	hasFilter := false
	if filter, exists := m.columnFilters[columnID]; exists && filter.Value() != "" {
		hasFilter = true
	}

	maxVisibleItems := m.getMaxVisibleItems()
	totalPages := m.getTotalPages(len(originalItems))
	currentPage := m.getCurrentPage(columnID)

	if !hasFilter {
		// No filter: page-based navigation with original items
		if isUp {
			if currentIndex > 0 {
				// Check if we're at the first item of current page
				pageStart := m.getPageStartIndex(currentPage)
				if currentIndex == pageStart && currentPage > 1 {
					// Jump to last item of previous page
					prevPage := currentPage - 1
					prevPageStart := m.getPageStartIndex(prevPage)
					prevPageEnd := min(prevPageStart+maxVisibleItems-1, len(originalItems)-1)
					m.navState.SelectedIndices[depth] = prevPageEnd
					m.scrollOffsets[columnID] = prevPageStart
				} else {
					m.navState.SelectedIndices[depth]--
				}
				m.navigator.PropagateSelection(m.navState)
			} else {
				// Wrap to bottom (last item of last page)
				m.navState.SelectedIndices[depth] = len(originalItems) - 1
				lastPage := m.getTotalPages(len(originalItems))
				m.scrollOffsets[columnID] = m.getPageStartIndex(lastPage)
				m.navigator.PropagateSelection(m.navState)
			}
		} else {
			if currentIndex < len(originalItems)-1 {
				// Check if we're at the last item of current page
				pageStart := m.getPageStartIndex(currentPage)
				pageEnd := min(pageStart+maxVisibleItems-1, len(originalItems)-1)
				if currentIndex == pageEnd && currentPage < totalPages {
					// Jump to first item of next page
					nextPage := currentPage + 1
					nextPageStart := m.getPageStartIndex(nextPage)
					m.navState.SelectedIndices[depth] = nextPageStart
					m.scrollOffsets[columnID] = nextPageStart
				} else {
					m.navState.SelectedIndices[depth]++
				}
				m.navigator.PropagateSelection(m.navState)
			} else {
				// Wrap to top (first item of first page)
				m.navState.SelectedIndices[depth] = 0
				m.scrollOffsets[columnID] = 0
				m.navigator.PropagateSelection(m.navState)
			}
		}
		return
	}

	// Filter is active: page-based navigation within filtered list
	filteredIndex := findFilteredIndex(originalItems, filteredItems, currentIndex)
	if filteredIndex < 0 {
		// Current selection not in filtered list, select first filtered item
		newOriginalIndex := findOriginalIndex(originalItems, filteredItems, 0)
		if newOriginalIndex >= 0 {
			m.navState.SelectedIndices[depth] = newOriginalIndex
			m.navigator.PropagateSelection(m.navState)
			m.scrollOffsets[columnID] = 0
		}
		return
	}

	totalPagesFiltered := m.getTotalPages(len(filteredItems))
	currentPageFiltered := m.getCurrentPage(columnID)

	// Move within filtered list with page-based navigation
	if isUp {
		if filteredIndex > 0 {
			// Check if we're at the first item of current page
			pageStart := m.getPageStartIndex(currentPageFiltered)
			if filteredIndex == pageStart && currentPageFiltered > 1 {
				// Jump to last item of previous page
				prevPage := currentPageFiltered - 1
				prevPageStart := m.getPageStartIndex(prevPage)
				prevPageEnd := min(prevPageStart+maxVisibleItems-1, len(filteredItems)-1)
				filteredIndex = prevPageEnd
				m.scrollOffsets[columnID] = prevPageStart
			} else {
				filteredIndex--
			}
		} else {
			// Wrap to bottom
			filteredIndex = len(filteredItems) - 1
			lastPage := m.getTotalPages(len(filteredItems))
			m.scrollOffsets[columnID] = m.getPageStartIndex(lastPage)
		}
	} else {
		if filteredIndex < len(filteredItems)-1 {
			// Check if we're at the last item of current page
			pageStart := m.getPageStartIndex(currentPageFiltered)
			pageEnd := min(pageStart+maxVisibleItems-1, len(filteredItems)-1)
			if filteredIndex == pageEnd && currentPageFiltered < totalPagesFiltered {
				// Jump to first item of next page
				nextPage := currentPageFiltered + 1
				nextPageStart := m.getPageStartIndex(nextPage)
				filteredIndex = nextPageStart
				m.scrollOffsets[columnID] = nextPageStart
			} else {
				filteredIndex++
			}
		} else {
			// Wrap to top
			filteredIndex = 0
			m.scrollOffsets[columnID] = 0
		}
	}

	// Map back to original index and update
	newOriginalIndex := findOriginalIndex(originalItems, filteredItems, filteredIndex)
	if newOriginalIndex >= 0 {
		m.navState.SelectedIndices[depth] = newOriginalIndex
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
		if maxVisibleDepth > m.maxNavigationColumns {
			m.navigationOffset = maxVisibleDepth - m.maxNavigationColumns
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
		// Window shows levels: navigationOffset, ..., navigationOffset+(maxNavigationColumns-1)
		// Focus is at column index (1 + depth), so depth = focusedColumn - 1
		if m.focusedColumn > 0 {
			depth := m.focusedColumn - 1 // Convert to 0-based depth
			if depth > m.navigationOffset+(m.maxNavigationColumns-1) {
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

// adjustSelectionAfterFilter adjusts the current selection to ensure it's within the filtered list.
func (m *Model) adjustSelectionAfterFilter() {
	if m.activeFilterColumn < 0 {
		return
	}

	if m.activeFilterColumn == 0 {
		// Commands column
		filteredCommands := m.getFilteredCommands()
		if len(filteredCommands) == 0 {
			return
		}

		// Check if current selection is in filtered list
		filteredIndex := findFilteredIndex(m.commands, filteredCommands, m.selectedCommand)
		if filteredIndex < 0 {
			// Current selection not visible, select first filtered item
			m.selectedCommand = findOriginalIndex(m.commands, filteredCommands, 0)
		}
	} else {
		// Navigation column
		depth := m.activeFilterColumn - 1
		if depth < 0 || depth >= len(m.navState.Columns) {
			return
		}

		filteredItems := m.getFilteredNavigationItems(depth)
		if len(filteredItems) == 0 {
			return
		}

		originalItems := m.navState.Columns[depth]
		currentIndex := m.navState.SelectedIndices[depth]

		// Check if current selection is in filtered list
		filteredIndex := findFilteredIndex(originalItems, filteredItems, currentIndex)
		if filteredIndex < 0 {
			// Current selection not visible, select first filtered item
			newIndex := findOriginalIndex(originalItems, filteredItems, 0)
			if newIndex >= 0 {
				m.navState.SelectedIndices[depth] = newIndex
				m.navigator.PropagateSelection(m.navState)
			}
		}
	}
}

// isCommandsColumnFocused returns true if the commands column is focused.
func (m Model) isCommandsColumnFocused() bool {
	return m.focusedColumn == 0
}

// getAvailableHeight calculates the available height for list items.
// Subtracts header, footer, breadcrumb bar, column title, and padding.
func (m Model) getAvailableHeight() int {
	// Total reserved space:
	// - HeaderHeight (1)
	// - Breadcrumb bar (1)
	// - Column title (1)
	// - Empty line after title (1)
	// - FooterHeight (1)
	// - ColumnPadding (4) - includes borders and internal padding
	reservedSpace := HeaderHeight + 1 + 1 + 1 + FooterHeight + ColumnPadding
	availableHeight := m.height - reservedSpace

	if availableHeight < 1 {
		return 1 // Minimum height to avoid division by zero
	}
	return availableHeight
}

// getMaxVisibleItems returns the maximum number of items that can be displayed
// in a column given the current terminal height.
// Reserves 1 line for pagination indicators to ensure consistent column heights.
func (m Model) getMaxVisibleItems() int {
	availableHeight := m.getAvailableHeight()

	// Reserve 1 line for pagination indicators to ensure all columns have same height.
	// Even if a column doesn't need pagination, this reserved space keeps heights consistent.
	reservedForPagination := 1
	maxItems := availableHeight - reservedForPagination

	if maxItems < 1 {
		return 1
	}

	return maxItems
}

// getTotalPages calculates the total number of pages for a list.
func (m Model) getTotalPages(totalItems int) int {
	maxVisibleItems := m.getMaxVisibleItems()
	if maxVisibleItems <= 0 {
		return 1
	}
	if totalItems <= maxVisibleItems {
		return 1
	}
	// Calculate pages (ceiling division)
	pages := (totalItems + maxVisibleItems - 1) / maxVisibleItems
	return pages
}

// getCurrentPage calculates the current page number (1-indexed) based on scroll offset.
func (m Model) getCurrentPage(columnID int) int {
	maxVisibleItems := m.getMaxVisibleItems()
	if maxVisibleItems <= 0 {
		return 1
	}
	scrollOffset := m.scrollOffsets[columnID]
	currentPage := (scrollOffset / maxVisibleItems) + 1
	return currentPage
}

// getPageStartIndex returns the start index for a given page number (1-indexed).
func (m Model) getPageStartIndex(pageNumber int) int {
	maxVisibleItems := m.getMaxVisibleItems()
	if pageNumber <= 1 {
		return 0
	}
	return (pageNumber - 1) * maxVisibleItems
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
// Delegates to Navigator for path construction business logic.
func (m Model) getCurrentNavigationPath() string {
	depth := m.getNavigationDepth()
	return m.navigator.GetNavigationPath(m.navState, depth)
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

// filterItems filters a list of items based on the filter text (case-insensitive).
func filterItems(items []string, filterText string) []string {
	if filterText == "" {
		return items
	}

	filtered := make([]string, 0)
	filterLower := strings.ToLower(filterText)

	for _, item := range items {
		if strings.Contains(strings.ToLower(item), filterLower) {
			filtered = append(filtered, item)
		}
	}

	return filtered
}

// getFilteredCommands returns the commands list with active filter applied.
func (m *Model) getFilteredCommands() []string {
	if filter, exists := m.columnFilters[0]; exists {
		filterValue := filter.Value()
		if filterValue != "" {
			return filterItems(m.commands, filterValue)
		}
	}
	return m.commands
}

// getFilteredNavigationItems returns the navigation items for a depth with active filter applied.
func (m *Model) getFilteredNavigationItems(depth int) []string {
	if depth < 0 || depth >= len(m.navState.Columns) {
		return []string{}
	}

	items := m.navState.Columns[depth]
	columnID := depth + 1

	if filter, exists := m.columnFilters[columnID]; exists {
		filterValue := filter.Value()
		if filterValue != "" {
			return filterItems(items, filterValue)
		}
	}
	return items
}

// findOriginalIndex maps a filtered index back to the original unfiltered index.
func findOriginalIndex(originalItems []string, filteredItems []string, filteredIndex int) int {
	if filteredIndex < 0 || filteredIndex >= len(filteredItems) {
		return -1
	}

	targetItem := filteredItems[filteredIndex]
	for i, item := range originalItems {
		if item == targetItem {
			return i
		}
	}
	return -1
}

// findFilteredIndex maps an original index to the filtered index.
func findFilteredIndex(originalItems []string, filteredItems []string, originalIndex int) int {
	if originalIndex < 0 || originalIndex >= len(originalItems) {
		return -1
	}

	targetItem := originalItems[originalIndex]
	for i, item := range filteredItems {
		if item == targetItem {
			return i
		}
	}
	return -1
}

// ShouldReExecuteFromHistory returns true if a history entry was selected for re-execution.
func (m Model) ShouldReExecuteFromHistory() bool {
	return m.reExecuteFromHistory
}

// GetSelectedHistoryEntry returns the history entry selected for re-execution.
// Returns nil if no entry was selected.
func (m Model) GetSelectedHistoryEntry() *history.ExecutionLogEntry {
	return m.selectedHistoryEntry
}
