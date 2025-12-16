package history

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetHistoryFilePath(t *testing.T) {
	path, err := GetHistoryFilePath()
	require.NoError(t, err)
	assert.NotEmpty(t, path)
	assert.Contains(t, path, ConfigDirName)
	assert.Contains(t, path, HistoryFileName)

	// Verify directory exists after call
	dir := filepath.Dir(path)
	info, err := os.Stat(dir)
	require.NoError(t, err)
	assert.True(t, info.IsDir())
}

func TestAppendToHistory(t *testing.T) {
	ctx := context.Background()

	// Setup: Create a temporary directory for testing
	tempDir := t.TempDir()
	tempHistoryPath := filepath.Join(tempDir, HistoryFileName)

	// Override the history file path for testing
	originalFunc := historyFilePathFunc
	historyFilePathFunc = func() (string, error) {
		return tempHistoryPath, nil
	}
	defer func() {
		historyFilePathFunc = originalFunc
	}()

	tests := []struct {
		name  string
		entry ExecutionLogEntry
	}{
		{
			name: "successful plan execution",
			entry: ExecutionLogEntry{
				ID:        1,
				Timestamp: time.Date(2025, 12, 16, 10, 30, 0, 0, time.UTC),
				User:      "testuser",
				StackPath: "/home/user/infra/dev/us-east-1",
				Command:   "plan",
				ExitCode:  0,
				DurationS: 12.5,
				Summary:   "3 to add, 0 to change, 0 to destroy",
			},
		},
		{
			name: "failed apply execution",
			entry: ExecutionLogEntry{
				ID:        2,
				Timestamp: time.Date(2025, 12, 16, 10, 35, 0, 0, time.UTC),
				User:      "testuser",
				StackPath: "/home/user/infra/prod/us-west-2",
				Command:   "apply",
				ExitCode:  1,
				DurationS: 45.2,
				Summary:   "Error: resource creation failed",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := AppendToHistory(ctx, tt.entry)
			require.NoError(t, err)

			// Verify file exists
			_, err = os.Stat(tempHistoryPath)
			require.NoError(t, err)

			// Read and verify the last line
			data, err := os.ReadFile(tempHistoryPath)
			require.NoError(t, err)
			assert.NotEmpty(t, data)

			// Parse the JSON to verify it's valid
			lines := splitLines(string(data))
			lastLine := lines[len(lines)-1]

			var parsed ExecutionLogEntry
			err = json.Unmarshal([]byte(lastLine), &parsed)
			require.NoError(t, err)
			assert.Equal(t, tt.entry.ID, parsed.ID)
			assert.Equal(t, tt.entry.Command, parsed.Command)
			assert.Equal(t, tt.entry.StackPath, parsed.StackPath)
		})
	}
}

func TestTrimHistory(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name           string
		initialEntries int
		maxEntries     int
		expectError    bool
		expectedLines  int
	}{
		{
			name:           "no trimming needed - under limit",
			initialEntries: 5,
			maxEntries:     10,
			expectError:    false,
			expectedLines:  5,
		},
		{
			name:           "trim to exact limit",
			initialEntries: 15,
			maxEntries:     10,
			expectError:    false,
			expectedLines:  10,
		},
		{
			name:           "trim significantly",
			initialEntries: 100,
			maxEntries:     20,
			expectError:    false,
			expectedLines:  20,
		},
		{
			name:           "invalid maxEntries - zero",
			initialEntries: 10,
			maxEntries:     0,
			expectError:    true,
			expectedLines:  10,
		},
		{
			name:           "invalid maxEntries - negative",
			initialEntries: 10,
			maxEntries:     -5,
			expectError:    true,
			expectedLines:  10,
		},
		{
			name:           "empty file - no trimming",
			initialEntries: 0,
			maxEntries:     10,
			expectError:    false,
			expectedLines:  0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup: Create a temporary directory for testing
			tempDir := t.TempDir()
			tempHistoryPath := filepath.Join(tempDir, HistoryFileName)

			// Override the history file path for testing
			originalFunc := historyFilePathFunc
			historyFilePathFunc = func() (string, error) {
				return tempHistoryPath, nil
			}
			defer func() {
				historyFilePathFunc = originalFunc
			}()

			// Create initial entries
			for i := 1; i <= tt.initialEntries; i++ {
				entry := ExecutionLogEntry{
					ID:        i,
					Timestamp: time.Now(),
					User:      "testuser",
					StackPath: "/test/path",
					Command:   "plan",
					ExitCode:  0,
					DurationS: 1.0,
					Summary:   "test",
				}
				err := AppendToHistory(ctx, entry)
				require.NoError(t, err)
			}

			// Execute trim
			err := TrimHistory(ctx, tt.maxEntries)

			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}

			// Verify the number of lines
			if tt.initialEntries > 0 && !tt.expectError {
				data, err := os.ReadFile(tempHistoryPath)
				require.NoError(t, err)
				lines := splitLines(string(data))
				assert.Equal(t, tt.expectedLines, len(lines), "Expected %d lines but got %d", tt.expectedLines, len(lines))

				// Verify that the kept entries are the most recent ones
				if tt.expectedLines > 0 && tt.initialEntries > tt.maxEntries {
					var firstEntry ExecutionLogEntry
					err = json.Unmarshal([]byte(lines[0]), &firstEntry)
					require.NoError(t, err)
					// First kept entry should have ID = (initialEntries - maxEntries + 1)
					expectedFirstID := tt.initialEntries - tt.maxEntries + 1
					assert.Equal(t, expectedFirstID, firstEntry.ID, "First entry after trim should have ID %d", expectedFirstID)
				}
			}
		})
	}
}

func TestTrimHistoryNonExistentFile(t *testing.T) {
	ctx := context.Background()

	// Setup: Create a temporary directory without creating the file
	tempDir := t.TempDir()
	tempHistoryPath := filepath.Join(tempDir, HistoryFileName)

	originalFunc := historyFilePathFunc
	historyFilePathFunc = func() (string, error) {
		return tempHistoryPath, nil
	}
	defer func() {
		historyFilePathFunc = originalFunc
	}()

	// Should not error when file doesn't exist
	err := TrimHistory(ctx, 10)
	assert.NoError(t, err)
}

func TestExecutionLogEntry_JSONSerialization(t *testing.T) {
	entry := ExecutionLogEntry{
		ID:        42,
		Timestamp: time.Date(2025, 12, 16, 10, 30, 45, 0, time.UTC),
		User:      "john.doe",
		StackPath: "/home/john/terraform/prod/vpc",
		Command:   "apply",
		ExitCode:  0,
		DurationS: 123.456,
		Summary:   "5 added, 2 changed, 0 destroyed",
	}

	// Serialize to JSON
	jsonData, err := json.Marshal(entry)
	require.NoError(t, err)
	assert.NotEmpty(t, jsonData)

	// Deserialize from JSON
	var parsed ExecutionLogEntry
	err = json.Unmarshal(jsonData, &parsed)
	require.NoError(t, err)

	// Verify all fields match
	assert.Equal(t, entry.ID, parsed.ID)
	assert.True(t, entry.Timestamp.Equal(parsed.Timestamp))
	assert.Equal(t, entry.User, parsed.User)
	assert.Equal(t, entry.StackPath, parsed.StackPath)
	assert.Equal(t, entry.Command, parsed.Command)
	assert.Equal(t, entry.ExitCode, parsed.ExitCode)
	assert.Equal(t, entry.DurationS, parsed.DurationS)
	assert.Equal(t, entry.Summary, parsed.Summary)
}

func TestGetCurrentUser(t *testing.T) {
	user := GetCurrentUser()
	assert.NotEmpty(t, user)
	// Should either be a real username or "unknown"
	assert.NotEqual(t, "", user)
}

func TestGetNextID(t *testing.T) {
	ctx := context.Background()

	t.Run("empty file returns 1", func(t *testing.T) {
		tempDir := t.TempDir()
		tempHistoryPath := filepath.Join(tempDir, HistoryFileName)

		originalFunc := historyFilePathFunc
		historyFilePathFunc = func() (string, error) {
			return tempHistoryPath, nil
		}
		defer func() {
			historyFilePathFunc = originalFunc
		}()

		id, err := GetNextID(ctx)
		require.NoError(t, err)
		assert.Equal(t, 1, id)
	})

	t.Run("returns max ID + 1", func(t *testing.T) {
		tempDir := t.TempDir()
		tempHistoryPath := filepath.Join(tempDir, HistoryFileName)

		originalFunc := historyFilePathFunc
		historyFilePathFunc = func() (string, error) {
			return tempHistoryPath, nil
		}
		defer func() {
			historyFilePathFunc = originalFunc
		}()

		// Add some entries
		for i := 1; i <= 5; i++ {
			entry := ExecutionLogEntry{
				ID:        i,
				Timestamp: time.Now(),
				User:      "test",
				StackPath: "/test",
				Command:   "plan",
				ExitCode:  0,
				DurationS: 1.0,
				Summary:   "test",
			}
			err := AppendToHistory(ctx, entry)
			require.NoError(t, err)
		}

		id, err := GetNextID(ctx)
		require.NoError(t, err)
		assert.Equal(t, 6, id)
	})

	t.Run("handles non-sequential IDs", func(t *testing.T) {
		tempDir := t.TempDir()
		tempHistoryPath := filepath.Join(tempDir, HistoryFileName)

		originalFunc := historyFilePathFunc
		historyFilePathFunc = func() (string, error) {
			return tempHistoryPath, nil
		}
		defer func() {
			historyFilePathFunc = originalFunc
		}()

		// Add entries with non-sequential IDs
		ids := []int{1, 5, 3, 10, 7}
		for _, id := range ids {
			entry := ExecutionLogEntry{
				ID:        id,
				Timestamp: time.Now(),
				User:      "test",
				StackPath: "/test",
				Command:   "plan",
				ExitCode:  0,
				DurationS: 1.0,
				Summary:   "test",
			}
			err := AppendToHistory(ctx, entry)
			require.NoError(t, err)
		}

		nextID, err := GetNextID(ctx)
		require.NoError(t, err)
		assert.Equal(t, 11, nextID) // max is 10, so next is 11
	})
}

func TestAppendAndTrimIntegration(t *testing.T) {
	ctx := context.Background()

	tempDir := t.TempDir()
	tempHistoryPath := filepath.Join(tempDir, HistoryFileName)

	originalFunc := historyFilePathFunc
	historyFilePathFunc = func() (string, error) {
		return tempHistoryPath, nil
	}
	defer func() {
		historyFilePathFunc = originalFunc
	}()

	// Simulate realistic usage pattern
	const maxEntries = 10
	const totalExecutions = 25

	// Add many entries
	for i := 1; i <= totalExecutions; i++ {
		entry := ExecutionLogEntry{
			ID:        i,
			Timestamp: time.Now().Add(time.Duration(i) * time.Second),
			User:      GetCurrentUser(),
			StackPath: filepath.Join("/infra", "env", "stack"),
			Command:   "plan",
			ExitCode:  i % 2, // Alternate between success/failure
			DurationS: float64(i) * 1.5,
			Summary:   "Integration test entry",
		}
		err := AppendToHistory(ctx, entry)
		require.NoError(t, err)
	}

	// Verify all entries were added
	data, err := os.ReadFile(tempHistoryPath)
	require.NoError(t, err)
	lines := splitLines(string(data))
	assert.Equal(t, totalExecutions, len(lines))

	// Trim to keep only the last maxEntries
	err = TrimHistory(ctx, maxEntries)
	require.NoError(t, err)

	// Verify trimming worked
	data, err = os.ReadFile(tempHistoryPath)
	require.NoError(t, err)
	lines = splitLines(string(data))
	assert.Equal(t, maxEntries, len(lines))

	// Verify the kept entries are the most recent ones (IDs 16-25)
	for i, line := range lines {
		var entry ExecutionLogEntry
		err := json.Unmarshal([]byte(line), &entry)
		require.NoError(t, err)
		expectedID := totalExecutions - maxEntries + i + 1
		assert.Equal(t, expectedID, entry.ID, "Line %d should have ID %d", i, expectedID)
	}

	// Add more entries after trimming
	for i := totalExecutions + 1; i <= totalExecutions+3; i++ {
		entry := ExecutionLogEntry{
			ID:        i,
			Timestamp: time.Now(),
			User:      GetCurrentUser(),
			StackPath: "/infra/new",
			Command:   "apply",
			ExitCode:  0,
			DurationS: 10.0,
			Summary:   "Post-trim entry",
		}
		err := AppendToHistory(ctx, entry)
		require.NoError(t, err)
	}

	// Verify new entries were appended
	data, err = os.ReadFile(tempHistoryPath)
	require.NoError(t, err)
	lines = splitLines(string(data))
	assert.Equal(t, maxEntries+3, len(lines))
}

// Helper function to split lines and filter empty ones
func splitLines(s string) []string {
	var lines []string
	for _, line := range strings.Split(s, "\n") {
		if line != "" {
			lines = append(lines, line)
		}
	}
	return lines
}
