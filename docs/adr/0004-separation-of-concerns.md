# ADR-0004: Separation of Concerns

**Status**: Accepted

**Date**: 2025-12-27

**Deciders**: TerraX Core Team

## Context

TerraX is a complex application with multiple responsibilities:

- **Business logic**: Tree scanning, navigation, selection propagation.
- **UI state management**: Focus, offsets, selections, dimensions.
- **Rendering**: Layout calculation, styling, terminal output.
- **CLI coordination**: Argument parsing, initialization, error handling.

Without clear boundaries, these concerns can become intertwined, leading to:

- Untestable code (business logic coupled to UI framework).
- Difficult maintenance (changes ripple across layers).
- Poor reusability (logic tied to specific interfaces).
- Cognitive overload (mixed responsibilities in single files).

## Decision

We enforce strict **Separation of Concerns (SoC)** with clearly defined layers and responsibilities.

### Layer Architecture

```text
terrax/
├── cmd/                    # CLI Layer (coordination only)
│   └── root.go
├── internal/
│   ├── stack/              # Business Logic Layer
│   │   ├── tree.go         # Filesystem scanning, tree building
│   │   └── navigator.go    # Navigation algorithms
│   └── tui/                # Presentation Layer
│       ├── model.go        # UI state (delegates to Navigator)
│       ├── view.go         # Rendering (LayoutCalculator, Renderer)
│       └── constants.go    # UI configuration
└── main.go                 # Entry point
```

### Layer Responsibilities

#### 1. CLI Layer (`cmd/`)

**Owns**:
- Command-line argument parsing.
- Application initialization coordination.
- Error handling at boundaries.

**Does NOT own**:
- Business logic.
- UI state management.
- Rendering logic.

**Dependencies**: Can depend on `internal/stack` and `internal/tui`, but should be thin glue code.

#### 2. Business Logic Layer (`internal/stack/`)

**Owns**:
- Tree structure and filesystem scanning.
- Navigation algorithms (traversal, selection, breadcrumbs).
- Path resolution and validation.
- Domain models (`Node`, `NavigationState`).

**Does NOT own**:
- UI state (focus, offsets, dimensions).
- Rendering logic or styling.
- User input handling.
- Framework-specific code.

**Dependencies**: **ZERO** dependencies on Bubble Tea or any UI framework.

#### 3. Presentation Layer (`internal/tui/`)

**Owns**:
- UI state management (Bubble Tea `Model`).
- User input handling (Bubble Tea `Update`).
- Rendering logic (Bubble Tea `View`).
- Layout calculations (`LayoutCalculator`).
- Styling (`Renderer`, Lipgloss).

**Does NOT own**:
- Business logic (delegates to `Navigator`).
- Tree traversal algorithms.
- Filesystem operations.

**Dependencies**: Depends on `internal/stack` for business logic, Bubble Tea for UI framework, Lipgloss for styling.

### Communication Patterns

```
User Input → TUI (Update) → Navigator (Business Logic) → TUI (Model Update) → TUI (View) → Terminal Output
```

1. User presses key → Bubble Tea sends `KeyMsg` to `Update`.
2. `Update` calls `Navigator` method with current `NavigationState`.
3. `Navigator` performs business logic, returns result.
4. `Update` updates `Model` state based on result.
5. `View` reads `Model` state and renders UI via `Renderer`.

### Rendering Separation

Within the presentation layer, further separation:

- **Model** (`model.go`): State only, no rendering code.
- **LayoutCalculator** (`view.go`): Pure layout math (column positions, visible ranges).
- **Renderer** (`view.go`): Styled rendering with Lipgloss.
- **View** (`model.go`): Orchestrates `LayoutCalculator` and `Renderer`.

## Consequences

### Positive

- **Testability**: Business logic tested independently without UI mocking.
- **Maintainability**: Changes isolated to relevant layers.
- **Reusability**: Navigator can power different interfaces (TUI, CLI, API).
- **Clarity**: Clear responsibilities reduce cognitive load.
- **Scalability**: Easy to add new features or layers.
- **Parallel development**: Team members can work on different layers independently.

### Negative

- **Indirection**: More layers means more files and function calls.
- **Boilerplate**: Clear separation requires additional types and interfaces.
- **Learning curve**: Contributors must understand layer boundaries.
- **Initial overhead**: Setting up proper separation takes more upfront time.

## Alternatives Considered

### Option 1: Monolithic Model (All Logic in TUI)

**Description**: Implement all logic directly in Bubble Tea Model and Update.

**Pros**:
- Fewer files and types.
- Direct implementation, no indirection.
- Faster initial development.

**Cons**:
- Impossible to test without Bubble Tea.
- Cannot reuse logic for CLI or API.
- Model becomes massive and unmaintainable.
- Violates single responsibility principle.

**Why rejected**: Creates technical debt from day one. Business logic coupled to UI framework prevents testing and reuse.

### Option 2: Shared Logic Package (No Clear Layers)

**Description**: Put shared code in `internal/common` or `internal/util` without layer discipline.

**Pros**:
- Flexible, no strict rules.
- Easy to move code around.
- Less upfront planning.

**Cons**:
- "Common" packages become dumping grounds.
- No clear ownership or boundaries.
- Responsibilities blur over time.
- Difficult to enforce patterns.

**Why rejected**: Lack of discipline leads to architecture erosion. Clear layers prevent gradual degradation.

### Option 3: Hexagonal Architecture (Ports & Adapters)

**Description**: Implement formal ports/adapters pattern with interfaces for all dependencies.

**Pros**:
- Maximum decoupling and testability.
- Clear dependency inversion.
- Highly flexible for different implementations.

**Cons**:
- Over-engineered for TerraX's scope.
- Excessive interfaces and boilerplate.
- Harder for contributors to understand.

**Why rejected**: TerraX is a CLI tool, not a complex enterprise application. Current SoC pattern provides sufficient decoupling without over-engineering.

## Enforcement

### Code Review Checklist

- [ ] Business logic in `internal/stack/`, not in `internal/tui/`.
- [ ] `internal/stack/` has zero Bubble Tea imports.
- [ ] `internal/tui/model.go` delegates to Navigator for business logic.
- [ ] Rendering logic in `view.go`, not in `model.go`.
- [ ] `cmd/` files are thin coordination code only.

### Violations to Watch For

**Red flags**:
- `import tea "github.com/charmbracelet/bubbletea"` in `internal/stack/`.
- Navigation logic in `Update` methods instead of `Navigator`.
- Lipgloss styles in `model.go`.
- Business logic in `cmd/root.go`.

### Testing Strategy

- **`internal/stack/`**: Unit tests with no UI dependencies.
- **`internal/tui/`**: UI logic tests with mocked Navigator (if needed).
- **Integration**: End-to-end tests with real Navigator and TUI.

## Migration Path

If you find violations of this architecture:

1. **Identify**: Locate business logic in presentation layer or vice versa.
2. **Extract**: Move logic to appropriate layer.
3. **Interface**: Define clear interface between layers.
4. **Test**: Add unit tests for extracted logic.
5. **Refactor**: Update calling code to use new interface.
6. **Verify**: Ensure no regression.

## References

- Implementation: [internal/stack/navigator.go](../../internal/stack/navigator.go)
- Implementation: [internal/tui/model.go](../../internal/tui/model.go)
- Implementation: [internal/tui/view.go](../../internal/tui/view.go)
- Related: [ADR-0001: Bubble Tea Architecture](0001-bubble-tea-architecture.md)
- Related: [ADR-0002: Navigator Pattern](0002-navigator-pattern.md)
- Pattern: [Separation of Concerns](https://en.wikipedia.org/wiki/Separation_of_concerns)
