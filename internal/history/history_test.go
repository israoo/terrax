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

// Helper to split lines
func splitLines(s string) []string {
	var lines []string
	for _, line := range strings.Split(s, "\n") {
		if line != "" {
			lines = append(lines, line)
		}
	}
	return lines
}

func TestGetHistoryFilePath(t *testing.T) {
	// This tests the DEFAULT path logic
	path, err := GetDefaultHistoryFilePath()
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

	repo, err := NewFileRepository(tempHistoryPath)
	require.NoError(t, err)
	svc := NewService(repo, "root.hcl")

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
			err := svc.Append(ctx, tt.entry)
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

			repo, err := NewFileRepository(tempHistoryPath)
			require.NoError(t, err)
			svc := NewService(repo, "root.hcl")

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
				err := svc.Append(ctx, entry)
				require.NoError(t, err)
			}

			// Execute trim
			err = svc.TrimHistory(ctx, tt.maxEntries)

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

	repo, err := NewFileRepository(tempHistoryPath)
	require.NoError(t, err)
	svc := NewService(repo, "root.hcl")

	// Should not error when file doesn't exist
	err = svc.TrimHistory(ctx, 10)
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

		repo, err := NewFileRepository(tempHistoryPath)
		require.NoError(t, err)
		svc := NewService(repo, "root.hcl")

		id, err := svc.GetNextID(ctx)
		require.NoError(t, err)
		assert.Equal(t, 1, id)
	})

	t.Run("returns max ID + 1", func(t *testing.T) {
		tempDir := t.TempDir()
		tempHistoryPath := filepath.Join(tempDir, HistoryFileName)

		repo, err := NewFileRepository(tempHistoryPath)
		require.NoError(t, err)
		svc := NewService(repo, "root.hcl")

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
			}
			err := svc.Append(ctx, entry)
			require.NoError(t, err)
		}

		id, err := svc.GetNextID(ctx)
		require.NoError(t, err)
		assert.Equal(t, 6, id)
	})
}

func TestAppendAndTrimIntegration(t *testing.T) {
	ctx := context.Background()

	tempDir := t.TempDir()
	tempHistoryPath := filepath.Join(tempDir, HistoryFileName)

	repo, err := NewFileRepository(tempHistoryPath)
	require.NoError(t, err)
	svc := NewService(repo, "root.hcl")

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
		}
		err := svc.Append(ctx, entry)
		require.NoError(t, err)
	}

	// Verify all entries were added
	data, err := os.ReadFile(tempHistoryPath)
	require.NoError(t, err)
	lines := splitLines(string(data))
	assert.Equal(t, totalExecutions, len(lines))

	// Trim to keep only the last maxEntries
	err = svc.TrimHistory(ctx, maxEntries)
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
}

func TestGetLastExecution(t *testing.T) {
	ctx := context.Background()

	t.Run("empty file returns nil", func(t *testing.T) {
		tempDir := t.TempDir()
		tempHistoryPath := filepath.Join(tempDir, HistoryFileName)

		repo, err := NewFileRepository(tempHistoryPath)
		require.NoError(t, err)
		svc := NewService(repo, "root.hcl")

		entry, err := svc.GetLastExecutionForProject(ctx)
		require.NoError(t, err)
		assert.Nil(t, entry)
	})

	t.Run("returns entry with highest ID", func(t *testing.T) {
		tempDir := t.TempDir()
		tempHistoryPath := filepath.Join(tempDir, HistoryFileName)

		repo, err := NewFileRepository(tempHistoryPath)
		require.NoError(t, err)
		svc := NewService(repo, "root.hcl")

		// Create a mock project root so filtering works
		require.NoError(t, os.WriteFile(filepath.Join(tempDir, "root.hcl"), []byte("# root"), 0644))

		// Create stack directories so EvalSymlinks works (required for macOS /var vs /private/var match)
		require.NoError(t, os.MkdirAll(filepath.Join(tempDir, "path/1"), 0755))
		require.NoError(t, os.MkdirAll(filepath.Join(tempDir, "path/5"), 0755))

		// Add entries with non-sequential IDs
		entries := []ExecutionLogEntry{
			{
				ID:           1,
				Timestamp:    time.Date(2025, 12, 16, 10, 0, 0, 0, time.UTC),
				User:         "user1",
				StackPath:    "/path/1",
				AbsolutePath: filepath.Join(tempDir, "path/1"),
				Command:      "plan",
				ExitCode:     0,
				DurationS:    1.0,
			},
			{
				ID:           5,
				Timestamp:    time.Date(2025, 12, 16, 11, 0, 0, 0, time.UTC),
				User:         "user2",
				StackPath:    "/path/5",
				AbsolutePath: filepath.Join(tempDir, "path/5"),
				Command:      "apply",
				ExitCode:     0,
				DurationS:    2.0,
			},
		}

		// Change WD to project root for filtering to work
		origWd, _ := os.Getwd()
		defer func() {
			_ = os.Chdir(origWd)
		}()
		require.NoError(t, os.Chdir(tempDir))

		for _, entry := range entries {
			err := svc.Append(ctx, entry)
			require.NoError(t, err)
		}

		// Should return entry with ID 5 (highest, which is first in loaded list)
		lastEntry, err := svc.GetLastExecutionForProject(ctx)
		require.NoError(t, err)
		require.NotNil(t, lastEntry)
		assert.Equal(t, 5, lastEntry.ID)
	})
}

func TestFindProjectRoot(t *testing.T) {
	// Create temporary directory structure
	tmpDir := t.TempDir()

	// Create project structure:
	// tmpDir/
	//   root.hcl
	//   dev/
	projectRoot := tmpDir
	devDir := filepath.Join(tmpDir, "dev")
	require.NoError(t, os.MkdirAll(devDir, 0755))

	// Create root.hcl at project root
	rootHclPath := filepath.Join(projectRoot, "root.hcl")
	require.NoError(t, os.WriteFile(rootHclPath, []byte("# root config"), 0644))

	tests := []struct {
		name           string
		startPath      string
		rootConfigFile string
		expectedRoot   string
	}{
		{
			name:           "find root from deep nested path",
			startPath:      devDir,
			rootConfigFile: "root.hcl",
			expectedRoot:   projectRoot,
		},
		{
			name:           "find root from project root",
			startPath:      projectRoot,
			rootConfigFile: "root.hcl",
			expectedRoot:   projectRoot,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			root, err := FindProjectRoot(tt.startPath, tt.rootConfigFile)
			require.NoError(t, err)
			assert.Equal(t, tt.expectedRoot, root)
		})
	}
}

func TestGetRelativeStackPath(t *testing.T) {
	// Create temporary directory structure
	tmpDir := t.TempDir()

	projectRoot := tmpDir
	devDir := filepath.Join(tmpDir, "dev")
	stackDir := filepath.Join(devDir, "vpc")

	require.NoError(t, os.MkdirAll(stackDir, 0755))

	// Create root.hcl at project root
	rootHclPath := filepath.Join(projectRoot, "root.hcl")
	require.NoError(t, os.WriteFile(rootHclPath, []byte("# root config"), 0644))

	tests := []struct {
		name            string
		absolutePath    string
		rootConfigFile  string
		expectedRelPath string
	}{
		{
			name:            "calculate relative path from deep nested stack",
			absolutePath:    stackDir,
			rootConfigFile:  "root.hcl",
			expectedRelPath: filepath.Join("dev", "vpc"),
		},
		{
			name:            "project root returns dot",
			absolutePath:    projectRoot,
			rootConfigFile:  "root.hcl",
			expectedRelPath: ".",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			relPath, err := GetRelativeStackPath(tt.absolutePath, tt.rootConfigFile)
			require.NoError(t, err)
			assert.Equal(t, tt.expectedRelPath, relPath)
		})
	}
}

func TestFilterHistoryByProject(t *testing.T) {
	// Create temporary directory structure
	tmpDir := t.TempDir()

	// Create two separate projects
	project1 := filepath.Join(tmpDir, "project1")
	project2 := filepath.Join(tmpDir, "project2")

	require.NoError(t, os.MkdirAll(filepath.Join(project1, "dev/vpc"), 0755))
	require.NoError(t, os.MkdirAll(filepath.Join(project2, "prod/rds"), 0755))

	// Create root.hcl in both projects
	require.NoError(t, os.WriteFile(filepath.Join(project1, "root.hcl"), []byte("# project 1"), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(project2, "root.hcl"), []byte("# project 2"), 0644))

	// Create history entries
	entries := []ExecutionLogEntry{
		{
			ID:           1,
			AbsolutePath: filepath.Join(project1, "dev/vpc"),
		},
		{
			ID:           3,
			AbsolutePath: filepath.Join(project2, "prod/rds"),
		},
	}

	repo, _ := NewFileRepository("")
	svc := NewService(repo, "root.hcl")

	// Save current directory and change to project1
	originalDir, err := os.Getwd()
	require.NoError(t, err)
	defer func() {
		_ = os.Chdir(originalDir)
	}()

	require.NoError(t, os.Chdir(filepath.Join(project1, "dev/vpc")))

	filtered, err := svc.FilterByCurrentProject(entries)
	require.NoError(t, err)
	assert.Len(t, filtered, 1)
	assert.Equal(t, 1, filtered[0].ID)

	// Change to project2
	require.NoError(t, os.Chdir(filepath.Join(project2, "prod/rds")))
	filtered, err = svc.FilterByCurrentProject(entries)
	require.NoError(t, err)
	assert.Len(t, filtered, 1)
	assert.Equal(t, 3, filtered[0].ID)
}

func TestHasPrefix(t *testing.T) {
	// Helper internal function, testing indirectly or we can export it if needed?
	// It is unexported 'hasPrefix'. Test file is "package history" so it can access it.

	tests := []struct {
		name     string
		path     string
		prefix   string
		expected bool
	}{
		{
			name:     "exact match",
			path:     "/path/to/project",
			prefix:   "/path/to/project",
			expected: true,
		},
		{
			name:     "path has prefix with subdirectory",
			path:     "/path/to/project/subdir",
			prefix:   "/path/to/project",
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := hasPrefix(tt.path, tt.prefix)
			assert.Equal(t, tt.expected, result)
		})
	}
}
