package cmd

import (
	"bytes"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/spf13/cobra"

	"github.com/israoo/terrax/internal/stack"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTreeCommand_OutputsValidJSON(t *testing.T) {
	tmpDir := t.TempDir()
	subDir := filepath.Join(tmpDir, "networking")
	require.NoError(t, os.MkdirAll(subDir, 0755))
	require.NoError(t, os.WriteFile(
		filepath.Join(subDir, "terragrunt.hcl"), []byte(""), 0644,
	))

	// Capture stdout by redirecting os.Stdout.
	oldStdout := os.Stdout
	r, w, err := os.Pipe()
	require.NoError(t, err)
	os.Stdout = w

	// Create a mock command with the dir flag.
	cmd := &cobra.Command{}
	cmd.Flags().String("dir", tmpDir, "Working directory")

	// Call runTree directly.
	treeErr := runTree(cmd, []string{})
	require.NoError(t, w.Close())
	os.Stdout = oldStdout

	// Collect output.
	var buf bytes.Buffer
	_, err = io.Copy(&buf, r)
	require.NoError(t, err)
	output := buf.String()

	require.NoError(t, treeErr)

	var root stack.Node
	require.NoError(t, json.Unmarshal([]byte(output), &root), "output must be valid JSON")
	assert.Equal(t, filepath.Base(tmpDir), root.Name)
	// Note: root.Path may differ from tmpDir on macOS due to symlink resolution (/private/tmp vs /tmp).
	assert.True(t, filepath.IsAbs(root.Path))
	require.Len(t, root.Children, 1)
	assert.Equal(t, "networking", root.Children[0].Name)
	assert.True(t, root.Children[0].IsStack)
}

func TestTreeCommand_InvalidDir(t *testing.T) {
	// Create a mock command with an invalid dir flag.
	cmd := &cobra.Command{}
	cmd.Flags().String("dir", "/nonexistent/path-that-does-not-exist", "Working directory")

	// Call runTree directly.
	err := runTree(cmd, []string{})
	assert.Error(t, err)
}
