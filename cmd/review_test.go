package cmd

import (
	"os"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestReviewCmd_NoPlanDir(t *testing.T) {
	tmpDir := t.TempDir()

	cmd := &cobra.Command{}
	cmd.Flags().String("dir", "", "")
	require.NoError(t, cmd.ParseFlags([]string{"--dir", tmpDir}))

	err := runReviewCmd(cmd, []string{})

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no plan results found")
}

func TestReviewCmd_UsesCwd(t *testing.T) {
	// When --dir is not set, runReviewCmd uses the current working directory.
	// A fresh temp dir has no plan output, so the same error is expected.
	originalWd, err := os.Getwd()
	require.NoError(t, err)

	// t.TempDir() must be registered before the Chdir cleanup so that LIFO order
	// restores the original working directory before Go attempts to remove the temp
	// directory — on Windows a directory cannot be deleted while it is the cwd.
	tmpDir := t.TempDir()
	t.Cleanup(func() { _ = os.Chdir(originalWd) })

	require.NoError(t, os.Chdir(tmpDir))

	cmd := &cobra.Command{}
	cmd.Flags().String("dir", "", "")
	require.NoError(t, cmd.ParseFlags([]string{}))

	err = runReviewCmd(cmd, []string{})
	assert.Error(t, err)
}
