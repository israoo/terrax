package cmd

import (
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
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
