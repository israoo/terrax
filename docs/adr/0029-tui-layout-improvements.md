# ADR-0029: TUI Layout Improvements

**Status**: Accepted

**Date**: 2026-06-28

**Deciders**: TerraX Core Team

## Context

TerraX is frequently used in VS Code's integrated terminal, where the available
width is significantly narrower than a dedicated full-screen terminal. Three
problems were observed:

1. **Unused space on the right**: with `maxNavigationColumns = 3`, the column
   width was calculated by dividing `(width - ColumnOverhead × N) / N` where
   `ColumnOverhead = 8`. This value was wrong: Lipgloss `Width()` includes
   internal padding, so the only external overhead is `Margin(0,1)` = 2 chars per
   column. The over-subtraction left up to 24 chars of empty space on the right
   for a 4-column layout, and even more for shallow trees where fewer columns are
   rendered.

2. **Arrow indicators consuming column space**: the left (`«`) and right (`»`)
   overflow arrows were rendered as full-height columns inserted into the
   horizontal join, stealing width from navigation columns and visually cluttering
   the central area.

3. **Breadcrumb wrapping**: when the current path exceeded the terminal width the
   breadcrumb bar wrapped to a second line, hiding the title bar above it and
   breaking the fixed-height layout.

## Decision

### Fix 1 — Correct `ColumnOverhead` and use actual column count

`ColumnOverhead` was corrected from `8` to `2` (the actual external cost per
column: left + right margin from `Margin(0, 1)`).

`calculateColumnWidth` was updated to use `min(maxDepth, maxNavigationColumns)`
instead of always `maxNavigationColumns`. For trees shallower than the configured
window size, columns now expand to fill the available width:

```go
actualNavCols := min(maxDepth, m.maxNavigationColumns)
actualVisibleColumns := 1 + actualNavCols

arrowOverhead := 0
if maxDepth > m.maxNavigationColumns {
    arrowOverhead = ArrowIndicatorWidth // 3 chars: 1 char + Padding(0,1)
}

colWidth := (m.width - ColumnOverhead*actualVisibleColumns - arrowOverhead) / actualVisibleColumns
```

### Fix 2 — Replace inline arrow columns with a depth indicator row

The `«` / `»` arrow elements were removed from `renderColumnsWithArrows`. Overflow
is now communicated by a dedicated **depth indicator row** inserted between the
breadcrumb bar and the columns. Each dot represents one global tree level with
three visual states:

| Symbol | Color | Meaning |
|--------|-------|---------|
| `●` | cyan (`#00D9FF`) | Level is currently visible in the column window |
| `○` | dim grey (`#888888`) | Level is reachable from the current node but off-screen |
| `·` | very dark (`#3A3A3A`) | Level exists globally but is unreachable from the current node |

The "unreachable" state is determined by inspecting `navState.Columns[i]`: when
`PropagateSelection` reaches a leaf it calls `clearColumnsFrom`, setting all
deeper columns to `[]string{}`. A depth `i` is reachable if and only if
`navState.Columns[i]` is non-empty.

The indicator is rendered only when `maxDepth > 1`. One line is permanently
reserved in `GetContentHeight` and `getAvailableHeight` via the new constant
`DepthIndicatorLineCount = 1`.

### Fix 3 — Truncate breadcrumb from the left

`renderBreadcrumbBar` now computes the maximum path width as
`width - styleHPadding(4) - iconWidth(3)` and truncates the path from the left
when it exceeds this budget, keeping the deepest (most relevant) portion:

```go
if len(navPath) > maxPathWidth {
    navPath = "..." + navPath[len(navPath)-(maxPathWidth-EllipsisWidth):]
}
```

This guarantees the breadcrumb always occupies exactly one line regardless of
path length.

## Consequences

### Positive

- Columns fill the full terminal width; no wasted space on the right even in
  narrow VS Code integrated terminals.
- Shallow trees (fewer levels than `maxNavigationColumns`) get proportionally
  wider columns, making item names less likely to be truncated.
- The depth indicator gives more information than the old arrows: users can see
  the total tree depth, how many levels are off-screen in each direction, and
  which levels are unreachable from the current node — all at a glance.
- The breadcrumb never wraps, so the fixed-height layout is stable at any
  terminal width.

### Negative

- The depth indicator row costs 1 line of terminal height permanently (even for
  2-level trees where overflow is impossible).
- The "unreachable" dots (`·`) reflect the currently selected path, not the
  global maximum reachable depth from any node. A user may see `·` dots for
  levels that are reachable via a sibling node, which could be momentarily
  confusing.
- Left-truncated breadcrumbs lose the repo root prefix; in multi-repo setups
  it is no longer immediately obvious which repo is active from the breadcrumb
  alone.

## Alternatives Considered

### Option A: Breadcrumb with inline `«` / `»` text suffixes

**Description**: Keep the arrow logic but move the `«` / `»` characters to the
end of the breadcrumb bar as plain text, rather than as separate full-height
columns.

**Pros**:

- No extra line reserved; zero height cost.
- Minimal code change: remove arrow columns, append text to breadcrumb.

**Cons**:

- Provides no information about total tree depth or how many levels are off-screen
  in each direction — only "there is something to the left/right".
- Does not distinguish between "reachable but off-screen" and "globally exists but
  unreachable from here".

**Why rejected**: The user specifically wanted a richer indicator that
communicates both the global depth context and the local reachability from the
current node. A simple `«`/`»` suffix in the breadcrumb carries no more
information than the removed arrow columns, just at a different position.

### Option B: Level range label in breadcrumb (e.g. `Lvl 2–4 / 6`)

**Description**: Show a compact `Lvl start–end / total` label at the right edge
of the breadcrumb, indicating which window slice is currently visible.

**Pros**:

- Zero height cost.
- Communicates total depth and visible range with numeric precision.

**Cons**:

- Does not indicate which levels are reachable from the current node vs.
  unreachable (the "unreachable" state the user asked about).
- Competing for space with the path in the same bar; in narrow terminals both
  would need truncation logic.

**Why rejected**: The user's follow-up question was specifically about the
three-state distinction (visible / reachable-but-off-screen / globally-exists-but-unreachable-here), which a numeric label cannot express without additional
symbols anyway.

### Option C: Dots in breadcrumb bar (no extra line)

**Description**: Append the dot row directly at the right edge of the breadcrumb
bar instead of adding a dedicated indicator line.

**Pros**:

- No extra height reserved.
- Keeps the three-state dot semantics.

**Cons**:

- Path and dots compete for horizontal space; the path would need additional
  truncation to leave room for up to `maxDepth` dots.
- Right-aligning dots while left-aligning the path requires explicit width math
  that couples the breadcrumb renderer to both the path length and `maxDepth`.

**Why rejected**: The breadcrumb already needed truncation logic added in this
same change. Adding dots to the same bar would make the truncation significantly
more complex and fragile, with two variable-width elements sharing a fixed row.
A dedicated line is simpler and keeps each concern isolated.

## Future Enhancements

**Potential Improvements**:

1. Make the depth indicator optional via a `ui.depth_indicator` config key for
   users who prefer the single-line saving.
2. Align the dots horizontally over the navigation columns (offset by the commands
   column width) rather than centering them over the full terminal width, so the
   visual correspondence between dots and columns is more direct.
3. Add click/mouse support to the depth indicator dots to jump directly to a
   specific level offset.

## References

- [`internal/tui/model.go` — `calculateColumnWidth`](../../internal/tui/model.go)
- [`internal/tui/view_navigation.go` — `renderDepthIndicator`](../../internal/tui/view_navigation.go)
- [`internal/tui/view_common.go` — `renderBreadcrumbBar`](../../internal/tui/view_common.go)
- [`internal/tui/constants.go` — `ColumnOverhead`, `DepthIndicatorLineCount`](../../internal/tui/constants.go)
- [ADR-0003: Sliding Window for Deep Hierarchies](0003-sliding-window-navigation.md)
