# Pitfall: Deleting Helpful Comments

**Category**: Code Quality

**Severity**: Medium

**Date Identified**: 2025-12-27

## Description

Removing existing comments during refactoring or "cleanup" without careful consideration, losing valuable context about why code exists, how it works, or what edge cases it handles.

## Impact

Deleting comments creates problems:

- **Lost context**: Future developers (including you) lose understanding of why decisions were made.
- **Repeated mistakes**: Without comments documenting pitfalls, same bugs get reintroduced.
- **Longer onboarding**: New contributors must reverse-engineer intent from code.
- **Maintenance burden**: Understanding complex code without comments requires deep analysis.
- **Knowledge loss**: Tribal knowledge becomes inaccessible when original author leaves.
- **Debugging difficulty**: Missing comments about edge cases make bugs harder to diagnose.

## Root Cause

Common reasons comments get deleted:

1. **"Clean code" dogma**: Belief that "code should be self-documenting" taken to extreme.
2. **Superficial refactoring**: "Cleaning up" without understanding comment value.
3. **Mistaking comment types**: Removing explanatory comments while only bad comments (stating obvious) should go.
4. **Time pressure**: "This comment is outdated, delete it" instead of updating it.
5. **Automated tools**: Linters or formatters removing comments incorrectly.

## How to Avoid

### Do

- **Read before deleting**: Understand what the comment explains before removing it.
- **Update, don't delete**: If comment is outdated, fix it to match current code.
- **Preserve "why" comments**: Comments explaining rationale, trade-offs, or edge cases are valuable.
- **Keep context comments**: Comments referencing issues, PRs, or external docs should stay.
- **Document non-obvious**: If code isn't immediately clear, comment explains intent.

### Don't

- **Don't delete without reading**: Always understand comment purpose first.
- **Don't remove edge case docs**: Comments about subtle bugs or corner cases are critical.
- **Don't assume obvious**: What's obvious to you may not be to others.
- **Don't remove TODO comments**: These track technical debt; update or address them instead.
- **Don't apply "no comments" rule blindly**: Context matters.

## Detection

Warning signs of problematic comment deletion:

- **PRs removing many comments**: Large comment deletions should be reviewed carefully.
- **Code becomes unclear**: If reviewers ask "why does this code do X?", comment was probably needed.
- **Repeated questions**: Team members asking same questions suggests missing documentation.
- **Bug reintroduction**: Fixed bugs reappearing indicates lost edge case documentation.
- **Long code review discussions**: Extensive explanation in PR review suggests missing inline comments.

## Remediation

If helpful comments have been deleted:

1. **Review git history**: Use `git log -p` to find deleted comments.

   ```bash
   git log -p --all -S "comment text" -- path/to/file.go
   ```

2. **Assess value**: Determine if comment provided important context.

3. **Restore if needed**: Add comment back, potentially improved.

   ```go
   // Restored comment with better clarity
   // Original context from commit abc123
   ```

4. **Update if outdated**: Fix comment to match current code.

5. **Add issue reference**: Link to issue or PR for future context.

## Related

- [Standard: Comment Guidelines](../../standards/comment-guidelines.md)
- [ADR-0004: Separation of Concerns](../../adr/0004-separation-of-concerns.md)

## Examples

### Bad: Deleting Valuable Context

**Before**:
```go
// PropagateSelection updates selection indices when parent selection changes.
// This is necessary because children lists change when parent selection moves,
// so child indices must be reset to 0 to avoid out-of-bounds access.
func (n *Navigator) PropagateSelection(state *NavigationState) {
    // Truncate selections beyond current depth
    state.SelectedIndices = state.SelectedIndices[:depth+1]

    // Reset child selections to 0
    for i := depth + 1; i < len(state.SelectedIndices); i++ {
        state.SelectedIndices[i] = 0
    }
}
```

**After (WRONG)**:
```go
func (n *Navigator) PropagateSelection(state *NavigationState) {
    state.SelectedIndices = state.SelectedIndices[:depth+1]

    for i := depth + 1; i < len(state.SelectedIndices); i++ {
        state.SelectedIndices[i] = 0
    }
}
```

**Problem**: Lost explanation of why propagation is necessary. Future developer may not understand the out-of-bounds risk.

### Good: Preserving Important Comments

**Before**:
```go
// Window slides right when focus exceeds visible columns.
// This maintains the invariant that focusedColumn < maxVisibleNavColumns.
func (m Model) handleRightNavigation() Model {
    if m.focusedColumn >= maxVisibleNavColumns {
        m.navigationOffset++
        m.focusedColumn = maxVisibleNavColumns - 1
    }
    return m
}
```

**After (CORRECT)**:
```go
// Window slides right when focus exceeds visible columns.
// This maintains the invariant that focusedColumn < maxVisibleNavColumns,
// ensuring sliding window always shows current focus position.
func (m Model) handleRightNavigation() Model {
    if m.focusedColumn >= maxVisibleNavColumns {
        m.navigationOffset++
        m.focusedColumn = maxVisibleNavColumns - 1
    }
    return m
}
```

**Better**: Enhanced comment with additional clarity while preserving original context.

### Bad Comments to Remove

```go
// This function gets children
func (n *Navigator) GetChildren() []*Node {
    return n.root.Children // Returns children
}
```

**These SHOULD be deleted**: They state the obvious and add no value.

### Good Comments to Keep

```go
// GetChildrenAtDepth returns children at the specified depth level,
// accounting for the current selection path. Returns nil if depth
// exceeds the tree depth or if no node is selected at parent levels.
//
// Note: Depth is 0-indexed, so depth=0 returns root's children.
func (n *Navigator) GetChildrenAtDepth(state *NavigationState, depth int) []*Node {
    // ...
}
```

**These SHOULD be kept**: They explain behavior, edge cases, and non-obvious details.

## Comment Preservation Checklist

Before deleting a comment, ask:

- [ ] Does this comment explain **why**, not just **what**?
- [ ] Does it document an edge case or gotcha?
- [ ] Does it reference an issue, PR, or external resource?
- [ ] Would a new contributor find this helpful?
- [ ] Does it explain a non-obvious trade-off or decision?
- [ ] Is the code's purpose immediately clear without it?

If you answered "yes" to any of the first 5 questions, **keep and potentially improve the comment** rather than deleting it.

## When Comments ARE Appropriate to Remove

- Comment states the obvious: `// Increment counter` above `counter++`.
- Comment is factually wrong and misleading.
- Code being commented is deleted entirely.
- Comment is outdated TODO that's been completed.
- Comment duplicates what code clearly expresses.

## TerraX-Specific Guidelines

Per [CLAUDE.md](../../CLAUDE.md):

> **Comment Preservation (MANDATORY)**
>
> **NEVER delete existing comments without a very strong reason.** Comments document why/how/what/where.

In TerraX, this is a **MANDATORY** standard. Comments are documentation, not clutter. Preserve them.
