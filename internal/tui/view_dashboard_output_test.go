package tui

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/israoo/terrax/internal/history"
)

func TestRenderDashboardOutput_IdleHeader(t *testing.T) {
	m := makeDashModel(t)
	result := m.renderDashboardOutput(80, 20)
	assert.Contains(t, result, "Output")
}

func TestRenderDashboardOutput_RunningHeader(t *testing.T) {
	m := makeDashModel(t)
	m.dashRunning = true
	m.dashRunCmd = "plan"
	result := m.renderDashboardOutput(80, 20)
	assert.Contains(t, result, "plan")
}

func TestRenderDashboardOutput_FinishedSuccess(t *testing.T) {
	m := makeDashModel(t)
	m.dashRunning = false
	m.dashRunCmd = "plan"
	entry := history.ExecutionLogEntry{Command: "plan", ExitCode: 0, DurationS: 1.2}
	m.dashLastRun = &entry
	result := m.renderDashboardOutput(80, 20)
	assert.Contains(t, result, "✓")
}

func TestRenderDashboardOutput_FinishedFailure(t *testing.T) {
	m := makeDashModel(t)
	m.dashRunning = false
	m.dashRunCmd = "plan"
	entry := history.ExecutionLogEntry{Command: "plan", ExitCode: 1, DurationS: 0.5}
	m.dashLastRun = &entry
	result := m.renderDashboardOutput(80, 20)
	assert.Contains(t, result, "✗")
}

func TestRenderDashboardOutput_ScrollOffset(t *testing.T) {
	m := makeDashModel(t)
	for i := range 30 {
		m.dashOutputLines = append(m.dashOutputLines, fmt.Sprintf("line %d", i))
	}
	m.dashOutputOffset = 10
	result := m.renderDashboardOutput(80, 10)
	assert.Contains(t, result, "line 10")
	assert.NotContains(t, result, "line 0")
}
