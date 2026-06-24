# ADR-0003: Sliding Window for Deep Hierarchies

**Status**: Accepted

**Date**: 2025-12-27

**Deciders**: TerraX Core Team

## Context

TerraX navigates hierarchical Terragrunt stacks that can be arbitrarily deep (5, 10, or more levels). Terminal width is limited, typically 80-200 columns.

### Problem

Displaying all navigation levels simultaneously in deep hierarchies creates several issues:

1. **Screen real estate**: Each level requires ~30 columns minimum. At 5+ levels, this exceeds most terminal widths.
2. **Visual clutter**: Showing all levels at once overwhelms users with information.
3. **Focus fragmentation**: Users can't focus on current navigation context when distant levels are visible.
4. **Usability**: Horizontal scrolling is cumbersome and breaks mental model.

### Requirements

- Support navigation of arbitrarily deep hierarchies.
- Maintain clear visual focus on current navigation context.
- Work within standard terminal widths (80-200 columns).
- Preserve full breadcrumb trail for orientation.
- Provide intuitive navigation experience.

## Decision

We implement a **sliding window** pattern that shows a maximum of 3 navigation columns plus 1 commands column, with the window sliding horizontally as users navigate deeper.

### Key Concepts

1. **Fixed window size**: Always show maximum 3 navigation columns (+ 1 commands column).
2. **Navigation offset**: Track the left-most visible level (`navigationOffset`).
3. **Visible range**: Display levels `[offset, offset+3)`.
4. **Window sliding**: Offset increases when navigating right beyond window, decreases when navigating left before window.
5. **Full breadcrumbs**: Always show complete path regardless of window position.

### Implementation

```go
// Model tracks navigation offset
type Model struct {
    navigationOffset int    // Left-most visible navigation level
    focusedColumn    int    // Currently focused column (0 = commands, 1+ = nav)
    // ... other fields
}

// Slide window right
if m.focusedColumn >= maxVisibleNavColumns {
    m.navigationOffset++
    m.focusedColumn = maxVisibleNavColumns - 1
}

// Slide window left
if m.focusedColumn < 1 && m.navigationOffset > 0 {
    m.navigationOffset--
    m.focusedColumn = 1
}
```

### Layout Calculation

```go
func (lc LayoutCalculator) CalculateVisibleColumns(
    maxDepth, offset int,
) (startDepth, endDepth int) {
    startDepth = offset
    endDepth = min(offset+maxVisibleNavColumns, maxDepth)
    return
}
```

## Consequences

### Positive

- **Scalability**: Supports arbitrarily deep hierarchies without layout issues.
- **Focus**: Users see only relevant navigation context (current and adjacent levels).
- **Terminal compatibility**: Works within standard terminal widths (4 columns × 30 chars = 120 chars).
- **Orientation**: Full breadcrumbs preserve spatial awareness.
- **Predictability**: Window movement follows intuitive left/right navigation.
- **Performance**: Renders only visible columns, reducing rendering overhead.

### Negative

- **Hidden context**: Users can't see all levels simultaneously in deep trees.
- **Cognitive load**: Users must mentally track position in deep hierarchies (mitigated by breadcrumbs).
- **Implementation complexity**: Window management adds logic to navigation handling.
- **Edge case handling**: Requires careful handling of window boundaries.

## Alternatives Considered

### Option 1: Horizontal Scrolling (All Columns)

**Description**: Show all navigation levels with horizontal scrolling.

**Pros**:
- Complete visibility of all levels.
- No hidden information.
- Simpler conceptual model.

**Cons**:
- Poor terminal UX (scrolling breaks visual continuity).
- Wide displays become unusable.
- Users lose orientation when scrolling.
- Difficult to maintain focus on current context.

**Why rejected**: Horizontal scrolling in terminals is awkward and breaks user focus. Breadcrumbs provide better orientation than seeing all levels.

### Option 2: Collapsible Columns

**Description**: Allow users to collapse/expand navigation columns.

**Pros**:
- User control over visible information.
- Can show all or few levels as needed.
- Flexible for different terminal sizes.

**Cons**:
- Requires additional key bindings and UI.
- State management complexity (track collapsed state).
- Cognitive overhead (users must manage collapse state).
- Doesn't solve fundamental screen space problem.

**Why rejected**: Adds complexity without solving the core issue. Users would need to constantly toggle columns, creating friction.

### Option 3: Vertical Stacking

**Description**: Stack navigation levels vertically instead of horizontally.

**Pros**:
- No horizontal space constraints.
- Can show many levels simultaneously.
- Natural top-to-bottom reading flow.

**Cons**:
- Consumes vertical space (limited to ~50 lines typically).
- Harder to show sibling options at each level.
- Breaks conventional file browser mental model.
- Limits number of items visible per level.

**Why rejected**: Vertical space is also limited. Horizontal layout better matches user mental model from tools like Miller columns, macOS Finder, and file browsers.

### Option 4: Single Column with Depth Indication

**Description**: Show only current level with depth indicators (like `cd` + `ls`).

**Pros**:
- Minimal screen space usage.
- Simple implementation.
- Familiar from traditional CLI tools.

**Cons**:
- No parallel view of adjacent levels.
- Breaks interactive navigation UX.
- Requires more key presses to navigate.
- Loses context of parent/child relationships.

**Why rejected**: TerraX's value proposition is visual hierarchical navigation. Single column defeats the purpose of a TUI.

## Implementation Guidelines

### Window Movement Rules

1. **Focus enters commands column (0)**: No window movement.
2. **Focus enters first navigation column (1)**: Slide window left if possible.
3. **Focus exits last visible navigation column**: Slide window right.
4. **Focus moves within visible window**: No window movement.

### Edge Cases

- **Shallow hierarchies** (depth ≤ 3): No sliding needed, show all levels.
- **At beginning** (offset = 0): Cannot slide left.
- **At end** (offset + 3 ≥ maxDepth): Cannot slide right.

### User Feedback

- **Breadcrumbs**: Always show full path for orientation.
- **Column headers**: Show depth level numbers if helpful.
- **Visual indicators**: Consider showing "..." or arrows to indicate hidden levels.

## Future Considerations

- **Dynamic window size**: Adjust visible columns based on terminal width.
- **Configurable window size**: Allow users to set visible column count.
- **Smooth transitions**: Animate window sliding for better UX (if terminal supports).

## References

- Implementation: [internal/tui/model.go](../../internal/tui/model.go) (window management)
- Implementation: [internal/tui/view.go](../../internal/tui/view.go) (layout calculation)
- Related: [ADR-0001: Bubble Tea Architecture](0001-bubble-tea-architecture.md)
- Pattern inspiration: [Miller Columns](https://en.wikipedia.org/wiki/Miller_columns)
