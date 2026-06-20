// Package tui provides the terminal user interface for TerraX.
//
// It implements the Bubble Tea architecture (Model-Update-View) to render the
// hierarchical navigation, command execution history, and interactive filtering.
// This package handles all user input and rendering logic, delegating business
// logic to the stack package.
package tui

import (
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/israoo/terrax/internal/history"
	"github.com/israoo/terrax/internal/plan"
	"github.com/israoo/terrax/internal/stack"
)

// AppState represents the current state of the application.
type AppState int

const (
	// StateNavigation is the default state for navigating stacks and commands.
	StateNavigation AppState = iota
	// StateHistory is the state for viewing execution history.
	StateHistory
	// StatePlanReview is the state for analyzing plan results.
	StatePlanReview
)

// ColumnType represents the type of column being focused.
type ColumnType int

const (
	// CommandsColumn represents the commands column.
	CommandsColumn ColumnType = iota
	// NavigationColumn represents a navigation column.
	NavigationColumn
)

// Model is the main TUI model following Bubble Tea architecture.
// It maintains minimal state and delegates business logic to Navigator.
type Model struct {
	// Application State
	state AppState

	// Navigation
	navigator *stack.Navigator
	navState  *stack.NavigationState

	// Commands
	commands        []string
	selectedCommand int

	// History
	history              []history.ExecutionLogEntry
	historyCursor        int
	selectedHistoryEntry *history.ExecutionLogEntry // Entry selected for re-execution
	reExecuteFromHistory bool                       // Flag to indicate re-execution from history

	// Plan Review
	planReport               *plan.PlanReport
	planTreeRoots            []*plan.TreeNode
	planFlatItems            []*plan.TreeNode // Flattened view for linear navigation
	planListCursor           int
	planDetailScrollOffset   int
	planReviewFocusedElement int // 0 = Master List, 1 = Detail View
	planTargetStats          plan.StackStats
	planDependencyStats      plan.StackStats

	// UI State
	focusedColumn    int  // 0 = commands, 1+ = navigation columns
	navigationOffset int  // First visible navigation level (sliding window)
	confirmed        bool // Whether user confirmed selection

	// Layout
	width                int
	height               int
	columnWidth          int // Pre-calculated static column width
	maxNavigationColumns int // Maximum navigation columns visible (sliding window)

	// Filtering (per-column)
	columnFilters      map[int]textinput.Model // Filter inputs per column (0=commands, 1+=navigation)
	activeFilterColumn int                     // Which column's filter is currently being edited (-1 = none)

	// Scrolling (per-column vertical viewport)
	scrollOffsets map[int]int // Scroll offset per column (0=commands, 1+=navigation)

	// State flags
	ready bool
}

// NewModel creates a new TUI model instance.
// commands: List of available Terragrunt commands to display.
// maxNavigationColumns: Maximum number of navigation columns visible simultaneously (must be validated before calling).
func NewModel(stackRoot *stack.Node, maxDepth int, commands []string, maxNavigationColumns int) Model {
	navigator := stack.NewNavigator(stackRoot, maxDepth)
	navState := stack.NewNavigationState(maxDepth)

	m := Model{
		state:                StateNavigation,
		navigator:            navigator,
		navState:             navState,
		commands:             commands,
		selectedCommand:      0,
		focusedColumn:        0,
		navigationOffset:     0,
		confirmed:            false,
		ready:                false,
		maxNavigationColumns: maxNavigationColumns,
		columnFilters:        make(map[int]textinput.Model),
		activeFilterColumn:   -1,
		scrollOffsets:        make(map[int]int),
		history:              nil,
		historyCursor:        0,
		selectedHistoryEntry: nil,
		reExecuteFromHistory: false,
	}

	navigator.PropagateSelection(navState)

	return m
}

// NewHistoryModel creates a model initialized in history viewing mode.
func NewHistoryModel(historyEntries []history.ExecutionLogEntry) Model {
	m := Model{
		state:                StateHistory,
		history:              historyEntries,
		historyCursor:        0,
		ready:                false,
		selectedHistoryEntry: nil,
		reExecuteFromHistory: false,
	}
	return m
}

// NewPlanReviewModel creates a model initialized in plan review mode.
func NewPlanReviewModel(report *plan.PlanReport) Model {
	// Filter stacks to only show those with changes
	targetStats := plan.StackStats{}
	dependencyStats := plan.StackStats{}

	for _, stack := range report.Stacks {
		if !stack.HasChanges {
			continue
		}

		if stack.IsDependency {
			dependencyStats.Add += stack.Stats.Add
			dependencyStats.Change += stack.Stats.Change
			dependencyStats.Destroy += stack.Stats.Destroy
		} else {
			targetStats.Add += stack.Stats.Add
			targetStats.Change += stack.Stats.Change
			targetStats.Destroy += stack.Stats.Destroy
		}
	}

	// Build the tree from all stacks
	roots := plan.BuildTree(report.Stacks)

	// Flatten the tree for navigation
	flatItems := flattenTree(roots)

	// Filter flattened items to only include those with changes
	var filteredFlatItems []*plan.TreeNode
	for _, item := range flatItems {
		if item.HasChanges {
			filteredFlatItems = append(filteredFlatItems, item)
		}
	}

	return Model{
		state:                    StatePlanReview,
		planReport:               report,
		planTreeRoots:            roots,
		planFlatItems:            filteredFlatItems,
		planListCursor:           0,
		planDetailScrollOffset:   0,
		planReviewFocusedElement: 0,
		planTargetStats:          targetStats,
		planDependencyStats:      dependencyStats,
		ready:                    false,
	}
}

// flattenTree converts the recursive tree into a linear list (DFS)
func flattenTree(nodes []*plan.TreeNode) []*plan.TreeNode {
	var items []*plan.TreeNode
	for _, node := range nodes {
		items = append(items, node)
		if len(node.Children) > 0 {
			items = append(items, flattenTree(node.Children)...)
		}
	}
	return items
}

// Init initializes the model (BubbleTea interface).
func (m Model) Init() tea.Cmd {
	return nil
}

// Update handles messages and updates the model (BubbleTea interface).
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch m.state {
	case StateNavigation:
		return m.handleNavigationUpdate(msg)
	case StateHistory:
		return m.handleHistoryUpdate(msg)
	case StatePlanReview:
		return m.handlePlanReviewUpdate(msg)
	}
	return m, nil
}

// View renders the UI (BubbleTea interface).
func (m Model) View() string {
	switch m.state {
	case StateNavigation:
		return m.renderNavigationView()
	case StateHistory:
		return m.renderHistoryView()
	case StatePlanReview:
		return m.renderPlanReviewView()
	}
	return "Unknown state"
}

// calculateColumnWidth computes the static width for all columns.
// Uses configured maxNavigationColumns for sliding window.
func (m Model) calculateColumnWidth() int {
	maxDepth := m.navigator.GetMaxDepth()
	if maxDepth == 0 {
		return MinColumnWidth
	}

	// Calculate for 1 commands + N navigation columns max (configurable)
	maxVisibleColumns := 1 + m.maxNavigationColumns
	totalOverhead := ColumnOverhead * maxVisibleColumns
	availableWidth := m.width - totalOverhead
	colWidth := availableWidth / maxVisibleColumns

	if colWidth < MinColumnWidth {
		return MinColumnWidth
	}
	return colWidth
}

// isCommandsColumnFocused returns true if the commands column is focused.
func (m Model) isCommandsColumnFocused() bool {
	return m.focusedColumn == 0
}

// getAvailableHeight calculates the available height for list items.
// Subtracts header, footer, breadcrumb bar, column title, and padding.
func (m Model) getAvailableHeight() int {
	// Total reserved space:
	// - HeaderHeight (1)
	// - Breadcrumb bar (1)
	// - Column title (1)
	// - Empty line after title (1)
	// - FooterHeight (1)
	// - ColumnPadding (4) - includes borders and internal padding
	reservedSpace := HeaderHeight + 1 + 1 + 1 + FooterHeight + ColumnPadding
	availableHeight := m.height - reservedSpace

	if availableHeight < 1 {
		return 1 // Minimum height to avoid division by zero
	}
	return availableHeight
}

// getMaxVisibleItems returns the maximum number of items that can be displayed
// in a column given the current terminal height.
// Reserves 1 line for pagination indicators to ensure consistent column heights.
func (m Model) getMaxVisibleItems() int {
	availableHeight := m.getAvailableHeight()

	// Reserve 1 line for pagination indicators to ensure all columns have same height.
	// Even if a column doesn't need pagination, this reserved space keeps heights consistent.
	reservedForPagination := 1
	maxItems := availableHeight - reservedForPagination

	if maxItems < 1 {
		return 1
	}

	return maxItems
}

// getTotalPages calculates the total number of pages for a list.
func (m Model) getTotalPages(totalItems int) int {
	maxVisibleItems := m.getMaxVisibleItems()
	if maxVisibleItems <= 0 {
		return 1
	}
	if totalItems <= maxVisibleItems {
		return 1
	}
	pages := (totalItems + maxVisibleItems - 1) / maxVisibleItems
	return pages
}

// getCurrentPage calculates the current page number (1-indexed) based on scroll offset.
func (m Model) getCurrentPage(columnID int) int {
	maxVisibleItems := m.getMaxVisibleItems()
	if maxVisibleItems <= 0 {
		return 1
	}
	scrollOffset := m.scrollOffsets[columnID]
	currentPage := (scrollOffset / maxVisibleItems) + 1
	return currentPage
}

// getPageStartIndex returns the start index for a given page number (1-indexed).
func (m Model) getPageStartIndex(pageNumber int) int {
	maxVisibleItems := m.getMaxVisibleItems()
	if pageNumber <= 1 {
		return 0
	}
	return (pageNumber - 1) * maxVisibleItems
}

// getNavigationDepth returns the current navigation depth (0-indexed).
// Returns -1 if not in a navigation column.
func (m Model) getNavigationDepth() int {
	if m.isCommandsColumnFocused() {
		return -1
	}
	return m.focusedColumn - 1
}

// GetSelectedCommand returns the currently selected command name.
func (m Model) GetSelectedCommand() string {
	if m.selectedCommand >= 0 && m.selectedCommand < len(m.commands) {
		return m.commands[m.selectedCommand]
	}
	return NoItemSelected
}

// GetSelectedStackPath returns the selected stack path.
func (m Model) GetSelectedStackPath() string {
	var targetNode *stack.Node

	if m.isCommandsColumnFocused() {
		targetNode = m.navigator.GetRoot()
	} else {
		depth := m.getNavigationDepth()
		targetNode = m.navigator.GetNodeAtDepth(m.navState, depth)
	}

	if targetNode != nil {
		return targetNode.Path
	}
	return NoItemSelected
}

// IsConfirmed returns whether the user confirmed the selection.
func (m Model) IsConfirmed() bool {
	return m.confirmed
}

// getCurrentNavigationPath returns the current navigation path as a string.
// Delegates to Navigator for path construction business logic.
func (m Model) getCurrentNavigationPath() string {
	depth := m.getNavigationDepth()
	return m.navigator.GetNavigationPath(m.navState, depth)
}

// hasLeftOverflow returns true if there are navigation columns to the left.
func (m Model) hasLeftOverflow() bool {
	return m.navigationOffset > 0
}

// canAdvanceFurther returns true if the currently focused node has children.
// This determines if the user can navigate deeper into the hierarchy.
func (m Model) canAdvanceFurther() bool {
	if m.isCommandsColumnFocused() {
		return false
	}

	depth := m.getNavigationDepth()
	if depth < 0 || depth >= len(m.navState.CurrentNodes) {
		return false
	}

	currentNode := m.navState.CurrentNodes[depth]
	if currentNode == nil {
		return false
	}

	return currentNode.HasChildren()
}

// hasRightOverflow returns true if there are navigation columns to the right.
// Shows indicator if: 1) sliding window doesn't cover last levels AND 2) current node has children.
func (m Model) hasRightOverflow() bool {
	maxDepth := m.navigator.GetMaxDepth()

	if m.navigationOffset+3 >= maxDepth {
		return false
	}

	// (Don't show arrow if we're at a leaf node even if maxDepth is deeper)
	if !m.canAdvanceFurther() {
		return false
	}

	return true
}

// filterItems filters a list of items based on the filter text (case-insensitive).
func filterItems(items []string, filterText string) []string {
	if filterText == "" {
		return items
	}

	filtered := make([]string, 0)
	filterLower := strings.ToLower(filterText)

	for _, item := range items {
		if strings.Contains(strings.ToLower(item), filterLower) {
			filtered = append(filtered, item)
		}
	}

	return filtered
}

// getFilteredCommands returns the commands list with active filter applied.
func (m *Model) getFilteredCommands() []string {
	if filter, exists := m.columnFilters[0]; exists {
		filterValue := filter.Value()
		if filterValue != "" {
			return filterItems(m.commands, filterValue)
		}
	}
	return m.commands
}

// getFilteredNavigationItems returns the navigation items for a depth with active filter applied.
func (m *Model) getFilteredNavigationItems(depth int) []string {
	if depth < 0 || depth >= len(m.navState.Columns) {
		return []string{}
	}

	items := m.navState.Columns[depth]
	columnID := depth + 1

	if filter, exists := m.columnFilters[columnID]; exists {
		filterValue := filter.Value()
		if filterValue != "" {
			return filterItems(items, filterValue)
		}
	}
	return items
}

// findOriginalIndex maps a filtered index back to the original unfiltered index.
func findOriginalIndex(originalItems []string, filteredItems []string, filteredIndex int) int {
	if filteredIndex < 0 || filteredIndex >= len(filteredItems) {
		return -1
	}

	targetItem := filteredItems[filteredIndex]
	for i, item := range originalItems {
		if item == targetItem {
			return i
		}
	}
	return -1
}

// findFilteredIndex maps an original index to the filtered index.
func findFilteredIndex(originalItems []string, filteredItems []string, originalIndex int) int {
	if originalIndex < 0 || originalIndex >= len(originalItems) {
		return -1
	}

	targetItem := originalItems[originalIndex]
	for i, item := range filteredItems {
		if item == targetItem {
			return i
		}
	}
	return -1
}

// ShouldReExecuteFromHistory returns true if a history entry was selected for re-execution.
func (m Model) ShouldReExecuteFromHistory() bool {
	return m.reExecuteFromHistory
}

// GetSelectedHistoryEntry returns the history entry selected for re-execution.
// Returns nil if no entry was selected.
func (m Model) GetSelectedHistoryEntry() *history.ExecutionLogEntry {
	return m.selectedHistoryEntry
}
