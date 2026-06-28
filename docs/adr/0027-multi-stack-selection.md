# ADR-0027: Multi-Stack Selection

**Status**: Accepted

**Date**: 2026-06-28

**Deciders**: TerraX Core Team

**Related**:
- [ADR-0017: Filter-Based Execution Strategy](0017-filter-based-execution-strategy.md)
- [ADR-0008: Dual-Mode TUI Architecture](0008-dual-mode-tui-architecture.md)

## Context

TerraX previously executed Terragrunt commands on a single stack selected via the TUI navigation tree. Users working across multiple stacks (e.g., deploying several microservices together, or running plan across a subset of an environment) had to execute TerraX once per stack, losing the benefit of Terragrunt's dependency-aware parallel execution.

Requirements:
1. Allow marking N stacks or directories in the TUI before executing.
2. Marking a directory must implicitly cover all descendant stacks.
3. Marking a specific stack when a parent directory is already marked must remove the parent mark and expand the siblings — keeping the parent's full coverage minus the deselected item.
4. The selection must feed into the existing `collectTransitiveDeps` → `buildGroupedExecution` → `executor.Run` pipeline without changing how execution works.
5. Zero breaking changes to the existing single-stack workflow.

## Decision

**Selection state lives in `tui.Model` as `selectedPaths map[string]bool`** — a set of explicitly marked absolute paths (stacks or directories). All paths are normalized to forward slashes via `filepath.ToSlash` at entry for cross-platform consistency.

### Key behaviors

- **`Space`** marks/unmarks the item under the cursor.
- **Marking a parent** with existing child marks removes the children and adds the parent (parent covers all descendants; keeping children would cause duplicate `--filter` flags).
- **Marking a child** when a parent is already marked removes the parent and expands all sibling branches at every intermediate level, leaving only the pressed item unmarked.
- **`Esc`/`q`** with marks active clears all marks (first press); second press quits — recovering from accidental marks without restarting.

### Execution integration

`cmd/root.go` determines the execution paths after the TUI exits:

```go
var execPaths []string
if model.HasSelectedPaths() {
    execPaths = model.GetSelectedStackPaths()
} else {
    execPaths = []string{stackPath} // existing single-stack behavior
}
repoRoot, filterPaths := collectTransitiveDeps(execPaths)
```

`collectTransitiveDeps` was extended from `(stackPath string)` to `(stackPaths []string)`. It seeds the BFS queue from all input paths, expanding non-leaf directories via `stack.CollectStackPaths`, and unions all resulting filter paths (deduped via the `visited` map). The downstream `buildGroupedExecution` → `executor.Run` chain is unchanged.

### Visual feedback

Navigation columns show `●` (accent color) for marked items and `○` (dim color) for unmarked items when any selection is active. Both the item itself and its descendants show `●` if any ancestor is in `selectedPaths`. The footer switches from static help text to a dynamic count: `enter: run on marked (N) | esc: clear all`.

### New Navigator method

`GetPathAtDepthAndIndex(state, depth, index) string` returns the absolute path of the item at position `index` in the column at navigation `depth`. This lets `handleSpaceKey` and `buildNavigationList` resolve paths without coupling to `Node.Path` access patterns in the update/view layer.

### Parent-expand algorithm

When pressing `Space` on a child whose ancestor is marked, `excludeChildFromAncestorMark` walks from the marked ancestor node downward, adding sibling branches at each level except the branch leading to the pressed item. This is implemented as `expandSiblingsExcluding(node, childPath, selectedPaths)` — a DFS that calls `continue` (not `return`) on the exact target node so all siblings are processed.

## Consequences

### Positive

- Multi-stack execution reuses the existing `--filter`-based pipeline; Terragrunt handles dependency ordering and parallelism automatically.
- Single-stack workflow is entirely unchanged — `HasSelectedPaths() == false` falls through to existing behavior.
- Directory marks give users a one-key way to select entire subtrees without navigating every leaf.
- The parent-expand behavior makes selection editing intuitive: removing one item from a directory mark is a single `Space` press, not a multi-step deselect-all-then-reselect sequence.

### Negative

- `selectedPaths` is session-only; marks are lost when the TUI exits. Users running the same multi-stack operation repeatedly must re-mark each time.
- `collectTransitiveDeps` now accepts a slice, requiring all callers (`execute.go`, `run.go`, `groups.go`) to wrap single-path invocations in `[]string{path}`.
- `expandSiblingsExcluding` requires `Navigator.FindNodeByPath` — a full tree traversal — which is O(n) in tree size. Acceptable for typical stack counts but not O(1).

## Alternatives Considered

### Option 1: Selection state in NavigationState / Navigator

Store `MarkedPaths map[string]bool` on `stack.NavigationState` and expose `ToggleMark` / `GetMarkedPaths` on `Navigator`.

**Pros**:

- Selection travels with the navigation state.

**Cons**:

- `NavigationState` and `Navigator` are pure business-logic types with no UI dependencies. "Marked for execution" is a UI concern — it maps to how a user is interacting with the TUI, not to the structure of the stack tree itself.

**Why rejected**: The layer rule in CLAUDE.md is explicit — `internal/stack/` must remain UI-free. Adding execution-selection state to Navigator would blur this boundary and make the package harder to test and reason about independently of the TUI.

### Option 2: Separate SelectionSet type in the TUI package

A dedicated `tui.SelectionSet` struct with `Toggle`, `Has`, `All`, `Clear` methods, embedded in `Model`.

**Pros**:

- Better encapsulation if selection logic grows (limits, validation, persistence).
- Testable in isolation.

**Cons**:

- The methods (`toggleSelectedPath`, `clearSelectedPaths`, `hasMarkedAncestor`, `expandSiblingsExcluding`) are already cohesive functions on `Model`. Adding a wrapper type adds indirection with no current benefit.

**Why rejected**: YAGNI. The functionality is straightforward enough that a dedicated type would be premature abstraction. The methods live directly on `Model` where they can access `m.navigator` and `m.selectedPaths` without passing additional parameters.

### Option 3: Multiple separate `executor.Run` invocations

Execute `executor.Run` once per selected stack in sequence, rather than computing a union of filter paths and running once.

**Pros**:

- Simpler change — no modification to `collectTransitiveDeps`.

**Cons**:

- Loses Terragrunt's cross-stack dependency awareness. If stack A depends on stack B and both are selected, two separate runs might try to apply them in the wrong order or apply B twice (once as a dependency of A, once as a direct target).
- History entries would record N separate executions instead of one coherent multi-stack run.
- Does not match how users think about "run these stacks together."

**Why rejected**: The existing filter-based pipeline was specifically designed (ADR-0017) to give Terragrunt full visibility into the dependency graph. Running stacks independently bypasses that design and introduces ordering bugs for any selection that spans dependent stacks.

## Future Enhancements

**Potential Improvements**:

1. **Persistent named selections**: Save a marked set to `.terrax.yaml` under a user-defined name (e.g., `selections.staging-infra`) for quick recall across sessions.
2. **Selection history**: Persist the last N multi-stack selections in the execution history log so users can re-select the same set from the history TUI.
3. **VS Code integration**: Expose `GetSelectedStackPaths()` via the `terrax tree --json` output or a new `terrax selection` subcommand so the VS Code extension can display and re-trigger multi-stack runs.

## References

- [`internal/tui/model.go`](../../internal/tui/model.go) — `selectedPaths`, `toggleSelectedPath`, `excludeChildFromAncestorMark`, `expandSiblingsExcluding`
- [`internal/stack/navigator.go`](../../internal/stack/navigator.go) — `GetPathAtDepthAndIndex`, `FindNodeByPath`
- [`internal/tui/update.go`](../../internal/tui/update.go) — `handleSpaceKey`
- [`internal/tui/view_navigation.go`](../../internal/tui/view_navigation.go) — `isMarkedOrAncestorMarked`, marker rendering
- [`cmd/root.go`](../../cmd/root.go) — `collectTransitiveDeps` multi-path extension
- [ADR-0017: Filter-Based Execution Strategy](0017-filter-based-execution-strategy.md)
