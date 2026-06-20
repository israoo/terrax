---
name: terrax-architecture-guard
description: Use when writing, modifying, or reviewing code in internal/stack, internal/tui, internal/executor, internal/plan, or cmd/root.go. Use before adding imports, creating new functions, or moving logic between packages. Do NOT use for config-only (.terrax.yaml) or docs-only changes.
---

# TerraX Architecture Guard

## Layer Map (mandatory — memorize before touching any file)

```
cmd/                    CLI glue only — thin, no business logic
internal/
  config/               Constants — no imports of other internal packages
  history/              Persistence — imports config only
  stack/                Business logic — imports config only. ZERO UI.
  executor/             Command execution — imports history, config only. ZERO UI.
  plan/                 Plan analysis — imports history, config only. ZERO UI.
  tui/                  Presentation — imports stack, history, plan, config. ZERO executor direct calls.
```

**Import direction (one-way, no exceptions):**
```
cmd → any internal package
tui → stack, history, plan, config
executor → history, config
plan → history, config
stack → config
history → config
config → (nothing internal)
```

Reversing this hierarchy is an architecture violation. No exceptions.

## The Three Hard Rules

**Rule 1 — No UI in business logic.**
`internal/stack/`, `internal/executor/`, `internal/plan/` must never import `github.com/charmbracelet/bubbletea`, `github.com/charmbracelet/lipgloss`, or `github.com/israoo/terrax/internal/tui`.

**Rule 2 — view.go is read-only.**
`internal/tui/view.go` renders state; it never writes it. Every method must have the signature `func (m Model) ...` (value receiver). If you write `m.anything =` in view.go, you are breaking the architecture.

**Rule 3 — Navigator owns navigation logic.**
Tree traversal, path resolution, breadcrumb generation, selection propagation → `internal/stack/navigator.go`. If you find yourself iterating over `*Node` children inside `model.go` or `update.go`, stop and move it to Navigator.

## Detection — run these before opening a PR

```bash
# Rule 1: UI imports in business logic packages
grep -r "charmbracelet" internal/stack internal/executor internal/plan
grep -r "terrax/internal/tui" internal/stack internal/executor internal/plan

# Rule 2: State mutation in view.go
grep -n "m\." internal/tui/view.go | grep "="

# Rule 3: Tree iteration in TUI layer
grep -n "\.Children" internal/tui/model.go internal/tui/update.go
```

All three commands must return no output. If any does, fix before committing.

## Quick Decision Table

| "I need to add..." | Where it goes |
|---|---|
| New Terragrunt flag | `internal/executor/executor.go` → `appendXxxFlags` helper |
| New navigation behavior | `internal/stack/navigator.go` |
| New UI state field | `internal/tui/model.go` only |
| New rendered element | `internal/tui/view_*.go` only |
| New plan parsing logic | `internal/plan/collector.go` or `plan/tree.go` |
| New history query | `internal/history/service.go` |
| New CLI flag | `cmd/root.go` only |
| New config default | `internal/config/defaults.go` |

## AppState Machine (TUI modes — do not mix)

```
StateNavigation  → tree nav, filtering, sliding window
StateHistory     → history viewer, table rendering
StatePlanReview  → plan master/detail viewer
```

Each state has its own update handler (`update.go`, `update_plan.go`) and view (`view_navigation.go`, `view_history.go`, `view_plan.go`). Never add navigation logic to history handlers or vice versa.

## Rationalization Table

| Excuse | Reality |
|---|---|
| "It's just one line in model.go" | One line becomes ten. Architecture erodes gradually. |
| "It's easier to access Navigator from view.go" | View is pure rendering. Compute in Update, render in View. |
| "I'll refactor it later" | Later never comes. Do it right now. |
| "The test passes so it's fine" | Tests pass on wrong architecture constantly. Check the layer rules. |
| "It's not really UI logic, it's just..." | If it touches `tea.Msg` or `lipgloss`, it's UI. Keep it in `internal/tui/`. |

## Red Flags — STOP if you see these

- `import tea "github.com/charmbracelet/bubbletea"` anywhere outside `internal/tui/`
- `func (m *Model)` in `view.go` (pointer receiver = mutation)
- `node.Children` loop in `model.go`, `update.go`, or any `view_*.go`
- `viper.Get*` called in `internal/tui/` directly (config access belongs in executor/cmd layer, passed as params)
- A new `internal/tui/` file importing `internal/executor/`

## References

- `docs/adr/0004-separation-of-concerns.md` — architectural decisions and rationale
- `docs/adr/0009-executor-isolation-pattern.md` — executor isolation design
- `docs/pitfalls/architecture/mixing-business-logic-ui.md` — documented violations and fixes
- `docs/pitfalls/architecture/tight-coupling.md` — coupling patterns to avoid
