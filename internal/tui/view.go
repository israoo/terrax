package tui

import (
	"github.com/charmbracelet/lipgloss"
)

// View renders the complete UI (BubbleTea interface).
func (m Model) View() string {
	if m.state == StateHistory {
		return m.renderHistoryView()
	}

	if !m.ready || m.width == 0 {
		return Initializing
	}

	if m.navigator.GetMaxDepth() == 0 || m.columnWidth == 0 {
		return ScanningStacks
	}

	layout := NewLayoutCalculator(m.width, m.height, m.columnWidth)
	renderer := NewRenderer(m, layout)

	return renderer.Render()
}

// Renderer handles the actual rendering of UI components.
type Renderer struct {
	model  Model
	layout *LayoutCalculator
}

// NewRenderer creates a new renderer instance.
func NewRenderer(model Model, layout *LayoutCalculator) *Renderer {
	return &Renderer{
		model:  model,
		layout: layout,
	}
}

// Render builds the complete UI view.
func (r *Renderer) Render() string {
	columns := r.renderColumnsWithArrows()
	content := lipgloss.JoinHorizontal(lipgloss.Top, columns...)

	header := r.renderHeader()
	breadcrumbBar := r.renderBreadcrumbBar()
	footer := r.renderFooter()

	return lipgloss.JoinVertical(lipgloss.Left, header, breadcrumbBar, content, footer)
}
