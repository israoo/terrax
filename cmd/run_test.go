package cmd

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/israoo/terrax/internal/config"
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
	tmpDir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "terragrunt.hcl"), []byte(""), 0644))

	viper.SetDefault("commands", config.DefaultCommands)
	t.Cleanup(viper.Reset)

	cmd := &cobra.Command{}
	cmd.Flags().String("dir", "", "")
	cmd.Flags().String("plans-dir", "", "")
	require.NoError(t, cmd.ParseFlags([]string{"--dir", tmpDir, "--plans-dir", "/custom/plans"}))

	// runCommand will fail (no valid stack) but viper.Set for --plans-dir
	// must happen before collectTransitiveDeps — assert it did.
	_ = runCommand(cmd, []string{"plan"})

	assert.Equal(t, "/custom/plans", viper.GetString("plan.json_out_dir"))
}
