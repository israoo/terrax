package tui

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/israoo/terrax/internal/history"
)

func TestRenderDashboardBottom_DepsTab(t *testing.T) {
	m := makeDashModel(t)
	m.dashActiveTab = 0
	m.dashTreeCursor = 1 // mgmt leaf
	m.dashTreeNodes[1].Node.Dependencies = []string{"/root/infra/dev"}
	result := m.renderDashboardBottom(80, 15)
	assert.Contains(t, result, "Dependencies")
}

func TestRenderDashboardBottom_LastRunTab(t *testing.T) {
	m := makeDashModel(t)
	m.dashActiveTab = 1
	m.dashLastRun = &history.ExecutionLogEntry{
		Command:   "plan",
		ExitCode:  0,
		DurationS: 1.5,
		Timestamp: time.Now(),
	}
	result := m.renderDashboardBottom(80, 15)
	assert.Contains(t, result, "plan")
	assert.Contains(t, result, "Last Run")
}

func TestRenderDashboardBottom_PlanDiffTab_NoPlan(t *testing.T) {
	m := makeDashModel(t)
	m.dashActiveTab = 2
	m.dashPlanReport = nil
	result := m.renderDashboardBottom(80, 15)
	assert.Contains(t, result, "Plan Diff")
	assert.Contains(t, result, "No plan")
}

func TestRenderDashboardBottom_TabSwitching(t *testing.T) {
	m := makeDashModel(t)
	m.dashActiveTab = 0
	result0 := m.renderDashboardBottom(80, 15)
	m.dashActiveTab = 1
	result1 := m.renderDashboardBottom(80, 15)
	// Tab headers should differ
	assert.NotEqual(t, result0, result1)
}
