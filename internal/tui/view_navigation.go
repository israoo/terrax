package tui

import (
	"fmt"

	"github.com/charmbracelet/lipgloss"
)

// renderColumnsWithArrows renders all visible columns with overflow indicators.
func (r *Renderer) renderColumnsWithArrows() []string {
	columns := make([]string, 0)

	// Render commands column (always visible)
	commandsView := r.renderCommandsColumn()
	styledCommands := r.styleColumn(commandsView, r.model.isCommandsColumnFocused())
	columns = append(columns, styledCommands)

	// Left overflow indicator
	if r.model.hasLeftOverflow() {
		leftArrow := r.renderArrowIndicator("«")
		columns = append(columns, leftArrow)
	}

	// Render navigation columns in sliding window (configurable max visible)
	maxDepth := r.model.navigator.GetMaxDepth()
	startDepth := r.model.navigationOffset
	endDepth := min(startDepth+r.model.maxNavigationColumns, maxDepth)

	for depth := startDepth; depth < endDepth; depth++ {
		// Skip empty columns
		if len(r.model.navState.Columns[depth]) == 0 {
			break
		}

		navView := r.renderNavigationColumn(depth)
		isFocused := r.model.focusedColumn == depth+1
		styledNav := r.styleColumn(navView, isFocused)
		columns = append(columns, styledNav)
	}

	// Right overflow indicator
	if r.model.hasRightOverflow() {
		rightArrow := r.renderArrowIndicator("»")
		columns = append(columns, rightArrow)
	}

	return columns
}

// renderCommandsColumn renders the commands column content.
func (r *Renderer) renderCommandsColumn() string {
	parts := []string{}

	// Show filter input OR title (not both)
	// Show filter if it exists (even if empty, user might be typing)
	if filter, exists := r.model.columnFilters[0]; exists {
		// Show filter input instead of title
		filterView := filter.View()
		filterStyle := lipgloss.NewStyle().
			Foreground(lipgloss.Color("#00D9FF")).
			Padding(0, 1)
		parts = append(parts, filterStyle.Render(filterView))
	} else {
		// Show normal title
		title := titleStyle.Render("⚡" + CommandsTitle)
		parts = append(parts, title)
	}

	parts = append(parts, "") // Empty line after title/filter

	content := r.buildCommandList()
	parts = append(parts, content)

	return lipgloss.JoinVertical(lipgloss.Left, parts...)
}

// buildCommandList builds the list of commands with selection indicator.
func (r *Renderer) buildCommandList() string {
	originalCommands := r.model.commands
	commands := r.model.commands

	// Apply filter if this column (0) has an active filter
	if filter, exists := r.model.columnFilters[0]; exists {
		filterValue := filter.Value()
		if filterValue != "" {
			commands = filterItems(commands, filterValue)
		}
	}

	// Map original selected index to filtered index
	var selectedFilteredIndex int
	if len(commands) < len(originalCommands) {
		// Filter is active, need to map
		selectedFilteredIndex = findFilteredIndex(originalCommands, commands, r.model.selectedCommand)
	} else {
		// No filter, use original index
		selectedFilteredIndex = r.model.selectedCommand
	}

	// Apply scrolling window
	maxVisibleItems := r.model.getMaxVisibleItems()
	scrollOffset := r.model.scrollOffsets[0] // columnID = 0 for commands

	// Calculate visible range
	startIdx, endIdx := calculatePaginatedRange(scrollOffset, maxVisibleItems, len(commands))

	// Render items with pagination
	maxTextWidth := r.getMaxItemTextWidth()
	totalPages := r.model.getTotalPages(len(commands))
	currentPage := r.model.getCurrentPage(0) // columnID = 0 for commands

	return renderItemList(
		commands,
		startIdx, endIdx,
		selectedFilteredIndex,
		maxVisibleItems,
		maxTextWidth,
		totalPages, currentPage,
	)
}

// renderNavigationColumn renders a navigation column at the given depth.
func (r *Renderer) renderNavigationColumn(depth int) string {
	parts := []string{}

	// Show filter input OR title (not both)
	// Navigation columns: depth 0 -> columnID 1, depth 1 -> columnID 2, etc.
	columnID := depth + 1
	if filter, exists := r.model.columnFilters[columnID]; exists {
		// Show filter input instead of title
		filterView := filter.View()
		filterStyle := lipgloss.NewStyle().
			Foreground(lipgloss.Color("#00D9FF")).
			Padding(0, 1)
		parts = append(parts, filterStyle.Render(filterView))
	} else {
		// Show normal title
		title := titleStyle.Render("📦 " + r.getLevelTitle(depth))
		parts = append(parts, title)
	}

	parts = append(parts, "") // Empty line after title/filter

	content := r.buildNavigationList(depth)
	parts = append(parts, content)

	return lipgloss.JoinVertical(lipgloss.Left, parts...)
}

// buildNavigationList builds the list of items for a navigation column.
func (r *Renderer) buildNavigationList(depth int) string {
	originalItems := r.model.navState.Columns[depth]
	items := r.model.navState.Columns[depth]
	selectedIndex := r.model.navState.SelectedIndices[depth]

	// Apply filter if this navigation column has an active filter
	// Navigation columns are indexed as: depth 0 -> columnID 1, depth 1 -> columnID 2, etc.
	columnID := depth + 1
	if filter, exists := r.model.columnFilters[columnID]; exists {
		filterValue := filter.Value()
		if filterValue != "" {
			items = filterItems(items, filterValue)
		}
	}

	// Map original selected index to filtered index
	var selectedFilteredIndex int
	if len(items) < len(originalItems) {
		// Filter is active, need to map
		selectedFilteredIndex = findFilteredIndex(originalItems, items, selectedIndex)
	} else {
		// No filter, use original index
		selectedFilteredIndex = selectedIndex
	}

	// Apply scrolling window
	maxVisibleItems := r.model.getMaxVisibleItems()
	scrollOffset := r.model.scrollOffsets[columnID]

	// Calculate visible range
	startIdx, endIdx := calculatePaginatedRange(scrollOffset, maxVisibleItems, len(items))

	// Render items with pagination
	maxTextWidth := r.getMaxItemTextWidth()
	totalPages := r.model.getTotalPages(len(items))
	currentPage := r.model.getCurrentPage(columnID)

	return renderItemList(
		items,
		startIdx, endIdx,
		selectedFilteredIndex,
		maxVisibleItems,
		maxTextWidth,
		totalPages, currentPage,
	)
}

// renderItemList renders a list of items with pagination.
func renderItemList(
	items []string,
	startIdx, endIdx int,
	selectedFilteredIndex int,
	maxVisibleItems int,
	maxTextWidth int,
	totalPages, currentPage int,
) string {
	var content string
	itemsRendered := 0

	// Render visible items
	for i := startIdx; i < endIdx; i++ {
		cursor := " "
		style := itemStyle

		if i == selectedFilteredIndex {
			cursor = "►"
			style = selectedItemStyle
		}

		// Truncate text to fit within column width
		displayText := truncateText(items[i], maxTextWidth)
		content += fmt.Sprintf("%s %s\n", cursor, style.Render(displayText))
		itemsRendered++
	}

	// Add empty lines to fill remaining space up to maxVisibleItems
	// This ensures all columns have the same height
	for itemsRendered < maxVisibleItems {
		content += "\n"
		itemsRendered++
	}

	// Add page indicators (without extra newline before or after)
	pageIndicators := renderPageIndicators(currentPage, totalPages)
	if pageIndicators != "" {
		content += pageIndicators
	}

	return content
}

// styleColumn applies styling to a column based on focus state.
func (r *Renderer) styleColumn(content string, isFocused bool) string {
	columnWidth := r.layout.GetColumnWidth()

	// Adjust width for focused columns to account for border width.
	// Lipgloss adds borders outside the content width, so we need to subtract
	// the border width (2 chars total: 1 left + 1 right) to maintain consistent
	// total rendered width across all columns.
	if isFocused {
		columnWidth -= ColumnBorderWidth // Subtract border width
	}

	// Don't apply Height() here - let content determine its own height naturally.
	// All columns will have the same content height because:
	// 1. All have title (1 line) + empty line (1 line)
	// 2. All show same number of items (controlled by getMaxVisibleItems)
	// 3. All reserve space for pagination indicators (1 line)
	// This ensures consistent column heights without forcing artificial padding.
	return columnStyle(isFocused).
		Width(columnWidth).
		Render(content)
}

// getLevelTitle returns the title for a navigation level.
func (r *Renderer) getLevelTitle(depth int) string {
	return fmt.Sprintf("Level %d", depth+1)
}

// columnStyle returns the appropriate style for a column based on focus.
func columnStyle(focused bool) lipgloss.Style {
	if focused {
		// Focused column: border with normal padding
		return lipgloss.NewStyle().
			Padding(1, 2).
			Margin(0, 1).
			Border(focusedBorder).
			BorderForeground(primaryColor)
	}

	// Unfocused column: extra padding to compensate for missing border
	// Border takes 1 char on each side, so add 1 to padding (vertical and horizontal)
	return lipgloss.NewStyle().
		Padding(2, 3). // +1 on each side to match border width
		Margin(0, 1)
}

// getMaxItemTextWidth calculates the maximum width available for item text.
// Takes into account cursor (2 chars: "► "), style padding, and column width.
func (r *Renderer) getMaxItemTextWidth() int {
	columnWidth := r.layout.GetColumnWidth()

	// Account for:
	// - Cursor and space: "► " = 2 chars
	// - Item style padding: 0,1 = 2 chars total (left + right)
	// - Column padding from columnStyle: focused=1,2 unfocused=2,3
	// - Border for focused column: 2 chars
	// Use the larger padding case (unfocused: 2,3 = 6 total)
	reservedSpace := CursorWidth + ItemStylePadding + ColumnStylePadding

	maxWidth := columnWidth - reservedSpace

	// Ensure minimum width
	if maxWidth < MinItemTextWidth {
		maxWidth = MinItemTextWidth
	}

	return maxWidth
}

// calculatePaginatedRange calculates the start and end indices for paginated items.
func calculatePaginatedRange(scrollOffset, maxVisibleItems, totalItems int) (startIdx, endIdx int) {
	startIdx = scrollOffset
	endIdx = scrollOffset + maxVisibleItems

	if endIdx > totalItems {
		endIdx = totalItems
	}
	if startIdx >= totalItems {
		startIdx = 0
	}

	return startIdx, endIdx
}
