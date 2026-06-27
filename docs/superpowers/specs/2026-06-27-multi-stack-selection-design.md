# Multi-Stack Selection Design

**Date:** 2026-06-27
**Status:** Approved

## Overview

Allow users to mark multiple stacks (and/or directories) in the TerraX TUI and execute a Terragrunt command across all of them in a single run. The marked paths feed into the existing transitive-dependency analysis, producing a single `terragrunt run --filter ...` invocation — the same mechanism used today for single-stack execution.

## Goals

- Select N stacks across any depth of the tree via `Space`
- Marking a directory implicitly marks all descendant stacks
- Execute a command on the full union of selected stacks (plus their transitive deps) with `Enter`
- Zero breaking changes to single-stack workflow

## Non-Goals

- Parallel execution across stacks (Terragrunt handles ordering via dependency graph)
- Per-stack command overrides (same command for all marked stacks)
- Persistent selection across TUI sessions
- Selection limits

---

## Architecture

### Approach

Mapa de paths seleccionados en el `Model` (Opción A). Selection is UI state — it does not belong in the Navigator or `NavigationState`.

---

## Section 1 — State and Model

**New field in `internal/tui/model.go`:**

```go
selectedPaths map[string]bool // absolute paths of explicitly marked nodes (stacks or directories)
```

Initialized as `make(map[string]bool)` in `NewModel()`.

**New method in `internal/stack/navigator.go`:**

```go
// GetPathAtDepthAndIndex returns the absolute path for the item at index i
// within the column at the given depth. Returns empty string if depth or index
// are out of bounds.
func (n *Navigator) GetPathAtDepthAndIndex(state *NavigationState, depth, index int) string
```

Implementation: walk the tree following `SelectedIndices[0..depth-2]` to reach the parent node at `depth-1`, then return `parent.Children[index].Path`. For depth 0 (root column), walk from the root node directly.

---

## Section 2 — Keyboard Handling and Execution Flow

### Space key (`update.go`)

1. Get the path of the item under the cursor: `navigator.GetPathAtDepthAndIndex(navState, focusedColumn, selectedIndex)`
2. If the path is empty: no-op.
3. If any ancestor path of the current item is already in `selectedPaths`: no-op (parent covers it).
4. Otherwise: toggle the path in `m.selectedPaths` (add if absent, remove if present).

### Enter key (`handleEnterKey()`)

- `len(m.selectedPaths) == 0` → current behavior (uses cursor path).
- `len(m.selectedPaths) > 0` → sets `m.confirmed = true`; `GetSelectedStackPaths()` returns all marked paths.

### `GetSelectedStackPaths()` (new method on Model)

Returns `[]string` of all paths currently in `selectedPaths`. Used by `cmd/root.go` to drive execution.

### `cmd/root.go` — extending `collectTransitiveDeps`

Signature change:

```go
// Before
func collectTransitiveDeps(root *stack.Node, stackPath string) (repoRoot string, filterPaths []string, err error)

// After
func collectTransitiveDeps(root *stack.Node, stackPaths []string) (repoRoot string, filterPaths []string, err error)
```

Steps:
1. **Expand directories:** For each path in `stackPaths`, if the corresponding node is not a leaf stack (`isStack == false`), replace it with all descendant stacks via DFS. Collect the expanded list of stack paths (deduped).
2. **Union of transitive deps:** Run the existing BFS analysis for each stack path, union all resulting filter paths (deduped set).
3. The final `filterPaths` slice feeds into `executor.Run` as `--filter` flags — same as today.

Single-stack callers are updated to pass `[]string{stackPath}`.

---

## Section 3 — Visual Rendering

### Marker logic

A helper in `view_navigation.go`:

```go
func isMarkedOrAncestorMarked(path string, selectedPaths map[string]bool) bool
```

Checks if `path` is in `selectedPaths`, then checks each parent prefix (splitting on `filepath.Separator`) up to the repo root. Returns true on first match.

### Item rendering

- `isMarkedOrAncestorMarked(itemPath, selectedPaths) == true` → prefix `●` rendered in accent color (same yellow/orange used for current highlights in `styles.go`).
- No marks exist (`len(selectedPaths) == 0`) → no prefix rendered (current behavior preserved).
- Marks exist but item is not marked → prefix `○` rendered in muted color.

The cursor highlight (row background) is independent of the mark — an item can be both the cursor and marked simultaneously.

### Footer / help bar

When `len(selectedPaths) == 0`:
```
space  mark   ↑↓  navigate   enter  run
```

When `len(selectedPaths) > 0`:
```
space  mark/unmark   enter  run on marked (N)   esc  clear all
```

`esc` with marks present clears `selectedPaths` before quitting (or a dedicated clear shortcut — see edge cases).

---

## Section 4 — Edge Cases and Error Handling

**Space on descendant of a marked directory:** No-op. The parent's mark already covers all descendants. To unmark, press `Space` on the parent directory.

**Filtered column with marked items hidden:** Paths remain in `selectedPaths` while filtered out. The `●` is not visible but the selection persists. When the filter is cleared, the marker reappears.

**Enter from commands column with marks present:** Commands column has no path of its own — if marks exist, they are used. If no marks, falls back to the cursor path in the navigation column.

**Directory expansion yields no stacks:** If a marked directory has no descendant stacks (empty subtree), it contributes nothing to the filter list. No error; the union simply excludes it.

**Path not found in tree during `collectTransitiveDeps`:** Returns `fmt.Errorf("path not found in tree: %s", path)` for the first missing path.

**`esc` behavior with active marks:** Pressing `esc` (or `q`) while marks exist: first press clears `selectedPaths`; second press quits. This preserves the ability to recover from accidental marks without restarting.

---

## Files Affected

| File | Change |
|------|--------|
| `internal/tui/model.go` | Add `selectedPaths map[string]bool`; add `GetSelectedStackPaths() []string` |
| `internal/tui/update.go` | Handle `Space` key; update `handleEnterKey()` |
| `internal/tui/view_navigation.go` | Render `●`/`○` prefixes; update footer |
| `internal/tui/view_common.go` | Update footer helper to accept mark count |
| `internal/stack/navigator.go` | Add `GetPathAtDepthAndIndex()` |
| `cmd/root.go` | Extend `collectTransitiveDeps` to `[]string`; call `GetSelectedStackPaths()` |

---

## Testing

- `navigator_test.go`: Table-driven tests for `GetPathAtDepthAndIndex` at various depths and indices, including out-of-bounds.
- `model_test.go` (or new `selection_test.go`): Toggle logic, ancestor-no-op rule, clear behavior.
- `root_test.go`: `collectTransitiveDeps` with multiple paths, directory expansion, deduplication of filter paths.
- `view_navigation_test.go`: `isMarkedOrAncestorMarked` with path prefixes.
