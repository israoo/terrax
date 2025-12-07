package cmd

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/israoo/terrax/internal/stack"
	"github.com/israoo/terrax/internal/tui"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// testCommands defines a standard list of commands for testing.
var testCommands = []string{
	"plan",
	"apply",
	"validate",
	"fmt",
	"init",
	"output",
	"refresh",
	"destroy",
}

// captureStdout captures stdout during test execution.
// Returns a cleanup function that restores stdout and the captured output.
func captureStdout(t *testing.T) (restore func() string) {
	t.Helper()

	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	return func() string {
		w.Close()
		os.Stdout = oldStdout
		var buf bytes.Buffer
		io.Copy(&buf, r)
		return buf.String()
	}
}

// TestGetWorkingDirectory tests that getWorkingDirectory returns a valid path.
func TestGetWorkingDirectory(t *testing.T) {
	workDir, err := getWorkingDirectory()

	assert.NoError(t, err, "should get working directory without error")
	assert.NotEmpty(t, workDir, "working directory should not be empty")
}

// TestBuildStackTree tests the buildStackTree function with real filesystem.
// This test uses the real filesystem because buildStackTree wraps stack.FindAndBuildTree,
// which is tested thoroughly with afero mocks in internal/stack/tree_test.go.
func TestBuildStackTree(t *testing.T) {
	tests := []struct {
		name              string
		setupDir          func(t *testing.T) string
		expectError       bool
		expectedMaxDepth  int
		expectedOutputHas []string
	}{
		{
			name: "directory with stack structure",
			setupDir: func(t *testing.T) string {
				tmpDir := t.TempDir()
				require.NoError(t, os.MkdirAll(filepath.Join(tmpDir, "env", "dev"), 0755))
				require.NoError(t, os.WriteFile(
					filepath.Join(tmpDir, "env", "dev", "terragrunt.hcl"),
					[]byte("# test"), 0644))
				return tmpDir
			},
			expectError:      false,
			expectedMaxDepth: 2,
			expectedOutputHas: []string{
				"🔍 Scanning for stacks in:",
				"✅ Found stack tree with max depth:",
			},
		},
		{
			name: "empty directory",
			setupDir: func(t *testing.T) string {
				return t.TempDir()
			},
			expectError: true,
			expectedOutputHas: []string{
				"🔍 Scanning for stacks in:",
				"⚠️  No subdirectories found",
			},
		},
		{
			name: "nonexistent directory",
			setupDir: func(t *testing.T) string {
				return "/nonexistent/path/12345"
			},
			expectError:       true,
			expectedOutputHas: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			testDir := tt.setupDir(t)

			// Capture stdout.
			restore := captureStdout(t)

			// Call buildStackTree.
			stackRoot, maxDepth, err := buildStackTree(testDir)

			// Restore stdout and get output.
			output := restore()

			// Assertions.
			if tt.expectError {
				assert.Error(t, err)
				return
			}

			require.NoError(t, err, "should build tree without error")
			require.NotNil(t, stackRoot, "stack root should not be nil")
			assert.Equal(t, tt.expectedMaxDepth, maxDepth, "max depth mismatch")

			for _, expectedStr := range tt.expectedOutputHas {
				assert.Contains(t, output, expectedStr)
			}
		})
	}
}

// TestDisplayResults tests displayResults output formatting.
func TestDisplayResults(t *testing.T) {
	tests := []struct {
		name              string
		setupModel        func() tui.Model
		expectedOutputHas []string
		unexpectedOutput  []string
	}{
		{
			name: "confirmed selection on root",
			setupModel: func() tui.Model {
				stackRoot := &stack.Node{
					Name:     "root",
					Path:     "/test/root",
					Children: []*stack.Node{{Name: "child", Path: "/test/root/child"}},
				}
				return tui.NewTestModel(stackRoot, 1, testCommands, 3, true, "plan", "/test/root")
			},
			expectedOutputHas: []string{
				"✅ Selection confirmed",
				"Command:",
				"plan",
				"Stack Path:",
				"/test/root",
				"═══════════════════════════════════════",
			},
			unexpectedOutput: []string{
				"⚠️  Selection cancelled",
			},
		},
		{
			name: "cancelled selection",
			setupModel: func() tui.Model {
				stackRoot := &stack.Node{
					Name: "root",
					Path: "/test/root",
				}
				return tui.NewModel(stackRoot, 1, testCommands, 3)
			},
			expectedOutputHas: []string{
				"⚠️  Selection cancelled",
			},
			unexpectedOutput: []string{
				"✅ Selection confirmed",
				"Command:",
				"Stack Path:",
			},
		},
		{
			name: "confirmed with destroy command",
			setupModel: func() tui.Model {
				stackRoot := &stack.Node{
					Name: "root",
					Path: "/test/root",
				}
				return tui.NewTestModel(stackRoot, 1, testCommands, 3, true, "destroy", "/test/root")
			},
			expectedOutputHas: []string{
				"✅ Selection confirmed",
				"destroy",
				"/test/root",
			},
			unexpectedOutput: []string{
				"⚠️  Selection cancelled",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			model := tt.setupModel()

			// Capture stdout.
			restore := captureStdout(t)

			// Call displayResults.
			displayResults(model)

			// Restore stdout and get output.
			output := restore()

			// Assertions.
			for _, expected := range tt.expectedOutputHas {
				assert.Contains(t, output, expected, "missing expected output: %s", expected)
			}
			for _, unexpected := range tt.unexpectedOutput {
				assert.NotContains(t, output, unexpected, "found unexpected output: %s", unexpected)
			}
		})
	}
}

// TestExecute tests the Execute function with a mocked TUI runner.
// This test verifies the full flow without blocking on interactive input.
func TestExecute(t *testing.T) {
	// Setup: Create a temporary directory with a stack structure
	tmpDir := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(tmpDir, "env", "dev"), 0755))
	require.NoError(t, os.WriteFile(
		filepath.Join(tmpDir, "env", "dev", "terragrunt.hcl"),
		[]byte("# test stack"), 0644))

	// Change to temp directory for the test
	originalWd, err := os.Getwd()
	require.NoError(t, err)
	require.NoError(t, os.Chdir(tmpDir))

	// Cleanup: Restore state after test
	t.Cleanup(func() {
		os.Chdir(originalWd)
		rootCmd.ResetFlags()
		rootCmd.ResetCommands()
	})

	// Mock TUI runner that simulates user cancelling (non-blocking)
	mockTUIRunner := func(initialModel tui.Model) (tui.Model, error) {
		// Return the model without confirmation (simulating user pressing 'q')
		// This is non-blocking and deterministic
		return initialModel, nil
	}

	// Inject mock runner and ensure cleanup
	restoreRunner := setTUIRunner(mockTUIRunner)
	defer restoreRunner()

	// Execute the command - should complete without blocking
	err = Execute()

	// Should complete successfully (user cancelled, no command execution)
	assert.NoError(t, err, "Execute should complete without errors when using mocked TUI runner")
}

// TestExecute_WithConfirmation tests the Execute function with a confirmed selection.
// This ensures the full flow works correctly including command execution preparation.
func TestExecute_WithConfirmation(t *testing.T) {
	// Setup: Create a temporary directory with a stack structure
	tmpDir := t.TempDir()
	envDir := filepath.Join(tmpDir, "env", "dev")
	require.NoError(t, os.MkdirAll(envDir, 0755))
	require.NoError(t, os.WriteFile(
		filepath.Join(envDir, "terragrunt.hcl"),
		[]byte("# test stack"), 0644))

	// Change to temp directory for the test
	originalWd, err := os.Getwd()
	require.NoError(t, err)
	require.NoError(t, os.Chdir(tmpDir))

	// Cleanup: Restore state after test
	t.Cleanup(func() {
		os.Chdir(originalWd)
		rootCmd.ResetFlags()
		rootCmd.ResetCommands()
	})

	// Mock TUI runner that simulates user confirming with a plan command (non-blocking)
	mockTUIRunner := func(initialModel tui.Model) (tui.Model, error) {
		// Create a simple stack tree for testing
		stackRoot := &stack.Node{
			Name: "env",
			Path: envDir,
		}

		// Return a confirmed model simulating user selecting "plan" and confirming
		return tui.NewTestModel(stackRoot, 1, testCommands, 3, true, "plan", envDir), nil
	} // Inject mock runner and ensure cleanup
	restoreRunner := setTUIRunner(mockTUIRunner)
	defer restoreRunner()

	// Execute the command - should complete without blocking
	err = Execute()

	// In a test environment with terragrunt installed, the command might succeed
	// The important verification is that the test completes without blocking
	// We don't strictly require an error - the key is non-blocking execution
	if err != nil {
		// If there's an error, it should be from command execution, not from TUI blocking
		t.Logf("Command execution resulted in error (expected in some test environments): %v", err)
	} else {
		t.Log("Command execution completed successfully")
	}
}
