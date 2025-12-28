# Pitfall: Mixing Business Logic with UI

**Category**: Architecture

**Severity**: Critical

**Date Identified**: 2025-12-27

## Description

Implementing business logic (tree traversal, selection propagation, breadcrumb generation) directly in Bubble Tea's `Model` or `Update` methods instead of in the dedicated `Navigator` component.

## Impact

This architectural violation creates severe problems:

- **Untestable code**: Business logic coupled to Bubble Tea cannot be unit tested without complex UI mocking.
- **No reusability**: Logic tied to TUI cannot be used for CLI commands or future API endpoints.
- **Maintenance nightmare**: Changes require understanding both business logic and UI framework simultaneously.
- **Difficult debugging**: Mixed concerns make it hard to isolate where bugs originate.
- **Architecture erosion**: Once pattern is broken, it degrades rapidly as others follow the precedent.
- **Performance issues**: UI framework overhead added to pure computation.

## Root Cause

Common reasons this happens:

1. **Convenience**: "It's faster to just add it here in Update."
2. **Unfamiliarity**: New contributor doesn't understand Navigator pattern.
3. **Time pressure**: "We'll refactor it later" (spoiler: we won't).
4. **Unclear boundaries**: Documentation doesn't make separation obvious enough.
5. **Small changes**: "It's just one line" gradually accumulates into large violations.

## How to Avoid

### Do

- **Always check first**: Before adding logic to `model.go`, ask "Is this business logic or UI state?"
- **Use Navigator**: Delegate all tree operations, selection, and path resolution to `Navigator`.
- **Think reusability**: If this logic could be useful in a CLI command, it belongs in `internal/stack/`.
- **Follow the pattern**: Look at existing code to see how navigation is delegated.
- **Write tests first**: If you can't unit test it without Bubble Tea, it's in the wrong place.

### Don't

- **Don't put tree traversal in Update methods**: This is business logic.
- **Don't manipulate tree structure in Model**: Model should only track UI state.
- **Don't generate breadcrumbs in View**: Delegate to `Navigator.GenerateBreadcrumbs()`.
- **Don't calculate paths in rendering code**: Use `Navigator.GetSelectedPath()`.
- **Don't rationalize**: "It's just one method" becomes "it's just ten methods."

## Detection

Watch for these warning signs:

- **Imports**: `internal/tui/` files importing filesystem or path manipulation packages.
- **Method names**: Methods like `(m Model) traverseTree()` or `(m Model) findNode()`.
- **Complexity**: `Update()` methods longer than 50 lines often contain business logic.
- **Loops over tree**: Iteration over `Node` children in `model.go` or `view.go`.
- **Tests**: Inability to test navigation logic without instantiating Bubble Tea model.

## Remediation

If you've already mixed concerns, here's how to fix it:

1. **Identify the business logic**: Find code that operates on tree structure, not UI state.

2. **Extract to Navigator**: Move logic to appropriate method in `internal/stack/navigator.go`.

   ```go
   // Navigator method (business logic)
   func (n *Navigator) GetChildrenAtDepth(state *NavigationState, depth int) []*Node {
       // Pure business logic here
   }
   ```

3. **Update Model to delegate**: Replace direct logic with Navigator call.

   ```go
   // In Update method
   children := m.navigator.GetChildrenAtDepth(&m.state, depth)
   ```

4. **Write unit tests**: Test extracted Navigator method independently.

   ```go
   func TestNavigator_GetChildrenAtDepth(t *testing.T) {
       // No Bubble Tea dependencies needed
   }
   ```

5. **Verify**: Ensure TUI still works and Navigator can be tested independently.

## Related

- [ADR-0002: Navigator Pattern](../../adr/0002-navigator-pattern.md)
- [ADR-0004: Separation of Concerns](../../adr/0004-separation-of-concerns.md)
- [Standard: File Organization](../../standards/file-organization.md)

## Examples

### Bad: Business Logic in Model

```go
// internal/tui/model.go
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
    switch msg := msg.(type) {
    case tea.KeyMsg:
        if msg.String() == "enter" {
            // WRONG: Business logic in Update
            current := m.root
            for i, idx := range m.selectedIndices {
                if i < len(current.Children) {
                    current = current.Children[idx]
                }
            }
            m.selectedPath = current.Path
        }
    }
    return m, nil
}
```

**Problems**:
- Tree traversal logic in UI layer.
- Cannot test without Bubble Tea.
- Cannot reuse for CLI or API.
- Difficult to maintain.

### Good: Business Logic in Navigator

```go
// internal/stack/navigator.go
func (n *Navigator) GetSelectedPath(state *NavigationState) string {
    current := n.root
    for i, idx := range state.SelectedIndices {
        if i < len(current.Children) && idx < len(current.Children) {
            current = current.Children[idx]
        }
    }
    return current.Path
}

// internal/tui/model.go
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
    switch msg := msg.(type) {
    case tea.KeyMsg:
        if msg.String() == "enter" {
            // CORRECT: Delegate to Navigator
            m.selectedPath = m.navigator.GetSelectedPath(&m.state)
        }
    }
    return m, nil
}
```

**Benefits**:
- Clear separation of concerns.
- Navigator testable independently.
- Reusable across interfaces.
- Easy to maintain and extend.

### Bad: Breadcrumb Generation in View

```go
// internal/tui/view.go
func (m Model) View() string {
    // WRONG: Business logic in rendering
    var breadcrumbs []string
    current := m.root
    for _, idx := range m.selectedIndices {
        current = current.Children[idx]
        breadcrumbs = append(breadcrumbs, current.Name)
    }
    breadcrumbStr := strings.Join(breadcrumbs, " > ")
    // ... rendering
}
```

### Good: Breadcrumb Generation Delegated

```go
// internal/stack/navigator.go
func (n *Navigator) GenerateBreadcrumbs(state *NavigationState) []string {
    var breadcrumbs []string
    current := n.root
    for _, idx := range state.SelectedIndices {
        if idx < len(current.Children) {
            current = current.Children[idx]
            breadcrumbs = append(breadcrumbs, current.Name)
        }
    }
    return breadcrumbs
}

// internal/tui/view.go
func (m Model) View() string {
    // CORRECT: Use pre-computed breadcrumbs
    breadcrumbStr := strings.Join(m.breadcrumbs, " > ")
    // ... rendering
}
```

## Testing

To verify proper separation:

```bash
# Navigator should have no Bubble Tea dependencies
grep -r "bubbletea" internal/stack/
# Should return nothing

# Navigator tests should not import Bubble Tea
grep -r "tea" internal/stack/*_test.go
# Should return nothing

# Business logic should be testable
go test ./internal/stack/... -v
# Should run without UI dependencies
```
