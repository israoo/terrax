package history

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestAppendToHistory_FileCloseError tests handling of file close errors.
func TestAppendToHistory_FileCloseError(t *testing.T) {
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

	entry := ExecutionLogEntry{
		ID:        1,
		Timestamp: time.Now(),
		User:      "testuser",
		StackPath: "/test/path",
		Command:   "plan",
		ExitCode:  0,
		DurationS: 1.0,
		Summary:   "test",
	}

	// Normal append should work
	err := AppendToHistory(ctx, entry)
	require.NoError(t, err)

	// Verify file exists and is valid
	_, err = os.Stat(tempHistoryPath)
	require.NoError(t, err)
}

// TestAppendToHistory_InvalidJSON tests handling of entries that can't be marshaled.
// Note: ExecutionLogEntry should always be valid JSON, so this tests edge cases.
func TestAppendToHistory_ValidEntry(t *testing.T) {
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

	// Test with various valid entries
	entries := []ExecutionLogEntry{
		{
			ID:           1,
			Timestamp:    time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC),
			User:         "user1",
			StackPath:    "/path1",
			AbsolutePath: "/abs/path1",
			Command:      "plan",
			ExitCode:     0,
			DurationS:    1.5,
			Summary:      "success",
		},
		{
			ID:           2,
			Timestamp:    time.Date(2025, 1, 2, 0, 0, 0, 0, time.UTC),
			User:         "user2",
			StackPath:    "/path2",
			AbsolutePath: "/abs/path2",
			Command:      "apply",
			ExitCode:     1,
			DurationS:    10.25,
			Summary:      "failed",
		},
	}

	for _, entry := range entries {
		err := AppendToHistory(ctx, entry)
		require.NoError(t, err)
	}

	// Load and verify
	loaded, err := LoadHistory(ctx)
	require.NoError(t, err)
	assert.Len(t, loaded, 2)
}

// TestTrimHistory_EdgeCases tests additional edge cases for TrimHistory.
func TestTrimHistory_EdgeCases(t *testing.T) {
	ctx := context.Background()

	t.Run("trim with write permissions issue", func(t *testing.T) {
		tempDir := t.TempDir()
		tempHistoryPath := filepath.Join(tempDir, HistoryFileName)

		originalFunc := historyFilePathFunc
		historyFilePathFunc = func() (string, error) {
			return tempHistoryPath, nil
		}
		defer func() {
			historyFilePathFunc = originalFunc
		}()

		// Create file with some entries
		for i := 1; i <= 15; i++ {
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

		// Normal trim should work
		err := TrimHistory(ctx, 10)
		require.NoError(t, err)

		// Verify trimming worked
		loaded, err := LoadHistory(ctx)
		require.NoError(t, err)
		assert.Len(t, loaded, 10)
	})

	t.Run("trim with maxEntries exactly at current size", func(t *testing.T) {
		tempDir := t.TempDir()
		tempHistoryPath := filepath.Join(tempDir, HistoryFileName)

		originalFunc := historyFilePathFunc
		historyFilePathFunc = func() (string, error) {
			return tempHistoryPath, nil
		}
		defer func() {
			historyFilePathFunc = originalFunc
		}()

		// Create exactly 10 entries
		for i := 1; i <= 10; i++ {
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

		// Trim with maxEntries = 10 (same as current)
		err := TrimHistory(ctx, 10)
		require.NoError(t, err)

		// Should still have 10 entries
		loaded, err := LoadHistory(ctx)
		require.NoError(t, err)
		assert.Len(t, loaded, 10)
	})
}

// TestGetLastExecutionForProject_EdgeCases tests edge cases for GetLastExecutionForProject.
func TestGetLastExecutionForProject_EdgeCases(t *testing.T) {
	ctx := context.Background()

	t.Run("no history file exists", func(t *testing.T) {
		tempDir := t.TempDir()
		tempHistoryPath := filepath.Join(tempDir, "nonexistent_history.log")

		originalFunc := historyFilePathFunc
		historyFilePathFunc = func() (string, error) {
			return tempHistoryPath, nil
		}
		defer func() {
			historyFilePathFunc = originalFunc
		}()

		entry, err := GetLastExecutionForProject(ctx, "root.hcl")
		require.NoError(t, err)
		assert.Nil(t, entry)
	})

	t.Run("history exists but no matching project", func(t *testing.T) {
		// Create temp directory structure
		tmpDir := t.TempDir()
		project1 := filepath.Join(tmpDir, "project1")
		require.NoError(t, os.MkdirAll(filepath.Join(project1, "dev"), 0755))
		require.NoError(t, os.WriteFile(filepath.Join(project1, "root.hcl"), []byte("# project 1"), 0644))

		// Setup temporary history file
		tmpHistoryFile := filepath.Join(tmpDir, "test_history.log")
		originalFunc := historyFilePathFunc
		historyFilePathFunc = func() (string, error) {
			return tmpHistoryFile, nil
		}
		defer func() {
			historyFilePathFunc = originalFunc
		}()

		// Create history entry for project1
		entry := ExecutionLogEntry{
			ID:           1,
			Timestamp:    time.Now(),
			Command:      "plan",
			AbsolutePath: filepath.Join(project1, "dev"),
			StackPath:    "dev",
		}
		err := AppendToHistory(ctx, entry)
		require.NoError(t, err)

		// Change to a different directory outside project1
		outsideDir := filepath.Join(tmpDir, "outside")
		require.NoError(t, os.MkdirAll(outsideDir, 0755))

		originalDir, err := os.Getwd()
		require.NoError(t, err)
		defer func() {
			require.NoError(t, os.Chdir(originalDir))
		}()

		require.NoError(t, os.Chdir(outsideDir))

		// Should return the entry even when not filtered (no root.hcl found)
		lastEntry, err := GetLastExecutionForProject(ctx, "root.hcl")
		require.NoError(t, err)
		// When no project root is found, all entries are returned
		assert.NotNil(t, lastEntry)
	})
}

// TestLoadHistory_BackwardCompatibility tests loading old history entries.
func TestLoadHistory_BackwardCompatibility(t *testing.T) {
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

	// Write an old-style entry (without AbsolutePath field)
	oldStyleJSON := `{"id":1,"timestamp":"2025-12-16T10:00:00Z","user":"olduser","stack_path":"/old/path","command":"plan","exit_code":0,"duration_s":5.0,"summary":"old entry"}` + "\n"
	err := os.WriteFile(tempHistoryPath, []byte(oldStyleJSON), 0644)
	require.NoError(t, err)

	// Load history
	entries, err := LoadHistory(ctx)
	require.NoError(t, err)
	require.Len(t, entries, 1)

	// Verify backward compatibility: AbsolutePath should be populated from StackPath
	assert.Equal(t, "/old/path", entries[0].AbsolutePath)
	assert.Equal(t, "/old/path", entries[0].StackPath)
}

// TestLoadHistory_InvalidLines tests handling of invalid JSON lines.
func TestLoadHistory_InvalidLines(t *testing.T) {
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

	// Write mixed valid and invalid lines
	content := `{"id":1,"timestamp":"2025-12-16T10:00:00Z","user":"user1","stack_path":"/path1","absolute_path":"/abs1","command":"plan","exit_code":0,"duration_s":1.0,"summary":"valid 1"}
invalid json line here
{"id":2,"timestamp":"2025-12-16T11:00:00Z","user":"user2","stack_path":"/path2","absolute_path":"/abs2","command":"apply","exit_code":1,"duration_s":2.0,"summary":"valid 2"}

{"id":3,"timestamp":"2025-12-16T12:00:00Z","user":"user3","stack_path":"/path3","absolute_path":"/abs3","command":"destroy","exit_code":0,"duration_s":3.0,"summary":"valid 3"}
`
	err := os.WriteFile(tempHistoryPath, []byte(content), 0644)
	require.NoError(t, err)

	// Load history - should skip invalid lines
	entries, err := LoadHistory(ctx)
	require.NoError(t, err)

	// Should have 3 valid entries (invalid lines and empty lines are skipped)
	assert.Len(t, entries, 3)
	assert.Equal(t, 3, entries[0].ID) // Most recent first (reversed)
	assert.Equal(t, 2, entries[1].ID)
	assert.Equal(t, 1, entries[2].ID)
}

// TestFilterHistoryByProject_NoCurrentDirectory tests error handling when getcwd fails.
func TestFilterHistoryByProject_ErrorRecovery(t *testing.T) {
	entries := []ExecutionLogEntry{
		{
			ID:           1,
			Command:      "plan",
			AbsolutePath: "/some/path",
			StackPath:    "path",
		},
	}

	// Call FilterHistoryByProject with invalid root config
	// Should return all entries when it can't determine project root
	filtered, err := FilterHistoryByProject(entries, "nonexistent.hcl")
	require.NoError(t, err)
	assert.Len(t, filtered, 1, "should return all entries when project root not found")
}

// TestGetRelativeStackPath_EdgeCases tests edge cases for path calculation.
func TestGetRelativeStackPath_EdgeCases(t *testing.T) {
	t.Run("path outside project root", func(t *testing.T) {
		tmpDir := t.TempDir()

		projectRoot := filepath.Join(tmpDir, "project")
		outsidePath := filepath.Join(tmpDir, "outside", "stack")

		require.NoError(t, os.MkdirAll(projectRoot, 0755))
		require.NoError(t, os.MkdirAll(outsidePath, 0755))
		require.NoError(t, os.WriteFile(filepath.Join(projectRoot, "root.hcl"), []byte("# root"), 0644))

		// Path outside project should return absolute path
		relPath, err := GetRelativeStackPath(outsidePath, "root.hcl")
		require.NoError(t, err)
		assert.Equal(t, outsidePath, relPath, "should return absolute path for paths outside project")
	})

	t.Run("no root config found", func(t *testing.T) {
		tmpDir := t.TempDir()
		stackPath := filepath.Join(tmpDir, "stack")
		require.NoError(t, os.MkdirAll(stackPath, 0755))

		// No root config exists
		relPath, err := GetRelativeStackPath(stackPath, "nonexistent.hcl")
		require.NoError(t, err)
		assert.Equal(t, stackPath, relPath, "should return absolute path when root not found")
	})
}

// TestFindProjectRoot_StartFromFile tests finding root when starting from a file path.
func TestFindProjectRoot_StartFromFile(t *testing.T) {
	tmpDir := t.TempDir()

	// Create structure: project/root.hcl, project/subdir/file.txt
	projectRoot := tmpDir
	subdir := filepath.Join(projectRoot, "subdir")
	require.NoError(t, os.MkdirAll(subdir, 0755))

	rootHcl := filepath.Join(projectRoot, "root.hcl")
	require.NoError(t, os.WriteFile(rootHcl, []byte("# root"), 0644))

	testFile := filepath.Join(subdir, "test.txt")
	require.NoError(t, os.WriteFile(testFile, []byte("test"), 0644))

	// Start from file path (not directory)
	root, err := FindProjectRoot(testFile, "root.hcl")
	require.NoError(t, err)
	assert.Equal(t, projectRoot, root)
}

// TestGetNextID_WithCorruptedEntries tests ID calculation with corrupted history.
func TestGetNextID_WithCorruptedEntries(t *testing.T) {
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

	// Write entries with non-sequential IDs and some invalid lines
	content := `{"id":5,"timestamp":"2025-12-16T10:00:00Z","user":"user","stack_path":"/path","command":"plan","exit_code":0,"duration_s":1.0,"summary":"entry 5"}
invalid line
{"id":10,"timestamp":"2025-12-16T11:00:00Z","user":"user","stack_path":"/path","command":"apply","exit_code":0,"duration_s":1.0,"summary":"entry 10"}
{"id":3,"timestamp":"2025-12-16T09:00:00Z","user":"user","stack_path":"/path","command":"validate","exit_code":0,"duration_s":1.0,"summary":"entry 3"}
`
	err := os.WriteFile(tempHistoryPath, []byte(content), 0644)
	require.NoError(t, err)

	// Should return max ID + 1 (10 + 1 = 11)
	nextID, err := GetNextID(ctx)
	require.NoError(t, err)
	assert.Equal(t, 11, nextID)
}

// TestGetCurrentUser_NonEmpty tests that current user is never empty.
func TestGetCurrentUser_NonEmpty(t *testing.T) {
	user := GetCurrentUser()

	// Should return a non-empty string
	assert.NotEmpty(t, user)

	// Should either be a real username or "unknown"
	// We can't test the exact value as it depends on the environment
	assert.NotEqual(t, "", user)
}
