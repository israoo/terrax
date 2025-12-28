# ADR-0001: Bubble Tea Architecture

**Status**: Accepted

**Date**: 2025-12-27

**Deciders**: TerraX Core Team

## Context

TerraX requires a robust Terminal UI (TUI) framework to provide interactive, hierarchical navigation of Terragrunt stacks. The UI must be:

- Responsive and performant in terminal environments.
- Maintainable and testable.
- Cross-platform (Linux, macOS, Windows).
- Capable of handling complex state management and rendering.

Several TUI frameworks exist in the Go ecosystem, each with different architectural approaches and trade-offs.

## Decision

We adopted **Bubble Tea** as our TUI framework, implementing the strict **Elm Architecture** pattern with Model-Update-View separation.

### Core Principles

1. **Model**: Holds UI state only (selections, focus, offsets, dimensions).
2. **Update**: Processes messages (key presses, window resize) through pure functions.
3. **View**: Renders UI from model state without side effects.

### Implementation

- Model delegates business logic to `stack.Navigator`.
- Update methods are pure functions returning updated model and optional commands.
- View is composed of pure rendering functions using Lipgloss for styling.

## Consequences

### Positive

- **Clear separation of concerns**: UI state, business logic, and rendering are isolated.
- **Testability**: Business logic can be tested independently of UI framework.
- **Predictability**: Unidirectional data flow makes state changes traceable.
- **Maintainability**: Strict patterns reduce cognitive load and prevent anti-patterns.
- **Community support**: Active ecosystem with good documentation and examples.
- **Performance**: Efficient rendering with minimal re-renders.

### Negative

- **Learning curve**: Team must understand Elm Architecture concepts.
- **Boilerplate**: Strict separation requires more files and types.
- **Framework lock-in**: Deep integration makes switching frameworks costly.
- **Debugging complexity**: Message-based flow can be harder to debug than imperative code.

## Alternatives Considered

### Option 1: tview

**Description**: High-level TUI framework with pre-built components.

**Pros**:
- Rich set of built-in widgets (tables, trees, forms).
- Less boilerplate for simple UIs.
- Easier to get started quickly.

**Cons**:
- Less control over rendering and styling.
- Harder to implement custom navigation patterns (sliding window).
- More difficult to test business logic separately.
- Less flexible for complex state management.

**Why rejected**: TerraX requires custom navigation patterns and strict separation of concerns that tview's widget-based approach doesn't facilitate well.

### Option 2: termui

**Description**: Lower-level TUI library focused on rendering.

**Pros**:
- Maximum control over rendering.
- Minimal abstraction overhead.
- Good for dashboard-style UIs.

**Cons**:
- No built-in state management pattern.
- More imperative code, harder to test.
- Less suitable for interactive navigation.
- Smaller community and ecosystem.

**Why rejected**: Lack of structured state management would lead to complex, hard-to-test code. TerraX benefits from Bubble Tea's architectural discipline.

### Option 3: Custom TUI with tcell

**Description**: Build custom TUI directly on tcell terminal library.

**Pros**:
- Complete control and flexibility.
- No framework opinions or constraints.
- Minimal dependencies.

**Cons**:
- Significant development effort to build state management.
- High risk of architectural mistakes.
- No community patterns or best practices.
- Longer time to production.

**Why rejected**: Building a robust, testable architecture from scratch would take significant time and likely result in reinventing patterns that Bubble Tea already provides.

## References

- [Bubble Tea Documentation](https://github.com/charmbracelet/bubbletea)
- [Elm Architecture Guide](https://guide.elm-lang.org/architecture/)
- TerraX Implementation: [internal/tui/model.go](../../internal/tui/model.go)
- TerraX Implementation: [internal/tui/view.go](../../internal/tui/view.go)
