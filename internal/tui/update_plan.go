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

		case KeyUp:
			// Master List Navigation
			if m.planListCursor > 0 {
				m.planListCursor--
			}
			return m, nil

		case KeyDown:
			// Master List Navigation
			if m.planFlatItems != nil && m.planListCursor < len(m.planFlatItems)-1 {
				m.planListCursor++
			}
			return m, nil
		}
	}

	return m, nil
}
