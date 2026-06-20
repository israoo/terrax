package tui

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/israoo/terrax/internal/plan"
)

// PlanReportMsg indicates that plan collection has finished.
type PlanReportMsg struct {
	Report *plan.PlanReport
	Err    error
}

// handlePlanReviewUpdate handles update messages for the plan review state.
func (m Model) handlePlanReviewUpdate(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyEsc, tea.KeyCtrlC:
			return m, tea.Quit

		case tea.KeyRunes:
			if msg.String() == "q" {
				return m, tea.Quit
			}

		case tea.KeyLeft, tea.KeyRight, tea.KeyTab, tea.KeyShiftTab:
			m.planReviewFocusedElement = 1 - m.planReviewFocusedElement
			return m, nil

		case tea.KeyUp, tea.KeyDown, tea.KeyPgUp, tea.KeyPgDown:
			if m.planReviewFocusedElement == 0 {
				m.handlePlanListNavigation(msg)
			} else {
				m.handlePlanDetailNavigation(msg)
			}
			return m, nil
		}
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.ready = true
	}

	return m, nil
}

func (m *Model) handlePlanListNavigation(msg tea.KeyMsg) {
	switch msg.Type {
	case tea.KeyUp:
		if m.planListCursor > 0 {
			m.planListCursor--
			m.planDetailScrollOffset = 0
		}
	case tea.KeyDown:
		if m.planListCursor < len(m.planFlatItems)-1 {
			m.planListCursor++
			m.planDetailScrollOffset = 0
		}
	case tea.KeyPgUp:
		// Page up in list
		pageSize := m.height - PlanContentFrame
		m.planListCursor -= pageSize
		if m.planListCursor < 0 {
			m.planListCursor = 0
		}
		m.planDetailScrollOffset = 0
	case tea.KeyPgDown:
		// Page down in list
		pageSize := m.height - PlanContentFrame
		m.planListCursor += pageSize
		if m.planListCursor >= len(m.planFlatItems) {
			m.planListCursor = len(m.planFlatItems) - 1
		}
		m.planDetailScrollOffset = 0
	}
}

func (m *Model) handlePlanDetailNavigation(msg tea.KeyMsg) {
	lines := m.getPlanDetailLines()
	totalLines := len(lines)
	visibleHeight := m.height - PlanVerticalFrame - 2
	if visibleHeight < 1 {
		visibleHeight = 1
	}
	maxOffset := totalLines - visibleHeight
	if maxOffset < 0 {
		maxOffset = 0
	}

	switch msg.Type {
	case tea.KeyUp:
		if m.planDetailScrollOffset > 0 {
			m.planDetailScrollOffset--
		}
	case tea.KeyDown:
		if m.planDetailScrollOffset < maxOffset {
			m.planDetailScrollOffset++
		}
	case tea.KeyPgUp:
		m.planDetailScrollOffset -= visibleHeight
		if m.planDetailScrollOffset < 0 {
			m.planDetailScrollOffset = 0
		}
	case tea.KeyPgDown:
		m.planDetailScrollOffset += visibleHeight
		if m.planDetailScrollOffset > maxOffset {
			m.planDetailScrollOffset = maxOffset
		}
	}
}
