package cmd

import (
	"bytes"
	"encoding/json"
	"io"
	"os"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/require"
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
	_ = cmd.ParseFlags([]string{"--dir", tmpDir})

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
