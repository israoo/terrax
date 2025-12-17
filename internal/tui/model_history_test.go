package tui

import (
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/assert"

	"github.com/israoo/terrax/internal/history"
	"github.com/israoo/terrax/internal/stack"
)

// TestNewHistoryModel tests creating a model in history viewing mode.
func TestNewHistoryModel(t *testing.T) {
	tests := []struct {
		name           string
		historyEntries []history.ExecutionLogEntry
		expectedState  AppState
		expectedCursor int
		expectedReady  bool
		expectedReExec bool
		expectedEntry  *history.ExecutionLogEntry
	}{
		{
			name:           "empty history",
			historyEntries: []history.ExecutionLogEntry{},
			expectedState:  StateHistory,
			expectedCursor: 0,
			expectedReady:  false,
			expectedReExec: false,
			expectedEntry:  nil,
		},
		{
			name: "single entry",
			historyEntries: []history.ExecutionLogEntry{
				{
					ID:        1,
					Timestamp: time.Now(),
					Command:   "plan",
					StackPath: "/test/path",
					ExitCode:  0,
				},
			},
			expectedState:  StateHistory,
			expectedCursor: 0,
			expectedReady:  false,
			expectedReExec: false,
			expectedEntry:  nil,
		},
		{
			name: "multiple entries",
			historyEntries: []history.ExecutionLogEntry{
				{
					ID:        1,
					Timestamp: time.Now(),
					Command:   "plan",
					StackPath: "/test/path1",
					ExitCode:  0,
				},
				{
					ID:        2,
					Timestamp: time.Now(),
					Command:   "apply",
					StackPath: "/test/path2",
					ExitCode:  0,
				},
				{
					ID:        3,
					Timestamp: time.Now(),
					Command:   "destroy",
					StackPath: "/test/path3",
					ExitCode:  1,
				},
			},
			expectedState:  StateHistory,
			expectedCursor: 0,
			expectedReady:  false,
			expectedReExec: false,
			expectedEntry:  nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := NewHistoryModel(tt.historyEntries)

			assert.Equal(t, tt.expectedState, m.state)
			assert.Equal(t, tt.expectedCursor, m.historyCursor)
			assert.Equal(t, tt.expectedReady, m.ready)
			assert.Equal(t, tt.expectedReExec, m.reExecuteFromHistory)
			assert.Equal(t, tt.expectedEntry, m.selectedHistoryEntry)
			assert.Equal(t, len(tt.historyEntries), len(m.history))
		})
	}
}

// TestModel_HandleHistoryUpdate tests history state message handling.
func TestModel_HandleHistoryUpdate(t *testing.T) {
	tests := []struct {
		name         string
		initialModel Model
		msg          tea.Msg
		checkResult  func(t *testing.T, m tea.Model, cmd tea.Cmd)
	}{
		{
			name: "window size makes model ready",
			initialModel: NewHistoryModel([]history.ExecutionLogEntry{
				{ID: 1, Command: "plan"},
			}),
			msg: tea.WindowSizeMsg{
				Width:  120,
				Height: 30,
			},
			checkResult: func(t *testing.T, m tea.Model, cmd tea.Cmd) {
				model := m.(Model)
				assert.True(t, model.ready)
				assert.Equal(t, 120, model.width)
				assert.Equal(t, 30, model.height)
				assert.Nil(t, cmd)
			},
		},
		{
			name: "q key quits",
			initialModel: NewHistoryModel([]history.ExecutionLogEntry{
				{ID: 1, Command: "plan"},
			}),
			msg: tea.KeyMsg{
				Type:  tea.KeyRunes,
				Runes: []rune{'q'},
			},
			checkResult: func(t *testing.T, m tea.Model, cmd tea.Cmd) {
				assert.NotNil(t, cmd, "should return quit command")
			},
		},
		{
			name: "escape key quits",
			initialModel: NewHistoryModel([]history.ExecutionLogEntry{
				{ID: 1, Command: "plan"},
			}),
			msg: tea.KeyMsg{
				Type: tea.KeyEsc,
			},
			checkResult: func(t *testing.T, m tea.Model, cmd tea.Cmd) {
				assert.NotNil(t, cmd, "should return quit command")
			},
		},
		{
			name: "up key moves cursor up",
			initialModel: func() Model {
				m := NewHistoryModel([]history.ExecutionLogEntry{
					{ID: 1, Command: "plan"},
					{ID: 2, Command: "apply"},
					{ID: 3, Command: "destroy"},
				})
				m.historyCursor = 1
				return m
			}(),
			msg: tea.KeyMsg{
				Type: tea.KeyUp,
			},
			checkResult: func(t *testing.T, m tea.Model, cmd tea.Cmd) {
				model := m.(Model)
				assert.Equal(t, 0, model.historyCursor)
				assert.Nil(t, cmd)
			},
		},
		{
			name: "up key wraps to last when at first",
			initialModel: func() Model {
				m := NewHistoryModel([]history.ExecutionLogEntry{
					{ID: 1, Command: "plan"},
					{ID: 2, Command: "apply"},
					{ID: 3, Command: "destroy"},
				})
				m.historyCursor = 0
				return m
			}(),
			msg: tea.KeyMsg{
				Type: tea.KeyUp,
			},
			checkResult: func(t *testing.T, m tea.Model, cmd tea.Cmd) {
				model := m.(Model)
				assert.Equal(t, 2, model.historyCursor, "should wrap to last item")
				assert.Nil(t, cmd)
			},
		},
		{
			name: "down key moves cursor down",
			initialModel: func() Model {
				m := NewHistoryModel([]history.ExecutionLogEntry{
					{ID: 1, Command: "plan"},
					{ID: 2, Command: "apply"},
					{ID: 3, Command: "destroy"},
				})
				m.historyCursor = 0
				return m
			}(),
			msg: tea.KeyMsg{
				Type: tea.KeyDown,
			},
			checkResult: func(t *testing.T, m tea.Model, cmd tea.Cmd) {
				model := m.(Model)
				assert.Equal(t, 1, model.historyCursor)
				assert.Nil(t, cmd)
			},
		},
		{
			name: "down key wraps to first when at last",
			initialModel: func() Model {
				m := NewHistoryModel([]history.ExecutionLogEntry{
					{ID: 1, Command: "plan"},
					{ID: 2, Command: "apply"},
					{ID: 3, Command: "destroy"},
				})
				m.historyCursor = 2
				return m
			}(),
			msg: tea.KeyMsg{
				Type: tea.KeyDown,
			},
			checkResult: func(t *testing.T, m tea.Model, cmd tea.Cmd) {
				model := m.(Model)
				assert.Equal(t, 0, model.historyCursor, "should wrap to first item")
				assert.Nil(t, cmd)
			},
		},
		{
			name: "enter selects entry for re-execution and quits",
			initialModel: func() Model {
				entries := []history.ExecutionLogEntry{
					{ID: 1, Command: "plan", StackPath: "/test/path1"},
					{ID: 2, Command: "apply", StackPath: "/test/path2"},
				}
				m := NewHistoryModel(entries)
				m.historyCursor = 1
				return m
			}(),
			msg: tea.KeyMsg{
				Type: tea.KeyEnter,
			},
			checkResult: func(t *testing.T, m tea.Model, cmd tea.Cmd) {
				model := m.(Model)
				assert.True(t, model.reExecuteFromHistory, "should be marked for re-execution")
				assert.NotNil(t, model.selectedHistoryEntry, "should have selected entry")
				assert.Equal(t, 2, model.selectedHistoryEntry.ID)
				assert.Equal(t, "apply", model.selectedHistoryEntry.Command)
				assert.NotNil(t, cmd, "should return quit command")
			},
		},
		{
			name: "enter on empty history does nothing",
			initialModel: func() Model {
				m := NewHistoryModel([]history.ExecutionLogEntry{})
				return m
			}(),
			msg: tea.KeyMsg{
				Type: tea.KeyEnter,
			},
			checkResult: func(t *testing.T, m tea.Model, cmd tea.Cmd) {
				model := m.(Model)
				assert.False(t, model.reExecuteFromHistory)
				assert.Nil(t, model.selectedHistoryEntry)
				assert.NotNil(t, cmd, "should still quit")
			},
		},
		{
			name: "up on empty history does nothing",
			initialModel: func() Model {
				m := NewHistoryModel([]history.ExecutionLogEntry{})
				return m
			}(),
			msg: tea.KeyMsg{
				Type: tea.KeyUp,
			},
			checkResult: func(t *testing.T, m tea.Model, cmd tea.Cmd) {
				model := m.(Model)
				assert.Equal(t, 0, model.historyCursor)
				assert.Nil(t, cmd)
			},
		},
		{
			name: "down on empty history does nothing",
			initialModel: func() Model {
				m := NewHistoryModel([]history.ExecutionLogEntry{})
				return m
			}(),
			msg: tea.KeyMsg{
				Type: tea.KeyDown,
			},
			checkResult: func(t *testing.T, m tea.Model, cmd tea.Cmd) {
				model := m.(Model)
				assert.Equal(t, 0, model.historyCursor)
				assert.Nil(t, cmd)
			},
		},
		{
			name: "unknown key does nothing",
			initialModel: NewHistoryModel([]history.ExecutionLogEntry{
				{ID: 1, Command: "plan"},
			}),
			msg: tea.KeyMsg{
				Type:  tea.KeyRunes,
				Runes: []rune{'x'},
			},
			checkResult: func(t *testing.T, m tea.Model, cmd tea.Cmd) {
				assert.Nil(t, cmd)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			updatedModel, cmd := tt.initialModel.handleHistoryUpdate(tt.msg)
			tt.checkResult(t, updatedModel, cmd)
		})
	}
}

// TestModel_ShouldReExecuteFromHistory tests the re-execution flag getter.
func TestModel_ShouldReExecuteFromHistory(t *testing.T) {
	tests := []struct {
		name     string
		model    Model
		expected bool
	}{
		{
			name: "not marked for re-execution",
			model: Model{
				reExecuteFromHistory: false,
			},
			expected: false,
		},
		{
			name: "marked for re-execution",
			model: Model{
				reExecuteFromHistory: true,
			},
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.model.ShouldReExecuteFromHistory()
			assert.Equal(t, tt.expected, result)
		})
	}
}

// TestModel_GetSelectedHistoryEntry tests the selected history entry getter.
func TestModel_GetSelectedHistoryEntry(t *testing.T) {
	testEntry := &history.ExecutionLogEntry{
		ID:        42,
		Command:   "plan",
		StackPath: "/test/path",
		ExitCode:  0,
	}

	tests := []struct {
		name     string
		model    Model
		expected *history.ExecutionLogEntry
	}{
		{
			name: "no entry selected",
			model: Model{
				selectedHistoryEntry: nil,
			},
			expected: nil,
		},
		{
			name: "entry selected",
			model: Model{
				selectedHistoryEntry: testEntry,
			},
			expected: testEntry,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.model.GetSelectedHistoryEntry()
			assert.Equal(t, tt.expected, result)
		})
	}
}

// TestModel_Update_HistoryState tests Update routing for history state.
func TestModel_Update_HistoryState(t *testing.T) {
	tests := []struct {
		name        string
		setupModel  func() Model
		msg         tea.Msg
		checkResult func(t *testing.T, m tea.Model, cmd tea.Cmd)
	}{
		{
			name: "history state routes to handleHistoryUpdate",
			setupModel: func() Model {
				return NewHistoryModel([]history.ExecutionLogEntry{
					{ID: 1, Command: "plan"},
				})
			},
			msg: tea.WindowSizeMsg{Width: 100, Height: 50},
			checkResult: func(t *testing.T, m tea.Model, cmd tea.Cmd) {
				model := m.(Model)
				assert.Equal(t, StateHistory, model.state)
				assert.True(t, model.ready)
				assert.Equal(t, 100, model.width)
			},
		},
		{
			name: "navigation state routes to normal handlers",
			setupModel: func() Model {
				root := &stack.Node{Name: "root"}
				m := NewModel(root, 1, []string{"plan"}, 3)
				m.state = StateNavigation
				return m
			},
			msg: tea.WindowSizeMsg{Width: 100, Height: 50},
			checkResult: func(t *testing.T, m tea.Model, cmd tea.Cmd) {
				model := m.(Model)
				assert.Equal(t, StateNavigation, model.state)
				assert.True(t, model.ready)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := tt.setupModel()
			updatedModel, cmd := m.Update(tt.msg)
			tt.checkResult(t, updatedModel, cmd)
		})
	}
}

// TestModel_HistoryCursorBounds tests cursor boundary validation.
func TestModel_HistoryCursorBounds(t *testing.T) {
	tests := []struct {
		name           string
		historySize    int
		initialCursor  int
		keySequence    []tea.KeyMsg
		expectedCursor int
	}{
		{
			name:          "single entry - up then down returns to same",
			historySize:   1,
			initialCursor: 0,
			keySequence: []tea.KeyMsg{
				{Type: tea.KeyUp},
				{Type: tea.KeyDown},
			},
			expectedCursor: 0,
		},
		{
			name:          "three entries - navigate to last",
			historySize:   3,
			initialCursor: 0,
			keySequence: []tea.KeyMsg{
				{Type: tea.KeyDown},
				{Type: tea.KeyDown},
			},
			expectedCursor: 2,
		},
		{
			name:          "three entries - wrap around down",
			historySize:   3,
			initialCursor: 2,
			keySequence: []tea.KeyMsg{
				{Type: tea.KeyDown},
			},
			expectedCursor: 0,
		},
		{
			name:          "three entries - wrap around up",
			historySize:   3,
			initialCursor: 0,
			keySequence: []tea.KeyMsg{
				{Type: tea.KeyUp},
			},
			expectedCursor: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			entries := make([]history.ExecutionLogEntry, tt.historySize)
			for i := 0; i < tt.historySize; i++ {
				entries[i] = history.ExecutionLogEntry{
					ID:      i + 1,
					Command: "plan",
				}
			}

			m := NewHistoryModel(entries)
			m.historyCursor = tt.initialCursor

			for _, keyMsg := range tt.keySequence {
				updatedModel, _ := m.handleHistoryUpdate(keyMsg)
				m = updatedModel.(Model)
			}

			assert.Equal(t, tt.expectedCursor, m.historyCursor)
		})
	}
}

// TestModel_HistoryReExecution tests the full re-execution workflow.
func TestModel_HistoryReExecution(t *testing.T) {
	entries := []history.ExecutionLogEntry{
		{
			ID:           1,
			Command:      "plan",
			StackPath:    "dev/vpc",
			AbsolutePath: "/test/dev/vpc",
			ExitCode:     0,
			DurationS:    5.2,
		},
		{
			ID:           2,
			Command:      "apply",
			StackPath:    "prod/rds",
			AbsolutePath: "/test/prod/rds",
			ExitCode:     1,
			DurationS:    12.8,
		},
	}

	m := NewHistoryModel(entries)
	m.historyCursor = 1 // Select the second entry

	// Simulate enter key press
	msg := tea.KeyMsg{Type: tea.KeyEnter}
	updatedModel, cmd := m.handleHistoryUpdate(msg)
	finalModel := updatedModel.(Model)

	// Verify re-execution is set up correctly
	assert.True(t, finalModel.ShouldReExecuteFromHistory())
	assert.NotNil(t, finalModel.GetSelectedHistoryEntry())
	assert.Equal(t, 2, finalModel.GetSelectedHistoryEntry().ID)
	assert.Equal(t, "apply", finalModel.GetSelectedHistoryEntry().Command)
	assert.Equal(t, "prod/rds", finalModel.GetSelectedHistoryEntry().StackPath)
	assert.Equal(t, "/test/prod/rds", finalModel.GetSelectedHistoryEntry().AbsolutePath)
	assert.NotNil(t, cmd, "should quit to execute command")
}
