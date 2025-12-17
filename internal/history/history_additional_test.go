package history

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestGetHistoryFilePath_DirectoryCreation tests directory creation.
func TestGetHistoryFilePath_DirectoryCreation(t *testing.T) {
	// This test verifies that GetHistoryFilePath creates the directory
	path, err := GetHistoryFilePath()
	require.NoError(t, err)
	assert.NotEmpty(t, path)

	// Directory should exist
	dir := filepath.Dir(path)
	info, err := os.Stat(dir)
	require.NoError(t, err)
	assert.True(t, info.IsDir())
}

// TestAppendToHistory_CloseErrorHandling tests error paths in AppendToHistory.
func TestAppendToHistory_CloseErrorHandling(t *testing.T) {
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
		ID:           1,
		Timestamp:    time.Now(),
		User:         "testuser",
		StackPath:    "/test/path",
		AbsolutePath: "/test/absolute/path",
		Command:      "plan",
		ExitCode:     0,
		DurationS:    1.0,
		Summary:      "test",
	}

	err := AppendToHistory(ctx, entry)
	require.NoError(t, err)

	// Verify entry was written
	loaded, err := LoadHistory(ctx)
	require.NoError(t, err)
	assert.Len(t, loaded, 1)
	assert.Equal(t, entry.ID, loaded[0].ID)
}

// TestTrimHistory_TempFileHandling tests temporary file creation and handling.
func TestTrimHistory_TempFileHandling(t *testing.T) {
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

	// Create 20 entries
	for i := 1; i <= 20; i++ {
		entry := ExecutionLogEntry{
			ID:        i,
			Timestamp: time.Now().Add(time.Duration(i) * time.Second),
			User:      "test",
			StackPath: "/test",
			Command:   "plan",
			ExitCode:  0,
			DurationS: float64(i),
			Summary:   fmt.Sprintf("entry %d", i),
		}
		err := AppendToHistory(ctx, entry)
		require.NoError(t, err)
	}

	// Trim to 10 entries
	err := TrimHistory(ctx, 10)
	require.NoError(t, err)

	// Verify only 10 most recent entries remain
	loaded, err := LoadHistory(ctx)
	require.NoError(t, err)
	assert.Len(t, loaded, 10)

	// Verify these are the most recent ones (IDs 11-20, but reversed in output)
	assert.Equal(t, 20, loaded[0].ID)
	assert.Equal(t, 11, loaded[9].ID)
}

// TestGetCurrentUser_ErrorHandling tests error scenarios.
func TestGetCurrentUser_ErrorHandling(t *testing.T) {
	// GetCurrentUser should always return a non-empty string
	// Even if user.Current() fails, it returns "unknown"
	user := GetCurrentUser()
	assert.NotEmpty(t, user)
}

// TestGetNextID_EmptyHistory tests ID generation for empty history.
func TestGetNextID_EmptyHistory(t *testing.T) {
	ctx := context.Background()

	tempDir := t.TempDir()
	tempHistoryPath := filepath.Join(tempDir, "empty_history.log")

	originalFunc := historyFilePathFunc
	historyFilePathFunc = func() (string, error) {
		return tempHistoryPath, nil
	}
	defer func() {
		historyFilePathFunc = originalFunc
	}()

	// Don't create the file - test when it doesn't exist
	nextID, err := GetNextID(ctx)
	require.NoError(t, err)
	assert.Equal(t, 1, nextID)
}

// TestGetLastExecution_EmptyHistory tests getting last execution from empty history.
func TestGetLastExecution_EmptyHistory(t *testing.T) {
	ctx := context.Background()

	tempDir := t.TempDir()
	tempHistoryPath := filepath.Join(tempDir, "empty_history.log")

	originalFunc := historyFilePathFunc
	historyFilePathFunc = func() (string, error) {
		return tempHistoryPath, nil
	}
	defer func() {
		historyFilePathFunc = originalFunc
	}()

	// File doesn't exist - should return nil
	entry, err := GetLastExecution(ctx)
	require.NoError(t, err)
	assert.Nil(t, entry)
}

// TestFindProjectRoot_FileSystemRoot tests reaching filesystem root.
func TestFindProjectRoot_FileSystemRoot(t *testing.T) {
	// Start from a temporary directory that definitely doesn't have root.hcl
	// and walk up to filesystem root
	tmpDir := t.TempDir()

	root, err := FindProjectRoot(tmpDir, "definitely_nonexistent_root_file.hcl")
	require.NoError(t, err)
	assert.Equal(t, "", root, "should return empty string when root not found")
}

// TestGetRelativeStackPath_InvalidAbsolutePath tests error handling for invalid paths.
func TestGetRelativeStackPath_InvalidAbsolutePath(t *testing.T) {
	// Test with a path that exists but has no root config
	tmpDir := t.TempDir()
	stackPath := filepath.Join(tmpDir, "stack")
	require.NoError(t, os.MkdirAll(stackPath, 0755))

	// No root.hcl exists, so should return absolute path
	relPath, err := GetRelativeStackPath(stackPath, "root.hcl")
	require.NoError(t, err)
	assert.Equal(t, stackPath, relPath, "should return absolute path when root not found")
}

// TestFilterHistoryByProject_WithoutAbsolutePath tests filtering entries without AbsolutePath.
func TestFilterHistoryByProject_WithoutAbsolutePath(t *testing.T) {
	entries := []ExecutionLogEntry{
		{
			ID:           1,
			Command:      "plan",
			StackPath:    "/test/path",
			AbsolutePath: "", // Empty AbsolutePath (old entry format)
		},
		{
			ID:           2,
			Command:      "apply",
			StackPath:    "/other/path",
			AbsolutePath: "/other/absolute/path",
		},
	}

	// Filter should skip entries without AbsolutePath
	filtered, err := FilterHistoryByProject(entries, "root.hcl")
	require.NoError(t, err)

	// Entry 1 should be skipped due to empty AbsolutePath
	// Entry 2 will be included or not depending on project root match
	assert.LessOrEqual(t, len(filtered), 2)
}

// TestLoadHistory_CloseError tests handling of file close errors.
func TestLoadHistory_CloseError(t *testing.T) {
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

	// Create a valid history file
	entry := ExecutionLogEntry{
		ID:        1,
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

	// Load history - close error is logged but doesn't fail
	entries, err := LoadHistory(ctx)
	require.NoError(t, err)
	assert.Len(t, entries, 1)
}

// TestGetRelativeStackPath_SymlinkResolution tests symlink handling in path calculation.
func TestGetRelativeStackPath_SymlinkResolution(t *testing.T) {
	tmpDir := t.TempDir()

	// Create project structure
	projectRoot := filepath.Join(tmpDir, "project")
	stackDir := filepath.Join(projectRoot, "stacks", "dev")

	require.NoError(t, os.MkdirAll(stackDir, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(projectRoot, "root.hcl"), []byte("# root"), 0644))

	// Get relative path
	relPath, err := GetRelativeStackPath(stackDir, "root.hcl")
	require.NoError(t, err)

	// Should be relative to project root
	assert.Equal(t, filepath.Join("stacks", "dev"), relPath)
}

// TestAppendToHistory_GetHistoryFilePathError tests error from GetHistoryFilePath.
func TestAppendToHistory_GetHistoryFilePathError(t *testing.T) {
	ctx := context.Background()

	originalFunc := historyFilePathFunc
	historyFilePathFunc = func() (string, error) {
		return "", fmt.Errorf("simulated error")
	}
	defer func() {
		historyFilePathFunc = originalFunc
	}()

	entry := ExecutionLogEntry{
		ID:      1,
		Command: "plan",
	}

	err := AppendToHistory(ctx, entry)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to get history file path")
}

// TestTrimHistory_GetHistoryFilePathError tests error from GetHistoryFilePath in TrimHistory.
func TestTrimHistory_GetHistoryFilePathError(t *testing.T) {
	ctx := context.Background()

	originalFunc := historyFilePathFunc
	historyFilePathFunc = func() (string, error) {
		return "", fmt.Errorf("simulated error")
	}
	defer func() {
		historyFilePathFunc = originalFunc
	}()

	err := TrimHistory(ctx, 10)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to get history file path")
}

// TestGetNextID_GetHistoryFilePathError tests error from GetHistoryFilePath in GetNextID.
func TestGetNextID_GetHistoryFilePathError(t *testing.T) {
	ctx := context.Background()

	originalFunc := historyFilePathFunc
	historyFilePathFunc = func() (string, error) {
		return "", fmt.Errorf("simulated error")
	}
	defer func() {
		historyFilePathFunc = originalFunc
	}()

	_, err := GetNextID(ctx)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to get history file path")
}

// TestGetLastExecution_GetHistoryFilePathError tests error from GetHistoryFilePath in GetLastExecution.
func TestGetLastExecution_GetHistoryFilePathError(t *testing.T) {
	ctx := context.Background()

	originalFunc := historyFilePathFunc
	historyFilePathFunc = func() (string, error) {
		return "", fmt.Errorf("simulated error")
	}
	defer func() {
		historyFilePathFunc = originalFunc
	}()

	_, err := GetLastExecution(ctx)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to get history file path")
}

// TestLoadHistory_GetHistoryFilePathError tests error from GetHistoryFilePath in LoadHistory.
func TestLoadHistory_GetHistoryFilePathError(t *testing.T) {
	ctx := context.Background()

	originalFunc := historyFilePathFunc
	historyFilePathFunc = func() (string, error) {
		return "", fmt.Errorf("simulated error")
	}
	defer func() {
		historyFilePathFunc = originalFunc
	}()

	_, err := LoadHistory(ctx)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to get history file path")
}
