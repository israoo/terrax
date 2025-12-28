package tui

import (
	"fmt"

	"github.com/charmbracelet/lipgloss"
)

// LayoutCalculator handles all layout dimension calculations.
type LayoutCalculator struct {
	width       int
	height      int
	columnWidth int
}

// NewLayoutCalculator creates a new layout calculator.
func NewLayoutCalculator(width, height, columnWidth int) *LayoutCalculator {
	return &LayoutCalculator{
		width:       width,
		height:      height,
		columnWidth: columnWidth,
	}
}

// GetContentHeight returns the available height for column content.
// This must match the calculation in Model.getAvailableHeight() to ensure consistency.
func (lc *LayoutCalculator) GetContentHeight() int {
	// Total reserved space:
	// - HeaderHeight (1)
	// - Breadcrumb bar (1)
	// - FooterHeight (1)
	// Note: Column title (1) and empty line after title (1) are part of column content,
	// not subtracted here since they're rendered inside the column.
	reservedSpace := HeaderHeight + BreadcrumbLineCount + FooterHeight
	contentHeight := lc.height - reservedSpace

	if contentHeight < 1 {
		return 1
	}

	return contentHeight
}

// GetColumnWidth returns the width for each column.
func (lc *LayoutCalculator) GetColumnWidth() int {
	return lc.columnWidth
}

// renderHeader renders the header bar.
func (r *Renderer) renderHeader() string {
	return headerStyle.Width(r.model.width).Render("🌍 " + AppTitle)
}

// renderBreadcrumbBar renders the navigation context bar below the header.
func (r *Renderer) renderBreadcrumbBar() string {
	navPath := r.model.getCurrentNavigationPath()

	// Format breadcrumb - always show the full path
	content := fmt.Sprintf("📁 %s", navPath)

	return breadcrumbBarStyle.Width(r.model.width).Render(content)
}

// renderFooter renders the footer with help text only.
func (r *Renderer) renderFooter() string {
	return footerStyle.Render(HelpText)
}

// renderArrowIndicator renders an arrow indicator for overflow.
func (r *Renderer) renderArrowIndicator(arrow string) string {
	content := arrowStyle.Render(arrow)
	return lipgloss.NewStyle().
		Height(r.layout.GetContentHeight()).
		Padding(0, 0).
		AlignVertical(lipgloss.Center).
		Render(content)
}

// renderPageIndicators renders pagination dots showing current page position.
// Returns empty string if only one page exists.
func renderPageIndicators(currentPage, totalPages int) string {
	if totalPages <= 1 {
		return ""
	}

	var dots string
	for i := 1; i <= totalPages; i++ {
		if i == currentPage {
			dots += activePageIndicatorStyle.Render("•")
		} else {
			dots += pageIndicatorStyle.Render("•")
		}
	}

	return dots
}

// min returns the minimum of two integers.
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// truncateText truncates text to fit within maxWidth, adding "..." if truncated.
// maxWidth: Maximum width in characters (including the "..." if added).
// Returns the original text if it fits, or truncated text with "..." appended.
func truncateText(text string, maxWidth int) string {
	// If maxWidth is too small to be useful, just return empty or minimal text
	if maxWidth <= EllipsisWidth {
		if maxWidth <= 0 {
			return ""
		}
		return text[:min(len(text), maxWidth)]
	}

	// If text fits, return as-is
	if len(text) <= maxWidth {
		return text
	}

	// Truncate and add ellipsis
	// Reserve space for ellipsis
	truncateAt := maxWidth - EllipsisWidth
	if truncateAt < 0 {
		truncateAt = 0
	}

	return text[:truncateAt] + "..."
}
