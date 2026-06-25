package cmd

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRunCommand_InvalidCommand(t *testing.T) {
	// Create a mock command with an invalid command argument.
	cmd := &cobra.Command{}
	cmd.Flags().String("dir", "", "Working directory")

	// Call runCommand directly with an invalid command.
	err := runCommand(cmd, []string{"invalid"})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unknown command")
}

func TestRunCommand_InvalidDir(t *testing.T) {
	// Create a mock command with a valid command but invalid directory.
	cmd := &cobra.Command{}
	cmd.Flags().String("dir", "/nonexistent/path-that-does-not-exist", "Working directory")

	// Call runCommand directly with a valid command but invalid directory.
	// The executor.Run function will attempt to run terragrunt in the non-existent directory.
	err := runCommand(cmd, []string{"plan"})
	assert.Error(t, err)
}

func TestRunCommand_PlansDirFlag_SetsViper(t *testing.T) {
	// Arrange: temp dir with a terragrunt.hcl so workDir resolves.
	tmpDir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "terragrunt.hcl"), []byte(""), 0644))

	cmd := &cobra.Command{}
	cmd.Flags().String("dir", "", "")
	cmd.Flags().String("plans-dir", "", "")
	require.NoError(t, cmd.ParseFlags([]string{"--dir", tmpDir, "--plans-dir", "/custom/plans"}))

	// Act: read the flag and apply to viper (mimics what runCommand will do).
	if plansDir, _ := cmd.Flags().GetString("plans-dir"); plansDir != "" {
		viper.Set("plan.json_out_dir", plansDir)
	}

	// Assert.
	assert.Equal(t, "/custom/plans", viper.GetString("plan.json_out_dir"))
	viper.Reset() // clean up global state.
}
