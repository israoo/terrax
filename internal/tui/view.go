package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/israoo/terrax/internal/history"
)

// Styles using lipgloss
var (
	// Colors
	primaryColor   = lipgloss.Color("#7D56F4")
	secondaryColor = lipgloss.Color("#00D9FF")
	accentColor    = lipgloss.Color("#FF6B9D")
	textColor      = lipgloss.Color("#FFFFFF")
	dimColor       = lipgloss.Color("#888888")

	// Column styles
	focusedBorder = lipgloss.RoundedBorder()
	normalBorder  = lipgloss.NormalBorder()

	// Header style
	headerStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(textColor).
			Background(primaryColor).
			Padding(0, 1).
			Align(lipgloss.Center)

	// Footer style
	footerStyle = lipgloss.NewStyle().
			Foreground(dimColor).
			Padding(0, 1).
			Italic(true)

	// Column title style
	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(secondaryColor).
			Padding(0, 1)

	// Normal item style
	itemStyle = lipgloss.NewStyle().
			Foreground(textColor).
			Padding(0, 1)

	// Selected item style
	selectedItemStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(accentColor).
				Padding(0, 1)

	// Arrow indicator style
	arrowStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(secondaryColor).
			Padding(0, 1)

	// Breadcrumb bar style (prominent top bar below header)
	breadcrumbBarStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(textColor).
				Background(lipgloss.Color("#2E2E2E")).
				Padding(0, 2).
				Margin(0, 0)
)

// LayoutCalculator handles all layout dimension calculations.
type LayoutCalculator struct {
	width       int
	height      int
	columnWidth int
}

// NewLayoutCalculator creates a new layout calculator.
func NewLayoutCalculator(width, height, columnWidth int) *LayoutCalculator {
	return &LayoutCalculator{
		width:       width,
		height:      height,
		columnWidth: columnWidth,
	}
}

// GetContentHeight returns the available height for column content.
func (lc *LayoutCalculator) GetContentHeight() int {
	return lc.height - HeaderHeight - FooterHeight - ColumnPadding
}

// GetColumnWidth returns the width for each column.
func (lc *LayoutCalculator) GetColumnWidth() int {
	return lc.columnWidth
}

// View renders the complete UI (BubbleTea interface).
func (m Model) View() string {
	// Delegate to history view if in StateHistory
	if m.state == StateHistory {
		return m.renderHistoryView()
	}

	// Default: StateNavigation view
	if !m.ready || m.width == 0 {
		return Initializing
	}

	if m.navigator.GetMaxDepth() == 0 || m.columnWidth == 0 {
		return ScanningStacks
	}

	layout := NewLayoutCalculator(m.width, m.height, m.columnWidth)
	renderer := NewRenderer(m, layout)

	return renderer.Render()
}

// Renderer handles the actual rendering of UI components.
type Renderer struct {
	model  Model
	layout *LayoutCalculator
}

// NewRenderer creates a new renderer instance.
func NewRenderer(model Model, layout *LayoutCalculator) *Renderer {
	return &Renderer{
		model:  model,
		layout: layout,
	}
}

// Render builds the complete UI view.
func (r *Renderer) Render() string {
	columns := r.renderColumnsWithArrows()
	content := lipgloss.JoinHorizontal(lipgloss.Top, columns...)

	header := r.renderHeader()
	breadcrumbBar := r.renderBreadcrumbBar()
	footer := r.renderFooter()

	return lipgloss.JoinVertical(lipgloss.Left, header, breadcrumbBar, content, footer)
}

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

// renderArrowIndicator renders an arrow indicator for overflow.
func (r *Renderer) renderArrowIndicator(arrow string) string {
	content := arrowStyle.Render(arrow)
	return lipgloss.NewStyle().
		Height(r.layout.GetContentHeight()).
		Padding(0, 0).
		AlignVertical(lipgloss.Center).
		Render(content)
}

// min returns the minimum of two integers.
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
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

	var content string
	for i, cmd := range commands {
		cursor := " "
		style := itemStyle

		if i == selectedFilteredIndex {
			cursor = "►"
			style = selectedItemStyle
		}

		content += fmt.Sprintf("%s %s\n", cursor, style.Render(cmd))
	}
	return content
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

	var content string
	for i, item := range items {
		cursor := " "
		style := itemStyle

		if i == selectedFilteredIndex {
			cursor = "►"
			style = selectedItemStyle
		}

		content += fmt.Sprintf("%s %s\n", cursor, style.Render(item))
	}
	return content
}

// styleColumn applies styling to a column based on focus state.
func (r *Renderer) styleColumn(content string, isFocused bool) string {
	return columnStyle(isFocused).
		Width(r.layout.GetColumnWidth()).
		Height(r.layout.GetContentHeight()).
		Render(content)
}

// renderHeader renders the header bar.
func (r *Renderer) renderHeader() string {
	return headerStyle.Width(r.model.width).Render("🌍 " + AppTitle)
}

// renderBreadcrumbBar renders the navigation context bar below the header.
func (r *Renderer) renderBreadcrumbBar() string {
	navPath := r.model.getCurrentNavigationPath()

	// Format breadcrumb - always show the full path
	content := fmt.Sprintf("📁 %s", navPath)

	return breadcrumbBarStyle.Width(r.model.width).Render(content)
}

// renderFooter renders the footer with help text only.
func (r *Renderer) renderFooter() string {
	return footerStyle.Render(HelpText)
}

// getLevelTitle returns the title for a navigation level.
func (r *Renderer) getLevelTitle(depth int) string {
	return fmt.Sprintf("Level %d", depth+1)
}

// columnStyle returns the appropriate style for a column based on focus.
func columnStyle(focused bool) lipgloss.Style {
	style := lipgloss.NewStyle().
		Padding(1, 2).
		Margin(0, 1)

	if focused {
		return style.
			Border(focusedBorder).
			BorderForeground(primaryColor)
	}

	return style.
		Border(normalBorder).
		BorderForeground(dimColor)
}

// historyTableStyles holds all the lipgloss styles for the history table
type historyTableStyles struct {
	headerRow   lipgloss.Style
	cursor      lipgloss.Style
	normalRow   lipgloss.Style
	successIcon lipgloss.Style
	errorIcon   lipgloss.Style
}

// newHistoryTableStyles creates the styles for the history table
func newHistoryTableStyles() historyTableStyles {
	return historyTableStyles{
		headerRow: lipgloss.NewStyle().
			Bold(true).
			Foreground(secondaryColor),
		cursor: lipgloss.NewStyle().
			Bold(true).
			Foreground(accentColor).
			Background(lipgloss.Color("#3A3A3A")),
		normalRow: lipgloss.NewStyle().
			Foreground(textColor),
		successIcon: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#00FF00")).
			Bold(true),
		errorIcon: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FF0000")).
			Bold(true),
	}
}

// historyTableColumns defines the column widths for the history table
type historyTableColumns struct {
	id        int
	timestamp int
	command   int
	stackPath int
	exitCode  int
	duration  int
	cursor    int
}

// newHistoryTableColumns creates the column definitions with dynamic stackPath width
func newHistoryTableColumns(terminalWidth int) historyTableColumns {
	// Fixed widths for all columns except stackPath
	id := 4
	timestamp := 19
	command := 8
	exitCode := 9
	duration := 10
	cursor := 2  // "▶ " prefix
	spaces := 10 // Spacing between columns (2 spaces * 5 separators)

	// Calculate remaining width for stackPath
	fixedWidths := id + timestamp + command + exitCode + duration + cursor + spaces
	stackPath := terminalWidth - fixedWidths

	// Ensure minimum width for readability
	if stackPath < 20 {
		stackPath = 20
	}

	return historyTableColumns{
		id:        id,
		timestamp: timestamp,
		command:   command,
		stackPath: stackPath,
		exitCode:  exitCode,
		duration:  duration,
		cursor:    cursor,
	}
}

// formatExitCode formats the exit code without applying lipgloss styles
// to avoid breaking the row's background when the cursor style is applied
func formatExitCode(exitCode int, styles historyTableStyles, width int) string {
	var icon string
	if exitCode == 0 {
		icon = "✓"
	} else {
		icon = "✗"
	}

	display := fmt.Sprintf("%s %d", icon, exitCode)

	// Calculate padding: icon (1) + space (1) + number length
	visualWidth := 2 + len(fmt.Sprintf("%d", exitCode))
	paddingNeeded := width - visualWidth
	if paddingNeeded > 0 {
		display += strings.Repeat(" ", paddingNeeded)
	}

	return display
}

// calculateVisibleRange calculates the start and end indices for the visible window
func calculateVisibleRange(totalItems, cursorPos, contentHeight int) (start, end int) {
	start = 0
	end = totalItems

	if totalItems <= contentHeight {
		return start, end
	}

	// Center window around cursor
	start = cursorPos - contentHeight/2
	if start < 0 {
		start = 0
	}

	end = start + contentHeight
	if end > totalItems {
		end = totalItems
		start = end - contentHeight
		if start < 0 {
			start = 0
		}
	}

	return start, end
}

// buildHistoryTableHeader builds the table header row
func buildHistoryTableHeader(cols historyTableColumns, style lipgloss.Style) string {
	return style.Render(
		fmt.Sprintf(
			"  %-*s  %-*s  %-*s  %-*s  %-*s  %s",
			cols.id, "#",
			cols.timestamp, "Timestamp",
			cols.command, "Command",
			cols.stackPath, "Stack Path",
			cols.exitCode, "Exit Code",
			"Duration",
		),
	)
}

// buildHistoryTableRow builds a single data row for the history table
// displayID is the sequential ID to show (1, 2, 3...) instead of the actual entry ID
func buildHistoryTableRow(entry history.ExecutionLogEntry, displayID int, cols historyTableColumns, styles historyTableStyles) string {
	exitCodeStr := formatExitCode(entry.ExitCode, styles, cols.exitCode)
	timestampStr := entry.Timestamp.Format("2006-01-02 15:04:05")
	durationStr := fmt.Sprintf("%.2fs", entry.DurationS)

	// Truncate stack path if it exceeds the column width
	// Show the end of the path (most relevant) instead of the beginning
	stackPathDisplay := entry.StackPath
	if len(stackPathDisplay) > cols.stackPath {
		if cols.stackPath > 3 {
			// Take the last (cols.stackPath - 3) characters and prepend "..."
			stackPathDisplay = "..." + stackPathDisplay[len(stackPathDisplay)-(cols.stackPath-3):]
		} else {
			// If width is too small, just take the last characters
			stackPathDisplay = stackPathDisplay[len(stackPathDisplay)-cols.stackPath:]
		}
	}

	return fmt.Sprintf(
		"%-*d  %-*s  %-*s  %-*s  %s  %s",
		cols.id, displayID,
		cols.timestamp, timestampStr,
		cols.command, entry.Command,
		cols.stackPath, stackPathDisplay,
		exitCodeStr,
		durationStr,
	)
}

// renderHistoryView renders the history viewing interface as a formatted table.
func (m Model) renderHistoryView() string {
	if !m.ready || m.width == 0 {
		return Initializing
	}

	header := headerStyle.Width(m.width).Render("📜 Execution History")

	if len(m.history) == 0 {
		return m.renderEmptyHistory(header)
	}

	styles := newHistoryTableStyles()
	cols := newHistoryTableColumns(m.width)

	tableHeader := buildHistoryTableHeader(cols, styles.headerRow)
	separator := lipgloss.NewStyle().Foreground(dimColor).Render(strings.Repeat("─", m.width))

	contentHeight := m.height - HeaderHeight - FooterHeight - 6
	startIdx, endIdx := calculateVisibleRange(len(m.history), m.historyCursor, contentHeight)

	rows := m.buildHistoryTableRows(startIdx, endIdx, cols, styles)
	tableContent := lipgloss.JoinVertical(lipgloss.Left, rows...)

	footer := m.buildHistoryFooter(startIdx, endIdx)

	return lipgloss.JoinVertical(
		lipgloss.Left,
		header,
		"",
		tableHeader,
		separator,
		tableContent,
		"",
		footer,
	)
}

// renderEmptyHistory renders the view when there's no history
func (m Model) renderEmptyHistory(header string) string {
	emptyMsg := lipgloss.NewStyle().
		Foreground(dimColor).
		Padding(2, 4).
		Render("No execution history found.\nExecute commands through TerraX to build history.")

	footer := footerStyle.Render("Press 'q' or 'esc' to exit")

	return lipgloss.JoinVertical(
		lipgloss.Left,
		header,
		"",
		emptyMsg,
		"",
		footer,
	)
}

// buildHistoryTableRows builds all visible rows for the history table
func (m Model) buildHistoryTableRows(startIdx, endIdx int, cols historyTableColumns, styles historyTableStyles) []string {
	rows := make([]string, 0, endIdx-startIdx)

	for i := startIdx; i < endIdx; i++ {
		// Display sequential ID starting from 1
		displayID := i + 1
		row := buildHistoryTableRow(m.history[i], displayID, cols, styles)

		if i == m.historyCursor {
			// Set width to ensure the background extends to the terminal edge
			row = styles.cursor.Width(m.width).Render("▶ " + row)
		} else {
			row = styles.normalRow.Width(m.width).Render("  " + row)
		}

		rows = append(rows, row)
	}

	return rows
}

// buildHistoryFooter builds the footer with navigation info
func (m Model) buildHistoryFooter(startIdx, endIdx int) string {
	footerText := fmt.Sprintf(
		"Showing %d-%d of %d entries | Use ↑/↓ to navigate | Press 'q' or 'esc' to exit",
		startIdx+1,
		endIdx,
		len(m.history),
	)
	return footerStyle.Render(footerText)
}
