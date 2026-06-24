package tui

import (
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/israoo/terrax/internal/stack"
)

// Update handles messages and updates state (BubbleTea interface).
// handleNavigationUpdate handles messages and updates state for navigation mode.
func (m Model) handleNavigationUpdate(msg tea.Msg) (tea.Model, tea.Cmd) {
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
	if m.navigator != nil {
		m.columnWidth = m.calculateColumnWidth()
	}
	m.ready = true
	return m
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
		switch msg.Type {
		case tea.KeyEsc:
			return m, tea.Quit

		case tea.KeyRunes:
			if msg.String() == KeyQ {
				return m, tea.Quit
			}

		case tea.KeyUp:
			if len(m.history) > 0 {
				m.historyCursor--
				if m.historyCursor < 0 {
					// Cyclic wrap to last item
					m.historyCursor = len(m.history) - 1
				}
			}
			return m, nil

		case tea.KeyDown:
			if len(m.history) > 0 {
				m.historyCursor++
				if m.historyCursor >= len(m.history) {
					// Cyclic wrap to first item
					m.historyCursor = 0
				}
			}
			return m, nil

		case tea.KeyPgUp, tea.KeyPgDown:
			if len(m.history) > 0 {
				// Calculate content height matches view_history.go
				// contentHeight := m.height - HeaderHeight - FooterHeight - 6
				const historyFrameOverhead = 8 // 1+1+6
				visibleHeight := m.height - historyFrameOverhead
				if visibleHeight < 1 {
					visibleHeight = 1
				}

				if msg.Type == tea.KeyPgDown {
					m.historyCursor += visibleHeight
					if m.historyCursor >= len(m.history) {
						m.historyCursor = len(m.history) - 1
					}
				} else {
					m.historyCursor -= visibleHeight
					if m.historyCursor < 0 {
						m.historyCursor = 0
					}
				}
			}
			return m, nil

		case tea.KeyEnter:
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
	// Normal navigation mode (always available)
	switch msg.Type {
	case tea.KeyCtrlC, tea.KeyEsc:
		return m, tea.Quit

	case tea.KeyRunes:
		if msg.String() == KeyQ {
			return m, tea.Quit
		}
		if msg.String() == KeySlash {
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
		}

	case tea.KeyEnter:
		return m.handleEnterKey()
	case tea.KeyUp:
		return m.handleVerticalMove(true), nil
	case tea.KeyDown:
		return m.handleVerticalMove(false), nil
	case tea.KeyLeft:
		return m.handleHorizontalMove(true)
	case tea.KeyRight:
		return m.handleHorizontalMove(false)
	case tea.KeyPgUp:
		return m.handlePageMove(true), nil
	case tea.KeyPgDown:
		return m.handlePageMove(false), nil
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

// handlePageMove processes page up/down navigation.
func (m Model) handlePageMove(isUp bool) Model {
	if m.isCommandsColumnFocused() {
		m.moveCommandSelectionPage(isUp)
	} else {
		m.moveNavigationSelectionPage(isUp)
	}
	return m
}

// moveCommandSelectionPage moves selection in commands column by a full page.
func (m *Model) moveCommandSelectionPage(isUp bool) {
	filteredCommands := m.getFilteredCommands()
	if len(filteredCommands) == 0 {
		return
	}

	if m.scrollOffsets == nil {
		m.scrollOffsets = make(map[int]int)
	}

	maxVisibleItems := m.getMaxVisibleItems()

	// Check if filter is active
	hasFilter := false
	if filter, exists := m.columnFilters[0]; exists && filter.Value() != "" {
		hasFilter = true
	}

	var currentIdx, totalItems int
	if hasFilter {
		currentIdx = findFilteredIndex(m.commands, filteredCommands, m.selectedCommand)
		totalItems = len(filteredCommands)
	} else {
		currentIdx = m.selectedCommand
		totalItems = len(m.commands)
	}

	if currentIdx < 0 && hasFilter {
		currentIdx = 0
	}

	// Calculate target index
	var targetIdx int
	if isUp {
		targetIdx = currentIdx - maxVisibleItems
		if targetIdx < 0 {
			targetIdx = 0
		}
	} else {
		targetIdx = currentIdx + maxVisibleItems
		if targetIdx >= totalItems {
			targetIdx = totalItems - 1
		}
	}

	// Update selection and scroll offset
	if hasFilter {
		m.selectedCommand = findOriginalIndex(m.commands, filteredCommands, targetIdx)
		// Update scroll offset to ensure visibility
		pageIndex := targetIdx / maxVisibleItems
		m.scrollOffsets[0] = pageIndex * maxVisibleItems
	} else {
		m.selectedCommand = targetIdx
		// Update scroll offset
		pageIndex := targetIdx / maxVisibleItems
		m.scrollOffsets[0] = pageIndex * maxVisibleItems
	}
}

// moveNavigationSelectionPage moves selection in navigation column by a full page.
func (m *Model) moveNavigationSelectionPage(isUp bool) {
	depth := m.getNavigationDepth()
	if depth < 0 {
		return
	}

	filteredItems := m.getFilteredNavigationItems(depth)
	if len(filteredItems) == 0 {
		return
	}

	if m.scrollOffsets == nil {
		m.scrollOffsets = make(map[int]int)
	}

	originalItems := m.navState.Columns[depth]
	currentIdxOriginal := m.navState.SelectedIndices[depth]
	columnID := depth + 1

	maxVisibleItems := m.getMaxVisibleItems()

	// Check filter
	hasFilter := false
	if filter, exists := m.columnFilters[columnID]; exists && filter.Value() != "" {
		hasFilter = true
	}

	var currentIdx, totalItems int
	if hasFilter {
		currentIdx = findFilteredIndex(originalItems, filteredItems, currentIdxOriginal)
		totalItems = len(filteredItems)
	} else {
		currentIdx = currentIdxOriginal
		totalItems = len(originalItems)
	}

	if currentIdx < 0 && hasFilter {
		currentIdx = 0
	}

	// Calculate target index
	var targetIdx int
	if isUp {
		targetIdx = currentIdx - maxVisibleItems
		if targetIdx < 0 {
			targetIdx = 0
		}
	} else {
		targetIdx = currentIdx + maxVisibleItems
		if targetIdx >= totalItems {
			targetIdx = totalItems - 1
		}
	}

	// Update selection and scroll offset
	if hasFilter {
		newOriginalIndex := findOriginalIndex(originalItems, filteredItems, targetIdx)
		if newOriginalIndex >= 0 {
			m.navState.SelectedIndices[depth] = newOriginalIndex
			m.navigator.PropagateSelection(m.navState)
			pageIndex := targetIdx / maxVisibleItems
			m.scrollOffsets[columnID] = pageIndex * maxVisibleItems
		}
	} else {
		m.navState.SelectedIndices[depth] = targetIdx
		m.navigator.PropagateSelection(m.navState)
		pageIndex := targetIdx / maxVisibleItems
		m.scrollOffsets[columnID] = pageIndex * maxVisibleItems
	}
}
