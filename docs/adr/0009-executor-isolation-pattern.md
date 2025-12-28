# ADR-0009: Executor Isolation Pattern

**Status**: Accepted

**Date**: 2025-12-28

**Deciders**: TerraX Core Team

**Related**:
- [ADR-0004: Separation of Concerns](0004-separation-of-concerns.md)
- [ADR-0006: Execution History Management](0006-execution-history-management.md)

## Context

Executing Terragrunt commands involves complex interactions: constructing arguments based on config, streaming standard I/O to the user, capturing exit codes, measuring duration, and logging to history.

Embedding this logic directly into the UI (TUI) layer or the CLI command layer (Cobra) makes the system hard to test and violates the Single Responsibility Principle.

## Decision

We will implement a dedicated **Executor** package (`internal/executor`) using **Dependency Injection**.

The solution includes:
1.  **Executor Struct**: Encapsulates all execution logic (running the command, timing, I/O streaming).
2.  **Interface Dependency**: The Executor will depend on a `HistoryLogger` interface, not the concrete History service.
3.  **Non-Blocking operations**: Secondary concerns (like logging to history) are handled in a way that does not interrupt the primary command execution flow.
4.  **Cancellable Contexts**: All executions will accept a `context.Context` to support cancellation (e.g., user pressing Ctrl+C).

## Consequences

### Positive
*   **Testability**: By injecting the `HistoryLogger` interface, we can unit test the Executor with mocks without writing files or depending on the full history subsystem.
*   **Reusability**: The executor can be used by the TUI, the CLI "last run" command, or any future interface (e.g., a web server).
*   **Robustness**: Isolating the complexity of `os/exec` and signal handling prevents leakage into the UI code.

### Negative
*   **Indirection**: Adds a layer of abstraction. Simple "run command" logic is now split across interfaces and structs.
*   **Boilerplate**: Requires defining interfaces and mock implementations for testing.

## Alternatives Considered

### Option 1: Execute in TUI

**Description**: Put execution logic directly inside the Bubble Tea model's `Update` loop.

**Pros**:

- Simpler code structure (fewer packages to manage).
- Direct access to UI state elements.

**Cons**:

- Violates separation of concerns (UI shouldn't know about `os/exec`).
- Makes TUI logic messy and hard to read.
- Prevents reusing the execution logic in a CLI-only mode.

**Why rejected**: Poor architectural hygiene and makes testing the UI logic much harder.

### Option 2: Execute in CMD Package

**Description**: Place the execution logic in `cmd/root.go` or a similar CLI entry point.

**Pros**:

- Close to the CLI flag parsing logic.
- "Simple" to just write the code where the command starts.

**Cons**:

- The `cmd` package is intended for wiring and flag parsing, not core business logic.
- Harder to unit test due to coupling with Cobra command structures.

**Why rejected**: Business logic should live in `internal/`, keeping the CLI layer thin.

### Option 3: Global Dependency

**Description**: Use a global `history.DefaultService` singleton inside the executor instead of dependency injection.

**Pros**:

- Easy access to services without passing arguments.
- Less boilerplate code.

**Cons**:

- Extremely tight coupling.
- Makes unit testing impossible without side effects (writing to real history files during tests).

**Why rejected**: Dependency Injection is critical for writing safe, isolated unit tests with mocks.

## Future Enhancements

**Potential Improvements**:
1.  **Output Capture**: Optionally capture stdout/stderr to a buffer for parsing (e.g., to display a summary of the plan in the TUI after execution).
2.  **Dry Run**: Add a mode to print the generated Terragrunt command without executing it, for debugging.
3.  **Retry Logic**: Add automatic retries for transient failures (e.g., network locks).

## References

- **Dependency Injection**: "Dependency Injection in .NET" by Mark Seemann
- **Single Responsibility Principle**: "Clean Architecture" by Robert C. Martin
- **os/exec Package**: https://pkg.go.dev/os/exec
