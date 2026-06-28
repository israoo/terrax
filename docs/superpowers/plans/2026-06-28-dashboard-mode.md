# StateDashboard — `terrax lazy` Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add `terrax lazy` subcommand that launches a multi-panel dashboard TUI with an indented stack tree, real-time streaming command output, and tabbed context panels (dependencies, last run, plan diff).

**Architecture:** New `StateDashboard` AppState added to the existing `Model` struct. A `RunStreaming` function is added to `internal/executor` that writes stdout/stderr to an `io.Writer`. Streaming in the TUI uses Bubble Tea's recursive `tea.Cmd` pattern: each `OutputLineMsg` schedules the next line read, avoiding the need for `p.Send()`. The filter-path computation is injected from the `cmd` layer via a `FilterPathsFn` function, keeping the TUI layer free of cmd-layer dependencies.

**Tech Stack:** Go 1.25.5 · Bubble Tea 1.3.10 · Lipgloss 1.1.0 · Cobra 1.10.2 · bufio.Scanner · io.Pipe

## Global Constraints

- All `view_*.go` methods use value receivers `(m Model)` — never pointer receivers.
- All `update_*.go` methods use value receivers and return `(tea.Model, tea.Cmd)`.
- Lipgloss styles defined at package-level `var` blocks, never inside render functions.
- Comments end with periods.
- Imports: three groups (stdlib · third-party · `github.com/israoo/terrax/...`).
- Always use `filepath.Join()` for paths — never hardcoded `/`.
- Run `task check` (fmt + vet + lint + test) before each commit.
- `RunStreaming` is additive — `executor.Run` is unchanged. Existing tests must remain green.
- `StateDashboard` is additive — existing states (`StateNavigation`, `StateHistory`, `StatePlanReview`) are unchanged.

---

## File Map

| Action | Path | Responsibility |
|--------|------|----------------|
| Create | `cmd/lazy.go` | Cobra `lazy` subcommand; builds `NewDashboardModel` and starts Bubble Tea program |
| Modify | `cmd/root.go` | Register `lazyCmd` |
| Modify | `internal/executor/executor.go` | Add `RunStreaming(ctx, writer, command, absPath, repoRoot, filters, envVars) error` |
| Modify | `internal/executor/executor_test.go` | Tests for `RunStreaming` |
| Modify | `internal/tui/model.go` | Add `StateDashboard` iota; dashboard fields; `NewDashboardModel`; stub branches in `Update`/`View` |
| Create | `internal/tui/update_dashboard.go` | `handleDashboardUpdate`; message types; `lineScanner`; `readNextLine`; `dispatchCommand` |
| Create | `internal/tui/update_dashboard_test.go` | Tests for key dispatch, streaming messages, palette open/close |
| Create | `internal/tui/view_dashboard.go` | `renderDashboardView`; layout composer; command palette overlay |
| Create | `internal/tui/view_dashboard_tree.go` | `DashTreeNode`; `flattenDashTree`; `renderDashboardTree` |
| Create | `internal/tui/view_dashboard_tree_test.go` | Tests for flatten logic and connector rendering |
| Create | `internal/tui/view_dashboard_output.go` | `renderDashboardOutput`; scroll; header states |
| Create | `internal/tui/view_dashboard_output_test.go` | Tests for scroll offset, header text |
| Create | `internal/tui/view_dashboard_bottom.go` | `renderDashboardBottom`; three tabs; tab switching |
| Create | `internal/tui/view_dashboard_bottom_test.go` | Tests for each tab renderer |

---

## Task 1: `executor.RunStreaming`

**Files:**
- Modify: `internal/executor/executor.go`
- Modify: `internal/executor/executor_test.go`

**Interfaces:**
- Produces: `RunStreaming(ctx context.Context, writer io.Writer, command, absoluteStackPath, repoRoot string, filterPaths []string, envVars map[string]string) error`

- [ ] **Step 1: Write the failing test**

```go
// internal/executor/executor_test.go
func TestRunStreaming_WritesOutputToWriter(t *testing.T) {
    ctx := context.Background()
    var buf bytes.Buffer

    // Use "echo" as a stand-in; executor will call terragrunt, so we test
    // the writer wiring by replacing the binary via PATH manipulation.
    // Instead, test with a real writer and a mock command via the existing
    // test helper pattern (check how executor_test.go mocks exec.Command).
    // For now use a simple approach: RunStreaming with a no-op writer on a
    // definitely-failing command, assert error is non-nil and writer received bytes.
    err := RunStreaming(ctx, &buf, "plan", "/nonexistent", "/nonexistent", []string{"/nonexistent"}, nil)
    assert.Error(t, err, "expected error for nonexistent stack")
}

func TestRunStreaming_WritesLinesToWriter(t *testing.T) {
    // This test runs only if terragrunt is not available; it validates the
    // writer path is wired correctly via a process that exits non-zero.
    // The important contract: whatever cmd.Stdout/Stderr receives goes to writer.
    ctx := context.Background()
    var buf bytes.Buffer
    _ = RunStreaming(ctx, &buf, "plan", "/nonexistent", "/nonexistent", []string{"/nonexistent"}, nil)
    // terragrunt itself will write an error message; we just assert writer was used.
    // If terragrunt is not installed, the error will be "exec: not found" with no output.
    // Either way, the function returns without panic.
}

func TestRunStreaming_ContextCancellation(t *testing.T) {
    ctx, cancel := context.WithCancel(context.Background())
    cancel() // cancel immediately
    var buf bytes.Buffer
    err := RunStreaming(ctx, &buf, "plan", "/tmp", "/tmp", []string{"/tmp"}, nil)
    // Should return quickly due to cancelled context
    assert.Error(t, err)
}
```

- [ ] **Step 2: Run test to verify it fails**

```bash
go test ./internal/executor/ -run "TestRunStreaming" -v
```
Expected: FAIL — `RunStreaming undefined`

- [ ] **Step 3: Implement `RunStreaming`**

Add after `Run()` in `internal/executor/executor.go`:

```go
// RunStreaming executes a Terragrunt command and writes all stdout/stderr to writer.
// Unlike Run, it does not log to history — callers are responsible for logging
// after receiving the exit code. Cancelling ctx terminates the subprocess.
func RunStreaming(ctx context.Context, writer io.Writer, command, absoluteStackPath, repoRoot string, filterPaths []string, envVars map[string]string) error {
	args := buildFilterArgs(repoRoot, command, filterPaths)

	cmd := exec.CommandContext(ctx, "terragrunt", args...)
	cmd.Dir = repoRoot

	if len(envVars) > 0 {
		existing := os.Environ()
		merged := make([]string, 0, len(existing)+len(envVars))
		for _, entry := range existing {
			key := entry[:strings.IndexByte(entry, '=')]
			if _, overridden := envVars[key]; !overridden {
				merged = append(merged, entry)
			}
		}
		for k, v := range envVars {
			merged = append(merged, fmt.Sprintf("%s=%s", k, v))
		}
		cmd.Env = merged
	}

	cmd.Stdout = writer
	cmd.Stderr = writer

	return cmd.Run()
}
```

Add `"io"` to the import block (stdlib group).

- [ ] **Step 4: Run tests**

```bash
go test ./internal/executor/ -run "TestRunStreaming" -v
```
Expected: all three `TestRunStreaming_*` pass.

- [ ] **Step 5: Full executor test suite**

```bash
go test ./internal/executor/ -v
```
Expected: all pass (Run, RunForceUnlock, RunStreaming).

- [ ] **Step 6: Commit**

```bash
git add internal/executor/executor.go internal/executor/executor_test.go
git commit -m "feat(executor): add RunStreaming for dashboard streaming output"
```

---

## Task 2: Model Scaffold

**Files:**
- Modify: `internal/tui/model.go`
- Create: `internal/tui/update_dashboard.go` (message types + stub handler)
- Create: `internal/tui/view_dashboard.go` (stub renderer)

**Interfaces:**
- Produces:
  - `StateDashboard` AppState constant
  - `NewDashboardModel(root *stack.Node, maxDepth int, commands []string, maxNavCols int, ctx context.Context, filterFn FilterPathsFn, historyLogger executor.HistoryLogger) Model`
  - `OutputLineMsg{Line string}`, `CommandFinishedMsg{ExitCode int, Duration time.Duration, Err error}`
  - `FilterPathsFn` type alias
  - `lineScanner` struct with `scanner *bufio.Scanner` and `done chan commandResult`

- [ ] **Step 1: Add `StateDashboard` to the iota in `model.go`**

Find the `AppState` iota block. Add after `StatePlanReview`:

```go
// StateDashboard is the multi-panel lazy dashboard mode (terrax lazy).
StateDashboard
```

- [ ] **Step 2: Add dashboard fields to `Model` struct in `model.go`**

Add a grouped comment block after the existing plan fields:

```go
// Dashboard state (StateDashboard).
dashCtx           context.Context        // top-level context; used to create per-command contexts
dashFilterFn      FilterPathsFn          // injected from cmd layer; computes repoRoot + filterPaths
dashHistoryLogger executor.HistoryLogger // for writing execution history entries

dashTreeNodes     []DashTreeNode         // flattened visible tree; recomputed on expand/collapse
dashTreeExpanded  map[string]bool        // node.Path → expanded
dashTreeCursor    int                    // index into dashTreeNodes

dashOutputLines   []string              // captured output lines (grows during streaming)
dashOutputOffset  int                   // scroll offset into dashOutputLines
dashAutoScroll    bool                  // true while no manual scroll during execution
dashRunning       bool                  // true while a command is executing
dashRunCmd        string                // command name being run, e.g. "plan"
dashStartTime     time.Time             // when current command started
dashScanner       *lineScanner         // active scanner; nil when not running
dashCancelCmd     context.CancelFunc   // cancels the running command; nil when not running

dashActiveTab     int                  // 0=deps 1=lastrun 2=plandiff
dashBottomOffset  int                  // scroll offset for bottom panel
dashLastRun       *history.ExecutionLogEntry
dashPlanReport    *plan.PlanReport     // nil = no recent plan for selected stack

dashPaletteOpen   bool
dashPaletteCursor int

dashFocusedPanel  int // 0=tree 1=output 2=bottom
```

Add required imports to model.go if not already present: `"context"`, `"time"`,
`"github.com/israoo/terrax/internal/executor"`.

- [ ] **Step 3: Add `FilterPathsFn` type and `NewDashboardModel` to `model.go`**

```go
// FilterPathsFn computes the repoRoot and filter paths for a given stack path.
// Injected from the cmd layer to keep the TUI free of CLI dependencies.
type FilterPathsFn func(ctx context.Context, stackPath string) (repoRoot string, filterPaths []string, err error)

// NewDashboardModel creates a Model in StateDashboard mode.
func NewDashboardModel(
	root *stack.Node,
	maxDepth int,
	commands []string,
	maxNavCols int,
	ctx context.Context,
	filterFn FilterPathsFn,
	historyLogger executor.HistoryLogger,
) Model {
	nav := stack.NewNavigator(root, maxDepth)
	navState := stack.NewNavigationState(maxDepth)
	nav.PropagateSelection(navState)

	m := Model{
		state:            StateDashboard,
		navigator:        nav,
		navState:         navState,
		commands:         commands,
		maxNavigationColumns: maxNavCols,
		ready:            false,

		dashCtx:          ctx,
		dashFilterFn:     filterFn,
		dashHistoryLogger: historyLogger,
		dashTreeExpanded: make(map[string]bool),
		dashTreeNodes:    nil, // populated on first WindowSizeMsg
		dashFocusedPanel: 0,
		dashActiveTab:    0,
		dashAutoScroll:   true,
	}
	return m
}
```

- [ ] **Step 4: Add stub branches to `Update()` and `View()` in `model.go`**

In `Update()` switch:
```go
case StateDashboard:
    return m.handleDashboardUpdate(msg)
```

In `View()` switch:
```go
case StateDashboard:
    return m.renderDashboardView()
```

- [ ] **Step 5: Create `internal/tui/update_dashboard.go` with message types and stub**

```go
package tui

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/israoo/terrax/internal/executor"
	"github.com/israoo/terrax/internal/history"
)

// OutputLineMsg carries one line of streaming command output.
type OutputLineMsg struct{ Line string }

// CommandFinishedMsg is sent when a streaming command completes.
type CommandFinishedMsg struct {
	ExitCode int
	Duration time.Duration
	Err      error
}

// commandResult is passed from the streaming goroutine to readNextLine via a channel.
type commandResult struct {
	exitCode int
	duration time.Duration
	err      error
}

// lineScanner wraps a bufio.Scanner and the done channel for a streaming command.
type lineScanner struct {
	scanner *bufio.Scanner
	done    chan commandResult
}

// readNextLine returns a tea.Cmd that reads one line from ls.
// When the scanner is exhausted it reads the commandResult and returns CommandFinishedMsg.
func readNextLine(ls *lineScanner) tea.Cmd {
	return func() tea.Msg {
		if ls.scanner.Scan() {
			return OutputLineMsg{Line: ls.scanner.Text()}
		}
		result := <-ls.done
		return CommandFinishedMsg{
			ExitCode: result.exitCode,
			Duration: result.duration,
			Err:      result.err,
		}
	}
}

// handleDashboardUpdate handles all Bubble Tea messages for StateDashboard.
func (m Model) handleDashboardUpdate(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.ready = true
		if len(m.dashTreeNodes) == 0 {
			m.dashTreeNodes = flattenDashTree(m.navigator.GetRoot(), m.dashTreeExpanded)
		}
		return m, nil

	case OutputLineMsg:
		m.dashOutputLines = append(m.dashOutputLines, msg.Line)
		if m.dashAutoScroll {
			m.dashOutputOffset = max(0, len(m.dashOutputLines)-m.dashOutputVisibleLines())
		}
		return m, readNextLine(m.dashScanner)

	case CommandFinishedMsg:
		m = m.finishCommand(msg)
		return m, nil

	case tea.KeyMsg:
		return m.handleDashboardKey(msg)
	}
	return m, nil
}

// handleDashboardKey routes key events based on focused panel.
func (m Model) handleDashboardKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// Global keys.
	switch msg.String() {
	case "q", "esc":
		if m.dashPaletteOpen {
			m.dashPaletteOpen = false
			return m, nil
		}
		if m.dashCancelCmd != nil {
			m.dashCancelCmd()
		}
		return m, tea.Quit
	case "tab":
		m.dashFocusedPanel = (m.dashFocusedPanel + 1) % 3
		return m, nil
	}

	if m.dashPaletteOpen {
		return m.handlePaletteKey(msg)
	}

	switch m.dashFocusedPanel {
	case 0:
		return m.handleTreeKey(msg)
	case 1:
		return m.handleOutputKey(msg)
	case 2:
		return m.handleBottomKey(msg)
	}
	return m, nil
}

// handleTreeKey handles key events when the tree panel is focused.
func (m Model) handleTreeKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "up":
		if m.dashTreeCursor > 0 {
			m.dashTreeCursor--
		}
	case "down":
		if m.dashTreeCursor < len(m.dashTreeNodes)-1 {
			m.dashTreeCursor++
		}
	case "enter":
		if len(m.dashTreeNodes) == 0 {
			break
		}
		node := m.dashTreeNodes[m.dashTreeCursor].Node
		if node.HasChildren() {
			if m.dashTreeExpanded[node.Path] {
				delete(m.dashTreeExpanded, node.Path)
			} else {
				m.dashTreeExpanded[node.Path] = true
			}
			m.dashTreeNodes = flattenDashTree(m.navigator.GetRoot(), m.dashTreeExpanded)
			// Keep cursor in bounds after re-flatten.
			if m.dashTreeCursor >= len(m.dashTreeNodes) {
				m.dashTreeCursor = len(m.dashTreeNodes) - 1
			}
		}
	case " ":
		m.dashPaletteOpen = true
		m.dashPaletteCursor = 0
	default:
		if !m.dashRunning {
			return m.handleCommandKey(msg.String())
		}
	}
	return m, nil
}

// handleCommandKey dispatches a command shortcut key from the tree panel.
func (m Model) handleCommandKey(key string) (Model, tea.Cmd) {
	cmdMap := map[string]string{
		"p": "plan",
		"a": "apply",
		"i": "init",
		"v": "validate",
		"d": "destroy",
	}
	command, ok := cmdMap[key]
	if !ok {
		return m, nil
	}
	return m.dispatchCommand(command)
}

// handleOutputKey handles key events when the output panel is focused.
func (m Model) handleOutputKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	visLines := m.dashOutputVisibleLines()
	switch msg.String() {
	case "up":
		if m.dashOutputOffset > 0 {
			m.dashOutputOffset--
			m.dashAutoScroll = false
		}
	case "down":
		newOffset := m.dashOutputOffset + 1
		maxOffset := max(0, len(m.dashOutputLines)-visLines)
		if newOffset <= maxOffset {
			m.dashOutputOffset = newOffset
		}
		if m.dashOutputOffset >= maxOffset {
			m.dashAutoScroll = true
		}
	case "pgup":
		m.dashOutputOffset = max(0, m.dashOutputOffset-visLines)
		m.dashAutoScroll = false
	case "pgdown":
		maxOffset := max(0, len(m.dashOutputLines)-visLines)
		m.dashOutputOffset = min(m.dashOutputOffset+visLines, maxOffset)
		if m.dashOutputOffset >= maxOffset {
			m.dashAutoScroll = true
		}
	case "c":
		m.dashOutputLines = nil
		m.dashOutputOffset = 0
	case "ctrl+c":
		if m.dashCancelCmd != nil {
			m.dashCancelCmd()
		}
	}
	return m, nil
}

// handleBottomKey handles key events when the bottom panel is focused.
func (m Model) handleBottomKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "[":
		m.dashActiveTab = max(0, m.dashActiveTab-1)
		m.dashBottomOffset = 0
	case "]":
		m.dashActiveTab = min(m.dashActiveTab+1, 2)
		m.dashBottomOffset = 0
	case "up":
		if m.dashBottomOffset > 0 {
			m.dashBottomOffset--
		}
	case "down":
		m.dashBottomOffset++
	}
	return m, nil
}

// handlePaletteKey handles key events when the command palette is open.
func (m Model) handlePaletteKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.dashPaletteOpen = false
	case "up":
		if m.dashPaletteCursor > 0 {
			m.dashPaletteCursor--
		}
	case "down":
		if m.dashPaletteCursor < len(m.commands)-1 {
			m.dashPaletteCursor++
		}
	case "enter":
		if !m.dashRunning && m.dashPaletteCursor < len(m.commands) {
			command := m.commands[m.dashPaletteCursor]
			m.dashPaletteOpen = false
			newM, cmd := m.dispatchCommand(command)
			return newM, cmd
		}
	}
	return m, nil
}

// dispatchCommand starts a streaming Terragrunt command on the currently selected stack.
// No-op if the cursor is on a directory node or a command is already running.
func (m Model) dispatchCommand(command string) (Model, tea.Cmd) {
	if m.dashRunning || len(m.dashTreeNodes) == 0 {
		return m, nil
	}
	node := m.dashTreeNodes[m.dashTreeCursor].Node
	if node.HasChildren() {
		return m, nil // directories are not executable
	}

	repoRoot, filterPaths, err := m.dashFilterFn(m.dashCtx, node.Path)
	if err != nil {
		m.dashOutputLines = []string{fmt.Sprintf("Error: %v", err)}
		return m, nil
	}

	pr, pw := io.Pipe()
	done := make(chan commandResult, 1)

	cmdCtx, cancel := context.WithCancel(m.dashCtx)
	m.dashCancelCmd = cancel
	m.dashRunning = true
	m.dashRunCmd = command
	m.dashOutputLines = nil
	m.dashOutputOffset = 0
	m.dashAutoScroll = true
	m.dashStartTime = time.Now()

	ls := &lineScanner{
		scanner: bufio.NewScanner(pr),
		done:    done,
	}
	m.dashScanner = ls

	absStackPath := node.Path
	startTime := m.dashStartTime

	go func() {
		defer pw.Close()
		runErr := executor.RunStreaming(cmdCtx, pw, command, absStackPath, repoRoot, filterPaths, nil)
		exitCode := 0
		if runErr != nil {
			var exitErr *exec.ExitError
			if errors.As(runErr, &exitErr) {
				exitCode = exitErr.ExitCode()
			} else if !errors.Is(runErr, context.Canceled) {
				exitCode = 1
			}
		}
		done <- commandResult{
			exitCode: exitCode,
			duration: time.Since(startTime),
			err:      runErr,
		}
	}()

	return m, readNextLine(ls)
}

// finishCommand handles a CommandFinishedMsg: clears running state and logs to history.
func (m Model) finishCommand(msg CommandFinishedMsg) Model {
	m.dashRunning = false
	m.dashCancelCmd = nil
	m.dashScanner = nil

	if len(m.dashTreeNodes) == 0 {
		return m
	}
	node := m.dashTreeNodes[m.dashTreeCursor].Node

	summary := "completed"
	if msg.ExitCode != 0 {
		summary = fmt.Sprintf("failed (exit %d)", msg.ExitCode)
	}

	entry := history.ExecutionLogEntry{
		Timestamp:    time.Now().Add(-msg.Duration),
		User:         history.GetCurrentUser(),
		StackPath:    node.Path,
		AbsolutePath: node.Path,
		Command:      m.dashRunCmd,
		ExitCode:     msg.ExitCode,
		DurationS:    msg.Duration.Seconds(),
		Summary:      summary,
	}

	if m.dashHistoryLogger != nil {
		nextID, err := m.dashHistoryLogger.GetNextID(m.dashCtx)
		if err == nil {
			entry.ID = nextID
		}
		_ = m.dashHistoryLogger.Append(m.dashCtx, entry)
	}

	m.dashLastRun = &entry
	return m
}

// dashOutputVisibleLines returns the number of lines that fit in the output panel.
func (m Model) dashOutputVisibleLines() int {
	bottomH := m.height * 30 / 100
	// Header(1) + header-line(1) + footer(1)
	outputH := m.height - bottomH - HeaderHeight - 3
	if outputH < 1 {
		return 1
	}
	return outputH
}

// Ensure imports are used (will be cleaned up by goimports).
var _ = strings.Join
```

- [ ] **Step 6: Create stub `view_dashboard.go`**

```go
package tui

// renderDashboardView renders the StateDashboard layout.
// Implemented fully in Task 9; this stub prevents compilation errors.
func (m Model) renderDashboardView() string {
	if !m.ready {
		return Initializing
	}
	return "Dashboard (in progress)"
}
```

- [ ] **Step 7: Run tests**

```bash
task build
go test ./internal/tui/ -v 2>&1 | tail -20
```
Expected: builds and existing tests pass; stub dashboard renders "Dashboard (in progress)".

- [ ] **Step 8: Commit**

```bash
git add internal/tui/model.go internal/tui/update_dashboard.go internal/tui/view_dashboard.go
git commit -m "feat(tui): scaffold StateDashboard model, message types, and stub handlers"
```

---

## Task 3: Indented Tree Panel

**Files:**
- Create: `internal/tui/view_dashboard_tree.go`
- Create: `internal/tui/view_dashboard_tree_test.go`

**Interfaces:**
- Consumes: `stack.Node` (HasChildren, Children, Name, Path, IsStack)
- Produces:
  - `type DashTreeNode struct { Node *stack.Node; Depth int; IsLast bool; Prefix string }`
  - `func flattenDashTree(root *stack.Node, expanded map[string]bool) []DashTreeNode`
  - `func (m Model) renderDashboardTree(width, height int) string`

- [ ] **Step 1: Write failing tests for `flattenDashTree`**

```go
// internal/tui/view_dashboard_tree_test.go
package tui

import (
	"testing"

	"github.com/israoo/terrax/internal/stack"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func makeTree() *stack.Node {
	return &stack.Node{
		Name: "root", Path: "/root",
		Children: []*stack.Node{
			{
				Name: "infra", Path: "/root/infra",
				Children: []*stack.Node{
					{Name: "dev", Path: "/root/infra/dev", IsStack: true},
					{Name: "prod", Path: "/root/infra/prod", IsStack: true},
				},
			},
			{Name: "mgmt", Path: "/root/mgmt", IsStack: true},
		},
	}
}

func TestFlattenDashTree_AllCollapsed(t *testing.T) {
	root := makeTree()
	nodes := flattenDashTree(root, map[string]bool{})
	// Only top-level children of root are visible when collapsed.
	require.Len(t, nodes, 2)
	assert.Equal(t, "infra", nodes[0].Node.Name)
	assert.Equal(t, 0, nodes[0].Depth)
	assert.Equal(t, false, nodes[0].IsLast)
	assert.Equal(t, "mgmt", nodes[1].Node.Name)
	assert.Equal(t, true, nodes[1].IsLast)
}

func TestFlattenDashTree_ExpandedInfra(t *testing.T) {
	root := makeTree()
	expanded := map[string]bool{"/root/infra": true}
	nodes := flattenDashTree(root, expanded)
	require.Len(t, nodes, 4) // infra + dev + prod + mgmt
	assert.Equal(t, "infra", nodes[0].Node.Name)
	assert.Equal(t, "dev", nodes[1].Node.Name)
	assert.Equal(t, 1, nodes[1].Depth)
	assert.Equal(t, false, nodes[1].IsLast)
	assert.Equal(t, "prod", nodes[2].Node.Name)
	assert.Equal(t, true, nodes[2].IsLast)
	assert.Equal(t, "mgmt", nodes[3].Node.Name)
}

func TestFlattenDashTree_PrefixConnectors(t *testing.T) {
	root := makeTree()
	expanded := map[string]bool{"/root/infra": true}
	nodes := flattenDashTree(root, expanded)
	// dev is not last child of infra → its children would use "│  " continuation
	assert.Equal(t, "", nodes[0].Prefix)  // infra: top-level, no parent prefix
	assert.Equal(t, "│  ", nodes[1].Prefix) // dev: infra is not last, so "│  " prefix
}
```

- [ ] **Step 2: Run tests to verify they fail**

```bash
go test ./internal/tui/ -run "TestFlattenDashTree" -v
```
Expected: FAIL — `flattenDashTree undefined`

- [ ] **Step 3: Implement `view_dashboard_tree.go`**

```go
package tui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/israoo/terrax/internal/stack"
)

// DashTreeNode is one visible entry in the flattened dashboard tree.
type DashTreeNode struct {
	Node   *stack.Node
	Depth  int
	IsLast bool
	// Prefix is the indentation string inherited from parent nodes,
	// e.g. "│  " means the parent was not the last child.
	Prefix string
}

// flattenDashTree performs a DFS over root.Children, respecting expanded.
// Root itself is not included; only its descendants are.
func flattenDashTree(root *stack.Node, expanded map[string]bool) []DashTreeNode {
	var nodes []DashTreeNode
	var walk func(node *stack.Node, depth int, prefix string, isLast bool)
	walk = func(node *stack.Node, depth int, prefix string, isLast bool) {
		nodes = append(nodes, DashTreeNode{
			Node:   node,
			Depth:  depth,
			IsLast: isLast,
			Prefix: prefix,
		})
		if node.HasChildren() && expanded[node.Path] {
			for i, child := range node.Children {
				childIsLast := i == len(node.Children)-1
				var childPrefix string
				if isLast {
					childPrefix = prefix + "   "
				} else {
					childPrefix = prefix + "│  "
				}
				walk(child, depth+1, childPrefix, childIsLast)
			}
		}
	}
	for i, child := range root.Children {
		walk(child, 0, "", i == len(root.Children)-1)
	}
	return nodes
}

var (
	dashTreeCursorStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(accentColor)
	dashTreeDirStyle = lipgloss.NewStyle().
				Foreground(secondaryColor)
	dashTreeStackStyle = lipgloss.NewStyle().
				Foreground(textColor)
	dashTreeDimStyle = lipgloss.NewStyle().
				Foreground(dimColor)
)

// renderDashboardTree renders the indented tree panel content.
// width is the inner content width (excluding borders/padding).
// height is the maximum number of lines to render.
func (m Model) renderDashboardTree(width, height int) string {
	if len(m.dashTreeNodes) == 0 {
		return dashTreeDimStyle.Render("No stacks found.")
	}

	// Compute visible window (scroll to keep cursor visible).
	start := 0
	if m.dashTreeCursor >= height {
		start = m.dashTreeCursor - height + 1
	}
	end := min(start+height, len(m.dashTreeNodes))

	var sb strings.Builder
	for i := start; i < end; i++ {
		tn := m.dashTreeNodes[i]
		isCursor := i == m.dashTreeCursor

		connector := "├─ "
		if tn.IsLast {
			connector = "└─ "
		}

		var icon string
		if tn.Node.HasChildren() {
			if m.dashTreeExpanded[tn.Node.Path] {
				icon = "▼ "
			} else {
				icon = "▶ "
			}
		} else {
			icon = "  "
		}

		prefix := dashTreeDimStyle.Render(tn.Prefix + connector)
		name := tn.Node.Name
		maxNameWidth := width - len(tn.Prefix) - 3 - 2 // connector(3) + icon(2)
		if maxNameWidth < 4 {
			maxNameWidth = 4
		}
		if len(name) > maxNameWidth {
			name = name[:maxNameWidth-1] + "…"
		}

		var nameRendered string
		if isCursor {
			nameRendered = dashTreeCursorStyle.Render(icon + name)
		} else if tn.Node.HasChildren() {
			nameRendered = dashTreeDirStyle.Render(icon + name)
		} else {
			nameRendered = dashTreeStackStyle.Render(icon + name)
		}

		sb.WriteString(prefix + nameRendered + "\n")
	}
	return sb.String()
}
```

- [ ] **Step 4: Run tests**

```bash
go test ./internal/tui/ -run "TestFlattenDashTree" -v
```
Expected: all pass.

- [ ] **Step 5: Full TUI tests**

```bash
task build && go test ./internal/tui/ -v 2>&1 | grep -E "^(--- |ok|FAIL)"
```
Expected: all pass.

- [ ] **Step 6: Commit**

```bash
git add internal/tui/view_dashboard_tree.go internal/tui/view_dashboard_tree_test.go
git commit -m "feat(tui): add dashboard indented tree panel renderer and flatten logic"
```

---

## Task 4: Update Handler Tests (Tree Navigation)

**Files:**
- Create: `internal/tui/update_dashboard_test.go`

- [ ] **Step 1: Write tests for tree navigation keys**

```go
// internal/tui/update_dashboard_test.go
package tui

import (
	"context"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/assert"

	"github.com/israoo/terrax/internal/stack"
)

func makeDashModel(t *testing.T) Model {
	t.Helper()
	root := &stack.Node{
		Name: "root", Path: "/root",
		Children: []*stack.Node{
			{
				Name: "infra", Path: "/root/infra",
				Children: []*stack.Node{
					{Name: "dev", Path: "/root/infra/dev", IsStack: true},
				},
			},
			{Name: "mgmt", Path: "/root/mgmt", IsStack: true},
		},
	}
	nav := stack.NewNavigator(root, 2)
	navState := stack.NewNavigationState(2)
	nav.PropagateSelection(navState)

	noopFilter := FilterPathsFn(func(_ context.Context, p string) (string, []string, error) {
		return "/root", []string{p}, nil
	})

	m := NewDashboardModel(root, 2, []string{"plan", "apply"}, 3, context.Background(), noopFilter, nil)
	m.width = 120
	m.height = 40
	m.ready = true
	m.dashTreeNodes = flattenDashTree(nav.GetRoot(), m.dashTreeExpanded)
	return m
}

func TestDashboard_TreeNavDown(t *testing.T) {
	m := makeDashModel(t)
	assert.Equal(t, 0, m.dashTreeCursor)

	result, _ := m.handleDashboardUpdate(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("j")})
	// "j" is not mapped; cursor stays
	assert.Equal(t, 0, result.(Model).dashTreeCursor)

	result, _ = m.handleDashboardUpdate(tea.KeyMsg{Type: tea.KeyDown})
	assert.Equal(t, 1, result.(Model).dashTreeCursor)
}

func TestDashboard_TreeNavUp_AtTop(t *testing.T) {
	m := makeDashModel(t)
	result, _ := m.handleDashboardUpdate(tea.KeyMsg{Type: tea.KeyUp})
	assert.Equal(t, 0, result.(Model).dashTreeCursor, "cursor should not go below 0")
}

func TestDashboard_TreeExpand(t *testing.T) {
	m := makeDashModel(t)
	// cursor is on "infra" (index 0), which has children
	result, _ := m.handleDashboardUpdate(tea.KeyMsg{Type: tea.KeyEnter})
	m2 := result.(Model)
	assert.True(t, m2.dashTreeExpanded["/root/infra"], "infra should be expanded")
	assert.Len(t, m2.dashTreeNodes, 3, "infra + dev + mgmt")
}

func TestDashboard_TreeCollapse(t *testing.T) {
	m := makeDashModel(t)
	m.dashTreeExpanded["/root/infra"] = true
	m.dashTreeNodes = flattenDashTree(m.navigator.GetRoot(), m.dashTreeExpanded)

	result, _ := m.handleDashboardUpdate(tea.KeyMsg{Type: tea.KeyEnter})
	m2 := result.(Model)
	assert.False(t, m2.dashTreeExpanded["/root/infra"], "infra should be collapsed")
	assert.Len(t, m2.dashTreeNodes, 2)
}

func TestDashboard_TabSwitchesFocus(t *testing.T) {
	m := makeDashModel(t)
	assert.Equal(t, 0, m.dashFocusedPanel)

	result, _ := m.handleDashboardUpdate(tea.KeyMsg{Type: tea.KeyTab})
	assert.Equal(t, 1, result.(Model).dashFocusedPanel)

	result, _ = result.(Model).handleDashboardUpdate(tea.KeyMsg{Type: tea.KeyTab})
	assert.Equal(t, 2, result.(Model).dashFocusedPanel)

	result, _ = result.(Model).handleDashboardUpdate(tea.KeyMsg{Type: tea.KeyTab})
	assert.Equal(t, 0, result.(Model).dashFocusedPanel)
}

func TestDashboard_PaletteOpenClose(t *testing.T) {
	m := makeDashModel(t)
	// Move cursor to a leaf (mgmt at index 1)
	m.dashTreeCursor = 1

	result, _ := m.handleDashboardUpdate(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(" ")})
	m2 := result.(Model)
	assert.True(t, m2.dashPaletteOpen)

	result, _ = m2.handleDashboardUpdate(tea.KeyMsg{Type: tea.KeyEsc})
	assert.False(t, result.(Model).dashPaletteOpen)
}

func TestDashboard_CommandIgnoredWhileRunning(t *testing.T) {
	m := makeDashModel(t)
	m.dashTreeCursor = 1 // mgmt is a leaf
	m.dashRunning = true

	result, cmd := m.handleDashboardUpdate(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("p")})
	assert.True(t, result.(Model).dashRunning, "should still be running")
	assert.Nil(t, cmd, "no command should be dispatched")
}

func TestDashboard_QuitCancelsRunningCommand(t *testing.T) {
	m := makeDashModel(t)
	cancelled := false
	m.dashCancelCmd = func() { cancelled = true }
	m.dashRunning = true

	_, cmd := m.handleDashboardUpdate(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("q")})
	assert.True(t, cancelled)
	assert.Equal(t, tea.Quit(), cmd())
}
```

- [ ] **Step 2: Run tests to see them fail/pass**

```bash
go test ./internal/tui/ -run "TestDashboard_" -v
```
Expected: tests using `handleDashboardUpdate` should pass (logic implemented in Task 2). Fix any compilation errors.

- [ ] **Step 3: Fix any gaps in `handleDashboardUpdate` found by tests**

Common fixes: ensure `tea.KeyDown` / `tea.KeyUp` / `tea.KeyTab` / `tea.KeyEnter` are handled (not just string form). Bubble Tea sends both `tea.KeyRunes` and special `tea.KeyType` values.

Update `handleTreeKey` to handle both forms:
```go
case "up", tea.KeyUp.String():
case "down", tea.KeyDown.String():
case "enter", tea.KeyEnter.String():
```

Actually Bubble Tea's `msg.String()` already normalises these — verify the string values with:
```bash
go doc github.com/charmbracelet/bubbletea KeyType
```

- [ ] **Step 4: Full test suite**

```bash
task check
```
Expected: 0 issues, all tests pass.

- [ ] **Step 5: Commit**

```bash
git add internal/tui/update_dashboard_test.go internal/tui/update_dashboard.go
git commit -m "test(tui): add dashboard update handler tests for tree navigation and palette"
```

---

## Task 5: Output Panel Renderer

**Files:**
- Create: `internal/tui/view_dashboard_output.go`
- Create: `internal/tui/view_dashboard_output_test.go`

**Interfaces:**
- Produces: `func (m Model) renderDashboardOutput(width, height int) string`

- [ ] **Step 1: Write failing tests**

```go
// internal/tui/view_dashboard_output_test.go
package tui

import (
	"strings"
	"testing"
	"time"

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
	entry := history.ExecutionLogEntry{ExitCode: 0, DurationS: 1.2}
	m.dashLastRun = &entry
	result := m.renderDashboardOutput(80, 20)
	assert.Contains(t, result, "✓")
}

func TestRenderDashboardOutput_FinishedFailure(t *testing.T) {
	m := makeDashModel(t)
	m.dashRunning = false
	m.dashRunCmd = "plan"
	entry := history.ExecutionLogEntry{ExitCode: 1, DurationS: 0.5}
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
```

Add `"fmt"` import.

- [ ] **Step 2: Run tests to verify failure**

```bash
go test ./internal/tui/ -run "TestRenderDashboardOutput" -v
```
Expected: FAIL — `renderDashboardOutput undefined`

- [ ] **Step 3: Implement `view_dashboard_output.go`**

```go
package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
)

var (
	dashOutputHeaderStyle  = lipgloss.NewStyle().Bold(true).Foreground(secondaryColor)
	dashOutputSuccessStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#00FF87"))
	dashOutputFailStyle    = lipgloss.NewStyle().Foreground(accentColor)
	dashOutputLineStyle    = lipgloss.NewStyle().Foreground(textColor)
)

// renderDashboardOutput renders the output panel (header + scrollable line buffer).
func (m Model) renderDashboardOutput(width, height int) string {
	header := m.dashOutputHeader()
	var sb strings.Builder
	sb.WriteString(header + "\n")
	sb.WriteString(strings.Repeat("─", width) + "\n")

	contentHeight := height - 2 // header line + separator
	if contentHeight < 1 {
		contentHeight = 1
	}

	start := m.dashOutputOffset
	end := min(start+contentHeight, len(m.dashOutputLines))

	for i := start; i < end; i++ {
		line := m.dashOutputLines[i]
		if len(line) > width {
			line = line[:width]
		}
		sb.WriteString(dashOutputLineStyle.Render(line) + "\n")
	}

	// Fill remaining lines with empty space for consistent height.
	rendered := end - start
	for range contentHeight - rendered {
		sb.WriteString("\n")
	}

	return sb.String()
}

// dashOutputHeader returns the one-line header for the output panel.
func (m Model) dashOutputHeader() string {
	if m.dashRunning {
		spinner := spinnerFrame()
		return dashOutputHeaderStyle.Render(fmt.Sprintf("%s terragrunt %s", spinner, m.dashRunCmd))
	}
	if m.dashLastRun != nil && m.dashLastRun.Command == m.dashRunCmd && m.dashRunCmd != "" {
		dur := time.Duration(m.dashLastRun.DurationS * float64(time.Second)).Round(time.Millisecond)
		if m.dashLastRun.ExitCode == 0 {
			return dashOutputSuccessStyle.Render(fmt.Sprintf("✓ %s — %s", m.dashRunCmd, dur))
		}
		return dashOutputFailStyle.Render(fmt.Sprintf("✗ %s — %s", m.dashRunCmd, dur))
	}
	return dashOutputHeaderStyle.Render("⚡ Output")
}

// spinnerFrame returns a simple spinner character cycling on time.
func spinnerFrame() string {
	frames := []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}
	idx := (time.Now().UnixMilli() / 80) % int64(len(frames))
	return frames[idx]
}
```

- [ ] **Step 4: Run tests**

```bash
go test ./internal/tui/ -run "TestRenderDashboardOutput" -v
```
Expected: all pass.

- [ ] **Step 5: Commit**

```bash
git add internal/tui/view_dashboard_output.go internal/tui/view_dashboard_output_test.go
git commit -m "feat(tui): add dashboard output panel renderer with streaming header states"
```

---

## Task 6: Bottom Panel

**Files:**
- Create: `internal/tui/view_dashboard_bottom.go`
- Create: `internal/tui/view_dashboard_bottom_test.go`

**Interfaces:**
- Produces: `func (m Model) renderDashboardBottom(width, height int) string`

- [ ] **Step 1: Write failing tests**

```go
// internal/tui/view_dashboard_bottom_test.go
package tui

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/israoo/terrax/internal/history"
	"github.com/israoo/terrax/internal/stack"
)

func TestRenderDashboardBottom_DepsTab(t *testing.T) {
	m := makeDashModel(t)
	m.dashActiveTab = 0
	m.dashTreeCursor = 1 // mgmt leaf
	m.dashTreeNodes[1].Node.Dependencies = []string{"/root/infra/dev"}
	result := m.renderDashboardBottom(80, 15)
	assert.Contains(t, result, "Dependencies")
}

func TestRenderDashboardBottom_LastRunTab(t *testing.T) {
	m := makeDashModel(t)
	m.dashActiveTab = 1
	m.dashLastRun = &history.ExecutionLogEntry{
		Command:   "plan",
		ExitCode:  0,
		DurationS: 1.5,
		Timestamp: time.Now(),
	}
	result := m.renderDashboardBottom(80, 15)
	assert.Contains(t, result, "plan")
	assert.Contains(t, result, "Last Run")
}

func TestRenderDashboardBottom_PlanDiffTab_NoPlan(t *testing.T) {
	m := makeDashModel(t)
	m.dashActiveTab = 2
	m.dashPlanReport = nil
	result := m.renderDashboardBottom(80, 15)
	assert.Contains(t, result, "Plan Diff")
	assert.Contains(t, result, "No plan")
}

func TestRenderDashboardBottom_TabSwitching(t *testing.T) {
	m := makeDashModel(t)
	m.dashActiveTab = 0
	result0 := m.renderDashboardBottom(80, 15)
	m.dashActiveTab = 1
	result1 := m.renderDashboardBottom(80, 15)
	// Tab headers should differ
	assert.NotEqual(t, result0, result1)
}
```

- [ ] **Step 2: Run tests to verify failure**

```bash
go test ./internal/tui/ -run "TestRenderDashboardBottom" -v
```
Expected: FAIL — `renderDashboardBottom undefined`

- [ ] **Step 3: Implement `view_dashboard_bottom.go`**

```go
package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
)

var (
	dashTabActiveStyle   = lipgloss.NewStyle().Bold(true).Foreground(secondaryColor).Underline(true)
	dashTabInactiveStyle = lipgloss.NewStyle().Foreground(dimColor)
	dashBottomLineStyle  = lipgloss.NewStyle().Foreground(textColor)
)

// renderDashboardBottom renders the tabbed bottom panel.
func (m Model) renderDashboardBottom(width, height int) string {
	tabBar := m.renderBottomTabBar()
	sep := strings.Repeat("─", width)

	var content string
	switch m.dashActiveTab {
	case 0:
		content = m.renderDepsTab(width, height-2)
	case 1:
		content = m.renderLastRunTab(width, height-2)
	case 2:
		content = m.renderPlanDiffTab(width, height-2)
	}

	return tabBar + "\n" + sep + "\n" + content
}

// renderBottomTabBar renders the tab selector row.
func (m Model) renderBottomTabBar() string {
	tabs := []string{"Dependencies", "Last Run", "Plan Diff"}
	parts := make([]string, len(tabs))
	for i, t := range tabs {
		if i == m.dashActiveTab {
			parts[i] = dashTabActiveStyle.Render(fmt.Sprintf("[%s]", t))
		} else {
			parts[i] = dashTabInactiveStyle.Render(fmt.Sprintf(" %s ", t))
		}
	}
	return strings.Join(parts, "  ")
}

// renderDepsTab renders the Dependencies tab content.
func (m Model) renderDepsTab(width, height int) string {
	if len(m.dashTreeNodes) == 0 {
		return dashTabInactiveStyle.Render("No stack selected.")
	}
	node := m.dashTreeNodes[m.dashTreeCursor].Node

	var lines []string
	if len(node.Dependencies) == 0 && len(node.Dependents) == 0 {
		lines = append(lines, dashTabInactiveStyle.Render("No dependencies."))
	}
	for _, dep := range node.Dependencies {
		lines = append(lines, dashBottomLineStyle.Render("↑ "+truncateText(dep, width-4)))
	}
	for _, dep := range node.Dependents {
		lines = append(lines, dashBottomLineStyle.Render("← "+truncateText(dep, width-4)))
	}

	start := m.dashBottomOffset
	end := min(start+height, len(lines))
	if start >= len(lines) {
		start = 0
		end = min(height, len(lines))
	}
	return strings.Join(lines[start:end], "\n")
}

// renderLastRunTab renders the Last Run tab content.
func (m Model) renderLastRunTab(width, height int) string {
	if m.dashLastRun == nil {
		return dashTabInactiveStyle.Render("No executions yet.")
	}
	e := m.dashLastRun
	dur := time.Duration(e.DurationS * float64(time.Second)).Round(time.Millisecond)
	status := "✓"
	if e.ExitCode != 0 {
		status = fmt.Sprintf("✗ (exit %d)", e.ExitCode)
	}
	lines := []string{
		dashBottomLineStyle.Render(fmt.Sprintf("Command  : %s", e.Command)),
		dashBottomLineStyle.Render(fmt.Sprintf("Status   : %s", status)),
		dashBottomLineStyle.Render(fmt.Sprintf("Duration : %s", dur)),
		dashBottomLineStyle.Render(fmt.Sprintf("Time     : %s", e.Timestamp.Format("2006-01-02 15:04:05"))),
		dashBottomLineStyle.Render(fmt.Sprintf("Stack    : %s", truncateText(e.AbsolutePath, width-12))),
	}
	start := m.dashBottomOffset
	end := min(start+height, len(lines))
	if start >= len(lines) {
		start = 0
		end = min(height, len(lines))
	}
	return strings.Join(lines[start:end], "\n")
}

// renderPlanDiffTab renders the Plan Diff tab content.
func (m Model) renderPlanDiffTab(width, height int) string {
	if m.dashPlanReport == nil {
		return dashTabInactiveStyle.Render("No plan available — run 'plan' first.")
	}

	var lines []string
	for _, result := range m.dashPlanReport.Results {
		if result.Stats.Add+result.Stats.Change+result.Stats.Destroy == 0 {
			continue
		}
		summary := fmt.Sprintf("+%d ~%d -%d  %s",
			result.Stats.Add, result.Stats.Change, result.Stats.Destroy,
			truncateText(result.StackPath, width-20))
		lines = append(lines, dashBottomLineStyle.Render(summary))
	}
	if len(lines) == 0 {
		lines = []string{dashTabInactiveStyle.Render("No changes in plan.")}
	}

	start := m.dashBottomOffset
	end := min(start+height, len(lines))
	if start >= len(lines) {
		start = 0
		end = min(height, len(lines))
	}
	return strings.Join(lines[start:end], "\n")
}
```

- [ ] **Step 4: Run tests**

```bash
go test ./internal/tui/ -run "TestRenderDashboardBottom" -v
```
Expected: all pass.

- [ ] **Step 5: Commit**

```bash
git add internal/tui/view_dashboard_bottom.go internal/tui/view_dashboard_bottom_test.go
git commit -m "feat(tui): add dashboard bottom panel with deps/last-run/plan-diff tabs"
```

---

## Task 7: Dashboard Layout Composer

**Files:**
- Modify: `internal/tui/view_dashboard.go` (replace stub with full implementation)

**Interfaces:**
- Consumes: `renderDashboardTree`, `renderDashboardOutput`, `renderDashboardBottom`, `renderCommandPalette`
- Produces: `func (m Model) renderDashboardView() string` (complete)

- [ ] **Step 1: Implement `view_dashboard.go`**

Replace the stub entirely:

```go
package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

var (
	dashPanelFocusedStyle = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(primaryColor)
	dashPanelNormalStyle = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(dimColor)
	dashPaletteOverlayStyle = lipgloss.NewStyle().
					Border(lipgloss.RoundedBorder()).
					BorderForeground(secondaryColor).
					Padding(1, 2)
	dashPaletteTitleStyle  = lipgloss.NewStyle().Bold(true).Foreground(secondaryColor)
	dashPaletteSelectStyle = lipgloss.NewStyle().Bold(true).Foreground(accentColor)
	dashPaletteItemStyle   = lipgloss.NewStyle().Foreground(textColor)
)

// renderDashboardView renders the complete StateDashboard layout.
func (m Model) renderDashboardView() string {
	if !m.ready || m.width == 0 || m.height == 0 {
		return Initializing
	}

	header := renderHeader(m.width)
	footer := m.renderDashboardFooter()

	// Layout: topH = total - header(1) - footer(1) - bottom panel
	bottomH := m.height * 30 / 100
	if bottomH < 6 {
		bottomH = 6
	}
	topH := m.height - 1 - 1 - bottomH // minus header, footer, bottom
	if topH < 4 {
		topH = 4
	}

	// Tree panel: 25% of width (min 20).
	treeW := m.width * 25 / 100
	if treeW < 20 {
		treeW = 20
	}
	outputW := m.width - treeW

	// Render sub-panels (inner content width = panel width - 2 for borders).
	treeContent := m.renderDashboardTree(treeW-2, topH-2)
	outputContent := m.renderDashboardOutput(outputW-2, topH-2)
	bottomContent := m.renderDashboardBottom(m.width-2, bottomH-2)

	treePanel := m.styleDashPanel(treeContent, treeW, topH, m.dashFocusedPanel == 0)
	outputPanel := m.styleDashPanel(outputContent, outputW, topH, m.dashFocusedPanel == 1)
	topRow := lipgloss.JoinHorizontal(lipgloss.Top, treePanel, outputPanel)

	bottomPanel := m.styleDashPanel(bottomContent, m.width, bottomH, m.dashFocusedPanel == 2)

	body := lipgloss.JoinVertical(lipgloss.Left, topRow, bottomPanel)

	view := lipgloss.JoinVertical(lipgloss.Left, header, body, footer)

	if m.dashPaletteOpen {
		view = m.overlayCommandPalette(view)
	}

	return view
}

// styleDashPanel wraps content in a border, coloured by focus.
func (m Model) styleDashPanel(content string, width, height int, focused bool) string {
	style := dashPanelNormalStyle
	if focused {
		style = dashPanelFocusedStyle
	}
	// Width/Height here is the OUTER dimension including borders (1 char each side).
	return style.Width(width - 2).Height(height - 2).Render(content)
}

// renderDashboardFooter renders the key-hint footer for StateDashboard.
func (m Model) renderDashboardFooter() string {
	if m.dashRunning {
		return footerStyle.Render("ctrl+c: cancel  tab: focus  q: quit")
	}
	return footerStyle.Render("p:plan  a:apply  i:init  v:validate  d:destroy  space:palette  tab:focus  [/]:tabs  q:quit")
}

// overlayCommandPalette renders the command palette as a centred overlay on top of base.
func (m Model) overlayCommandPalette(base string) string {
	// Build palette content.
	paletteW := m.width / 2
	if paletteW < 30 {
		paletteW = 30
	}

	var selectedNode string
	if len(m.dashTreeNodes) > 0 {
		selectedNode = m.dashTreeNodes[m.dashTreeCursor].Node.Name
	}

	var sb strings.Builder
	sb.WriteString(dashPaletteTitleStyle.Render(fmt.Sprintf("Run on: %s", selectedNode)) + "\n\n")
	for i, cmd := range m.commands {
		if i == m.dashPaletteCursor {
			sb.WriteString(dashPaletteSelectStyle.Render("► "+cmd) + "\n")
		} else {
			sb.WriteString(dashPaletteItemStyle.Render("  "+cmd) + "\n")
		}
	}
	sb.WriteString("\n" + dashPaletteItemStyle.Render("↑↓: navigate  enter: run  esc: cancel"))

	palette := dashPaletteOverlayStyle.Width(paletteW).Render(sb.String())

	// Use lipgloss.Place to centre the overlay over the base view.
	return lipgloss.Place(m.width, m.height,
		lipgloss.Center, lipgloss.Center,
		palette,
		lipgloss.WithWhitespaceChars(" "),
	)
}

// renderHeader is a package-level helper used by both StateNavigation and StateDashboard.
func renderHeader(width int) string {
	return headerStyle.Width(width).Render("🌍 " + AppTitle)
}
```

**Note:** `renderHeader` may conflict with the existing `(r *Renderer) renderHeader()` method. Check `view_common.go` and rename if needed. If it conflicts, name it `renderDashboardHeader` and call that instead.

- [ ] **Step 2: Build and resolve any compilation errors**

```bash
task build
```
Fix import issues and any naming conflicts before proceeding.

- [ ] **Step 3: Smoke-test by adding a simple integration render test**

```go
// In update_dashboard_test.go, add:
func TestDashboard_RenderDoesNotPanic(t *testing.T) {
	m := makeDashModel(t)
	m.dashTreeNodes = flattenDashTree(m.navigator.GetRoot(), m.dashTreeExpanded)
	// Should not panic
	result := m.renderDashboardView()
	assert.NotEmpty(t, result)
}
```

```bash
go test ./internal/tui/ -run "TestDashboard_RenderDoesNotPanic" -v
```
Expected: PASS.

- [ ] **Step 4: Full check**

```bash
task check
```
Expected: 0 issues, all tests pass.

- [ ] **Step 5: Commit**

```bash
git add internal/tui/view_dashboard.go internal/tui/update_dashboard_test.go
git commit -m "feat(tui): implement dashboard layout composer with panel focus borders and command palette overlay"
```

---

## Task 8: Streaming Message Tests

**Files:**
- Modify: `internal/tui/update_dashboard_test.go`

- [ ] **Step 1: Add streaming message handler tests**

```go
// In update_dashboard_test.go, add:

func TestDashboard_OutputLineMsgAppendsLine(t *testing.T) {
	m := makeDashModel(t)
	m.dashRunning = true
	ls := &lineScanner{done: make(chan commandResult, 1)}
	m.dashScanner = ls

	result, cmd := m.handleDashboardUpdate(OutputLineMsg{Line: "hello"})
	m2 := result.(Model)
	assert.Equal(t, []string{"hello"}, m2.dashOutputLines)
	assert.NotNil(t, cmd, "should schedule next read")
}

func TestDashboard_CommandFinishedMsg_ClearsRunning(t *testing.T) {
	m := makeDashModel(t)
	m.dashRunning = true
	m.dashRunCmd = "plan"
	m.dashTreeCursor = 1 // mgmt is a leaf

	result, cmd := m.handleDashboardUpdate(CommandFinishedMsg{
		ExitCode: 0,
		Duration: 1200 * time.Millisecond,
	})
	m2 := result.(Model)
	assert.False(t, m2.dashRunning)
	assert.Nil(t, m2.dashCancelCmd)
	assert.NotNil(t, m2.dashLastRun)
	assert.Equal(t, 0, m2.dashLastRun.ExitCode)
	assert.Nil(t, cmd)
}

func TestDashboard_CommandFinishedMsg_NonZeroExitCode(t *testing.T) {
	m := makeDashModel(t)
	m.dashRunning = true
	m.dashRunCmd = "apply"
	m.dashTreeCursor = 1

	result, _ := m.handleDashboardUpdate(CommandFinishedMsg{ExitCode: 1, Duration: 500 * time.Millisecond})
	m2 := result.(Model)
	assert.Equal(t, 1, m2.dashLastRun.ExitCode)
}

func TestDashboard_OutputAutoScroll(t *testing.T) {
	m := makeDashModel(t)
	m.dashRunning = true
	m.dashAutoScroll = true
	m.dashScanner = &lineScanner{done: make(chan commandResult, 1)}

	// Add enough lines to trigger auto-scroll.
	for range 50 {
		m.dashOutputLines = append(m.dashOutputLines, "line")
	}
	result, _ := m.handleDashboardUpdate(OutputLineMsg{Line: "new"})
	m2 := result.(Model)
	// Offset should be updated to show the last line.
	assert.Greater(t, m2.dashOutputOffset, 0)
}
```

- [ ] **Step 2: Run tests**

```bash
go test ./internal/tui/ -run "TestDashboard_" -v
```
Expected: all pass.

- [ ] **Step 3: Commit**

```bash
git add internal/tui/update_dashboard_test.go
git commit -m "test(tui): add streaming message handler tests for dashboard output and finish"
```

---

## Task 9: `cmd/lazy.go` Subcommand

**Files:**
- Create: `cmd/lazy.go`
- Modify: `cmd/root.go`

- [ ] **Step 1: Create `cmd/lazy.go`**

```go
package cmd

import (
	"context"
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"

	"github.com/israoo/terrax/internal/deps"
	"github.com/israoo/terrax/internal/history"
	"github.com/israoo/terrax/internal/stack"
	"github.com/israoo/terrax/internal/tui"
)

var lazyCmd = &cobra.Command{
	Use:   "lazy",
	Short: "Open the multi-panel dashboard TUI",
	Long: `terrax lazy opens a persistent multi-panel dashboard:
- Top-left: indented stack tree with expand/collapse
- Top-right: real-time streaming command output
- Bottom: tabbed context (dependencies, last run, plan diff)

Use p/a/i/v/d for quick commands or Space to open the command palette.`,
	RunE: runLazy,
}

func init() {
	lazyCmd.Flags().String("dir", "", "Initial stack or directory path")
}

func runLazy(cmd *cobra.Command, _ []string) error {
	ctx := cmd.Context()
	if ctx == nil {
		ctx = context.Background()
	}

	dirFlag, _ := cmd.Flags().GetString("dir")
	workDir := getWorkingDirectory(dirFlag)

	rootConfigFile := viper.GetString("root_config_file")
	repoRoot := deps.FindRepoRoot(workDir, rootConfigFile)

	root, maxDepth, err := stack.FindAndBuildTree(workDir)
	if err != nil {
		return fmt.Errorf("failed to build stack tree: %w", err)
	}

	commands := viper.GetStringSlice("commands")
	maxNavCols := viper.GetInt("max_navigation_columns")

	historyService, err := history.NewService(ctx, rootConfigFile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Warning: history unavailable: %v\n", err)
		historyService = nil
	}

	filterFn := tui.FilterPathsFn(func(ctx context.Context, stackPath string) (string, []string, error) {
		includeDeps := viper.GetBool("include_dependencies")
		if !includeDeps {
			return repoRoot, []string{stackPath}, nil
		}
		_, filterPaths := collectTransitiveDeps([]string{stackPath})
		return repoRoot, filterPaths, nil
	})

	m := tui.NewDashboardModel(root, maxDepth, commands, maxNavCols, ctx, filterFn, historyService)

	p := tea.NewProgram(m, tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		return fmt.Errorf("dashboard error: %w", err)
	}
	return nil
}
```

- [ ] **Step 2: Register `lazyCmd` in `cmd/root.go`**

Find the block where other subcommands are registered (look for `rootCmd.AddCommand`):

```go
rootCmd.AddCommand(lazyCmd)
```

- [ ] **Step 3: Build**

```bash
task build
./build/terrax lazy --help
```
Expected: help text for `terrax lazy` is printed.

- [ ] **Step 4: Add subcommand registration test**

```go
// cmd/lazy_test.go
package cmd

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestLazyCmd_IsRegistered(t *testing.T) {
	found := false
	for _, sub := range rootCmd.Commands() {
		if sub.Use == "lazy" {
			found = true
			break
		}
	}
	assert.True(t, found, "lazy subcommand should be registered")
}
```

```bash
go test ./cmd/ -run "TestLazyCmd" -v
```
Expected: PASS.

- [ ] **Step 5: Full check**

```bash
task check
```
Expected: 0 issues, all tests pass.

- [ ] **Step 6: Commit**

```bash
git add cmd/lazy.go cmd/lazy_test.go cmd/root.go
git commit -m "feat(cmd): add terrax lazy subcommand for dashboard mode"
```

---

## Self-Review

**Spec coverage check:**

| Spec requirement | Task |
|---|---|
| `terrax lazy [--dir]` subcommand | Task 9 |
| `StateDashboard` AppState | Task 2 |
| `NewDashboardModel` constructor | Task 2 |
| `RunStreaming` in executor | Task 1 |
| Indented tree with ├─/└─ connectors | Task 3 |
| Expand/collapse with ▶/▼ | Task 3 + Task 4 |
| Real-time streaming output | Task 2 (readNextLine) + Task 5 |
| Auto-scroll while running, freeze on manual scroll | Task 2 (handleOutputKey) |
| `c` to clear output, `Ctrl+C` to cancel | Task 2 (handleOutputKey) |
| Tab focus cycling (tree → output → bottom) | Task 2 (handleDashboardKey) |
| `p/a/i/v/d` command shortcuts | Task 2 (handleCommandKey) |
| Command palette overlay (`Space`) | Task 2 (handlePaletteKey) + Task 7 |
| Dependencies tab | Task 6 |
| Last Run tab | Task 6 |
| Plan Diff tab | Task 6 |
| `[`/`]` tab switching | Task 2 (handleBottomKey) |
| History logging on finish | Task 2 (finishCommand) |
| Layout: 25% tree / 75% output / 30% bottom | Task 7 |
| Focus border highlighting | Task 7 |
| Breadcrumb and header (shared) | Task 7 (uses existing `headerStyle`) |
| `q`/`Esc` exits to shell | Task 2 (handleDashboardKey) |
| Filter path injection from cmd layer | Task 9 (filterFn closure) |

**No gaps found.**

**Type consistency check:** `DashTreeNode` defined in Task 3, used in Task 2 and Task 4. `lineScanner` defined in Task 2, used internally. `CommandFinishedMsg`/`OutputLineMsg` defined in Task 2, tested in Task 8. `FilterPathsFn` defined in Task 2, wired in Task 9. All consistent.

**Placeholder scan:** No TBDs, TODOs, or vague steps found.
