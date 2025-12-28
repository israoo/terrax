# Error Handling Standards

**Status**: Active

**Last Updated**: 2025-12-27

## Overview

This document defines standards for error handling in the TerraX project. Proper error handling is critical for reliability, debuggability, and user experience.

## Core Principles

1. **Never ignore errors**: All errors must be handled explicitly.
2. **Add context at each layer**: Wrap errors with meaningful information.
3. **Handle at boundaries**: Process errors at application boundaries (main, CLI, API).
4. **Fail fast**: Return errors immediately rather than attempting recovery.
5. **Provide actionable messages**: Error messages should help users fix the problem.

## Basic Error Handling

### Always Check Errors (MANDATORY)

```go
// WRONG: Ignoring error
result, _ := someFunction()

// WRONG: Blank error variable
result, err := someFunction()
// err is never checked

// CORRECT: Check and handle
result, err := someFunction()
if err != nil {
    return fmt.Errorf("failed to do something: %w", err)
}
```

**Rule**: Every function that returns an error must have its error checked at the call site.

### Error Return Position

```go
// CORRECT: Error is last return value
func BuildTree(path string) (*Node, error)

// CORRECT: Multiple values with error last
func ParseConfig(data []byte) (Config, Metadata, error)

// WRONG: Error not last
func BadFunction(path string) (error, *Node)
```

**Rule**: Error is always the last return value.

## Error Wrapping

### Wrap Errors with Context

```go
// Add context at each layer
func BuildTree(rootPath string) (*Node, error) {
    entries, err := os.ReadDir(rootPath)
    if err != nil {
        // Wrap with context about what we were trying to do
        return nil, fmt.Errorf("failed to read directory %s: %w", rootPath, err)
    }

    // More operations
}
```

**Format**: `fmt.Errorf("context about what failed: %w", err)`

**Benefits**:
- Preserves original error for type checking (`errors.Is`, `errors.As`).
- Adds context for debugging (call chain visible in error message).
- Helps users understand what went wrong.

### Context Guidelines

Good context explains:
- **What operation was being performed**: "failed to build tree", "unable to scan directory"
- **What resource was involved**: Include paths, IDs, names
- **Why it matters**: Sometimes add context about impact

```go
// GOOD: Specific context
return fmt.Errorf("failed to build tree for %s: %w", rootPath, err)

// BETTER: Even more specific
return fmt.Errorf("failed to scan directory %s for terragrunt.hcl files: %w", dir, err)

// WRONG: Generic, unhelpful
return fmt.Errorf("error: %w", err)

// WRONG: No wrapping (loses original error)
return fmt.Errorf("failed to build tree for %s: %v", rootPath, err)
```

### Multi-Layer Wrapping

Errors accumulate context as they bubble up:

```go
// Layer 1: Low-level filesystem operation
func readConfig(path string) ([]byte, error) {
    data, err := os.ReadFile(path)
    if err != nil {
        return nil, fmt.Errorf("failed to read config file %s: %w", path, err)
    }
    return data, nil
}

// Layer 2: Business logic
func loadConfig(path string) (*Config, error) {
    data, err := readConfig(path)
    if err != nil {
        return nil, fmt.Errorf("failed to load configuration: %w", err)
    }

    var cfg Config
    if err := json.Unmarshal(data, &cfg); err != nil {
        return nil, fmt.Errorf("failed to parse config file %s: %w", path, err)
    }

    return &cfg, nil
}

// Layer 3: Application boundary
func main() {
    cfg, err := loadConfig("config.json")
    if err != nil {
        // Final error message includes full context:
        // "failed to load configuration: failed to read config file config.json: open config.json: no such file or directory"
        fmt.Fprintf(os.Stderr, "Error: %v\n", err)
        os.Exit(1)
    }
}
```

## Error Handling Patterns

### Early Return Pattern

Return errors immediately rather than nesting:

```go
// WRONG: Deep nesting
func BuildTree(path string) (*Node, error) {
    if info, err := os.Stat(path); err == nil {
        if info.IsDir() {
            if entries, err := os.ReadDir(path); err == nil {
                // Deep nesting continues...
            } else {
                return nil, err
            }
        } else {
            return nil, fmt.Errorf("not a directory")
        }
    } else {
        return nil, err
    }
}

// CORRECT: Early returns, flat structure
func BuildTree(path string) (*Node, error) {
    info, err := os.Stat(path)
    if err != nil {
        return nil, fmt.Errorf("failed to stat %s: %w", path, err)
    }

    if !info.IsDir() {
        return nil, fmt.Errorf("%s is not a directory", path)
    }

    entries, err := os.ReadDir(path)
    if err != nil {
        return nil, fmt.Errorf("failed to read directory %s: %w", path, err)
    }

    // Continue with flat structure
}
```

### Error Checking in Loops

```go
func processNodes(nodes []*Node) error {
    for i, node := range nodes {
        if err := processNode(node); err != nil {
            // Add context about which iteration failed
            return fmt.Errorf("failed to process node %d (%s): %w", i, node.Name, err)
        }
    }
    return nil
}
```

### Cleanup with Defer

```go
func processFile(path string) error {
    f, err := os.Open(path)
    if err != nil {
        return fmt.Errorf("failed to open %s: %w", path, err)
    }
    defer f.Close()  // Ensure cleanup happens

    // Process file
    // If any error occurs, file is still closed

    return nil
}
```

### Error Variable Shadowing (Avoid)

```go
// WRONG: Error shadowing
func example() error {
    result, err := operation1()
    if err != nil {
        return err
    }

    // SHADOWING: new 'err' shadows previous
    other, err := operation2()
    if err != nil {
        return err
    }

    return nil
}

// CORRECT: Reuse error variable or use :=
func example() error {
    var err error

    result, err := operation1()
    if err != nil {
        return err
    }

    // Reuse 'err' variable
    other, err := operation2()
    if err != nil {
        return err
    }

    return nil
}
```

## Custom Error Types

### When to Use Custom Errors

Create custom error types when:
- Errors need to be checked programmatically.
- Additional context beyond a string is needed.
- Specific error handling logic depends on error type.

### Sentinel Errors

```go
// Define sentinel errors for specific conditions
var (
    // ErrInvalidDepth indicates depth parameter is out of valid range.
    ErrInvalidDepth = errors.New("depth must be between 0 and MaxDepth")

    // ErrEmptyTree indicates tree has no nodes.
    ErrEmptyTree = errors.New("tree is empty")

    // ErrNotFound indicates requested resource does not exist.
    ErrNotFound = errors.New("not found")
)

// Usage
func GetChildrenAtDepth(depth int) ([]*Node, error) {
    if depth < 0 || depth > MaxDepth {
        return nil, ErrInvalidDepth
    }
    // ...
}

// Checking
children, err := GetChildrenAtDepth(10)
if errors.Is(err, ErrInvalidDepth) {
    // Handle invalid depth specifically
}
```

### Custom Error Structs

```go
// TreeBuildError represents an error during tree building.
type TreeBuildError struct {
    Path      string    // Path where error occurred
    Operation string    // Operation that failed
    Err       error     // Underlying error
}

// Error implements the error interface.
func (e *TreeBuildError) Error() string {
    return fmt.Sprintf("%s failed for %s: %v", e.Operation, e.Path, e.Err)
}

// Unwrap implements error unwrapping for errors.Is and errors.As.
func (e *TreeBuildError) Unwrap() error {
    return e.Err
}

// Usage
func BuildTree(path string) (*Node, error) {
    entries, err := os.ReadDir(path)
    if err != nil {
        return nil, &TreeBuildError{
            Path:      path,
            Operation: "directory scan",
            Err:       err,
        }
    }
    // ...
}

// Checking
var treeErr *TreeBuildError
if errors.As(err, &treeErr) {
    fmt.Printf("Tree build failed at: %s\n", treeErr.Path)
}
```

### Error Type Methods

```go
// Implement helpful methods on error types
type ValidationError struct {
    Field   string
    Value   interface{}
    Message string
}

func (e *ValidationError) Error() string {
    return fmt.Sprintf("validation failed for %s: %s (value: %v)",
        e.Field, e.Message, e.Value)
}

// IsRecoverable indicates if operation can be retried.
func (e *ValidationError) IsRecoverable() bool {
    // Validation errors are not recoverable without changing input
    return false
}
```

## Error Messages

### User-Facing Error Messages

Error messages shown to users should be:
- **Clear**: Explain what went wrong in plain language.
- **Actionable**: Suggest how to fix the problem.
- **Specific**: Include relevant details (paths, values).

```go
// GOOD: Clear, actionable
return fmt.Errorf("directory %s does not exist; please create it or specify a different path", dir)

// GOOD: Specific and helpful
return fmt.Errorf("failed to parse config file %s: invalid JSON at line %d", path, lineNum)

// WRONG: Vague and unhelpful
return fmt.Errorf("error")

// WRONG: Too technical for users
return fmt.Errorf("ENOENT on stat syscall for inode resolution")
```

### Developer-Facing Error Messages

Errors in internal packages can be more technical:

```go
// OK for internal package
return fmt.Errorf("failed to acquire file lock on %s: %w", lockFile, err)

// OK for debugging
return fmt.Errorf("navigation state invariant violated: depth=%d exceeds maxDepth=%d", depth, maxDepth)
```

## Error Handling at Boundaries

### CLI Error Handling

```go
// cmd/root.go
func Execute() error {
    rootCmd := &cobra.Command{
        RunE: func(cmd *cobra.Command, args []string) error {
            // Business logic that returns errors
            if err := runTUI(args); err != nil {
                return err  // Cobra handles error display
            }
            return nil
        },
    }

    return rootCmd.Execute()
}

// main.go
func main() {
    if err := cmd.Execute(); err != nil {
        // Final error handling at application boundary
        fmt.Fprintf(os.Stderr, "Error: %v\n", err)
        os.Exit(1)
    }
}
```

### TUI Error Handling

```go
// internal/tui/model.go
func (m Model) Init() tea.Cmd {
    // Errors during initialization are critical
    tree, err := stack.BuildTree(m.rootPath)
    if err != nil {
        // In TUI, we might show error in UI instead of exiting
        m.errorMessage = fmt.Sprintf("Failed to load tree: %v", err)
        m.hasError = true
        return nil
    }
    // ...
}
```

### Package Error Handling

```go
// internal/stack/tree.go
func BuildTree(rootPath string) (*Node, error) {
    // Package functions return errors, don't handle them
    // Let caller decide how to handle

    info, err := os.Stat(rootPath)
    if err != nil {
        return nil, fmt.Errorf("failed to stat %s: %w", rootPath, err)
    }

    // Return errors, don't log or exit
    if !info.IsDir() {
        return nil, fmt.Errorf("%s is not a directory", rootPath)
    }

    // ...
    return node, nil
}
```

## Error Checking

### Using errors.Is

Check if error is or wraps a specific error:

```go
import "errors"

var ErrNotFound = errors.New("not found")

func findNode(name string) (*Node, error) {
    // ...
    if node == nil {
        return nil, ErrNotFound
    }
    return node, nil
}

// Checking
node, err := findNode("example")
if errors.Is(err, ErrNotFound) {
    // Handle not found case specifically
    fmt.Println("Node not found, creating new one")
} else if err != nil {
    // Handle other errors
    return fmt.Errorf("unexpected error: %w", err)
}
```

### Using errors.As

Extract custom error type from error chain:

```go
import "errors"

type ValidationError struct {
    Field string
    Err   error
}

func (e *ValidationError) Error() string {
    return fmt.Sprintf("validation failed for %s: %v", e.Field, e.Err)
}

func (e *ValidationError) Unwrap() error {
    return e.Err
}

// Usage
err := validateConfig(cfg)
var validationErr *ValidationError
if errors.As(err, &validationErr) {
    // Access fields of ValidationError
    fmt.Printf("Invalid field: %s\n", validationErr.Field)
}
```

## Logging vs. Returning Errors

### Return Errors, Don't Log

In library code, return errors instead of logging:

```go
// WRONG: Logging in library code
func BuildTree(path string) (*Node, error) {
    entries, err := os.ReadDir(path)
    if err != nil {
        log.Printf("ERROR: Failed to read %s: %v", path, err)  // Don't log here
        return nil, err
    }
    // ...
}

// CORRECT: Return error, let caller decide
func BuildTree(path string) (*Node, error) {
    entries, err := os.ReadDir(path)
    if err != nil {
        return nil, fmt.Errorf("failed to read directory %s: %w", path, err)
    }
    // ...
}
```

**Rationale**: Library code shouldn't decide how to handle errors. Caller might want to recover, retry, or display errors differently.

### Log at Application Boundaries

Log errors at main, CLI, or API boundaries:

```go
// main.go
func main() {
    tree, err := stack.BuildTree(rootPath)
    if err != nil {
        // OK to log at application boundary
        log.Printf("Failed to build tree: %v", err)
        fmt.Fprintf(os.Stderr, "Error: %v\n", err)
        os.Exit(1)
    }
    // ...
}
```

## Error Handling Anti-Patterns

### ❌ Panic for Expected Errors

```go
// WRONG: Panic for expected error
func BuildTree(path string) *Node {
    entries, err := os.ReadDir(path)
    if err != nil {
        panic(err)  // Don't panic for expected errors
    }
    // ...
}

// CORRECT: Return error
func BuildTree(path string) (*Node, error) {
    entries, err := os.ReadDir(path)
    if err != nil {
        return nil, fmt.Errorf("failed to read directory: %w", err)
    }
    // ...
}
```

**Rule**: Only panic for programmer errors (bugs), not expected errors (file not found, network errors, invalid input).

### ❌ Ignoring Errors Silently

```go
// WRONG: Ignoring error
result, _ := someOperation()

// WRONG: Checking but not handling
result, err := someOperation()
if err != nil {
    // TODO: handle error
}

// CORRECT: Handle or propagate
result, err := someOperation()
if err != nil {
    return fmt.Errorf("operation failed: %w", err)
}
```

### ❌ Generic Error Messages

```go
// WRONG: No context
return fmt.Errorf("error: %w", err)

// WRONG: Vague
return fmt.Errorf("something went wrong: %w", err)

// CORRECT: Specific context
return fmt.Errorf("failed to build tree for %s: %w", rootPath, err)
```

### ❌ Not Using %w for Wrapping

```go
// WRONG: Using %v loses error chain
return fmt.Errorf("failed to read: %v", err)

// CORRECT: Using %w preserves error chain
return fmt.Errorf("failed to read: %w", err)
```

## Testing Error Handling

### Test Error Cases

```go
func TestBuildTree_ErrorCases(t *testing.T) {
    tests := []struct {
        name    string
        path    string
        wantErr error
    }{
        {
            name:    "nonexistent directory",
            path:    "/nonexistent/path",
            wantErr: os.ErrNotExist,
        },
        {
            name:    "file instead of directory",
            path:    "/path/to/file.txt",
            wantErr: ErrNotDirectory,
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            _, err := BuildTree(tt.path)
            if !errors.Is(err, tt.wantErr) {
                t.Errorf("expected error %v, got %v", tt.wantErr, err)
            }
        })
    }
}
```

### Test Error Messages

```go
func TestBuildTree_ErrorMessage(t *testing.T) {
    _, err := BuildTree("/nonexistent")
    if err == nil {
        t.Fatal("expected error, got nil")
    }

    // Check error message contains important context
    errMsg := err.Error()
    if !strings.Contains(errMsg, "/nonexistent") {
        t.Errorf("error message should contain path: %s", errMsg)
    }
}
```

## Code Review Checklist

When reviewing error handling:

- [ ] All errors are checked (no `_` for errors).
- [ ] Errors are wrapped with context using `%w`.
- [ ] Context includes relevant details (paths, values, operation).
- [ ] Errors are returned, not logged (except at boundaries).
- [ ] Error messages are clear and actionable.
- [ ] No panics for expected errors.
- [ ] Custom error types implement `Error()` and `Unwrap()`.
- [ ] Error cases are tested.

## Related Documentation

- [Go Coding Standards](go-coding-standards.md)
- [Testing Strategy](testing-strategy.md)
- [Pitfall: Ignoring Errors Silently](../pitfalls/code-quality/ignoring-errors.md) (planned)

## References

- [Effective Go: Errors](https://golang.org/doc/effective_go#errors)
- [Go Blog: Error Handling and Go](https://blog.golang.org/error-handling-and-go)
- [Go Blog: Working with Errors in Go 1.13](https://blog.golang.org/go1.13-errors)
- [Error Handling in Uber Go Style Guide](https://github.com/uber-go/guide/blob/master/style.md#errors)
