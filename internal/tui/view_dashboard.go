package tui

// renderDashboardView renders the StateDashboard layout.
// Implemented fully in Task 9; this stub prevents compilation errors.
func (m Model) renderDashboardView() string {
	if !m.ready {
		return Initializing
	}
	return "Dashboard (in progress)"
}
