# Pitfall: Tight Coupling Between Layers

**Category**: Architecture

**Severity**: High

**Date Identified**: 2025-12-27

## Description

Creating direct dependencies between architectural layers (CLI → TUI → Business Logic) instead of maintaining clear interfaces and separation, making the codebase rigid, difficult to test, and impossible to reuse components independently.

## Impact

Tight coupling creates severe architectural problems:

- **Untestable code**: Cannot test business logic without instantiating entire UI stack.
- **No component reuse**: Business logic locked to specific UI framework.
- **Difficult refactoring**: Changes ripple across layers uncontrollably.
- **Framework lock-in**: Impossible to switch UI frameworks without rewriting everything.
- **Poor maintainability**: Understanding one component requires understanding all layers.
- **Slow development**: Every feature change touches multiple tightly-coupled files.
- **Fragile codebase**: Small changes break distant, seemingly unrelated code.

## Root Cause

Common reasons tight coupling happens:

1. **Convenience**: "It's faster to just call this directly."
2. **Lack of planning**: Not designing interfaces upfront.
3. **Poor understanding**: Not recognizing where layer boundaries should be.
4. **Time pressure**: "We'll refactor it later" (never happens).
5. **Gradual erosion**: Small violations accumulate into major coupling.
6. **Shared state**: Global variables or singletons create hidden dependencies.

## How to Avoid

### Do

- **Define clear interfaces**: Establish contracts between layers.
- **Use dependency injection**: Pass dependencies explicitly, not via globals.
- **Maintain layer hierarchy**: CLI → TUI → Business Logic, never reverse.
- **Keep Navigator pure**: Zero UI framework imports in `internal/stack/`.
- **Pass data, not objects**: Use simple data structures between layers.
- **Test in isolation**: Each layer should be testable independently.

### Don't

- **Don't import up the stack**: Business logic should never import TUI.
- **Don't share UI types**: Don't pass Bubble Tea types to Navigator.
- **Don't use global state**: Avoid package-level variables that couple layers.
- **Don't mix concerns**: Keep presentation and business logic separate.
- **Don't bypass interfaces**: Always go through defined boundaries.

## Detection

Warning signs of tight coupling:

- **Circular imports**: `internal/stack/` imports `internal/tui/` or vice versa.
- **Framework imports**: Bubble Tea imports in business logic layer.
- **Shared mutable state**: Global variables accessed by multiple layers.
- **Can't test alone**: Navigator requires TUI to be instantiated for testing.
- **Cascading changes**: Small change in one layer breaks multiple others.
- **Deep import chains**: File imports transitively pull in entire framework.

### Code Smells

```go
// BAD: Business logic importing UI
package stack

import (
    tea "github.com/charmbracelet/bubbletea"  // ❌ Wrong direction
    "github.com/israoo/terrax/internal/tui"   // ❌ Importing up
)

// BAD: Passing UI types to business logic
func (n *Navigator) ProcessMessage(msg tea.Msg) {  // ❌ UI type in business layer
    // ...
}

// BAD: Shared global state
package common

var GlobalNavigator *Navigator  // ❌ Creates hidden coupling
```

## Remediation

If you've created tight coupling, here's how to fix it:

### 1. Identify Dependencies

Map out what depends on what:

```bash
# Find imports between layers
grep -r "github.com/israoo/terrax/internal/tui" internal/stack/
# Should return NOTHING

grep -r "bubbletea" internal/stack/
# Should return NOTHING
```

### 2. Extract Interfaces

Define clean contracts between layers:

```go
// internal/stack/navigator.go
// Pure business logic, no UI dependencies

type Navigator struct {
    root     *Node
    maxDepth int
}

// NavigationState is a pure data structure
type NavigationState struct {
    SelectedIndices []int
    BreadcrumbPath  []string
}

func (n *Navigator) GetChildrenAtDepth(state *NavigationState, depth int) []*Node {
    // Pure business logic
}
```

### 3. Use Dependency Injection

Pass dependencies explicitly:

```go
// internal/tui/model.go
type Model struct {
    navigator *stack.Navigator  // Injected, not global
    state     *stack.NavigationState
}

func NewModel(rootPath string) Model {
    tree, _ := stack.BuildTree(rootPath)
    nav := stack.NewNavigator(tree, 5)

    return Model{
        navigator: nav,
        state:     &stack.NavigationState{},
    }
}
```

### 4. Remove Framework Dependencies

Clean business logic of UI framework:

```go
// BEFORE (coupled)
func (n *Navigator) HandleKeyPress(msg tea.KeyMsg) tea.Cmd {
    // Business logic mixed with UI framework
}

// AFTER (decoupled)
// Business logic layer
func (n *Navigator) MoveSelection(direction string, state *NavigationState) {
    // Pure logic, no UI types
}

// UI layer
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
    switch msg := msg.(type) {
    case tea.KeyMsg:
        switch msg.String() {
        case "right":
            m.navigator.MoveSelection("right", &m.state)
        }
    }
    return m, nil
}
```

### 5. Test Independently

Verify each layer works in isolation:

```go
// internal/stack/navigator_test.go
func TestNavigator_MoveSelection(t *testing.T) {
    // Test business logic without ANY UI framework
    nav := NewNavigator(createTestTree(), 5)
    state := &NavigationState{SelectedIndices: []int{0}}

    nav.MoveSelection("right", state)

    // Assertions on pure data
    if len(state.SelectedIndices) != 2 {
        t.Error("Selection not propagated")
    }
}
```

## Related

- [ADR-0002: Navigator Pattern](../../adr/0002-navigator-pattern.md)
- [ADR-0004: Separation of Concerns](../../adr/0004-separation-of-concerns.md)
- [Pitfall: Mixing Business Logic with UI](mixing-business-logic-ui.md)
- [Standard: File Organization](../../standards/file-organization.md)

## Examples

### Bad: Tight Coupling

```go
// internal/stack/navigator.go
package stack

import (
    tea "github.com/charmbracelet/bubbletea"  // ❌ Framework coupling
    "github.com/israoo/terrax/internal/tui"   // ❌ Upward dependency
)

// ❌ Navigator coupled to Bubble Tea
func (n *Navigator) Update(msg tea.Msg) tea.Cmd {
    switch msg := msg.(type) {
    case tea.KeyMsg:
        // Business logic mixed with UI message handling
    }
    return nil
}

// ❌ Returning UI-specific types
func (n *Navigator) GetView() string {
    // Business logic generating UI strings
    return "rendered view"
}
```

**Problems**:
- Business logic depends on UI framework
- Cannot test Navigator without Bubble Tea
- Cannot reuse Navigator in different UI (CLI, web API)
- Framework upgrade requires changing business logic

### Good: Loose Coupling

```go
// internal/stack/navigator.go
package stack

// ✅ Zero UI framework imports
// import (
//     "path/filepath"
// )

// ✅ Pure data structures
type NavigationState struct {
    SelectedIndices []int
    BreadcrumbPath  []string
}

// ✅ Pure business logic methods
func (n *Navigator) MoveSelection(direction string, state *NavigationState) {
    switch direction {
    case "right":
        n.moveRight(state)
    case "left":
        n.moveLeft(state)
    }
}

func (n *Navigator) GetChildrenAtDepth(state *NavigationState, depth int) []*Node {
    // Pure tree traversal logic
    return children
}

// internal/tui/model.go
package tui

import (
    tea "github.com/charmbracelet/bubbletea"
    "github.com/israoo/terrax/internal/stack"
)

// ✅ UI layer depends on business layer (correct direction)
type Model struct {
    navigator *stack.Navigator
    state     *stack.NavigationState
}

// ✅ UI layer handles framework-specific code
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
    switch msg := msg.(type) {
    case tea.KeyMsg:
        // Translate UI events to business operations
        switch msg.String() {
        case "right":
            m.navigator.MoveSelection("right", &m.state)
        }
    }
    return m, nil
}
```

**Benefits**:
- Business logic has zero UI dependencies
- Navigator testable with simple unit tests
- Can reuse Navigator for CLI, web API, different TUI
- Framework changes don't affect business logic

### Bad: Shared Global State

```go
// internal/common/globals.go
package common

// ❌ Global state creates hidden coupling
var (
    CurrentNavigator *stack.Navigator
    AppConfig        *Config
)

// internal/stack/navigator.go
func DoSomething() {
    // ❌ Depends on global state
    nav := common.CurrentNavigator
}

// internal/tui/model.go
func (m Model) Init() tea.Cmd {
    // ❌ Modifies global state
    common.CurrentNavigator = m.navigator
    return nil
}
```

**Problems**:
- Hidden dependencies make testing impossible
- Race conditions in concurrent code
- Unclear initialization order
- Cannot run multiple instances

### Good: Dependency Injection

```go
// internal/tui/model.go
type Model struct {
    navigator *stack.Navigator  // ✅ Explicit dependency
    config    Config             // ✅ Local state
}

// ✅ Dependencies passed explicitly
func NewModel(rootPath string, cfg Config) Model {
    tree, _ := stack.BuildTree(rootPath)
    nav := stack.NewNavigator(tree, 5)

    return Model{
        navigator: nav,
        config:    cfg,
    }
}

// internal/stack/navigator.go
// ✅ No hidden dependencies
func (n *Navigator) Traverse(state *NavigationState) {
    // Only uses explicitly passed state
}
```

**Benefits**:
- All dependencies explicit and visible
- Easy to test with mock dependencies
- No hidden global state
- Safe for concurrent use

## Testing for Coupling

### Dependency Analysis

```bash
# Check for upward imports (should be none)
grep -r "internal/tui" internal/stack/
# Expected: No results

# Check for framework imports in business logic
grep -r "bubbletea" internal/stack/
# Expected: No results

# Check for circular dependencies
go mod graph | grep -E "(internal/stack.*internal/tui|internal/tui.*internal/stack)"
# Expected: No results
```

### Test Independence

```go
// Business logic should test without UI
func TestNavigator_WithoutUI(t *testing.T) {
    // ✅ No Bubble Tea imports needed
    nav := stack.NewNavigator(createTestTree(), 5)
    state := &stack.NavigationState{}

    nav.MoveSelection("right", state)

    // Test pure business logic
}

// UI tests should mock business logic if needed
func TestModel_Update(t *testing.T) {
    // Can test UI in isolation with mock Navigator if needed
}
```

## Architecture Principles

### Correct Layer Dependencies

```
┌─────────────┐
│     CLI     │  (cmd/)
└──────┬──────┘
       │ depends on
       ↓
┌─────────────┐
│     TUI     │  (internal/tui/)
└──────┬──────┘
       │ depends on
       ↓
┌─────────────┐
│   Business  │  (internal/stack/)
│    Logic    │  ✅ Zero dependencies on layers above
└─────────────┘
```

### Wrong Dependencies (Avoid)

```
┌─────────────┐
│     CLI     │
└──────┬──────┘
       │
       ↓
┌─────────────┐      ❌ WRONG
│     TUI     │ ←──────────┐
└──────┬──────┘            │
       │                   │
       ↓                   │
┌─────────────┐            │
│   Business  │────────────┘
│    Logic    │  Creates circular dependency
└─────────────┘
```

## Checklist: Avoiding Coupling

- [ ] Business logic has zero UI framework imports
- [ ] `internal/stack/` does not import `internal/tui/`
- [ ] Navigator methods use only pure data types
- [ ] No global state shared between layers
- [ ] Each layer testable independently
- [ ] Dependencies flow downward only (CLI → TUI → Business)
- [ ] Interfaces define clean boundaries
- [ ] No framework types in business logic signatures

## TerraX-Specific Rules

Per [ADR-0004: Separation of Concerns](../../adr/0004-separation-of-concerns.md):

> **Business Logic** (`internal/stack/navigator.go`):
> - **ZERO Bubble Tea dependencies**
> - Tree traversal algorithms, selection propagation, breadcrumb generation
> - Testable without any framework

This is a **MANDATORY** architectural principle. Any violation should be treated as a critical bug and fixed immediately.
