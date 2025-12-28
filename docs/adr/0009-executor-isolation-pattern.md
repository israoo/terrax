# ADR-0008: Executor Isolation Pattern with Dependency Injection

**Status**: Accepted

**Date**: 2025-12-28

**Deciders**: TerraX Core Team

**Related**:
- [ADR-0004: Separation of Concerns](0004-separation-of-concerns.md)
- [ADR-0006: Execution History Management](0006-execution-history-management.md)

## Context

TerraX needs to execute Terragrunt commands with:

1. **Proper argument construction**: Build complex terragrunt commands with multiple flags.
2. **Standard I/O passthrough**: Stream command output to user in real-time.
3. **Exit code capture**: Record success/failure for history logging.
4. **Execution timing**: Measure duration for performance tracking.
5. **History integration**: Log every execution with full context.
6. **Testability**: Must be testable without actually running terragrunt.

### Problem

Where should command execution logic live?

**Option 1**: Inside TUI (internal/tui/)
- **Problem**: Mixes UI concerns with execution logic.
- **Problem**: Can't reuse for CLI-only mode or future API.

**Option 2**: Inside History package (internal/history/)
- **Problem**: History package shouldn't know how to execute commands.
- **Problem**: Violates Single Responsibility Principle.

**Option 3**: In CMD package (cmd/)
- **Problem**: CMD should coordinate, not implement business logic.
- **Problem**: Harder to test (cobra complicates unit tests).

**Option 4**: Dedicated Executor package (internal/executor/)
- **Solution**: Single responsibility, reusable, testable.

### Requirements

- Isolated command execution logic.
- Support all Terragrunt flags from configuration.
- Stream output to user (real-time feedback).
- Capture exit codes and execution duration.
- Integrate with history logging.
- Testable via dependency injection (mock history logger).
- Cancellable via context.Context.
- Error handling with proper context wrapping.

## Decision

Implement **Executor Isolation Pattern** with:

1. **Dedicated Package**: `internal/executor/` for all execution logic.
2. **Dependency Injection**: Accept `HistoryLogger` interface, not concrete type.
3. **Command Building**: Pure function to construct terragrunt arguments.
4. **Standard I/O Passthrough**: Direct connection to os.Stdout/Stderr/Stdin.
5. **Exit Code Extraction**: Extract from exec.ExitError.
6. **Timing Instrumentation**: Measure execution duration.
7. **Non-Blocking History**: History failures don't block execution.

### Architecture

```
┌─────────────────────────────────────────────────────────────┐
│                     CMD Layer                                │
│  (Coordinates: TUI → User Selection → Executor)             │
└────────────────────────┬────────────────────────────────────┘
                         │
                         │ Calls Execute()
                         ↓
┌─────────────────────────────────────────────────────────────┐
│                  Executor Package                            │
│                                                              │
│  ┌────────────────────────────────────────────────────────┐ │
│  │              Executor Struct                           │ │
│  │                                                        │ │
│  │  - historyLogger HistoryLogger (interface)            │ │
│  │  - maxHistoryEntries int                              │ │
│  └────────────────────────────────────────────────────────┘ │
│                           ↓                                  │
│  ┌────────────────────────────────────────────────────────┐ │
│  │  Execute(ctx, command, stackPath, projectRoot)        │ │
│  │                                                        │ │
│  │  1. Build terragrunt args from command + config       │ │
│  │  2. Create exec.CommandContext                        │ │
│  │  3. Connect Stdout/Stderr/Stdin to terminal           │ │
│  │  4. Start timer                                        │ │
│  │  5. Run command (blocking, streams output)            │ │
│  │  6. Capture exit code                                 │ │
│  │  7. Measure duration                                   │ │
│  │  8. Log to history (non-blocking)                     │ │
│  │  9. Return exit code + error                          │ │
│  └────────────────────────────────────────────────────────┘ │
│                           ↓                                  │
│  ┌────────────────────────────────────────────────────────┐ │
│  │  buildTerragruntArgs(command, stackPath)              │ │
│  │                                                        │ │
│  │  Constructs: terragrunt run-all --working-dir <path>  │ │
│  │              [--log-level] [--log-format]             │ │
│  │              [--parallelism N]                         │ │
│  │              [--no-color] [--non-interactive]          │ │
│  │              [--terragrunt-ignore-dependency-errors]   │ │
│  │              [--terragrunt-ignore-external-deps]       │ │
│  │              [--terragrunt-include-external-deps]      │ │
│  │              [extra flags]                             │ │
│  │              -- <command>                              │ │
│  └────────────────────────────────────────────────────────┘ │
│                           ↓                                  │
│  ┌────────────────────────────────────────────────────────┐ │
│  │  logExecutionToHistory(entry)                         │ │
│  │                                                        │ │
│  │  - Calls historyLogger.LogExecution(entry)            │ │
│  │  - Failures logged but don't return error             │ │
│  │  - Graceful degradation                                │ │
│  └────────────────────────────────────────────────────────┘ │
└─────────────────────────────────────────────────────────────┘
                           ↓
                           │ HistoryLogger interface
                           ↓
┌─────────────────────────────────────────────────────────────┐
│                  History Service                             │
│  (Actual implementation of HistoryLogger)                   │
│                                                              │
│  func (s *Service) LogExecution(entry, maxEntries) error    │
└─────────────────────────────────────────────────────────────┘
```

### Executor Package Structure

```go
// internal/executor/executor.go
package executor

import (
    "context"
    "os/exec"
    "time"

    "github.com/israoo/terrax/internal/history"
)

// HistoryLogger defines the interface for logging command executions.
// This enables dependency injection and testing with mocks.
type HistoryLogger interface {
    LogExecution(entry history.ExecutionLogEntry, maxEntries int) error
}

// Executor executes Terragrunt commands and logs execution history.
type Executor struct {
    historyLogger     HistoryLogger
    maxHistoryEntries int
}

// NewExecutor creates a new Executor with the given history logger.
func NewExecutor(logger HistoryLogger, maxHistoryEntries int) *Executor {
    return &Executor{
        historyLogger:     logger,
        maxHistoryEntries: maxHistoryEntries,
    }
}

// Execute runs a Terragrunt command and logs the execution to history.
func (e *Executor) Execute(
    ctx context.Context,
    command string,
    stackPath string,
    projectRoot string,
) (int, error) {
    // Implementation details below
}
```

### Dependency Injection via Interface

**HistoryLogger Interface**:

```go
type HistoryLogger interface {
    LogExecution(entry history.ExecutionLogEntry, maxEntries int) error
}
```

**Benefits**:
- **Decoupling**: Executor doesn't depend on concrete history.Service.
- **Testability**: Can inject mock logger for unit tests.
- **Flexibility**: Can swap logging implementations.
- **Compiles without circular dependencies**: Executor → Interface ← History.

**Usage in Production**:

```go
// cmd/root.go
historyService := history.DefaultService
executor := executor.NewExecutor(historyService, maxHistoryEntries)
```

**Usage in Tests**:

```go
// executor_test.go
type mockHistoryLogger struct {
    loggedEntries []history.ExecutionLogEntry
}

func (m *mockHistoryLogger) LogExecution(entry history.ExecutionLogEntry, maxEntries int) error {
    m.loggedEntries = append(m.loggedEntries, entry)
    return nil
}

func TestExecutor_Execute(t *testing.T) {
    mock := &mockHistoryLogger{}
    exec := executor.NewExecutor(mock, 500)

    // Execute command
    exitCode, err := exec.Execute(ctx, "plan", "/path/to/stack", "/project")

    // Assert history was logged
    assert.Equal(t, 1, len(mock.loggedEntries))
}
```

### Command Building

Pure function to construct terragrunt arguments:

```go
// buildTerragruntArgs constructs the full argument list for terragrunt.
func buildTerragruntArgs(command, stackPath string) []string {
    args := []string{"run-all"}

    // Working directory (required)
    args = append(args, "--working-dir", stackPath)

    // Log level (from config)
    if logLevel := viper.GetString("log_level"); logLevel != "" {
        args = append(args, "--terragrunt-log-level", logLevel)
    }

    // Log format (from config)
    if logFormat := viper.GetString("log_format"); logFormat != "" {
        args = append(args, "--terragrunt-log-format", logFormat)

        // Custom format (if applicable)
        if customFormat := viper.GetString("log_custom_format"); customFormat != "" {
            args = append(args, "--terragrunt-log-custom-format", customFormat)
        }
    }

    // Parallelism (from config)
    if parallelism := viper.GetInt("terragrunt.parallelism"); parallelism > 0 {
        args = append(args, "--terragrunt-parallelism", fmt.Sprintf("%d", parallelism))
    }

    // Boolean flags (from config)
    if viper.GetBool("terragrunt.no_color") {
        args = append(args, "--terragrunt-no-color")
    }

    if viper.GetBool("terragrunt.non_interactive") {
        args = append(args, "--terragrunt-non-interactive")
    }

    if viper.GetBool("terragrunt.ignore_dependency_errors") {
        args = append(args, "--terragrunt-ignore-dependency-errors")
    }

    if viper.GetBool("terragrunt.ignore_external_dependencies") {
        args = append(args, "--terragrunt-ignore-external-dependencies")
    }

    if viper.GetBool("terragrunt.include_external_dependencies") {
        args = append(args, "--terragrunt-include-external-dependencies")
    }

    // Extra flags (from config)
    if extraFlags := viper.GetString("terragrunt.extra_flags"); extraFlags != "" {
        // Split by space and append
        args = append(args, strings.Fields(extraFlags)...)
    }

    // Command separator and actual command
    args = append(args, "--", command)

    return args
}
```

**Example Output**:

```bash
terragrunt run-all \
  --working-dir /path/to/stack \
  --terragrunt-log-level info \
  --terragrunt-log-format pretty \
  --terragrunt-parallelism 10 \
  --terragrunt-non-interactive \
  -- plan
```

### Standard I/O Passthrough

Connect command to terminal for real-time output:

```go
func (e *Executor) Execute(
    ctx context.Context,
    command string,
    stackPath string,
    projectRoot string,
) (int, error) {
    // Build command arguments
    args := buildTerragruntArgs(command, stackPath)

    // Create command with cancellation support
    cmd := exec.CommandContext(ctx, "terragrunt", args...)

    // CRITICAL: Connect to terminal for real-time streaming
    cmd.Stdout = os.Stdout
    cmd.Stderr = os.Stderr
    cmd.Stdin = os.Stdin

    // Set working directory
    cmd.Dir = stackPath

    // Start timer
    startTime := time.Now()

    // Execute (blocking, output streams to terminal)
    err := cmd.Run()

    // Calculate duration
    duration := time.Since(startTime).Seconds()

    // Extract exit code
    exitCode := extractExitCode(err)

    // Log to history (non-blocking)
    e.logExecutionToHistory(command, stackPath, projectRoot, exitCode, duration)

    return exitCode, err
}
```

**Benefits**:
- **Real-time feedback**: User sees output as it happens.
- **Interactive input**: User can respond to prompts (though non-interactive mode preferred).
- **Signal handling**: Ctrl+C propagates to terragrunt.
- **No buffering**: Output appears immediately.

### Exit Code Extraction

Extract exit code from error:

```go
func extractExitCode(err error) int {
    if err == nil {
        return 0 // Success
    }

    // Check if error is ExitError (command ran but failed)
    if exitErr, ok := err.(*exec.ExitError); ok {
        return exitErr.ExitCode()
    }

    // Command didn't run (e.g., binary not found)
    return 1 // Generic failure
}
```

**Cases Handled**:
- **Success**: `err == nil` → exit code 0
- **Command failure**: `ExitError` → actual exit code (e.g., 1, 2)
- **Execution failure**: Other error → exit code 1

### Timing Instrumentation

Measure execution duration:

```go
startTime := time.Now()

err := cmd.Run()

duration := time.Since(startTime).Seconds()
```

**Benefits**:
- **Performance tracking**: Identify slow operations.
- **History analytics**: Can query history for slow executions.
- **User feedback**: Show execution time after completion.

### Non-Blocking History Logging

History failures don't prevent execution:

```go
func (e *Executor) logExecutionToHistory(
    command string,
    stackPath string,
    projectRoot string,
    exitCode int,
    duration float64,
) {
    // Calculate relative path
    relPath, err := history.CalculateRelativePath(stackPath, projectRoot)
    if err != nil {
        relPath = stackPath // Fallback to absolute
    }

    // Get current user
    user, _ := user.Current()
    username := "unknown"
    if user != nil {
        username = user.Username
    }

    // Create log entry
    entry := history.ExecutionLogEntry{
        Timestamp:    time.Now(),
        Command:      command,
        StackPath:    relPath,
        AbsolutePath: stackPath,
        ProjectRoot:  projectRoot,
        ExitCode:     exitCode,
        Duration:     duration,
        User:         username,
    }

    // Log to history (failures logged but don't return error)
    if err := e.historyLogger.LogExecution(entry, e.maxHistoryEntries); err != nil {
        fmt.Fprintf(os.Stderr, "Warning: failed to log execution to history: %v\n", err)
        // CRITICAL: Don't return error, execution was successful
    }
}
```

**Graceful Degradation**:
- History append fails → warn user, continue.
- Path calculation fails → use absolute path.
- User detection fails → use "unknown".

### Context Cancellation Support

Accept context.Context for cancellation:

```go
func (e *Executor) Execute(ctx context.Context, ...) (int, error) {
    // Use CommandContext for cancellation support
    cmd := exec.CommandContext(ctx, "terragrunt", args...)

    // If context cancelled, command terminates
}
```

**Benefits**:
- **User cancellation**: Ctrl+C can cancel long-running commands.
- **Timeout support**: Can set execution timeout via context.
- **Resource cleanup**: Subprocess terminated on cancel.

### Error Handling

Wrap errors with context:

```go
if err := cmd.Run(); err != nil {
    return extractExitCode(err), fmt.Errorf(
        "failed to execute terragrunt %s in %s: %w",
        command, stackPath, err,
    )
}
```

**Benefits**:
- **Debugging**: Error messages include command and path.
- **Error chaining**: Original error preserved via %w.
- **User clarity**: Clear error messages.

## Consequences

### Positive

- **Isolated logic**: Execution logic in dedicated package.
- **Reusable**: Can use for CLI mode, API, tests.
- **Testable**: Dependency injection enables mocking.
- **Maintainable**: Single Responsibility Principle.
- **Real-time output**: Streams to terminal as it happens.
- **Accurate exit codes**: Captured from subprocess.
- **Non-blocking history**: Failures don't prevent execution.
- **Cancellable**: Context support for interruption.
- **Clear errors**: Context-wrapped error messages.
- **Configurable**: All terragrunt flags from configuration.

### Negative

- **Interface overhead**: HistoryLogger interface adds indirection.
- **Configuration coupling**: Depends on Viper for flag values (could be injected).
- **No output capture**: Output goes directly to terminal (can't parse in TUI).

### Neutral

- **Blocking execution**: Currently blocks until command completes (intentional).
- **No parallelization**: Executes one command at a time (sufficient for current needs).

## Alternatives Considered

### Alternative 1: Execute in TUI Package

Put execution logic in `internal/tui/`.

**Pros**:
- Fewer packages.
- Direct access to UI state.

**Cons**:
- Violates separation of concerns.
- Can't reuse for CLI-only mode.
- Harder to test (Bubble Tea complicates tests).
- TUI package becomes bloated.

**Decision**: Dedicated package is cleaner.

### Alternative 2: Execute in CMD Package

Put execution logic in `cmd/`.

**Pros**:
- Close to entry point.
- Easy to access cobra flags.

**Cons**:
- CMD should coordinate, not implement.
- Harder to unit test (cobra adds complexity).
- Can't reuse across multiple commands.

**Decision**: Business logic belongs in internal/.

### Alternative 3: Concrete Dependency on History Service

Directly import and use `history.Service` instead of interface.

```go
type Executor struct {
    historyService *history.Service  // Concrete type
}
```

**Pros**:
- Simpler (no interface).
- Direct method calls.

**Cons**:
- Tight coupling to history package.
- Can't mock for testing.
- Circular dependency risk.

**Decision**: Interface is better for testability.

### Alternative 4: Background Goroutine for Execution

Run command in background goroutine, stream output via channels.

**Pros**:
- Non-blocking execution.
- Can update UI during execution.

**Cons**:
- Complex concurrency management.
- Signal handling tricky.
- Harder to debug.
- TerraX currently exits to TTY during execution anyway.

**Decision**: Blocking execution is simpler and sufficient.

### Alternative 5: Shell Script Generation

Generate shell script, execute via `sh -c`.

**Pros**:
- Users can see exact command.
- Can copy/paste to run manually.

**Cons**:
- Shell escaping complexity.
- Platform-specific (different shells).
- Harder to capture exit code reliably.
- Less control over execution.

**Decision**: Direct exec is more reliable.

## Future Enhancements

**Potential Improvements**:
1. **Output capture**: Optionally capture output for parsing (e.g., JSON plan).
2. **Parallel execution**: Execute multiple stacks concurrently.
3. **Progress tracking**: Show progress in TUI during execution.
4. **Dry-run mode**: Show command without executing.
5. **Command preview**: Show full terragrunt command before execution.
6. **Retry logic**: Automatic retry on transient failures.
7. **Config injection**: Inject config struct instead of Viper dependency.

## References

- **Dependency Injection**: "Dependency Injection in .NET" by Mark Seemann
- **Single Responsibility Principle**: "Clean Architecture" by Robert C. Martin
- **os/exec Package**: https://pkg.go.dev/os/exec
- **Context Package**: https://pkg.go.dev/context
- **Related ADRs**: [ADR-0004: Separation of Concerns](0004-separation-of-concerns.md)
