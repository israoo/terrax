package cmd

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestInitConfig_WithConfigFile tests that initConfig reads from a config file.
func TestInitConfig_WithConfigFile(t *testing.T) {
	// Create a temporary directory with a config file
	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, ".terrax.yaml")

	configContent := `commands:
  - apply
  - plan
  - destroy
`
	require.NoError(t, os.WriteFile(configFile, []byte(configContent), 0644))

	// Change to temp directory
	originalWd, err := os.Getwd()
	require.NoError(t, err)
	require.NoError(t, os.Chdir(tmpDir))
	t.Cleanup(func() {
		os.Chdir(originalWd)
		viper.Reset() // Reset viper state
	})

	// Initialize config
	initConfig()

	// Verify that config was read
	commands := viper.GetStringSlice("commands")
	assert.Equal(t, []string{"apply", "plan", "destroy"}, commands)
}

// TestInitConfig_WithoutConfigFile tests that initConfig uses defaults when no config file exists.
func TestInitConfig_WithoutConfigFile(t *testing.T) {
	// Create a temporary directory without a config file
	tmpDir := t.TempDir()

	// Change to temp directory
	originalWd, err := os.Getwd()
	require.NoError(t, err)
	require.NoError(t, os.Chdir(tmpDir))
	t.Cleanup(func() {
		os.Chdir(originalWd)
		viper.Reset() // Reset viper state
	})

	// Initialize config
	initConfig()

	// Verify that default commands are used
	commands := viper.GetStringSlice("commands")
	assert.Equal(t, defaultCommands, commands)
}

// TestInitConfig_EmptyCommands tests fallback when config has empty commands list.
func TestInitConfig_EmptyCommands(t *testing.T) {
	// Create a temporary directory with a config file with empty commands
	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, ".terrax.yaml")

	configContent := `commands: []
`
	require.NoError(t, os.WriteFile(configFile, []byte(configContent), 0644))

	// Change to temp directory
	originalWd, err := os.Getwd()
	require.NoError(t, err)
	require.NoError(t, os.Chdir(tmpDir))
	t.Cleanup(func() {
		os.Chdir(originalWd)
		viper.Reset() // Reset viper state
	})

	// Initialize config
	initConfig()

	// Verify empty list is handled
	commands := viper.GetStringSlice("commands")

	// In our implementation, empty commands will trigger fallback in runTUI
	// Here we just verify viper returns empty as expected
	assert.Empty(t, commands)
}
