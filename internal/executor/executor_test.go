package executor

import (
	"testing"

	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
)

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

// TestBuildTerragruntArgs_DynamicFlags tests the buildTerragruntArgs function with dynamic terragrunt flags.
func TestBuildTerragruntArgs_DynamicFlags(t *testing.T) {
	tests := []struct {
		name                        string
		stackPath                   string
		command                     string
		parallelism                 int
		noColor                     bool
		nonInteractive              bool
		ignoreDependencyErrors      bool
		ignoreExternalDependencies  bool
		includeExternalDependencies bool
		extraFlags                  []string
		expected                    []string
	}{
		{
			name:        "parallelism flag",
			stackPath:   "/path/to/stack",
			command:     "plan",
			parallelism: 4,
			expected:    []string{"run", "--all", "--working-dir", "/path/to/stack", "--log-format", "pretty", "--terragrunt-parallelism", "4", "--", "plan"},
		},
		{
			name:      "no-color flag",
			stackPath: "/path/to/stack",
			command:   "apply",
			noColor:   true,
			expected:  []string{"run", "--all", "--working-dir", "/path/to/stack", "--log-format", "pretty", "--terragrunt-no-color", "--", "apply"},
		},
		{
			name:           "non-interactive flag",
			stackPath:      "/path/to/stack",
			command:        "destroy",
			nonInteractive: true,
			expected:       []string{"run", "--all", "--working-dir", "/path/to/stack", "--log-format", "pretty", "--terragrunt-non-interactive", "--", "destroy"},
		},
		{
			name:                   "ignore-dependency-errors flag",
			stackPath:              "/path/to/stack",
			command:                "validate",
			ignoreDependencyErrors: true,
			expected:               []string{"run", "--all", "--working-dir", "/path/to/stack", "--log-format", "pretty", "--terragrunt-ignore-dependency-errors", "--", "validate"},
		},
		{
			name:                       "ignore-external-dependencies flag",
			stackPath:                  "/path/to/stack",
			command:                    "init",
			ignoreExternalDependencies: true,
			expected:                   []string{"run", "--all", "--working-dir", "/path/to/stack", "--log-format", "pretty", "--terragrunt-ignore-external-dependencies", "--", "init"},
		},
		{
			name:                        "include-external-dependencies flag",
			stackPath:                   "/path/to/stack",
			command:                     "output",
			includeExternalDependencies: true,
			expected:                    []string{"run", "--all", "--working-dir", "/path/to/stack", "--log-format", "pretty", "--terragrunt-include-external-dependencies", "--", "output"},
		},
		{
			name:       "extra flags",
			stackPath:  "/path/to/stack",
			command:    "plan",
			extraFlags: []string{"--terragrunt-download-dir=/tmp/tg", "--terragrunt-source-update"},
			expected:   []string{"run", "--all", "--working-dir", "/path/to/stack", "--log-format", "pretty", "--terragrunt-download-dir=/tmp/tg", "--terragrunt-source-update", "--", "plan"},
		},
		{
			name:                        "multiple flags combined",
			stackPath:                   "/path/to/stack",
			command:                     "apply",
			parallelism:                 8,
			noColor:                     true,
			nonInteractive:              true,
			includeExternalDependencies: true,
			extraFlags:                  []string{"--terragrunt-source-update"},
			expected:                    []string{"run", "--all", "--working-dir", "/path/to/stack", "--log-format", "pretty", "--terragrunt-parallelism", "8", "--terragrunt-no-color", "--terragrunt-non-interactive", "--terragrunt-include-external-dependencies", "--terragrunt-source-update", "--", "apply"},
		},
		{
			name:        "parallelism zero does not add flag",
			stackPath:   "/path/to/stack",
			command:     "plan",
			parallelism: 0,
			expected:    []string{"run", "--all", "--working-dir", "/path/to/stack", "--log-format", "pretty", "--", "plan"},
		},
		{
			name:      "false boolean flags not added",
			stackPath: "/path/to/stack",
			command:   "plan",
			noColor:   false,
			expected:  []string{"run", "--all", "--working-dir", "/path/to/stack", "--log-format", "pretty", "--", "plan"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Reset viper before each test
			viper.Reset()

			// Set default log format
			viper.Set("log_format", "pretty")

			// Set configuration values
			if tt.parallelism > 0 {
				viper.Set("terragrunt.parallelism", tt.parallelism)
			}
			if tt.noColor {
				viper.Set("terragrunt.no_color", tt.noColor)
			}
			if tt.nonInteractive {
				viper.Set("terragrunt.non_interactive", tt.nonInteractive)
			}
			if tt.ignoreDependencyErrors {
				viper.Set("terragrunt.ignore_dependency_errors", tt.ignoreDependencyErrors)
			}
			if tt.ignoreExternalDependencies {
				viper.Set("terragrunt.ignore_external_dependencies", tt.ignoreExternalDependencies)
			}
			if tt.includeExternalDependencies {
				viper.Set("terragrunt.include_external_dependencies", tt.includeExternalDependencies)
			}
			if len(tt.extraFlags) > 0 {
				viper.Set("terragrunt.extra_flags", tt.extraFlags)
			}

			// Build arguments
			args := buildTerragruntArgs(tt.stackPath, tt.command)

			// Verify expected arguments
			assert.Equal(t, tt.expected, args, "Arguments should match expected output")
		})
	}
}
