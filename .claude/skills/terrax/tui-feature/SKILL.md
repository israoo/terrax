---
name: terrax-tui-feature
description: Use when adding, modifying, or reviewing any code in internal/tui — new AppState modes, new keyboard handlers, new rendered views, new message types, Lipgloss styling, or TUI layout changes. Use before touching model.go, update.go, view.go, or any view_*/update_* file.
---

# TerraX TUI Feature

## File Map

| File | Responsibility |
|---|---|
| `model.go` | `Model` struct fields + `AppState` consts + constructors + `Update()`/`View()` dispatchers |
| `update.go` | Navigation update handler + all key press handlers |
| `update_plan.go` | Plan review update handler + plan message types |
| `view.go` | `renderNavigationView()` + `LayoutCalculator` + `Renderer` |
| `view_navigation.go` | Column rendering, breadcrumbs, filters |
| `view_history.go` | History table rendering |
| `view_plan.go` | Plan master/detail rendering |
| `view_common.go` | Shared layout helpers (`LayoutCalculator` methods) |
| `styles.go` | Shared Lipgloss `var` block — global styles only |
| `constants.go` | Key bindings, help text, UI dimension constants |

**One rule:** `view_*.go` files are read-only renderers. `update_*.go` files are state mutators. Never mix.

## AppState Machine

```
StateNavigation  →  handleNavigationUpdate  →  renderNavigationView
StateHistory     →  handleHistoryUpdate     →  renderHistoryView
StatePlanReview  →  handlePlanReviewUpdate  →  renderPlanReviewView
```

`Update()` and `View()` in `model.go` are pure switch dispatchers — they never contain logic:

```go
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
```

## Adding a New AppState — Checklist

- [ ] Add constant to the `AppState` `iota` block in `model.go` with a doc comment ending in a period.
- [ ] Add state fields to `Model` struct in `model.go` (group them with a comment header).
- [ ] Add a constructor `NewXxxModel(...)` in `model.go` that sets `state: StateXxx` and initializes only the fields it owns.
- [ ] Add the `case StateXxx:` branch to `Update()` in `model.go` → calls `m.handleXxxUpdate(msg)`.
- [ ] Add the `case StateXxx:` branch to `View()` in `model.go` → calls `m.renderXxxView()`.
- [ ] Create `update_xxx.go` with `handleXxxUpdate(msg tea.Msg) (tea.Model, tea.Cmd)`.
- [ ] Create `view_xxx.go` with `renderXxxView() string`. All methods use **value receivers**.
- [ ] Create `update_xxx_test.go` and `view_xxx_test.go`.

## Value Receiver Rule (critical)

All methods in `view_*.go` must use **value receivers** (`func (m Model)`, not `func (m *Model)`). View renders state — it never mutates it. The compiler will not catch this mistake; you must enforce it manually.

```go
// ✅ Correct — value receiver, read-only
func (m Model) renderPlanReviewView() string { ... }

// ❌ Wrong — pointer receiver implies mutation
func (m *Model) renderPlanReviewView() string { ... }
```

Update handlers in `update_*.go` use value receivers too (Bubble Tea returns a new model, it does not mutate in place):

```go
func (m Model) handleXxxUpdate(msg tea.Msg) (tea.Model, tea.Cmd) {
    // mutate a copy: m.field = newValue
    return m, nil
}
```

## Defining a New Message Type

Define message types in the `update_xxx.go` file for the state that consumes them:

```go
// XxxResultMsg carries the result of an async operation for StateXxx.
type XxxResultMsg struct {
    Data SomeType
    Err  error
}
```

Return a `tea.Cmd` from an update handler to trigger async work:

```go
func fetchXxx() tea.Msg {
    result, err := doAsyncWork()
    return XxxResultMsg{Data: result, Err: err}
}

// In the handler:
return m, fetchXxx
```

Handle the message in the state's update handler:

```go
case XxxResultMsg:
    if msg.Err != nil {
        // handle error — store in model, render in view
        return m, nil
    }
    m.xxxData = msg.Data
    return m, nil
```

## Lipgloss Style Rules

- Use `lipgloss.NewStyle()` for every style — **never** `style.Copy()` (removed in Lipgloss 1.x).
- Styles **shared across views** → `styles.go` (global `var` block).
- Styles **local to one view** → top of `view_xxx.go` in its own `var` block.
- Never define styles inside render functions — define them at package level.

```go
// ✅ Correct — package-level var block in view_xxx.go
var (
    xxxTitleStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("205"))
    xxxItemStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("255"))
)

// ❌ Wrong — style created inside function on every render call
func (m Model) renderXxx() string {
    style := lipgloss.NewStyle().Bold(true) // allocates every frame
}
```

## Navigation Patterns

**Sliding window** — the visible column range is `[navigationOffset, navigationOffset+maxNavigationColumns)`. Increment/decrement `navigationOffset`, never move the columns themselves.

**Per-column filter** — `columnFilters map[int]textinput.Model` keyed by column index (0 = commands, 1+ = nav). Active filter column stored in `activeFilterColumn` (-1 = none active).

**Cyclic navigation** — wrap index on boundary:
```go
m.cursor--
if m.cursor < 0 {
    m.cursor = len(items) - 1
}
```

**Scroll offsets** — `scrollOffsets map[int]int` keyed by column. Reset to 0 when selection changes column.

## Quick Reference — Adding to an Existing State

| I want to... | Where to add it |
|---|---|
| New key binding | `update.go:handleKeyPress()` (navigation) or `update_xxx.go:handleXxxUpdate()` (other states) |
| New rendered element in navigation | `view_navigation.go` |
| New rendered element in plan view | `view_plan.go` |
| New constant (key name, help text) | `constants.go` |
| New shared Lipgloss style | `styles.go` |
| New per-view Lipgloss style | `var` block at top of `view_xxx.go` |
| New field tracking UI state | `Model` struct in `model.go`, initialized in the relevant constructor |
| New dimension calculation | `view_common.go` as a method on `LayoutCalculator` |

## Common Mistakes

| Mistake | Fix |
|---|---|
| Business logic (path resolution, tree traversal) in `model.go` or `update.go` | Move to `internal/stack/navigator.go`. See `terrax:architecture-guard`. |
| Pointer receiver on a `view_*.go` method | Change `(m *Model)` to `(m Model)`. |
| Style defined inside a render function | Move to package-level `var` block. |
| State transition logic split across Update and View | Transitions happen only in `update_*.go` handlers, never in view. |
| Adding a new AppState branch to `Update()` but forgetting `View()` | The dispatcher switch in `View()` will fall through to `"Unknown state"`. Always update both. |
