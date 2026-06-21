package stack

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDetectGroup(t *testing.T) {
	groups := map[string]GroupDetectConfig{
		"private":    {Detect: "require_private_connection = true"},
		"deprecated": {Detect: "deprecated = true"},
	}

	tests := []struct {
		name       string
		hclContent string
		writeFile  bool
		expected   string
	}{
		{
			name:       "matches first group",
			hclContent: "locals {\n  require_private_connection = true\n}",
			writeFile:  true,
			expected:   "private",
		},
		{
			name:       "matches second group",
			hclContent: "locals {\n  deprecated = true\n}",
			writeFile:  true,
			expected:   "deprecated",
		},
		{
			name:       "no match returns default",
			hclContent: "locals {\n  enabled_providers = [\"aws\"]\n}",
			writeFile:  true,
			expected:   "default",
		},
		{
			name:      "missing stack.hcl returns default",
			writeFile: false,
			expected:  "default",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			if tt.writeFile {
				require.NoError(t, os.WriteFile(filepath.Join(dir, "stack.hcl"), []byte(tt.hclContent), 0644))
			}
			assert.Equal(t, tt.expected, DetectGroup(dir, groups))
		})
	}
}

func TestTopologicalSort(t *testing.T) {
	tests := []struct {
		name      string
		groups    map[string]GroupDetectConfig
		wantOrder []string
		wantErr   bool
	}{
		{
			name:      "single group no deps",
			groups:    map[string]GroupDetectConfig{"default": {}},
			wantOrder: []string{"default"},
		},
		{
			name: "private depends on default",
			groups: map[string]GroupDetectConfig{
				"default": {},
				"private": {DependsOn: []string{"default"}},
			},
			wantOrder: []string{"default", "private"},
		},
		{
			name: "cycle returns error",
			groups: map[string]GroupDetectConfig{
				"a": {DependsOn: []string{"b"}},
				"b": {DependsOn: []string{"a"}},
			},
			wantErr: true,
		},
		{
			name: "depends_on undefined group is ignored",
			groups: map[string]GroupDetectConfig{
				"private": {DependsOn: []string{"nonexistent"}},
			},
			wantOrder: []string{"private"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := TopologicalSort(tt.groups)
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.wantOrder, result)
		})
	}
}
