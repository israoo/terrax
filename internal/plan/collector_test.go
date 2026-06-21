package plan

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMapActionsToChangeType(t *testing.T) {
	tests := []struct {
		name     string
		actions  []string
		expected ChangeType
	}{
		{"create", []string{"create"}, ChangeTypeCreate},
		{"delete", []string{"delete"}, ChangeTypeDelete},
		{"update", []string{"update"}, ChangeTypeUpdate},
		{"replace (create, delete)", []string{"delete", "create"}, ChangeTypeReplace},
		{"replace (delete, create)", []string{"create", "delete"}, ChangeTypeReplace},
		{"no-op", []string{"no-op"}, ChangeTypeNoOp},
		{"empty", []string{}, ChangeTypeNoOp},
		{"read", []string{"read"}, ChangeTypeNoOp},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, mapActionsToChangeType(tt.actions))
		})
	}
}

func TestPlanReport_CalculateSummary(t *testing.T) {
	report := &PlanReport{
		Stacks: []StackResult{
			{HasChanges: true, Stats: StackStats{Add: 2, Change: 1, Destroy: 0}},
			{HasChanges: false, Stats: StackStats{}},
			{HasChanges: true, Stats: StackStats{Add: 0, Change: 0, Destroy: 3}},
		},
	}
	report.calculateSummary()
	assert.Equal(t, 3, report.Summary.TotalStacks)
	assert.Equal(t, 2, report.Summary.StacksWithChanges)
	assert.Equal(t, 2, report.Summary.TotalAdd)
	assert.Equal(t, 1, report.Summary.TotalChange)
	assert.Equal(t, 3, report.Summary.TotalDestroy)
}

func TestCollectFromJSONDir_EmptyDir(t *testing.T) {
	// jsonDir = <tmp>/.terrax/plans, repoRoot = <tmp>
	tmp := t.TempDir()
	jsonDir := filepath.Join(tmp, ".terrax", "plans")
	require.NoError(t, os.MkdirAll(jsonDir, 0755))
	report, err := CollectFromJSONDir(context.Background(), jsonDir, tmp)
	require.NoError(t, err)
	assert.Empty(t, report.Stacks)
}

func TestCollectFromJSONDir_NonExistentDir(t *testing.T) {
	report, err := CollectFromJSONDir(context.Background(), "/nonexistent/xyz/.terrax/plans", "/nonexistent/xyz")
	require.NoError(t, err)
	assert.Empty(t, report.Stacks)
}

func TestCollectFromJSONDir_SingleStackWithChanges(t *testing.T) {
	tmp := t.TempDir()
	jsonDir := filepath.Join(tmp, ".terrax", "plans")
	stackDir := filepath.Join(jsonDir, "workloads", "dev", "acm")
	require.NoError(t, os.MkdirAll(stackDir, 0755))

	planJSON := `{"resource_changes":[{"address":"r","type":"r","name":"r","change":{"actions":["create"],"before":null,"after":{}}}]}`
	require.NoError(t, os.WriteFile(filepath.Join(stackDir, "plan.json"), []byte(planJSON), 0644))

	report, err := CollectFromJSONDir(context.Background(), jsonDir, tmp)
	require.NoError(t, err)
	require.Len(t, report.Stacks, 1)
	assert.True(t, report.Stacks[0].HasChanges)
	assert.Equal(t, 1, report.Stacks[0].Stats.Add)
	assert.Equal(t, "workloads/dev/acm", report.Stacks[0].StackPath)
	assert.Equal(t, 1, report.Summary.StacksWithChanges)
}

func TestCollectFromJSONDir_NoChanges(t *testing.T) {
	tmp := t.TempDir()
	jsonDir := filepath.Join(tmp, ".terrax", "plans")
	stackDir := filepath.Join(jsonDir, "workloads", "dev", "ecr")
	require.NoError(t, os.MkdirAll(stackDir, 0755))

	planJSON := `{"resource_changes":[{"address":"r","type":"r","name":"r","change":{"actions":["no-op"],"before":{},"after":{}}}]}`
	require.NoError(t, os.WriteFile(filepath.Join(stackDir, "plan.json"), []byte(planJSON), 0644))

	report, err := CollectFromJSONDir(context.Background(), jsonDir, tmp)
	require.NoError(t, err)
	require.Len(t, report.Stacks, 1)
	assert.False(t, report.Stacks[0].HasChanges)
	assert.Equal(t, 0, report.Summary.StacksWithChanges)
}

func TestCollectFromJSONDir_InvalidJSONWarnsAndContinues(t *testing.T) {
	tmp := t.TempDir()
	jsonDir := filepath.Join(tmp, ".terrax", "plans")
	for _, s := range []struct{ path, content string }{
		{"stacks/good", `{"resource_changes":[{"address":"r","type":"r","name":"r","change":{"actions":["create"],"before":null,"after":{}}}]}`},
		{"stacks/bad", "not-valid-json"},
	} {
		d := filepath.Join(jsonDir, filepath.FromSlash(s.path))
		require.NoError(t, os.MkdirAll(d, 0755))
		require.NoError(t, os.WriteFile(filepath.Join(d, "plan.json"), []byte(s.content), 0644))
	}
	report, err := CollectFromJSONDir(context.Background(), jsonDir, tmp)
	require.NoError(t, err)
	assert.Len(t, report.Stacks, 1)
}
