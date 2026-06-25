package cmd

import (
	"bytes"
	"io"
	"os"
	"testing"

	"github.com/adrg/xdg"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLastCmd_NoHistory(t *testing.T) {
	// Redirect XDG_CONFIG_HOME to an empty temp dir so the history service finds no entries.
	tmpDir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmpDir)
	xdg.Reload()
	t.Cleanup(func() {
		_ = os.Unsetenv("XDG_CONFIG_HOME")
		xdg.Reload()
	})

	// Capture stdout.
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
	err := runLastCmd(cmd, []string{})

	_ = w.Close()
	os.Stdout = oldStdout
	output := <-done

	require.NoError(t, err)
	assert.Contains(t, output, "No execution history found")
}
