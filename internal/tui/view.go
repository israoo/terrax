package tui

import (
	"fmt"

	"github.com/charmbracelet/lipgloss"
)

// Styles using lipgloss
var (
	// Colors
	primaryColor   = lipgloss.Color("#7D56F4")
	secondaryColor = lipgloss.Color("#00D9FF")
	accentColor    = lipgloss.Color("#FF6B9D")
	textColor      = lipgloss.Color("#FFFFFF")
	dimColor       = lipgloss.Color("#888888")

	// Column styles
	focusedBorder = lipgloss.RoundedBorder()
	normalBorder  = lipgloss.NormalBorder()

	// Header style
	headerStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(textColor).
			Background(primaryColor).
			Padding(0, 1).
			Align(lipgloss.Center)

	// Footer style
	footerStyle = lipgloss.NewStyle().
			Foreground(dimColor).
			Padding(0, 1).
			Italic(true)

	// Column title style
	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(secondaryColor).
			Padding(0, 1)

	// Normal item style
	itemStyle = lipgloss.NewStyle().
			Foreground(textColor).
			Padding(0, 1)

	// Selected item style
	selectedItemStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(accentColor).
				Padding(0, 1)

	// Arrow indicator style
	arrowStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(secondaryColor).
			Padding(0, 1)

	// Breadcrumb bar style (prominent top bar below header)
	breadcrumbBarStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(textColor).
				Background(lipgloss.Color("#2E2E2E")).
				Padding(0, 2).
				Margin(0, 0)

	// Breadcrumb style for footer (deprecated, kept for compatibility)
	breadcrumbStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(primaryColor).
			Italic(false)

	// Level counter style
	levelCounterStyle = lipgloss.NewStyle().
				Foreground(secondaryColor)
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
func (lc *LayoutCalculator) GetContentHeight() int {
	return lc.height - HeaderHeight - FooterHeight - ColumnPadding
}

// GetColumnWidth returns the width for each column.
func (lc *LayoutCalculator) GetColumnWidth() int {
	return lc.columnWidth
}

// View renders the complete UI (BubbleTea interface).
func (m Model) View() string {
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

// renderColumnsWithArrows renders all visible columns with overflow indicators.
func (r *Renderer) renderColumnsWithArrows() []string {
	columns := make([]string, 0)

	// Render commands column (always visible)
	commandsView := r.renderCommandsColumn()
	styledCommands := r.styleColumn(commandsView, r.model.isCommandsColumnFocused())
	columns = append(columns, styledCommands)

	// Left overflow indicator
	if r.model.hasLeftOverflow() {
		leftArrow := r.renderArrowIndicator("«")
		columns = append(columns, leftArrow)
	}

	// Render navigation columns in sliding window (configurable max visible)
	maxDepth := r.model.navigator.GetMaxDepth()
	startDepth := r.model.navigationOffset
	endDepth := min(startDepth+r.model.maxNavigationColumns, maxDepth)

	for depth := startDepth; depth < endDepth; depth++ {
		// Skip empty columns
		if len(r.model.navState.Columns[depth]) == 0 {
			break
		}

		navView := r.renderNavigationColumn(depth)
		isFocused := r.model.focusedColumn == depth+1
		styledNav := r.styleColumn(navView, isFocused)
		columns = append(columns, styledNav)
	}

	// Right overflow indicator
	if r.model.hasRightOverflow() {
		rightArrow := r.renderArrowIndicator("»")
		columns = append(columns, rightArrow)
	}

	return columns
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

// min returns the minimum of two integers.
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// renderCommandsColumn renders the commands column content.
func (r *Renderer) renderCommandsColumn() string {
	parts := []string{}

	// Show filter input OR title (not both)
	// Show filter if it exists (even if empty, user might be typing)
	if filter, exists := r.model.columnFilters[0]; exists {
		// Show filter input instead of title
		filterView := filter.View()
		filterStyle := lipgloss.NewStyle().
			Foreground(lipgloss.Color("#00D9FF")).
			Padding(0, 1)
		parts = append(parts, filterStyle.Render(filterView))
	} else {
		// Show normal title
		title := titleStyle.Render("⚡" + CommandsTitle)
		parts = append(parts, title)
	}

	parts = append(parts, "") // Empty line after title/filter

	content := r.buildCommandList()
	parts = append(parts, content)

	return lipgloss.JoinVertical(lipgloss.Left, parts...)
}

// buildCommandList builds the list of commands with selection indicator.
func (r *Renderer) buildCommandList() string {
	originalCommands := r.model.commands
	commands := r.model.commands

	// Apply filter if this column (0) has an active filter
	if filter, exists := r.model.columnFilters[0]; exists {
		filterValue := filter.Value()
		if filterValue != "" {
			commands = filterItems(commands, filterValue)
		}
	}

	// Map original selected index to filtered index
	selectedFilteredIndex := -1
	if len(commands) < len(originalCommands) {
		// Filter is active, need to map
		selectedFilteredIndex = findFilteredIndex(originalCommands, commands, r.model.selectedCommand)
	} else {
		// No filter, use original index
		selectedFilteredIndex = r.model.selectedCommand
	}

	var content string
	for i, cmd := range commands {
		cursor := " "
		style := itemStyle

		if i == selectedFilteredIndex {
			cursor = "►"
			style = selectedItemStyle
		}

		content += fmt.Sprintf("%s %s\n", cursor, style.Render(cmd))
	}
	return content
}

// renderNavigationColumn renders a navigation column at the given depth.
func (r *Renderer) renderNavigationColumn(depth int) string {
	parts := []string{}

	// Show filter input OR title (not both)
	// Navigation columns: depth 0 -> columnID 1, depth 1 -> columnID 2, etc.
	columnID := depth + 1
	if filter, exists := r.model.columnFilters[columnID]; exists {
		// Show filter input instead of title
		filterView := filter.View()
		filterStyle := lipgloss.NewStyle().
			Foreground(lipgloss.Color("#00D9FF")).
			Padding(0, 1)
		parts = append(parts, filterStyle.Render(filterView))
	} else {
		// Show normal title
		title := titleStyle.Render("📦 " + r.getLevelTitle(depth))
		parts = append(parts, title)
	}

	parts = append(parts, "") // Empty line after title/filter

	content := r.buildNavigationList(depth)
	parts = append(parts, content)

	return lipgloss.JoinVertical(lipgloss.Left, parts...)
}

// buildNavigationList builds the list of items for a navigation column.
func (r *Renderer) buildNavigationList(depth int) string {
	originalItems := r.model.navState.Columns[depth]
	items := r.model.navState.Columns[depth]
	selectedIndex := r.model.navState.SelectedIndices[depth]

	// Apply filter if this navigation column has an active filter
	// Navigation columns are indexed as: depth 0 -> columnID 1, depth 1 -> columnID 2, etc.
	columnID := depth + 1
	if filter, exists := r.model.columnFilters[columnID]; exists {
		filterValue := filter.Value()
		if filterValue != "" {
			items = filterItems(items, filterValue)
		}
	}

	// Map original selected index to filtered index
	selectedFilteredIndex := -1
	if len(items) < len(originalItems) {
		// Filter is active, need to map
		selectedFilteredIndex = findFilteredIndex(originalItems, items, selectedIndex)
	} else {
		// No filter, use original index
		selectedFilteredIndex = selectedIndex
	}

	var content string
	for i, item := range items {
		cursor := " "
		style := itemStyle

		if i == selectedFilteredIndex {
			cursor = "►"
			style = selectedItemStyle
		}

		content += fmt.Sprintf("%s %s\n", cursor, style.Render(item))
	}
	return content
}

// styleColumn applies styling to a column based on focus state.
func (r *Renderer) styleColumn(content string, isFocused bool) string {
	return columnStyle(isFocused).
		Width(r.layout.GetColumnWidth()).
		Height(r.layout.GetContentHeight()).
		Render(content)
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

// getLevelTitle returns the title for a navigation level.
func (r *Renderer) getLevelTitle(depth int) string {
	return fmt.Sprintf("Level %d", depth+1)
}

// columnStyle returns the appropriate style for a column based on focus.
func columnStyle(focused bool) lipgloss.Style {
	style := lipgloss.NewStyle().
		Padding(1, 2).
		Margin(0, 1)

	if focused {
		return style.
			Border(focusedBorder).
			BorderForeground(primaryColor)
	}

	return style.
		Border(normalBorder).
		BorderForeground(dimColor)
}
