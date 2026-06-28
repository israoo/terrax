# Multi-Stack Selection Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Allow users to mark N stacks (or directories) with `Space` in the TUI and execute a single Terragrunt command across all of them via the existing `--filter`-based execution pipeline.

**Architecture:** Selection state (`selectedPaths map[string]bool`) lives in `tui.Model` — UI concern, not navigator concern. The Navigator gains one new method to resolve item paths by depth+index. At execution time, all marked paths are expanded and fed into the existing `collectTransitiveDeps` → `buildGroupedExecution` → `executor.Run` chain.

**Tech Stack:** Go 1.25.5 · Bubble Tea 1.3.10 · Lipgloss 1.1.0

## Global Constraints

- All comments must end with periods.
- Imports: three groups (stdlib, third-party, internal), each alphabetically sorted.
- Errors wrapped with `fmt.Errorf("...: %w", err)`.
- Paths always via `filepath.Join()`.
- `lipgloss.Copy()` does not exist in Lipgloss 1.x — always `lipgloss.NewStyle()`.
- Navigator has zero Bubble Tea dependencies.
- `internal/tui/model.go` — UI state only; never modifies tree.
- `internal/tui/view.go` — pure rendering, never modifies state.
- `cmd/root.go` — CLI glue only.
- Test files live alongside implementation.
- Run `task check` before every commit.

---

### Task 1: Navigator.GetPathAtDepthAndIndex

**Files:**
- Modify: `internal/stack/navigator.go`
- Modify: `internal/stack/navigator_test.go`

**Interfaces:**
- Produces: `func (nav *Navigator) GetPathAtDepthAndIndex(state *NavigationState, depth, index int) string`
  - `depth`: 0-based navigation column index (0 = first nav column, not the commands column)
  - `index`: position within that column's item list (original, unfiltered)
  - Returns absolute path of the node at that position, or `""` if out of bounds

- [ ] **Step 1: Write the failing test**

Add to `internal/stack/navigator_test.go` after the last existing test function:

```go
func TestNavigator_GetPathAtDepthAndIndex(t *testing.T) {
	root := &Node{
		Name: "root",
		Path: "/repo",
		Children: []*Node{
			{
				Name: "env",
				Path: "/repo/env",
				Children: []*Node{
					{Name: "dev", Path: "/repo/env/dev", IsStack: true},
					{Name: "prod", Path: "/repo/env/prod", IsStack: true},
				},
			},
			{
				Name: "other",
				Path: "/repo/other",
			},
		},
	}

	nav := NewNavigator(root, 2)
	state := NewNavigationState(2)
	// Select index 0 at depth 0 ("env") so CurrentNodes[0] = env node.
	nav.PropagateSelection(state)

	tests := []struct {
		name     string
		depth    int
		index    int
		expected string
	}{
		{
			name:     "depth 0 index 0 returns first child of root",
			depth:    0,
			index:    0,
			expected: "/repo/env",
		},
		{
			name:     "depth 0 index 1 returns second child of root",
			depth:    0,
			index:    1,
			expected: "/repo/other",
		},
		{
			name:     "depth 1 index 0 returns first child of selected depth-0 node",
			depth:    1,
			index:    0,
			expected: "/repo/env/dev",
		},
		{
			name:     "depth 1 index 1 returns second child of selected depth-0 node",
			depth:    1,
			index:    1,
			expected: "/repo/env/prod",
		},
		{
			name:     "negative depth returns empty string",
			depth:    -1,
			index:    0,
			expected: "",
		},
		{
			name:     "depth out of bounds returns empty string",
			depth:    5,
			index:    0,
			expected: "",
		},
		{
			name:     "index out of bounds returns empty string",
			depth:    0,
			index:    99,
			expected: "",
		},
		{
			name:     "negative index returns empty string",
			depth:    0,
			index:    -1,
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := nav.GetPathAtDepthAndIndex(state, tt.depth, tt.index)
			assert.Equal(t, tt.expected, got)
		})
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

```bash
cd /Users/isra/Repos/israoo/terrax && go test ./internal/stack/... -run TestNavigator_GetPathAtDepthAndIndex -v
```

Expected: FAIL — `nav.GetPathAtDepthAndIndex undefined`

- [ ] **Step 3: Implement GetPathAtDepthAndIndex in navigator.go**

Add this method at the end of `internal/stack/navigator.go`, before the final closing of the file:

```go
// GetPathAtDepthAndIndex returns the absolute path of the item at position index
// in the navigation column at the given depth. depth is 0-based (0 = first nav column).
// Returns empty string if depth, index, or any required node is out of bounds or nil.
func (nav *Navigator) GetPathAtDepthAndIndex(state *NavigationState, depth, index int) string {
	if depth < 0 || depth >= nav.maxDepth || state == nil {
		return ""
	}

	var parent *Node
	if depth == 0 {
		parent = nav.root
	} else {
		parent = state.CurrentNodes[depth-1]
	}

	if parent == nil || index < 0 || index >= len(parent.Children) {
		return ""
	}

	return parent.Children[index].Path
}
```

- [ ] **Step 4: Run test to verify it passes**

```bash
cd /Users/isra/Repos/israoo/terrax && go test ./internal/stack/... -run TestNavigator_GetPathAtDepthAndIndex -v
```

Expected: PASS

- [ ] **Step 5: Run full stack tests**

```bash
cd /Users/isra/Repos/israoo/terrax && go test ./internal/stack/... -v
```

Expected: all PASS

- [ ] **Step 6: Commit**

```bash
git add internal/stack/navigator.go internal/stack/navigator_test.go
git commit -m "feat(navigator): add GetPathAtDepthAndIndex for multi-stack selection"
```

---

### Task 2: Selection State in Model

**Files:**
- Modify: `internal/tui/model.go`
- Modify: `internal/tui/model_test_helpers.go`
- Modify: `internal/tui/model_test.go` (add new tests at the end)

**Interfaces:**
- Consumes: `Navigator.GetPathAtDepthAndIndex` from Task 1
- Produces:
  - Field `selectedPaths map[string]bool` on `Model`
  - `func (m Model) GetSelectedStackPaths() []string` — returns sorted slice of all marked paths
  - `func (m Model) HasSelectedPaths() bool` — true when len(selectedPaths) > 0
  - `func (m *Model) toggleSelectedPath(path string)` — unexported, toggles path; if any ancestor is already marked, no-op
  - `func (m *Model) clearSelectedPaths()` — unexported, resets the map

- [ ] **Step 1: Write failing tests**

Add to `internal/tui/model_test.go` (at the end of the file):

```go
func TestModel_GetSelectedStackPaths_Empty(t *testing.T) {
	root := &stack.Node{
		Name:     "root",
		Path:     "/repo",
		Children: []*stack.Node{{Name: "env", Path: "/repo/env"}},
	}
	m := NewModel(root, 1, []string{"plan"}, 3)
	paths := m.GetSelectedStackPaths()
	assert.Empty(t, paths)
}

func TestModel_GetSelectedStackPaths_ReturnsMarked(t *testing.T) {
	root := &stack.Node{
		Name:     "root",
		Path:     "/repo",
		Children: []*stack.Node{{Name: "env", Path: "/repo/env"}},
	}
	m := NewModel(root, 1, []string{"plan"}, 3)
	m.selectedPaths = map[string]bool{
		"/repo/env": true,
		"/repo/app": true,
	}
	paths := m.GetSelectedStackPaths()
	assert.Len(t, paths, 2)
	assert.Contains(t, paths, "/repo/env")
	assert.Contains(t, paths, "/repo/app")
}

func TestModel_HasSelectedPaths(t *testing.T) {
	root := &stack.Node{Name: "root", Path: "/repo"}
	m := NewModel(root, 1, []string{"plan"}, 3)
	assert.False(t, m.HasSelectedPaths())

	m.selectedPaths["/repo/env"] = true
	assert.True(t, m.HasSelectedPaths())
}

func TestModel_ToggleSelectedPath_AddsAndRemoves(t *testing.T) {
	root := &stack.Node{Name: "root", Path: "/repo"}
	m := NewModel(root, 1, []string{"plan"}, 3)

	m.toggleSelectedPath("/repo/env")
	assert.True(t, m.selectedPaths["/repo/env"])

	m.toggleSelectedPath("/repo/env")
	assert.False(t, m.selectedPaths["/repo/env"])
}

func TestModel_ToggleSelectedPath_NoopWhenAncestorMarked(t *testing.T) {
	root := &stack.Node{Name: "root", Path: "/repo"}
	m := NewModel(root, 1, []string{"plan"}, 3)

	m.selectedPaths["/repo/env"] = true
	m.toggleSelectedPath("/repo/env/dev") // ancestor "/repo/env" is marked
	assert.False(t, m.selectedPaths["/repo/env/dev"], "descendant should not be added when ancestor is marked")
	assert.True(t, m.selectedPaths["/repo/env"], "ancestor mark must stay")
}

func TestModel_ClearSelectedPaths(t *testing.T) {
	root := &stack.Node{Name: "root", Path: "/repo"}
	m := NewModel(root, 1, []string{"plan"}, 3)
	m.selectedPaths["/repo/env"] = true
	m.selectedPaths["/repo/app"] = true

	m.clearSelectedPaths()
	assert.Empty(t, m.selectedPaths)
}
```

- [ ] **Step 2: Run tests to verify they fail**

```bash
cd /Users/isra/Repos/israoo/terrax && go test ./internal/tui/... -run "TestModel_GetSelectedStack|TestModel_HasSelected|TestModel_Toggle|TestModel_Clear" -v
```

Expected: compile error — `selectedPaths undefined`

- [ ] **Step 3: Add selectedPaths field and methods to model.go**

In `internal/tui/model.go`, add the field to the `Model` struct. Find the `// State flags` block and add after it:

```go
	// Multi-stack selection
	selectedPaths map[string]bool // absolute paths of explicitly marked nodes
```

In `NewModel`, after `scrollOffsets: make(map[int]int),` add:

```go
		selectedPaths: make(map[string]bool),
```

Add these methods at the end of `internal/tui/model.go` (before the end of the file). The `"path/filepath"` import must also be added to the import block.

```go
// GetSelectedStackPaths returns all explicitly marked paths as a slice.
// Returns nil when no paths are marked.
func (m Model) GetSelectedStackPaths() []string {
	if len(m.selectedPaths) == 0 {
		return nil
	}
	paths := make([]string, 0, len(m.selectedPaths))
	for p := range m.selectedPaths {
		paths = append(paths, p)
	}
	return paths
}

// HasSelectedPaths returns true when at least one path is marked.
func (m Model) HasSelectedPaths() bool {
	return len(m.selectedPaths) > 0
}

// toggleSelectedPath adds path to selectedPaths if absent, removes it if present.
// No-op when any ancestor path is already in selectedPaths.
func (m *Model) toggleSelectedPath(path string) {
	if path == "" {
		return
	}
	if hasMarkedAncestor(path, m.selectedPaths) {
		return
	}
	if m.selectedPaths[path] {
		delete(m.selectedPaths, path)
	} else {
		m.selectedPaths[path] = true
	}
}

// clearSelectedPaths removes all marks.
func (m *Model) clearSelectedPaths() {
	m.selectedPaths = make(map[string]bool)
}

// hasMarkedAncestor returns true if any strict ancestor directory of path
// is present in selectedPaths. Uses a separate cursor variable to avoid
// mutating the original path argument.
func hasMarkedAncestor(path string, selectedPaths map[string]bool) bool {
	cur := filepath.Dir(path)
	for cur != path {
		if selectedPaths[cur] {
			return true
		}
		path = cur
		cur = filepath.Dir(cur)
	}
	return false
}
```

The imports block in `model.go` must include `"path/filepath"`:

```go
import (
	"path/filepath"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/israoo/terrax/internal/history"
	"github.com/israoo/terrax/internal/plan"
	"github.com/israoo/terrax/internal/stack"
)
```

- [ ] **Step 4: Update model_test_helpers.go to initialize selectedPaths**

In `internal/tui/model_test_helpers.go`, find the `m := Model{...}` literal and add:

```go
		selectedPaths:        make(map[string]bool),
```

after the `columnWidth: 25,` line.

- [ ] **Step 5: Run tests to verify they pass**

```bash
cd /Users/isra/Repos/israoo/terrax && go test ./internal/tui/... -run "TestModel_GetSelectedStack|TestModel_HasSelected|TestModel_Toggle|TestModel_Clear" -v
```

Expected: all PASS

- [ ] **Step 6: Run full TUI tests**

```bash
cd /Users/isra/Repos/israoo/terrax && go test ./internal/tui/... -v
```

Expected: all PASS (no regressions from adding the field and methods)

- [ ] **Step 7: Commit**

```bash
git add internal/tui/model.go internal/tui/model_test_helpers.go internal/tui/model_test.go
git commit -m "feat(tui): add selectedPaths state and toggle/clear methods to Model"
```

---

### Task 3: Space Key Handler and Esc/Q Clear Behavior

**Files:**
- Modify: `internal/tui/update.go`
- Modify: `internal/tui/update_test.go`

**Interfaces:**
- Consumes: `Model.toggleSelectedPath(path string)`, `Model.clearSelectedPaths()`, `Model.HasSelectedPaths() bool` from Task 2
- Consumes: `Navigator.GetPathAtDepthAndIndex(state, depth, index)` from Task 1

- [ ] **Step 1: Write failing tests**

Add to `internal/tui/update_test.go` at the end:

```go
func TestModel_SpaceKey_MarksCurrentItem(t *testing.T) {
	root := &stack.Node{
		Name: "root",
		Path: "/repo",
		Children: []*stack.Node{
			{Name: "env", Path: "/repo/env", IsStack: true},
		},
	}
	m := NewModel(root, 1, []string{"plan"}, 3)
	m.width = 120
	m.height = 30
	m.columnWidth = 25
	m.ready = true
	m.focusedColumn = 1 // First nav column (depth 0)

	msg := tea.KeyMsg{Type: tea.KeySpace}
	updated, _ := m.handleKeyPress(msg)
	finalModel := updated.(Model)

	assert.True(t, finalModel.HasSelectedPaths(), "space should mark the current item")
	assert.Contains(t, finalModel.selectedPaths, "/repo/env")
}

func TestModel_SpaceKey_UnmarksAlreadyMarked(t *testing.T) {
	root := &stack.Node{
		Name: "root",
		Path: "/repo",
		Children: []*stack.Node{
			{Name: "env", Path: "/repo/env", IsStack: true},
		},
	}
	m := NewModel(root, 1, []string{"plan"}, 3)
	m.width = 120
	m.height = 30
	m.columnWidth = 25
	m.ready = true
	m.focusedColumn = 1
	m.selectedPaths["/repo/env"] = true

	msg := tea.KeyMsg{Type: tea.KeySpace}
	updated, _ := m.handleKeyPress(msg)
	finalModel := updated.(Model)

	assert.False(t, finalModel.HasSelectedPaths(), "second space should unmark the item")
}

func TestModel_SpaceKey_CommandsColumn_Noop(t *testing.T) {
	root := &stack.Node{
		Name: "root",
		Path: "/repo",
		Children: []*stack.Node{
			{Name: "env", Path: "/repo/env", IsStack: true},
		},
	}
	m := NewModel(root, 1, []string{"plan"}, 3)
	m.width = 120
	m.height = 30
	m.columnWidth = 25
	m.ready = true
	m.focusedColumn = 0 // Commands column

	msg := tea.KeyMsg{Type: tea.KeySpace}
	updated, _ := m.handleKeyPress(msg)
	finalModel := updated.(Model)

	assert.False(t, finalModel.HasSelectedPaths(), "space on commands column should be a no-op")
}

func TestModel_EscKey_ClearsMarksBeforeQuit(t *testing.T) {
	root := &stack.Node{Name: "root", Path: "/repo"}
	m := NewModel(root, 1, []string{"plan"}, 3)
	m.ready = true
	m.selectedPaths["/repo/env"] = true

	msg := tea.KeyMsg{Type: tea.KeyEsc}
	updated, cmd := m.handleKeyPress(msg)
	finalModel := updated.(Model)

	assert.Nil(t, cmd, "first esc with marks should clear, not quit")
	assert.False(t, finalModel.HasSelectedPaths(), "marks should be cleared")
}

func TestModel_EnterKey_WithMarks_UsesMarkedPaths(t *testing.T) {
	root := &stack.Node{
		Name: "root",
		Path: "/repo",
		Children: []*stack.Node{
			{Name: "env", Path: "/repo/env", IsStack: true},
		},
	}
	m := NewModel(root, 1, []string{"plan"}, 3)
	m.width = 120
	m.height = 30
	m.columnWidth = 25
	m.ready = true
	m.focusedColumn = 1
	m.selectedPaths["/repo/env"] = true

	updated, cmd := m.handleEnterKey()
	finalModel := updated.(Model)

	assert.NotNil(t, cmd, "enter with marks should return quit")
	assert.True(t, finalModel.confirmed)
	assert.True(t, finalModel.HasSelectedPaths(), "marks remain after confirmation")
}
```

- [ ] **Step 2: Run tests to verify they fail**

```bash
cd /Users/isra/Repos/israoo/terrax && go test ./internal/tui/... -run "TestModel_SpaceKey|TestModel_EscKey_Clears|TestModel_EnterKey_WithMarks" -v
```

Expected: FAIL

- [ ] **Step 3: Handle Space key in handleKeyPress**

In `internal/tui/update.go`, in the `switch msg.Type` block in `handleKeyPress` (normal navigation mode section), add a new case after `case tea.KeyEnter:`:

```go
	case tea.KeySpace:
		return m.handleSpaceKey(), nil
```

Then add the method at the end of `update.go`:

```go
// handleSpaceKey marks or unmarks the item under the cursor.
// No-op when the commands column is focused.
func (m Model) handleSpaceKey() Model {
	if m.isCommandsColumnFocused() {
		return m
	}
	depth := m.getNavigationDepth()
	if depth < 0 || depth >= len(m.navState.SelectedIndices) {
		return m
	}
	index := m.navState.SelectedIndices[depth]
	path := m.navigator.GetPathAtDepthAndIndex(m.navState, depth, index)
	if path == "" {
		return m
	}
	m.toggleSelectedPath(path)
	return m
}
```

- [ ] **Step 4: Update esc and q to clear marks before quitting**

In `handleKeyPress`, find:

```go
	case tea.KeyCtrlC, tea.KeyEsc:
		return m, tea.Quit
```

Replace with:

```go
	case tea.KeyCtrlC, tea.KeyEsc:
		if msg.Type == tea.KeyEsc && m.HasSelectedPaths() {
			m.clearSelectedPaths()
			return m, nil
		}
		return m, tea.Quit
```

Find the `q` handler:

```go
		if msg.String() == KeyQ {
			return m, tea.Quit
		}
```

Replace with:

```go
		if msg.String() == KeyQ {
			if m.HasSelectedPaths() {
				m.clearSelectedPaths()
				return m, nil
			}
			return m, tea.Quit
		}
```

- [ ] **Step 5: Update handleEnterKey to use selectedPaths when present**

`handleEnterKey` already works correctly — when `selectedPaths` has values, `confirmed = true` is set and `GetSelectedStackPaths()` will return them when called from `cmd/root.go`. No change needed to `handleEnterKey` itself: it sets `confirmed = true` and quits regardless, and `cmd/root.go` reads both `GetSelectedStackPath()` (single) and `GetSelectedStackPaths()` (multi) after the fact.

The only change is for the commands column + marks case: when marks are present and the user presses Enter from the commands column, they should be used. Currently `handleEnterKey` in commands-column mode uses root node — that's still correct for execution since `collectTransitiveDeps` in Task 6 will use `GetSelectedStackPaths()` instead. No change needed here.

- [ ] **Step 6: Run tests**

```bash
cd /Users/isra/Repos/israoo/terrax && go test ./internal/tui/... -run "TestModel_SpaceKey|TestModel_EscKey_Clears|TestModel_EnterKey_WithMarks" -v
```

Expected: all PASS

- [ ] **Step 7: Run full TUI tests**

```bash
cd /Users/isra/Repos/israoo/terrax && go test ./internal/tui/... -v
```

Expected: all PASS

- [ ] **Step 8: Commit**

```bash
git add internal/tui/update.go internal/tui/update_test.go
git commit -m "feat(tui): handle Space key for multi-stack selection and esc/q clear behavior"
```

---

### Task 4: Visual Markers in Navigation Columns

**Files:**
- Modify: `internal/tui/view_navigation.go`
- Modify: `internal/tui/styles.go`
- Modify: `internal/tui/view_navigation_test.go`

**Interfaces:**
- Consumes: `Model.selectedPaths`, `Model.HasSelectedPaths()`, `hasMarkedAncestor(path, map)` from Task 2
- Consumes: `Navigator.GetPathAtDepthAndIndex` from Task 1
- Produces: `isMarkedOrAncestorMarked(path string, selectedPaths map[string]bool) bool` (unexported helper)

- [ ] **Step 1: Write failing tests**

Add to `internal/tui/view_navigation_test.go` at the end:

```go
func TestIsMarkedOrAncestorMarked(t *testing.T) {
	selectedPaths := map[string]bool{
		"/repo/env": true,
	}

	tests := []struct {
		name     string
		path     string
		expected bool
	}{
		{
			name:     "exact match",
			path:     "/repo/env",
			expected: true,
		},
		{
			name:     "child of marked path",
			path:     "/repo/env/dev",
			expected: true,
		},
		{
			name:     "deep descendant of marked path",
			path:     "/repo/env/dev/app",
			expected: true,
		},
		{
			name:     "sibling - not marked",
			path:     "/repo/app",
			expected: false,
		},
		{
			name:     "parent of marked - not marked",
			path:     "/repo",
			expected: false,
		},
		{
			name:     "empty map",
			path:     "/repo/env",
			expected: false,
		},
	}

	emptyMap := map[string]bool{}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := selectedPaths
			if tt.name == "empty map" {
				m = emptyMap
			}
			got := isMarkedOrAncestorMarked(tt.path, m)
			assert.Equal(t, tt.expected, got)
		})
	}
}

func TestBuildNavigationList_ShowsMarkers(t *testing.T) {
	root := &stack.Node{
		Name: "root",
		Path: "/repo",
		Children: []*stack.Node{
			{Name: "env", Path: "/repo/env", IsStack: true},
			{Name: "app", Path: "/repo/app", IsStack: true},
		},
	}
	m := NewModel(root, 1, []string{"plan"}, 3)
	m.width = 120
	m.height = 30
	m.columnWidth = 25
	m.ready = true
	m.selectedPaths["/repo/env"] = true

	layout := NewLayoutCalculator(m.width, m.height, m.columnWidth)
	r := NewRenderer(m, layout)

	col := r.buildNavigationList(0)
	assert.Contains(t, col, "●", "marked item should show filled marker")
	assert.Contains(t, col, "○", "unmarked item should show empty marker when marks are active")
}
```

- [ ] **Step 2: Run tests to verify they fail**

```bash
cd /Users/isra/Repos/israoo/terrax && go test ./internal/tui/... -run "TestIsMarkedOrAncestorMarked|TestBuildNavigationList_ShowsMarkers" -v
```

Expected: FAIL

- [ ] **Step 3: Add marker style to styles.go**

In `internal/tui/styles.go`, add two new styles at the end of the `var` block:

```go
	// Marker styles for multi-stack selection.
	markedStyle   = lipgloss.NewStyle().Foreground(accentColor).Bold(true)
	unmarkedStyle = lipgloss.NewStyle().Foreground(dimColor)
```

- [ ] **Step 4: Add isMarkedOrAncestorMarked to view_navigation.go**

At the top of `internal/tui/view_navigation.go`, add the import `"path/filepath"` to the import block (currently only `"fmt"` and `"github.com/charmbracelet/lipgloss"`):

```go
import (
	"fmt"
	"path/filepath"

	"github.com/charmbracelet/lipgloss"
)
```

Add this helper function at the end of `view_navigation.go`:

```go
// isMarkedOrAncestorMarked returns true if path itself or any of its ancestor
// directories is in selectedPaths.
func isMarkedOrAncestorMarked(path string, selectedPaths map[string]bool) bool {
	if selectedPaths[path] {
		return true
	}
	cur := filepath.Dir(path)
	for cur != path {
		if selectedPaths[cur] {
			return true
		}
		path = cur
		cur = filepath.Dir(cur)
	}
	return false
}
```

- [ ] **Step 5: Update renderItemList signature and implementation**

The current `renderItemList` signature in `view_navigation.go`:

```go
func renderItemList(
	items []string,
	startIdx, endIdx int,
	selectedFilteredIndex int,
	maxVisibleItems int,
	maxTextWidth int,
	totalPages, currentPage int,
) string
```

Add a `markedItems []bool` parameter (nil = no markers shown):

```go
func renderItemList(
	items []string,
	startIdx, endIdx int,
	selectedFilteredIndex int,
	maxVisibleItems int,
	maxTextWidth int,
	totalPages, currentPage int,
	markedItems []bool,
) string
```

Update the rendering loop inside `renderItemList`:

Replace:

```go
		// Truncate text to fit within column width
		displayText := truncateText(items[i], maxTextWidth)
		content += fmt.Sprintf("%s %s\n", cursor, style.Render(displayText))
```

With:

```go
		// Truncate text to fit within column width
		displayText := truncateText(items[i], maxTextWidth)
		if markedItems != nil {
			var marker string
			if i < len(markedItems) && markedItems[i] {
				marker = markedStyle.Render("●") + " "
			} else {
				marker = unmarkedStyle.Render("○") + " "
			}
			content += fmt.Sprintf("%s %s%s\n", cursor, marker, style.Render(displayText))
		} else {
			content += fmt.Sprintf("%s %s\n", cursor, style.Render(displayText))
		}
```

- [ ] **Step 6: Update buildCommandList to pass nil for markedItems**

In `buildCommandList`, the final call to `renderItemList` currently ends without `markedItems`. Add `nil` as the last argument:

```go
	return renderItemList(
		commands,
		startIdx, endIdx,
		selectedFilteredIndex,
		maxVisibleItems,
		maxTextWidth,
		totalPages, currentPage,
		nil,
	)
```

- [ ] **Step 7: Update buildNavigationList to compute and pass markedItems**

In `buildNavigationList`, after computing `startIdx, endIdx` and before calling `renderItemList`, add:

```go
	// Compute marker state for each visible filtered item.
	var markedItems []bool
	if r.model.HasSelectedPaths() {
		markedItems = make([]bool, len(items))
		for i := range items {
			origIdx := findOriginalIndex(originalItems, items, i)
			if origIdx < 0 {
				continue
			}
			path := r.model.navigator.GetPathAtDepthAndIndex(r.model.navState, depth, origIdx)
			if path != "" {
				markedItems[i] = isMarkedOrAncestorMarked(path, r.model.selectedPaths)
			}
		}
	}
```

Then update the `renderItemList` call to pass `markedItems`:

```go
	return renderItemList(
		items,
		startIdx, endIdx,
		selectedFilteredIndex,
		maxVisibleItems,
		maxTextWidth,
		totalPages, currentPage,
		markedItems,
	)
```

- [ ] **Step 8: Update getMaxItemTextWidth to account for marker width**

In `view_navigation.go`, `getMaxItemTextWidth` currently:

```go
func (r *Renderer) getMaxItemTextWidth() int {
	columnWidth := r.layout.GetColumnWidth()
	reservedSpace := CursorWidth + ItemStylePadding + ColumnStylePadding
	maxWidth := columnWidth - reservedSpace
	if maxWidth < MinItemTextWidth {
		maxWidth = MinItemTextWidth
	}
	return maxWidth
}
```

Add a `hasMarkers bool` parameter:

```go
func (r *Renderer) getMaxItemTextWidth(hasMarkers bool) int {
	columnWidth := r.layout.GetColumnWidth()
	reservedSpace := CursorWidth + ItemStylePadding + ColumnStylePadding
	if hasMarkers {
		reservedSpace += MarkerWidth
	}
	maxWidth := columnWidth - reservedSpace
	if maxWidth < MinItemTextWidth {
		maxWidth = MinItemTextWidth
	}
	return maxWidth
}
```

Add `MarkerWidth = 4` to `internal/tui/constants.go` (the `MarkerWidth` constant — 2 chars for the marker rune "●" / "○" plus 1 space plus 1 for lipgloss padding):

```go
	MarkerWidth = 4 // Width of selection marker prefix "● " rendered by Lipgloss
```

Update the two call sites for `getMaxItemTextWidth`:

In `buildCommandList`:
```go
	maxTextWidth := r.getMaxItemTextWidth(false)
```

In `buildNavigationList`:
```go
	maxTextWidth := r.getMaxItemTextWidth(r.model.HasSelectedPaths())
```

- [ ] **Step 9: Run tests**

```bash
cd /Users/isra/Repos/israoo/terrax && go test ./internal/tui/... -run "TestIsMarkedOrAncestorMarked|TestBuildNavigationList_ShowsMarkers" -v
```

Expected: all PASS

- [ ] **Step 10: Run full TUI tests**

```bash
cd /Users/isra/Repos/israoo/terrax && go test ./internal/tui/... -v
```

Expected: all PASS

- [ ] **Step 11: Commit**

```bash
git add internal/tui/view_navigation.go internal/tui/styles.go internal/tui/constants.go internal/tui/view_navigation_test.go
git commit -m "feat(tui): render selection markers in navigation columns"
```

---

### Task 5: Dynamic Footer

**Files:**
- Modify: `internal/tui/constants.go`
- Modify: `internal/tui/view_common.go`
- Modify: `internal/tui/view_common_test.go`

**Interfaces:**
- Consumes: `Model.HasSelectedPaths()`, `Model.selectedPaths` from Task 2

- [ ] **Step 1: Write failing test**

In `internal/tui/view_common_test.go`, find the existing footer test (`TestRenderer_RenderFooter`) and add a new test after it:

```go
func TestRenderer_RenderFooter_WithMarks(t *testing.T) {
	root := &stack.Node{
		Name:     "root",
		Path:     "/repo",
		Children: []*stack.Node{{Name: "env", Path: "/repo/env"}},
	}
	m := NewModel(root, 1, []string{"plan"}, 3)
	m.width = 120
	m.height = 30
	m.columnWidth = 25
	m.ready = true
	m.selectedPaths["/repo/env"] = true
	m.selectedPaths["/repo/app"] = true

	layout := NewLayoutCalculator(m.width, m.height, m.columnWidth)
	r := NewRenderer(m, layout)

	footer := r.renderFooter()
	assert.Contains(t, footer, "2", "footer should show mark count")
	assert.Contains(t, footer, "esc", "footer should mention esc to clear")
	assert.NotContains(t, footer, HelpText, "footer should not show default help text when marks are active")
}
```

- [ ] **Step 2: Run test to verify it fails**

```bash
cd /Users/isra/Repos/israoo/terrax && go test ./internal/tui/... -run TestRenderer_RenderFooter_WithMarks -v
```

Expected: FAIL

- [ ] **Step 3: Add HelpTextWithMarks format string to constants.go**

In `internal/tui/constants.go`, after `HelpText`, add:

```go
	// HelpTextWithMarks is the footer hint shown when stacks are marked for multi-execution.
	HelpTextWithMarks = "space: mark/unmark | ↑↓: navigate | enter: run on marked (%d) | esc: clear all | q: quit"
```

- [ ] **Step 4: Update renderFooter in view_common.go**

In `internal/tui/view_common.go`, add `"fmt"` to the import block (it's already there).

Replace:

```go
func (r *Renderer) renderFooter() string {
	return footerStyle.Render(HelpText)
}
```

With:

```go
func (r *Renderer) renderFooter() string {
	if r.model.HasSelectedPaths() {
		text := fmt.Sprintf(HelpTextWithMarks, len(r.model.selectedPaths))
		return footerStyle.Render(text)
	}
	return footerStyle.Render(HelpText)
}
```

- [ ] **Step 5: Run test**

```bash
cd /Users/isra/Repos/israoo/terrax && go test ./internal/tui/... -run TestRenderer_RenderFooter_WithMarks -v
```

Expected: PASS

- [ ] **Step 6: Run full TUI tests**

```bash
cd /Users/isra/Repos/israoo/terrax && go test ./internal/tui/... -v
```

Expected: all PASS

- [ ] **Step 7: Commit**

```bash
git add internal/tui/constants.go internal/tui/view_common.go internal/tui/view_common_test.go
git commit -m "feat(tui): dynamic footer shows mark count and esc hint when stacks are selected"
```

---

### Task 6: collectTransitiveDeps Multi-Path and Execution Integration

**Files:**
- Modify: `cmd/root.go`
- Modify: `cmd/execute.go`
- Modify: `cmd/run.go`
- Modify: `cmd/groups.go`
- Modify: `cmd/root_test.go`

**Interfaces:**
- Consumes: `Model.GetSelectedStackPaths() []string`, `Model.HasSelectedPaths() bool` from Task 2
- Produces: `func collectTransitiveDeps(stackPaths []string) (repoRoot string, filterPaths []string)` — replaces single-path version

- [ ] **Step 1: Write failing test for multi-path collectTransitiveDeps**

In `internal/tui/cmd/root_test.go` — wait, `collectTransitiveDeps` is in `cmd/root.go`. Add to `cmd/root_test.go` at the end of the file:

```go
func TestCollectTransitiveDeps_MultiplePaths(t *testing.T) {
	// Create two independent leaf stacks.
	tmpDir := t.TempDir()
	envDev := filepath.Join(tmpDir, "env", "dev")
	envProd := filepath.Join(tmpDir, "env", "prod")
	require.NoError(t, os.MkdirAll(envDev, 0755))
	require.NoError(t, os.MkdirAll(envProd, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(envDev, "terragrunt.hcl"), []byte(""), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(envProd, "terragrunt.hcl"), []byte(""), 0644))

	repoRoot, filterPaths := collectTransitiveDeps([]string{envDev, envProd})

	assert.NotEmpty(t, repoRoot)
	assert.Len(t, filterPaths, 2, "both stacks should be in filter paths")
	// Paths should be relative slashes.
	for _, p := range filterPaths {
		assert.False(t, filepath.IsAbs(p), "filter path should be relative: %s", p)
	}
}

func TestCollectTransitiveDeps_DeduplicatesPaths(t *testing.T) {
	tmpDir := t.TempDir()
	envDev := filepath.Join(tmpDir, "env", "dev")
	require.NoError(t, os.MkdirAll(envDev, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(envDev, "terragrunt.hcl"), []byte(""), 0644))

	// Pass the same path twice.
	repoRoot, filterPaths := collectTransitiveDeps([]string{envDev, envDev})

	assert.NotEmpty(t, repoRoot)
	assert.Len(t, filterPaths, 1, "duplicate paths should be deduplicated")
}

func TestCollectTransitiveDeps_EmptyInput(t *testing.T) {
	repoRoot, filterPaths := collectTransitiveDeps([]string{})
	assert.Empty(t, repoRoot)
	assert.Empty(t, filterPaths)
}
```

- [ ] **Step 2: Run tests to verify they fail**

```bash
cd /Users/isra/Repos/israoo/terrax && go test ./cmd/... -run "TestCollectTransitiveDeps_MultiplePaths|TestCollectTransitiveDeps_Deduplicates|TestCollectTransitiveDeps_Empty" -v
```

Expected: FAIL (compile error — signature mismatch)

- [ ] **Step 3: Update collectTransitiveDeps signature in root.go**

In `cmd/root.go`, replace the entire `collectTransitiveDeps` function (lines ~414–471):

```go
// collectTransitiveDeps computes the filter list for one or more stack paths.
// When include_dependencies is true, transitive dependencies are resolved
// via static HCL parsing and included in the filter list.
// When false, only the selected stack(s) are included — no dependency traversal.
// Non-leaf directories are expanded to all leaf stacks they contain via CollectStackPaths.
// All paths must reside under the same repository root; repoRoot is derived from stackPaths[0].
func collectTransitiveDeps(stackPaths []string) (repoRoot string, filterPaths []string) {
	if len(stackPaths) == 0 {
		return "", nil
	}

	rootConfigFile := viper.GetString("root_config_file")
	if rootConfigFile == "" {
		rootConfigFile = config.DefaultRootConfigFile
	}
	repoRoot = deps.FindRepoRoot(stackPaths[0], rootConfigFile)
	includeExternal := viper.GetBool("include_dependencies")

	// Seed the BFS queue from all input paths, expanding non-leaf directories.
	var seeds []string
	for _, stackPath := range stackPaths {
		hclFile := filepath.Join(stackPath, "terragrunt.hcl")
		if _, err := os.Stat(hclFile); err == nil {
			seeds = append(seeds, stackPath)
		} else {
			leafPaths, err := stack.CollectStackPaths(stackPath)
			if err != nil || len(leafPaths) == 0 {
				seeds = append(seeds, stackPath) // fallback
			} else {
				seeds = append(seeds, leafPaths...)
			}
		}
	}

	visited := map[string]bool{}
	queue := seeds

	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]

		if visited[current] {
			continue
		}
		visited[current] = true

		rel, err := filepath.Rel(repoRoot, current)
		if err == nil {
			filterPaths = append(filterPaths, filepath.ToSlash(rel))
		}

		// Only resolve transitive dependencies when include_dependencies is enabled.
		if includeExternal {
			depHCL := filepath.Join(current, "terragrunt.hcl")
			for _, dep := range deps.ParseDependencies(depHCL, repoRoot) {
				if !visited[dep] {
					queue = append(queue, dep)
				}
			}
		}
	}

	return repoRoot, filterPaths
}
```

- [ ] **Step 4: Update callers of collectTransitiveDeps**

In `cmd/root.go`, in `runTUI`, find:

```go
		repoRoot, filterPaths := collectTransitiveDeps(stackPath)
```

Replace with:

```go
		var execPaths []string
		if model.HasSelectedPaths() {
			execPaths = model.GetSelectedStackPaths()
		} else {
			execPaths = []string{stackPath}
		}
		repoRoot, filterPaths := collectTransitiveDeps(execPaths)
```

In `cmd/execute.go`, find:

```go
	repoRoot, filterPaths := collectTransitiveDeps(absolutePath)
```

Replace with:

```go
	repoRoot, filterPaths := collectTransitiveDeps([]string{absolutePath})
```

In `cmd/run.go`, find:

```go
	repoRoot, filterPaths := collectTransitiveDeps(workDir)
```

Replace with:

```go
	repoRoot, filterPaths := collectTransitiveDeps([]string{workDir})
```

In `cmd/groups.go`, find:

```go
	repoRoot, filterPaths := collectTransitiveDeps(workDir)
```

Replace with:

```go
	repoRoot, filterPaths := collectTransitiveDeps([]string{workDir})
```

- [ ] **Step 5: Update displayResults to show multiple paths**

In `cmd/root.go`, replace `displayResults`:

```go
// displayResults shows the final selection to the user.
func displayResults(model tui.Model) {
	fmt.Println()

	if !model.IsConfirmed() {
		fmt.Println("⚠️  Selection cancelled")
		return
	}

	fmt.Println("═══════════════════════════════════════")
	fmt.Println("  ✅ Selection confirmed")
	fmt.Println("═══════════════════════════════════════")
	fmt.Printf("Command: %s\n", model.GetSelectedCommand())

	if model.HasSelectedPaths() {
		paths := model.GetSelectedStackPaths()
		fmt.Printf("Stacks (%d):\n", len(paths))
		for _, p := range paths {
			fmt.Printf("  • %s\n", p)
		}
	} else {
		fmt.Printf("Stack Path: %s\n", model.GetSelectedStackPath())
	}
	fmt.Println("═══════════════════════════════════════")
	fmt.Println()
}
```

- [ ] **Step 6: Run tests**

```bash
cd /Users/isra/Repos/israoo/terrax && go test ./cmd/... -run "TestCollectTransitiveDeps_MultiplePaths|TestCollectTransitiveDeps_Deduplicates|TestCollectTransitiveDeps_Empty" -v
```

Expected: all PASS

- [ ] **Step 7: Run full cmd tests**

```bash
cd /Users/isra/Repos/israoo/terrax && go test ./cmd/... -v
```

Expected: all PASS

- [ ] **Step 8: Run task check (full CI)**

```bash
cd /Users/isra/Repos/israoo/terrax && task check
```

Expected: all PASS (fmt + vet + lint + test)

- [ ] **Step 9: Commit**

```bash
git add cmd/root.go cmd/execute.go cmd/run.go cmd/groups.go cmd/root_test.go
git commit -m "feat: extend collectTransitiveDeps to accept multiple paths for multi-stack execution"
```
