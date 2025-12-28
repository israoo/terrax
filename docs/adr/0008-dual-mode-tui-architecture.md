# ADR-0008: Dual-Mode TUI Architecture

**Status**: Accepted

**Date**: 2025-12-28

**Deciders**: TerraX Core Team

**Related**:
- [ADR-0001: Bubble Tea Architecture](0001-bubble-tea-architecture.md)
- [ADR-0006: Execution History Management](0006-execution-history-management.md)

## Context

TerraX has two distinct primary workflows: **Navigation** (exploring the tree) and **History** (viewing past executions). While they share some UI elements (lists, footer), their input handling, data models, and rendering logic are distinct.

We need a way to manage these modes within a single standard Bubble Tea application without creating a monolithic, unmaintainable update loop.

## Decision

We will implement a **State Machine Architecture** within the single TUI Model.

The solution includes:
1.  **AppState Enum**: An explicit enum (`StateNavigation`, `StateHistory`) tracking the active mode.
2.  **Mode Switching**: The root `Update` function switches on `AppState` and delegates the message to mode-specific handler functions (e.g., `handleNavigationUpdate`, `handleHistoryUpdate`).
3.  **View Delegation**: Similarly, the `View` function delegates rendering to mode-specific renderers.
4.  **Shared Model**: A single `Model` struct holds state for both modes, but logic is strictly separated.

## Consequences

### Positive
*   **Cohesion**: A single binary handles all functionality; no need to spawn separate processes.
*   **Separation of Concerns**: Input handling and rendering for each mode are isolated, making the code easier to reason about and test.
*   **Extensibility**: Adding a new mode (e.g., "Help" or "Config Editor") is as simple as adding a new `AppState` and corresponding handlers.

### Negative
*   **Model Size**: The main `Model` struct can grow large as it accumulates fields for multiple modes.
*   **Complexity**: The main update loop acts as a router, which adds a layer of indirection compared to a simple single-view application.

## Alternatives Considered

### Option 1: Two Separate Binaries

**Description**: Create `terrax` (navigation) and `terrax-history` (viewer) as completely separate executables.

**Pros**:

- Strict isolation of concerns at the process level.
- Smaller, more focused binaries.

**Cons**:

- Disjointed user experience (context switching feels heavy).
- Code duplication for shared elements (styles, initialization protocols).
- More complex distribution and installation.

**Why rejected**: A single, cohesive application provides a smoother, more integrated user experience.

### Option 2: Separate Model Structs

**Description**: Define distinct `NavigationModel` and `HistoryModel` types instead of one shared `Model` with state.

**Pros**:

- Stronger type safety (e.g., a History model doesn't have a `Navigator` field).

**Cons**:

- Bubble Tea requires a single initial model. Switching at runtime requires a parent "wrapper" model, which effectively recreates the state machine pattern but with more boilerplate code.

**Why rejected**: The complexity of a wrapper model outweighs the benefits of strict type segregation.

### Option 3: Unified View

**Description**: Show both the navigation tree and the history log simultaneously on the same screen (e.g., via split panes).

**Pros**:

- High information density.
- No need to switch modes.

**Cons**:

- Terminal screen space is limited.
- Cluttered UI can be overwhelming.
- Dilutes user focus from the primary task (navigation vs review).

**Why rejected**: Dedicating the full screen to the active task provides a cleaner, more focused interface.

## Future Enhancements

**Potential Improvements**:
1.  **Modal Overlays**: Add a "modal" state for things like Help, Confirmations, or Input prompts that overlay the current mode.
2.  **Breadcrumb Transitions**: Support returning to the exact previous state (including cursor position) when switching back from History to Navigation.
3.  **Config Editor Mode**: Add a third mode for modifying `.terrax.yaml` directly in the TUI.

## References

- **State Machine Pattern**: "Design Patterns: Elements of Reusable Object-Oriented Software" by Gang of Four
- **Bubble Tea Documentation**: https://github.com/charmbracelet/bubbletea
- **Elm Architecture**: https://guide.elm-lang.org/architecture/
