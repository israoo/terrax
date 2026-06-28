# Design: StateDashboard — `terrax lazy`

**Date**: 2026-06-28
**Status**: Draft
**Author**: TerraX Core Team

---

## Problem

The current TUI has three fully separate modes (`StateNavigation`, `StateHistory`, `StatePlanReview`), each occupying the full screen. When running a command, the TUI disappears entirely — the executor writes directly to `os.Stdout` — and the user must re-enter the TUI to continue navigating. There is no way to see command output, stack state, dependencies, and the tree simultaneously.

This is especially noticeable in VS Code's integrated terminal, where the narrow width makes context-switching between modes more disruptive.

---

## Goal

A new `terrax lazy` subcommand that launches `StateDashboard`: a persistent multi-panel interface inspired by lazygit/lazydocker where the stack tree, live command output, and contextual information (dependencies, last run, plan diff) are all visible at the same time.

This is implemented as an additive `AppState` — it does not modify the existing states. If it proves useful, it can become the primary mode in a future iteration.

---

## Layout

```
┌─────────────────────────────────────────────────────────┐
│  🌍 TerraX - Terragrunt eXecutor                        │
├──────────────┬──────────────────────────────────────────┤
│ 🌲 Stacks    │ ⚡ Output                                │
│              │                                          │
│ ▼ infra      │ $ terragrunt plan                        │
│   ▶ shared   │ Initializing provider...                 │
│     ca-cent… │ Plan: 2 to add, 0 to change, 1 destroy   │
│     global   │ > aws_instance.web (add)                 │
│   ▶ mgmt     │ > aws_s3_bucket.logs (destroy)           │
│              │ ▌                                        │
├──────────────┴──────────────────────────────────────────┤
│ [Dependencies] [Last Run] [Plan Diff]                   │
│                                                         │
│  ↑ aws_iam_role.deployer                                │
│  → aws_vpc.main                                         │
│                                                         │
│ p:plan  a:apply  i:init  space:palette  tab:focus  q:quit│
└─────────────────────────────────────────────────────────┘
```

**Proportions:**
- Tree panel: 25% of width (minimum 20 chars)
- Output panel: remaining 75%
- Bottom panel: fixed height, default 30% of total height
- Header: 1 line (same as existing modes)
- Footer: 1 line (keybinding cheatsheet)

---

## Entry Point

New Cobra subcommand `terrax lazy`, parallel to `terrax history` and `terrax run`.

```
terrax lazy [--dir <path>]
```

- `--dir` sets the initial tree cursor to the given stack or directory (same resolution logic as `runTUI` in `cmd/root.go`).
- If `--dir` is omitted, the tree opens at the repo root.
- `q` / `Esc` exits to the shell. `StateDashboard` has no connection to `StateNavigation` — they are independent entry points.

---

## Architecture

### New files

| File | Responsibility |
|---|---|
| `cmd/lazy.go` | Cobra subcommand definition; calls `runDashboard()` |
| `internal/tui/update_dashboard.go` | `handleDashboardUpdate(msg tea.Msg)` — all keyboard and message handling for this state |
| `internal/tui/view_dashboard.go` | `renderDashboardView()` — composes the three panels |
| `internal/tui/view_dashboard_tree.go` | Indented tree renderer with expand/collapse |
| `internal/tui/view_dashboard_output.go` | Scrollable output buffer renderer |
| `internal/tui/view_dashboard_bottom.go` | Tabbed bottom panel (deps, last run, plan diff) |

### Changes to existing files

| File | Change |
|---|---|
| `internal/tui/model.go` | Add `StateDashboard` to `AppState` iota; add dashboard fields to `Model`; add `NewDashboardModel()` constructor |
| `internal/executor/executor.go` | Add `RunStreaming(ctx, writer io.Writer, command, absoluteStackPath, repoRoot string, filterPaths []string, envVars map[string]string) error` — same flag logic as `Run`, stdout/stderr piped to `writer` instead of `os.Stdout` |
| `cmd/root.go` | Register `lazyCmd` |

### New `Model` fields for `StateDashboard`

```go
// Dashboard — tree panel
dashTreeExpanded  map[string]bool  // node path → expanded
dashTreeCursor    int              // index in flattened visible node list
dashTreeNodes     []*stack.Node    // flattened visible tree (recomputed on expand/collapse)

// Dashboard — output panel
dashOutputLines   []string         // captured lines (grows during streaming)
dashOutputOffset  int              // scroll offset
dashRunning       bool             // true while command is executing
dashRunCmd        string           // e.g. "terragrunt plan" (shown in panel header)

// Dashboard — bottom panel
dashActiveTab     int              // 0=deps 1=lastrun 2=plandiff
dashLastRun       *history.ExecutionLogEntry
dashPlanReport    *plan.PlanReport // nil if no recent plan for selected stack

// Dashboard — command palette
dashPaletteOpen   bool
dashPaletteCursor int

// Dashboard — panel focus
dashFocusedPanel  int              // 0=tree 1=output 2=bottom
```

---

## Output Streaming

This is the most significant architectural change. The current `executor.Run` is blocking and writes directly to `os.Stdout`, which is incompatible with an active Bubble Tea program.

`RunStreaming` uses `io.Pipe` to capture output line by line and feed it into the Bubble Tea event loop:

```
handleDashboardUpdate receives key
    → dispatches RunStreaming in goroutine via tea.Cmd
    → RunStreaming writes to io.PipeWriter
    → reader goroutine reads lines from io.PipeReader
    → sends OutputLineMsg{Line: "..."} to tea.Program
    → handleDashboardUpdate appends to dashOutputLines
    → Bubble Tea re-renders output panel

command exits
    → goroutine sends CommandFinishedMsg{ExitCode: n, Duration: d}
    → handleDashboardUpdate sets dashRunning = false, updates dashLastRun
```

**New message types** (defined in `update_dashboard.go`):

```go
type OutputLineMsg struct{ Line string }
type CommandFinishedMsg struct {
    ExitCode int
    Duration time.Duration
    Err      error
}
```

The goroutine is launched as a `tea.Cmd` (not a raw goroutine) so Bubble Tea manages its lifecycle correctly.

---

## Keyboard Bindings

### Tree panel (focused)

| Key | Action |
|---|---|
| `↑` / `↓` | Move cursor |
| `Enter` | Expand/collapse directory; no-op on leaf |
| `p` | Run `plan` on selected stack |
| `a` | Run `apply` on selected stack |
| `i` | Run `init` on selected stack |
| `v` | Run `validate` on selected stack |
| `d` | Run `destroy` on selected stack |
| `Space` | Open command palette |

### Output panel (focused)

| Key | Action |
|---|---|
| `↑` / `↓` / `PgUp` / `PgDn` | Scroll buffer |
| `c` | Clear output lines |
| `Ctrl+C` | Cancel running command (sends os.Interrupt to subprocess) |

### Bottom panel (focused)

| Key | Action |
|---|---|
| `↑` / `↓` | Scroll content |
| `[` / `]` | Switch tabs |

### Global (any focus)

| Key | Action |
|---|---|
| `Tab` | Move focus: tree → output → bottom → tree |
| `q` / `Esc` | Exit to shell |

**While a command is running:** all command-trigger keys (`p`, `a`, `i`, `v`, `d`, `Enter` on palette) are ignored. `Ctrl+C` and scroll keys remain active.

---

## Panel Details

### Tree panel

- Renders `stack.Node` tree as indented text with `├─` / `└─` connectors
- Directories show `▶` (collapsed) or `▼` (expanded)
- Stack leaves show their name; truncated to fit panel width with `…`
- Selected node highlighted with `selectedItemStyle`
- Tree state (expanded nodes, cursor) initialised from `--dir` if provided

### Output panel

- Header line: spinner + command name while running; `✓ plan — 0.8s` or `✗ plan — 2.1s` when finished (green/red)
- Buffer: `dashOutputLines` displayed from `dashOutputOffset`; auto-scrolls to bottom while running, freezes on manual scroll
- Auto-scroll resumes if user scrolls back to bottom

### Bottom panel — tabs

**Dependencies tab**: lists direct dependencies (↑) and dependents (←) of the selected stack, sourced from `stack.Node.Dependencies` and `stack.Node.Dependents` already populated by `AnalyzeGraph`.

**Last Run tab**: shows command, exit code, duration, timestamp, and truncated output summary from `dashLastRun`. Sourced from `history.ExecutionLogEntry` written after `RunStreaming` completes.

**Plan Diff tab**: shows the plan summary for the selected stack if a `.terrax/plans/<stack-path>/tfplan.json` exists. Uses existing `plan.CollectFromJSONDir` and renders via adapted `view_plan.go` helpers. Tab is visually marked as unavailable (greyed label) when no plan file exists.

### Command palette

- Overlay centred on screen, width ~50% of terminal
- Lists all commands from `m.commands` (sourced from `.terrax.yaml`)
- Header shows selected stack path
- `↑↓` to navigate, `Enter` to execute, `Esc` to close without executing
- Closes automatically after dispatching a command

---

## Out of Scope (v1)

- Multi-stack selection (space-to-mark) — dashboard runs one stack at a time
- Launching `plan` automatically to populate Plan Diff tab — tab shows existing files only
- Full execution history browser — Last Run shows only the most recent entry for the selected stack
- Synchronisation of tree cursor position with `StateNavigation`
- Configurable panel proportions

---

## Testing Strategy

- **Unit tests** for `renderDashboardTree` (indented connectors, truncation, expand state)
- **Unit tests** for `renderDashboardOutput` (scroll offset, auto-scroll logic, header states)
- **Unit tests** for `handleDashboardUpdate` key dispatch (command ignored while running, tab switching, palette open/close)
- **Integration test** for `RunStreaming` using an `io.Pipe` + mock command: verify `OutputLineMsg` sequence and `CommandFinishedMsg`
- Existing tests for `executor.Run` must remain green — `RunStreaming` is additive, not a replacement

---

## Future Enhancements

1. **Multi-stack execution** — mark multiple stacks in the tree, run a command on all of them sequentially with aggregated output
2. **Become primary mode** — absorb `StateHistory` and `StatePlanReview` as tabs in the bottom panel, making `terrax lazy` the default entry point
3. **Configurable layout** — panel proportions via `.terrax.yaml` (`ui.dashboard.tree_width_ratio`, `ui.dashboard.bottom_height_ratio`)
4. **Live drift detection** — a background goroutine polls for state changes and updates a drift indicator on each tree node
