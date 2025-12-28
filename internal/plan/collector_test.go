package plan

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
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
		Timestamp: time.Now(),
		Stacks: []StackResult{
			{
				HasChanges: true,
				Stats: StackStats{
					Add:     2,
					Change:  1,
					Destroy: 0,
				},
			},
			{
				HasChanges: false,
				Stats: StackStats{
					Add:     0,
					Change:  0,
					Destroy: 0,
				},
			},
			{
				HasChanges: true,
				Stats: StackStats{
					Add:     0,
					Change:  0,
					Destroy: 3,
				},
			},
		},
	}

	report.calculateSummary()

	assert.Equal(t, 3, report.Summary.TotalStacks)
	assert.Equal(t, 2, report.Summary.StacksWithChanges)
	assert.Equal(t, 2, report.Summary.TotalAdd)
	assert.Equal(t, 1, report.Summary.TotalChange)
	assert.Equal(t, 3, report.Summary.TotalDestroy)
}
