# ADR-0002: Navigator Pattern for Business Logic

**Status**: Accepted

**Date**: 2025-12-27

**Deciders**: TerraX Core Team

## Context

TerraX's core functionality involves complex tree navigation logic:

- Traversing hierarchical directory structures.
- Propagating selections through tree levels.
- Generating breadcrumbs from navigation state.
- Resolving paths from selected nodes.

This business logic must be:

- Independent of the UI framework (Bubble Tea).
- Thoroughly testable without UI dependencies.
- Reusable across different interfaces (TUI, CLI, API).
- Maintainable and well-encapsulated.

The question is: where should this logic live and how should it be structured?

## Decision

We implement the **Navigator Pattern**: a dedicated `Navigator` type in the `internal/stack` package that encapsulates all tree navigation business logic.

### Structure

```go
// Navigator provides tree navigation operations.
type Navigator struct {
    root     *Node
    maxDepth int
}

// NavigationState tracks current navigation position.
type NavigationState struct {
    SelectedIndices []int
    BreadcrumbPath  []string
}

// Core operations
func (n *Navigator) GetChildrenAtDepth(state *NavigationState, depth int) []*Node
func (n *Navigator) PropagateSelection(state *NavigationState)
func (n *Navigator) GenerateBreadcrumbs(state *NavigationState) []string
func (n *Navigator) GetSelectedPath(state *NavigationState) string
```

### Responsibilities

**Navigator owns**:
- Tree traversal algorithms.
- Selection propagation logic.
- Breadcrumb generation.
- Path resolution.

**Navigator does NOT own**:
- UI state (focus, offsets, dimensions).
- Rendering logic.
- User input handling.
- Bubble Tea message processing.

## Consequences

### Positive

- **Zero UI coupling**: Navigator has zero Bubble Tea dependencies.
- **Testability**: Business logic tested with simple unit tests, no UI mocking needed.
- **Reusability**: Same Navigator can power TUI, CLI commands, or future web API.
- **Clear contracts**: Well-defined interfaces make behavior predictable.
- **Maintainability**: Business logic changes isolated from UI changes.
- **Performance**: Navigation operations optimized independently of rendering.

### Negative

- **Indirection**: UI must delegate to Navigator, adding a layer of calls.
- **State synchronization**: NavigationState must be kept in sync with UI state.
- **Additional types**: Requires separate NavigationState type alongside Model.
- **Learning curve**: Contributors must understand the delegation pattern.

## Alternatives Considered

### Option 1: Business Logic in Bubble Tea Model

**Description**: Implement navigation logic directly in `internal/tui/model.go`.

**Pros**:
- Fewer files and types.
- Direct access to UI state.
- Less indirection.

**Cons**:
- Violates separation of concerns.
- Impossible to test without Bubble Tea.
- Cannot reuse logic for CLI or API.
- Mixing UI framework with business logic creates tight coupling.

**Why rejected**: Tight coupling to Bubble Tea makes testing difficult and prevents reuse. Violates core architectural principle of separation of concerns.

### Option 2: Separate Navigation Functions (Non-OOP)

**Description**: Implement navigation as package-level functions in `internal/stack`.

**Pros**:
- Simple, functional approach.
- No state in Navigator struct.
- Easy to understand for small functions.

**Cons**:
- Tree and maxDepth must be passed to every function.
- No clear ownership or encapsulation.
- Harder to manage related state (caching, memoization).
- Doesn't scale well as complexity grows.

**Why rejected**: Package-level functions don't provide good encapsulation for complex stateful operations. Navigator pattern provides clearer ownership and better scales with growing complexity.

### Option 3: Service Layer Pattern

**Description**: Create a `NavigationService` with dependency injection.

**Pros**:
- Flexible dependency management.
- Easily mockable for testing.
- Common pattern in larger applications.

**Cons**:
- Over-engineered for current needs.
- Additional complexity with DI container.
- More boilerplate for minimal benefit.

**Why rejected**: TerraX is a CLI tool, not a web service. The simpler Navigator pattern provides sufficient encapsulation without DI overhead.

## Implementation Guidelines

### Do

- Keep all tree navigation logic in Navigator.
- Pass NavigationState explicitly to Navigator methods.
- Write unit tests for Navigator without UI dependencies.
- Use Navigator as the single source of truth for navigation operations.

### Don't

- Put business logic in Bubble Tea Update methods.
- Access tree structure directly from UI code.
- Mix rendering logic with Navigator methods.
- Create tight coupling between Navigator and UI state.

## References

- Implementation: [internal/stack/navigator.go](../../internal/stack/navigator.go)
- Usage: [internal/tui/model.go](../../internal/tui/model.go)
- Tests: [internal/stack/navigator_test.go](../../internal/stack/navigator_test.go)
- Related: [ADR-0001: Bubble Tea Architecture](0001-bubble-tea-architecture.md)
- Related: [ADR-0004: Separation of Concerns](0004-separation-of-concerns.md)
