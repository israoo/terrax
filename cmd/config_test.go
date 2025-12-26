package cmd

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/israoo/terrax/internal/config"
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
		_ = os.Chdir(originalWd)
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
		_ = os.Chdir(originalWd)
		viper.Reset() // Reset viper state
	})

	// Initialize config
	initConfig()

	// Verify that default commands are used
	commands := viper.GetStringSlice("commands")
	assert.Equal(t, config.DefaultCommands, commands)
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
		_ = os.Chdir(originalWd)
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

// TestInitConfig_MaxNavigationColumns tests max_navigation_columns configuration.
func TestInitConfig_MaxNavigationColumns(t *testing.T) {
	tests := []struct {
		name          string
		configContent string
		expectedValue int
		shouldUseDef  bool
	}{
		{
			name: "custom value in config",
			configContent: `max_navigation_columns: 5
commands:
  - plan
  - apply
`,
			expectedValue: 5,
			shouldUseDef:  false,
		},
		{
			name: "no value in config - uses default",
			configContent: `commands:
  - plan
  - apply
`,
			expectedValue: 3,
			shouldUseDef:  true,
		},
		{
			name: "invalid value (0) in config",
			configContent: `max_navigation_columns: 0
commands:
  - plan
`,
			expectedValue: 0, // Viper will read it, but runTUI will fallback to 3
			shouldUseDef:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a temporary directory with a config file
			tmpDir := t.TempDir()
			configFile := filepath.Join(tmpDir, ".terrax.yaml")

			require.NoError(t, os.WriteFile(configFile, []byte(tt.configContent), 0644))

			// Change to temp directory
			originalWd, err := os.Getwd()
			require.NoError(t, err)
			require.NoError(t, os.Chdir(tmpDir))
			defer func() {
				_ = os.Chdir(originalWd)
				viper.Reset() // Reset viper state
			}()

			// Initialize config
			initConfig()

			// Verify that max_navigation_columns was read correctly
			maxNavCols := viper.GetInt("max_navigation_columns")
			assert.Equal(t, tt.expectedValue, maxNavCols)
		})
	}
}

// TestBuildTerragruntArgs tests the buildTerragruntArgs function with different configurations.
func TestBuildTerragruntArgs(t *testing.T) {
	tests := []struct {
		name         string
		stackPath    string
		command      string
		logLevel     string
		logFormat    string
		logCustomFmt string
		expected     []string
	}{
		{
			name:      "basic command without logging config",
			stackPath: "/path/to/stack",
			command:   "plan",
			expected:  []string{"run", "--all", "--working-dir", "/path/to/stack", "--log-format", "pretty", "--", "plan"},
		},
		{
			name:      "command with log level",
			stackPath: "/path/to/stack",
			command:   "apply",
			logLevel:  "debug",
			expected:  []string{"run", "--all", "--working-dir", "/path/to/stack", "--log-level", "debug", "--log-format", "pretty", "--", "apply"},
		},
		{
			name:      "command with custom log format",
			stackPath: "/path/to/stack",
			command:   "destroy",
			logFormat: "json",
			expected:  []string{"run", "--all", "--working-dir", "/path/to/stack", "--log-format", "json", "--", "destroy"},
		},
		{
			name:      "command with all logging options",
			stackPath: "/path/to/stack",
			command:   "validate",
			logLevel:  "info",
			logFormat: "key-value",
			expected:  []string{"run", "--all", "--working-dir", "/path/to/stack", "--log-level", "info", "--log-format", "key-value", "--", "validate"},
		},
		{
			name:         "custom format takes priority over standard format",
			stackPath:    "/path/to/stack",
			command:      "init",
			logLevel:     "warn",
			logFormat:    "json",
			logCustomFmt: "%time %level %msg",
			expected:     []string{"run", "--all", "--working-dir", "/path/to/stack", "--log-level", "warn", "--log-custom-format", "%time %level %msg", "--", "init"},
		},
		{
			name:         "only custom format without log level",
			stackPath:    "/path/to/stack",
			command:      "output",
			logCustomFmt: "%time %level %msg",
			expected:     []string{"run", "--all", "--working-dir", "/path/to/stack", "--log-custom-format", "%time %level %msg", "--", "output"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Reset viper before each test
			viper.Reset()

			// Set configuration values
			if tt.logLevel != "" {
				viper.Set("log_level", tt.logLevel)
			}
			if tt.logFormat != "" {
				viper.Set("log_format", tt.logFormat)
			} else {
				// Set default log format
				viper.Set("log_format", "pretty")
			}
			if tt.logCustomFmt != "" {
				viper.Set("log_custom_format", tt.logCustomFmt)
			}

			// Build arguments
			args := buildTerragruntArgs(tt.stackPath, tt.command)

			// Verify expected arguments
			assert.Equal(t, tt.expected, args, "Arguments should match expected output")
		})
	}
}
