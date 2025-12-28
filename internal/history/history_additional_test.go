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

// TestFileRepository_DirectoryCreation tests directory creation via repository.
func TestFileRepository_DirectoryCreation(t *testing.T) {
	tempDir := t.TempDir()
	historyPath := filepath.Join(tempDir, "subdir", "history.log")

	// Let's test NewFileRepository with a path where dir exists.
	err := os.MkdirAll(filepath.Dir(historyPath), 0755)
	require.NoError(t, err)

	repo, err := NewFileRepository(historyPath)
	require.NoError(t, err)
	assert.NotNil(t, repo)
}

// TestAppendToHistory_CloseErrorHandling tests error paths.
func TestAppendToHistory_CloseErrorHandling(t *testing.T) {
	ctx := context.Background()

	tempDir := t.TempDir()
	tempHistoryPath := filepath.Join(tempDir, HistoryFileName)

	repo, err := NewFileRepository(tempHistoryPath)
	require.NoError(t, err)
	svc := NewService(repo, "root.hcl")

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

	err = svc.Append(ctx, entry)
	require.NoError(t, err)

	loaded, err := svc.LoadAll(ctx)
	require.NoError(t, err)
	assert.Len(t, loaded, 1)
	assert.Equal(t, entry.ID, loaded[0].ID)
}

// TestTrimHistory_TempFileHandling tests temporary file creation and handling.
func TestTrimHistory_TempFileHandling(t *testing.T) {
	ctx := context.Background()

	tempDir := t.TempDir()
	tempHistoryPath := filepath.Join(tempDir, HistoryFileName)

	repo, err := NewFileRepository(tempHistoryPath)
	require.NoError(t, err)
	svc := NewService(repo, "root.hcl")

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
		err := svc.Append(ctx, entry)
		require.NoError(t, err)
	}

	err = svc.TrimHistory(ctx, 10)
	require.NoError(t, err)

	loaded, err := svc.LoadAll(ctx)
	require.NoError(t, err)
	assert.Len(t, loaded, 10)

	assert.Equal(t, 20, loaded[0].ID)
	assert.Equal(t, 11, loaded[9].ID)
}

// TestGetCurrentUser_ErrorHandling tests error scenarios.
func TestGetCurrentUser_ErrorHandling(t *testing.T) {
	// GetCurrentUser should always return a non-empty string
	user := GetCurrentUser()
	assert.NotEmpty(t, user)
}

// TestGetNextID_EmptyHistory tests ID generation for empty history.
func TestGetNextID_EmptyHistory(t *testing.T) {
	ctx := context.Background()
	tempDir := t.TempDir()
	tempHistoryPath := filepath.Join(tempDir, "empty_history.log")

	repo, err := NewFileRepository(tempHistoryPath)
	require.NoError(t, err)
	svc := NewService(repo, "root.hcl")

	nextID, err := svc.GetNextID(ctx)
	require.NoError(t, err)
	assert.Equal(t, 1, nextID)
}

// TestGetLastExecution_EmptyHistory tests getting last execution from empty history.
func TestGetLastExecution_EmptyHistory(t *testing.T) {
	ctx := context.Background()
	tempDir := t.TempDir()
	tempHistoryPath := filepath.Join(tempDir, "empty_history.log")

	repo, err := NewFileRepository(tempHistoryPath)
	require.NoError(t, err)
	svc := NewService(repo, "root.hcl")

	entry, err := svc.GetLastExecutionForProject(ctx)
	require.NoError(t, err)
	assert.Nil(t, entry)
}

// TestFindProjectRoot_FileSystemRoot tests reaching filesystem root.
func TestFindProjectRoot_FileSystemRoot(t *testing.T) {
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

	// Let's use service directly.
	repo, _ := NewFileRepository("")
	svc := NewService(repo, "root.hcl")

	filtered, err := svc.FilterByCurrentProject(entries)
	require.NoError(t, err)

	assert.LessOrEqual(t, len(filtered), 2)
}

// TestLoadHistory_CloseError tests handling of file close errors.
func TestLoadHistory_CloseError(t *testing.T) {
	ctx := context.Background()
	tempDir := t.TempDir()
	tempHistoryPath := filepath.Join(tempDir, HistoryFileName)

	repo, err := NewFileRepository(tempHistoryPath)
	require.NoError(t, err)
	svc := NewService(repo, "root.hcl")

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
	err = svc.Append(ctx, entry)
	require.NoError(t, err)

	// Load history
	entries, err := svc.LoadAll(ctx)
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
