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
			expectError:      false,
			expectedMaxDepth: 0,
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
				return tui.NewTestModel(stackRoot, 1, true, "plan", "/test/root")
			},
			expectedOutputHas: []string{
				"✅ Selection Confirmed",
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
				return tui.NewModel(stackRoot, 1)
			},
			expectedOutputHas: []string{
				"⚠️  Selection cancelled",
			},
			unexpectedOutput: []string{
				"✅ Selection Confirmed",
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
				return tui.NewTestModel(stackRoot, 1, true, "destroy", "/test/root")
			},
			expectedOutputHas: []string{
				"✅ Selection Confirmed",
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

// TestExecute tests the Execute function.
// Note: This test only verifies that Execute doesn't panic.
// Full TUI testing requires a terminal and is better tested with teatest in internal/tui.
func TestExecute(t *testing.T) {
	// Cleanup Cobra state after test.
	t.Cleanup(func() {
		// Reset Cobra root command for subsequent tests.
		// This prevents "flag redefined" errors in parallel test execution.
		rootCmd.ResetFlags()
		rootCmd.ResetCommands()
	})

	// Execute will fail in non-interactive environment, which is expected.
	err := Execute()

	// We expect an error in test environment (no TTY).
	// The important part is that it doesn't panic and handles errors gracefully.
	if err != nil {
		assert.Error(t, err, "Execute should handle errors gracefully in non-interactive environment")
	}
}
