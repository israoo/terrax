package cmd

import (
	"bytes"
	"encoding/json"
	"io"
	"os"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/israoo/terrax/internal/tui"
)

func TestHistoryCommand_OutputsValidJSON(t *testing.T) {
	tmpDir := t.TempDir()

	// Save the original working directory and restore it after the test.
	originalWd, err := os.Getwd()
	require.NoError(t, err)
	t.Cleanup(func() {
		_ = os.Chdir(originalWd)
	})

	// Redirect stdout to a buffer instead of using pipes.
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	// Create a channel to get the output after close.
	done := make(chan string, 1)
	go func() {
		var buf bytes.Buffer
		_, _ = io.Copy(&buf, r)
		done <- buf.String()
	}()

	// Create a real cobra command with the required flags.
	cmd := &cobra.Command{}
	cmd.Flags().String("dir", "", "Working directory (overrides current directory)")
	cmd.Flags().Bool("json", false, "")
	_ = cmd.ParseFlags([]string{"--dir", tmpDir, "--json"})

	// Run the history command.
	err = runHistoryCmd(cmd, []string{})

	// Close stdout and wait for goroutine to finish.
	_ = w.Close()
	os.Stdout = oldStdout
	output := <-done

	require.NoError(t, err)

	// Output must be a valid JSON array (may be empty if no history file exists).
	var entries []map[string]interface{}
	require.NoError(t,
		json.Unmarshal([]byte(output), &entries),
		"output must be a valid JSON array, got: %s", output,
	)
}

func TestHistoryCommand_JSONFlag_OutputsJSON(t *testing.T) {
	tmpDir := t.TempDir()

	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	done := make(chan string, 1)
	go func() {
		var buf bytes.Buffer
		_, _ = io.Copy(&buf, r)
		done <- buf.String()
	}()

	cmd := &cobra.Command{}
	cmd.Flags().String("dir", "", "")
	cmd.Flags().Bool("json", false, "")
	_ = cmd.ParseFlags([]string{"--dir", tmpDir, "--json"})

	err := runHistoryCmd(cmd, []string{})

	_ = w.Close()
	os.Stdout = oldStdout
	output := <-done

	require.NoError(t, err)

	var entries []map[string]interface{}
	require.NoError(t,
		json.Unmarshal([]byte(output), &entries),
		"output with --json must be a valid JSON array, got: %s", output,
	)
}

func TestHistoryCommand_NoJSONFlag_DoesNotOutputJSON(t *testing.T) {
	// Without --json, runHistoryCmd must route to the TUI path (not JSON output).
	// Verify by checking that the TUI runner is called, not the JSON writer.
	tuiCalled := false
	restore := setHistoryTUIRunner(func(model tui.Model) (tui.Model, error) {
		tuiCalled = true
		return tui.Model{}, nil
	})
	t.Cleanup(restore)

	tmpDir := t.TempDir()

	cmd := &cobra.Command{}
	cmd.Flags().String("dir", "", "")
	cmd.Flags().Bool("json", false, "")
	require.NoError(t, cmd.ParseFlags([]string{"--dir", tmpDir}))

	err := runHistoryCmd(cmd, []string{})

	require.NoError(t, err)
	assert.True(t, tuiCalled, "without --json flag the TUI runner must be invoked")
}
