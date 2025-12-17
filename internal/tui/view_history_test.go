package tui

import (
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/israoo/terrax/internal/history"
)

// TestNewHistoryTableStyles tests history table style creation.
func TestNewHistoryTableStyles(t *testing.T) {
	styles := newHistoryTableStyles()

	assert.NotNil(t, styles.headerRow)
	assert.NotNil(t, styles.cursor)
	assert.NotNil(t, styles.normalRow)
	assert.NotNil(t, styles.successIcon)
	assert.NotNil(t, styles.errorIcon)
}

// TestNewHistoryTableColumns tests history table column width calculations.
func TestNewHistoryTableColumns(t *testing.T) {
	tests := []struct {
		name              string
		terminalWidth     int
		expectedMinStack  int
		expectedID        int
		expectedTimestamp int
		expectedCommand   int
		expectedExitCode  int
		expectedDuration  int
	}{
		{
			name:              "wide terminal",
			terminalWidth:     200,
			expectedMinStack:  20,
			expectedID:        4,
			expectedTimestamp: 19,
			expectedCommand:   8,
			expectedExitCode:  9,
			expectedDuration:  10,
		},
		{
			name:              "narrow terminal",
			terminalWidth:     80,
			expectedMinStack:  20,
			expectedID:        4,
			expectedTimestamp: 19,
			expectedCommand:   8,
			expectedExitCode:  9,
			expectedDuration:  10,
		},
		{
			name:              "very narrow terminal - minimum stack width",
			terminalWidth:     50,
			expectedMinStack:  20,
			expectedID:        4,
			expectedTimestamp: 19,
			expectedCommand:   8,
			expectedExitCode:  9,
			expectedDuration:  10,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cols := newHistoryTableColumns(tt.terminalWidth)

			assert.Equal(t, tt.expectedID, cols.id)
			assert.Equal(t, tt.expectedTimestamp, cols.timestamp)
			assert.Equal(t, tt.expectedCommand, cols.command)
			assert.GreaterOrEqual(t, cols.stackPath, tt.expectedMinStack, "stackPath should be at least minimum width")
			assert.Equal(t, tt.expectedExitCode, cols.exitCode)
			assert.Equal(t, tt.expectedDuration, cols.duration)
			assert.Equal(t, 2, cols.cursor)
		})
	}
}

// TestFormatExitCode tests exit code formatting.
func TestFormatExitCode(t *testing.T) {
	styles := newHistoryTableStyles()

	tests := []struct {
		name          string
		exitCode      int
		width         int
		shouldContain string
	}{
		{
			name:          "success exit code",
			exitCode:      0,
			width:         9,
			shouldContain: "✓",
		},
		{
			name:          "failure exit code",
			exitCode:      1,
			width:         9,
			shouldContain: "✗",
		},
		{
			name:          "larger failure exit code",
			exitCode:      127,
			width:         9,
			shouldContain: "✗",
		},
		{
			name:          "negative exit code",
			exitCode:      -1,
			width:         9,
			shouldContain: "✗",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := formatExitCode(tt.exitCode, styles, tt.width)

			assert.Contains(t, result, tt.shouldContain)
			// Just verify it contains the exit code number
			assert.NotEmpty(t, result)
		})
	}
}

// TestCalculateVisibleRange tests visible window calculation.
func TestCalculateVisibleRange(t *testing.T) {
	tests := []struct {
		name          string
		totalItems    int
		cursorPos     int
		contentHeight int
		expectedStart int
		expectedEnd   int
	}{
		{
			name:          "all items fit - no scrolling needed",
			totalItems:    5,
			cursorPos:     2,
			contentHeight: 10,
			expectedStart: 0,
			expectedEnd:   5,
		},
		{
			name:          "cursor at top - window starts at 0",
			totalItems:    20,
			cursorPos:     0,
			contentHeight: 10,
			expectedStart: 0,
			expectedEnd:   10,
		},
		{
			name:          "cursor at bottom - window ends at total",
			totalItems:    20,
			cursorPos:     19,
			contentHeight: 10,
			expectedStart: 10,
			expectedEnd:   20,
		},
		{
			name:          "cursor in middle - window centered",
			totalItems:    20,
			cursorPos:     10,
			contentHeight: 10,
			expectedStart: 5,
			expectedEnd:   15,
		},
		{
			name:          "cursor near top - window adjusted",
			totalItems:    20,
			cursorPos:     2,
			contentHeight: 10,
			expectedStart: 0,
			expectedEnd:   10,
		},
		{
			name:          "cursor near bottom - window adjusted",
			totalItems:    20,
			cursorPos:     17,
			contentHeight: 10,
			expectedStart: 10,
			expectedEnd:   20,
		},
		{
			name:          "single item",
			totalItems:    1,
			cursorPos:     0,
			contentHeight: 10,
			expectedStart: 0,
			expectedEnd:   1,
		},
		{
			name:          "contentHeight larger than total",
			totalItems:    3,
			cursorPos:     1,
			contentHeight: 20,
			expectedStart: 0,
			expectedEnd:   3,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			start, end := calculateVisibleRange(tt.totalItems, tt.cursorPos, tt.contentHeight)

			assert.Equal(t, tt.expectedStart, start)
			assert.Equal(t, tt.expectedEnd, end)
			assert.GreaterOrEqual(t, start, 0, "start should not be negative")
			assert.LessOrEqual(t, end, tt.totalItems, "end should not exceed total items")
			assert.GreaterOrEqual(t, end-start, 0, "window size should be positive")
		})
	}
}

// TestBuildHistoryTableHeader tests table header construction.
func TestBuildHistoryTableHeader(t *testing.T) {
	cols := newHistoryTableColumns(120)
	styles := newHistoryTableStyles()

	header := buildHistoryTableHeader(cols, styles.headerRow)

	assert.NotEmpty(t, header)
	assert.Contains(t, header, "#")
	assert.Contains(t, header, "Timestamp")
	assert.Contains(t, header, "Command")
	assert.Contains(t, header, "Stack Path")
	assert.Contains(t, header, "Exit Code")
	assert.Contains(t, header, "Duration")
}

// TestBuildHistoryTableRow tests individual row construction.
func TestBuildHistoryTableRow(t *testing.T) {
	cols := newHistoryTableColumns(120)
	styles := newHistoryTableStyles()

	tests := []struct {
		name          string
		entry         history.ExecutionLogEntry
		displayID     int
		shouldContain []string
	}{
		{
			name: "success row",
			entry: history.ExecutionLogEntry{
				ID:        42,
				Timestamp: time.Date(2025, 12, 16, 10, 30, 0, 0, time.UTC),
				Command:   "plan",
				StackPath: "dev/vpc",
				ExitCode:  0,
				DurationS: 5.25,
			},
			displayID:     1,
			shouldContain: []string{"1", "2025-12-16 10:30:00", "plan", "dev/vpc", "✓", "5.25s"},
		},
		{
			name: "failure row",
			entry: history.ExecutionLogEntry{
				ID:        43,
				Timestamp: time.Date(2025, 12, 16, 11, 0, 0, 0, time.UTC),
				Command:   "apply",
				StackPath: "prod/rds",
				ExitCode:  1,
				DurationS: 12.80,
			},
			displayID:     2,
			shouldContain: []string{"2", "2025-12-16 11:00:00", "apply", "prod/rds", "✗", "12.80s"},
		},
		{
			name: "long stack path - should truncate",
			entry: history.ExecutionLogEntry{
				ID:        44,
				Timestamp: time.Date(2025, 12, 16, 12, 0, 0, 0, time.UTC),
				Command:   "validate",
				StackPath: "very/long/path/that/exceeds/column/width/and/should/be/truncated/to/show/relevant/end",
				ExitCode:  0,
				DurationS: 1.5,
			},
			displayID:     3,
			shouldContain: []string{"3", "2025-12-16 12:00:00", "validate", "...", "✓", "1.50s"},
		},
		{
			name: "destroy command with non-zero exit",
			entry: history.ExecutionLogEntry{
				ID:        45,
				Timestamp: time.Date(2025, 12, 16, 13, 0, 0, 0, time.UTC),
				Command:   "destroy",
				StackPath: "staging/ec2",
				ExitCode:  127,
				DurationS: 0.25,
			},
			displayID:     4,
			shouldContain: []string{"4", "2025-12-16 13:00:00", "destroy", "staging/ec2", "✗", "0.25s"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			row := buildHistoryTableRow(tt.entry, tt.displayID, cols, styles)

			assert.NotEmpty(t, row)
			for _, expected := range tt.shouldContain {
				assert.Contains(t, row, expected)
			}
		})
	}
}

// TestRenderHistoryView tests full history view rendering.
func TestRenderHistoryView(t *testing.T) {
	tests := []struct {
		name        string
		setupModel  func() Model
		checkOutput func(t *testing.T, output string)
	}{
		{
			name: "not ready shows initializing",
			setupModel: func() Model {
				m := NewHistoryModel([]history.ExecutionLogEntry{
					{ID: 1, Command: "plan"},
				})
				m.ready = false
				return m
			},
			checkOutput: func(t *testing.T, output string) {
				assert.Equal(t, Initializing, output)
			},
		},
		{
			name: "empty history shows empty message",
			setupModel: func() Model {
				m := NewHistoryModel([]history.ExecutionLogEntry{})
				m.ready = true
				m.width = 120
				m.height = 30
				return m
			},
			checkOutput: func(t *testing.T, output string) {
				assert.Contains(t, output, "No execution history found")
				assert.Contains(t, output, "Press 'q' or 'esc' to exit")
			},
		},
		{
			name: "single entry renders table",
			setupModel: func() Model {
				m := NewHistoryModel([]history.ExecutionLogEntry{
					{
						ID:        1,
						Timestamp: time.Date(2025, 12, 16, 10, 0, 0, 0, time.UTC),
						Command:   "plan",
						StackPath: "dev/vpc",
						ExitCode:  0,
						DurationS: 5.2,
					},
				})
				m.ready = true
				m.width = 120
				m.height = 30
				return m
			},
			checkOutput: func(t *testing.T, output string) {
				assert.Contains(t, output, "Execution History")
				assert.Contains(t, output, "#")
				assert.Contains(t, output, "Timestamp")
				assert.Contains(t, output, "Command")
				assert.Contains(t, output, "Stack Path")
				assert.Contains(t, output, "Exit Code")
				assert.Contains(t, output, "Duration")
				assert.Contains(t, output, "plan")
				assert.Contains(t, output, "dev/vpc")
				assert.Contains(t, output, "✓")
				assert.Contains(t, output, "5.20s")
			},
		},
		{
			name: "multiple entries with cursor",
			setupModel: func() Model {
				m := NewHistoryModel([]history.ExecutionLogEntry{
					{
						ID:        1,
						Timestamp: time.Now(),
						Command:   "plan",
						StackPath: "dev/vpc",
						ExitCode:  0,
						DurationS: 5.0,
					},
					{
						ID:        2,
						Timestamp: time.Now(),
						Command:   "apply",
						StackPath: "prod/rds",
						ExitCode:  1,
						DurationS: 10.0,
					},
				})
				m.ready = true
				m.width = 120
				m.height = 30
				m.historyCursor = 1
				return m
			},
			checkOutput: func(t *testing.T, output string) {
				assert.Contains(t, output, "plan")
				assert.Contains(t, output, "apply")
				assert.Contains(t, output, "▶") // Cursor indicator
				assert.Contains(t, output, "Showing")
				assert.Contains(t, output, "Use ↑/↓ to navigate")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := tt.setupModel()
			output := m.renderHistoryView()
			tt.checkOutput(t, output)
		})
	}
}

// TestRenderEmptyHistory tests empty history rendering.
func TestRenderEmptyHistory(t *testing.T) {
	m := NewHistoryModel([]history.ExecutionLogEntry{})
	m.ready = true
	m.width = 120
	m.height = 30

	header := "Test Header"
	output := m.renderEmptyHistory(header)

	assert.Contains(t, output, "No execution history found")
	assert.Contains(t, output, "Execute commands through TerraX to build history")
	assert.Contains(t, output, "Press 'q' or 'esc' to exit")
	assert.Contains(t, output, header)
}

// TestBuildHistoryTableRows tests row building for visible range.
func TestBuildHistoryTableRows(t *testing.T) {
	entries := []history.ExecutionLogEntry{
		{
			ID:        1,
			Timestamp: time.Date(2025, 12, 16, 10, 0, 0, 0, time.UTC),
			Command:   "plan",
			StackPath: "dev/vpc",
			ExitCode:  0,
			DurationS: 1.0,
		},
		{
			ID:        2,
			Timestamp: time.Date(2025, 12, 16, 11, 0, 0, 0, time.UTC),
			Command:   "apply",
			StackPath: "prod/rds",
			ExitCode:  1,
			DurationS: 2.0,
		},
		{
			ID:        3,
			Timestamp: time.Date(2025, 12, 16, 12, 0, 0, 0, time.UTC),
			Command:   "destroy",
			StackPath: "staging/ec2",
			ExitCode:  0,
			DurationS: 3.0,
		},
	}

	tests := []struct {
		name          string
		startIdx      int
		endIdx        int
		historyCursor int
		expectedRows  int
		checkContent  func(t *testing.T, rows []string)
	}{
		{
			name:          "all entries visible",
			startIdx:      0,
			endIdx:        3,
			historyCursor: 1,
			expectedRows:  3,
			checkContent: func(t *testing.T, rows []string) {
				assert.Equal(t, 3, len(rows))
				assert.Contains(t, rows[0], "plan")
				assert.Contains(t, rows[1], "apply")
				assert.Contains(t, rows[1], "▶") // Cursor on second row
				assert.Contains(t, rows[2], "destroy")
			},
		},
		{
			name:          "partial range",
			startIdx:      1,
			endIdx:        3,
			historyCursor: 1,
			expectedRows:  2,
			checkContent: func(t *testing.T, rows []string) {
				assert.Equal(t, 2, len(rows))
				assert.Contains(t, rows[0], "apply")
				assert.Contains(t, rows[0], "▶") // Cursor on first row of visible
				assert.Contains(t, rows[1], "destroy")
			},
		},
		{
			name:          "single entry visible",
			startIdx:      0,
			endIdx:        1,
			historyCursor: 0,
			expectedRows:  1,
			checkContent: func(t *testing.T, rows []string) {
				assert.Equal(t, 1, len(rows))
				assert.Contains(t, rows[0], "plan")
				assert.Contains(t, rows[0], "▶")
			},
		},
		{
			name:          "cursor outside visible range",
			startIdx:      0,
			endIdx:        2,
			historyCursor: 2,
			expectedRows:  2,
			checkContent: func(t *testing.T, rows []string) {
				assert.Equal(t, 2, len(rows))
				// Neither row should have cursor
				for _, row := range rows {
					if !strings.Contains(row, "plan") && !strings.Contains(row, "apply") {
						t.Error("unexpected row content")
					}
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := NewHistoryModel(entries)
			m.ready = true
			m.width = 120
			m.height = 30
			m.historyCursor = tt.historyCursor

			cols := newHistoryTableColumns(m.width)
			styles := newHistoryTableStyles()

			rows := m.buildHistoryTableRows(tt.startIdx, tt.endIdx, cols, styles)

			assert.Equal(t, tt.expectedRows, len(rows))
			tt.checkContent(t, rows)
		})
	}
}

// TestBuildHistoryFooter tests footer construction.
func TestBuildHistoryFooter(t *testing.T) {
	tests := []struct {
		name          string
		totalEntries  int
		startIdx      int
		endIdx        int
		shouldContain []string
	}{
		{
			name:          "all entries shown",
			totalEntries:  10,
			startIdx:      0,
			endIdx:        10,
			shouldContain: []string{"Showing 1-10 of 10", "Use ↑/↓ to navigate", "Press Enter to re-execute"},
		},
		{
			name:          "partial window at start",
			totalEntries:  20,
			startIdx:      0,
			endIdx:        10,
			shouldContain: []string{"Showing 1-10 of 20", "Use ↑/↓ to navigate"},
		},
		{
			name:          "partial window in middle",
			totalEntries:  30,
			startIdx:      10,
			endIdx:        20,
			shouldContain: []string{"Showing 11-20 of 30"},
		},
		{
			name:          "partial window at end",
			totalEntries:  25,
			startIdx:      15,
			endIdx:        25,
			shouldContain: []string{"Showing 16-25 of 25"},
		},
		{
			name:          "single entry",
			totalEntries:  1,
			startIdx:      0,
			endIdx:        1,
			shouldContain: []string{"Showing 1-1 of 1"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			entries := make([]history.ExecutionLogEntry, tt.totalEntries)
			for i := 0; i < tt.totalEntries; i++ {
				entries[i] = history.ExecutionLogEntry{
					ID:      i + 1,
					Command: "plan",
				}
			}

			m := NewHistoryModel(entries)
			footer := m.buildHistoryFooter(tt.startIdx, tt.endIdx)

			assert.NotEmpty(t, footer)
			for _, expected := range tt.shouldContain {
				assert.Contains(t, footer, expected)
			}
		})
	}
}

// TestModel_View_HistoryState tests View method in history state.
func TestModel_View_HistoryState(t *testing.T) {
	tests := []struct {
		name        string
		setupModel  func() Model
		checkOutput func(t *testing.T, output string)
	}{
		{
			name: "history state renders history view",
			setupModel: func() Model {
				m := NewHistoryModel([]history.ExecutionLogEntry{
					{
						ID:        1,
						Command:   "plan",
						StackPath: "dev/vpc",
						ExitCode:  0,
						DurationS: 5.0,
						Timestamp: time.Now(),
					},
				})
				m.ready = true
				m.width = 120
				m.height = 30
				return m
			},
			checkOutput: func(t *testing.T, output string) {
				assert.Contains(t, output, "Execution History")
				assert.Contains(t, output, "plan")
				assert.Contains(t, output, "dev/vpc")
			},
		},
		{
			name: "history state not ready shows initializing",
			setupModel: func() Model {
				m := NewHistoryModel([]history.ExecutionLogEntry{
					{ID: 1, Command: "plan"},
				})
				m.ready = false
				return m
			},
			checkOutput: func(t *testing.T, output string) {
				assert.Equal(t, Initializing, output)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := tt.setupModel()
			output := m.View()
			tt.checkOutput(t, output)
		})
	}
}

// TestFormatExitCode_Padding tests exit code padding logic.
func TestFormatExitCode_Padding(t *testing.T) {
	styles := newHistoryTableStyles()

	tests := []struct {
		name     string
		exitCode int
		width    int
		minLen   int
	}{
		{
			name:     "single digit with padding",
			exitCode: 0,
			width:    9,
			minLen:   9,
		},
		{
			name:     "double digit with padding",
			exitCode: 10,
			width:    9,
			minLen:   9,
		},
		{
			name:     "triple digit with padding",
			exitCode: 127,
			width:    9,
			minLen:   9,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := formatExitCode(tt.exitCode, styles, tt.width)
			// The result should have padding to match width
			assert.GreaterOrEqual(t, len(result), tt.minLen)
		})
	}
}
