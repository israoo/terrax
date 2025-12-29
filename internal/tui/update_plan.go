package tui

import (
	tea "github.com/charmbracelet/bubbletea"
)

// handlePlanReviewUpdate handles updates when in StatePlanReview mode.
func (m Model) handlePlanReviewUpdate(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.ready = true
		return m, nil

	case tea.KeyMsg:
		switch msg.String() {
		case KeyQ, KeyEsc:
			// Exit Plan Review mode
			return m, tea.Quit

		case KeyRight, KeyEnter:
			// Switch focus to Detail View
			m.planReviewFocusedElement = 1
			return m, nil

		case KeyLeft:
			// Switch focus to Master List
			m.planReviewFocusedElement = 0
			return m, nil

		case KeyUp:
			if m.planReviewFocusedElement == 0 {
				// Master List Navigation
				if m.planListCursor > 0 {
					m.planListCursor--
					m.planDetailScrollOffset = 0 // Reset detail scroll
				}
			} else {
				// Detail View Scrolling
				if m.planDetailScrollOffset > 0 {
					m.planDetailScrollOffset--
				}
			}
			return m, nil

		case KeyDown:
			if m.planReviewFocusedElement == 0 {
				// Master List Navigation
				if m.planFlatItems != nil && m.planListCursor < len(m.planFlatItems)-1 {
					m.planListCursor++
					m.planDetailScrollOffset = 0 // Reset detail scroll
				}
			} else {
				// Detail View Scrolling
				lines := m.getPlanDetailLines()
				totalLines := len(lines)

				// Calculate visible height matching view_plan.go
				// PlanVerticalFrame = 2, Detail Border = 2
				visibleHeight := m.height - PlanVerticalFrame - 2
				if visibleHeight < 1 {
					visibleHeight = 1
				}

				maxOffset := totalLines - visibleHeight
				if maxOffset < 0 {
					maxOffset = 0
				}

				if m.planDetailScrollOffset < maxOffset {
					m.planDetailScrollOffset++
				}
			}
			return m, nil
		}
	}

	return m, nil
}
