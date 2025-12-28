package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestDefaultConstants verifies that default constants have expected values.
func TestDefaultConstants(t *testing.T) {
	tests := []struct {
		name     string
		actual   interface{}
		expected interface{}
	}{
		{
			name:     "DefaultMaxNavigationColumns",
			actual:   DefaultMaxNavigationColumns,
			expected: 3,
		},
		{
			name:     "MinMaxNavigationColumns",
			actual:   MinMaxNavigationColumns,
			expected: 1,
		},
		{
			name:     "DefaultHistoryMaxEntries",
			actual:   DefaultHistoryMaxEntries,
			expected: 500,
		},
		{
			name:     "MinHistoryMaxEntries",
			actual:   MinHistoryMaxEntries,
			expected: 10,
		},
		{
			name:     "DefaultRootConfigFile",
			actual:   DefaultRootConfigFile,
			expected: "root.hcl",
		},
		{
			name:     "DefaultLogFormat",
			actual:   DefaultLogFormat,
			expected: "pretty",
		},
		{
			name:     "DefaultParallelism",
			actual:   DefaultParallelism,
			expected: 0,
		},
		{
			name:     "DefaultNoColor",
			actual:   DefaultNoColor,
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.actual)
		})
	}
}

// TestDefaultCommands verifies the default commands list.
func TestDefaultCommands(t *testing.T) {
	expectedCommands := []string{
		"plan",
		"apply",
		"validate",
		"fmt",
		"init",
		"output",
		"refresh",
		"destroy",
	}

	assert.Equal(t, expectedCommands, DefaultCommands)
	assert.Len(t, DefaultCommands, 8)
}
