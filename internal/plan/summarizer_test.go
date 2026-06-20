package plan

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestCountChanges validates change counting logic across all change types.
func TestCountChanges(t *testing.T) {
	type rc = struct {
		Address string `json:"address"`
		Type    string `json:"type"`
		Name    string `json:"name"`
		Change  struct {
			Actions   []string    `json:"actions"`
			Before    interface{} `json:"before"`
			After     interface{} `json:"after"`
			Unknown   interface{} `json:"after_unknown"`
			Importing interface{} `json:"importing"`
		} `json:"change"`
	}
	make := func(actions []string, importing interface{}) rc {
		r := rc{}
		r.Change.Actions = actions
		r.Change.Importing = importing
		return r
	}

	tests := []struct {
		name     string
		changes  []rc
		expected planSummaryStats
	}{
		{"create → add", []rc{make([]string{"create"}, nil)}, planSummaryStats{Add: 1}},
		{"update → update", []rc{make([]string{"update"}, nil)}, planSummaryStats{Update: 1}},
		{"delete → delete", []rc{make([]string{"delete"}, nil)}, planSummaryStats{Delete: 1}},
		{"delete+create → recreate", []rc{make([]string{"delete", "create"}, nil)}, planSummaryStats{Recreate: 1}},
		{"create+importing → import", []rc{make([]string{"create"}, map[string]string{"id": "i-123"})}, planSummaryStats{Import: 1}},
		{"no-op → skipped", []rc{make([]string{"no-op"}, nil)}, planSummaryStats{}},
		{"mixed changes", []rc{
			make([]string{"create"}, nil),
			make([]string{"update"}, nil),
			make([]string{"delete"}, nil),
			make([]string{"delete", "create"}, nil),
			make([]string{"no-op"}, nil),
		}, planSummaryStats{Add: 1, Update: 1, Delete: 1, Recreate: 1}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := TerraformPlanJSON{}
			p.ResourceChanges = tt.changes
			assert.Equal(t, tt.expected, countChanges(p))
		})
	}
}

func TestSummarize_DirectoryNotExist(t *testing.T) {
	count, err := Summarize(context.Background(), "/nonexistent/path/xyz")
	require.NoError(t, err)
	assert.Equal(t, 0, count)
}

func TestSummarize_EmptyDirectory(t *testing.T) {
	count, err := Summarize(context.Background(), t.TempDir())
	require.NoError(t, err)
	assert.Equal(t, 0, count)
}

func TestSummarize_StackWithChanges(t *testing.T) {
	dir := t.TempDir()
	stackDir := filepath.Join(dir, "workloads", "dev", "acm")
	require.NoError(t, os.MkdirAll(stackDir, 0755))

	planJSON := `{"resource_changes":[{"address":"r","type":"r","name":"r","change":{"actions":["create"],"before":null,"after":{}}}]}`
	require.NoError(t, os.WriteFile(filepath.Join(stackDir, "plan.json"), []byte(planJSON), 0644))

	count, err := Summarize(context.Background(), dir)
	require.NoError(t, err)
	assert.Equal(t, 1, count)
}

func TestSummarize_StackNoChanges(t *testing.T) {
	dir := t.TempDir()
	stackDir := filepath.Join(dir, "workloads", "dev", "acm")
	require.NoError(t, os.MkdirAll(stackDir, 0755))

	planJSON := `{"resource_changes":[{"address":"r","type":"r","name":"r","change":{"actions":["no-op"],"before":{},"after":{}}}]}`
	require.NoError(t, os.WriteFile(filepath.Join(stackDir, "plan.json"), []byte(planJSON), 0644))

	count, err := Summarize(context.Background(), dir)
	require.NoError(t, err)
	assert.Equal(t, 0, count)
}

func TestSummarize_MultipleStacksPartialChanges(t *testing.T) {
	dir := t.TempDir()
	withChange := `{"resource_changes":[{"address":"r","type":"r","name":"r","change":{"actions":["create"],"before":null,"after":{}}}]}`
	noChange := `{"resource_changes":[{"address":"r","type":"r","name":"r","change":{"actions":["no-op"],"before":{},"after":{}}}]}`

	for _, s := range []struct{ path, content string }{
		{"workloads/dev/acm", withChange},
		{"workloads/dev/ecr", noChange},
		{"workloads/dev/vpc", withChange},
	} {
		d := filepath.Join(dir, filepath.FromSlash(s.path))
		require.NoError(t, os.MkdirAll(d, 0755))
		require.NoError(t, os.WriteFile(filepath.Join(d, "plan.json"), []byte(s.content), 0644))
	}

	count, err := Summarize(context.Background(), dir)
	require.NoError(t, err)
	assert.Equal(t, 2, count)
}

func TestSummarize_InvalidJSON_WarnsAndContinues(t *testing.T) {
	dir := t.TempDir()
	good := `{"resource_changes":[{"address":"r","type":"r","name":"r","change":{"actions":["create"],"before":null,"after":{}}}]}`

	for _, s := range []struct{ path, content string }{
		{"stacks/good", good},
		{"stacks/bad", "not-valid-json"},
	} {
		d := filepath.Join(dir, filepath.FromSlash(s.path))
		require.NoError(t, os.MkdirAll(d, 0755))
		require.NoError(t, os.WriteFile(filepath.Join(d, "plan.json"), []byte(s.content), 0644))
	}

	count, err := Summarize(context.Background(), dir)
	require.NoError(t, err)
	assert.Equal(t, 1, count)
}
