package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/israoo/terrax/internal/history"
)

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
		"Showing %d-%d of %d entries | Use ↑/↓ to navigate | Press Enter to re-execute | Press 'q' or 'esc' to exit",
		startIdx+1,
		endIdx,
		len(m.history),
	)
	return footerStyle.Render(footerText)
}
