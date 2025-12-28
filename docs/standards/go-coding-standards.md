# Go Coding Standards

**Status**: Active

**Last Updated**: 2025-12-27

## Overview

This document defines Go coding standards for the TerraX project. These standards ensure consistency, maintainability, and quality across the codebase.

## General Principles

1. **Follow Go conventions**: Adhere to official Go style guidelines and idioms.
2. **Simplicity over cleverness**: Write clear, straightforward code.
3. **Consistency**: Follow existing patterns in the codebase.
4. **Readability**: Code is read more often than written.

## Code Organization

### Import Organization (MANDATORY)

Three groups separated by blank lines, sorted alphabetically within each group:

1. Go standard library
2. Third-party packages
3. TerraX internal packages (`github.com/israoo/terrax/...`)

**Example**:

```go
import (
    "fmt"
    "os"
    "path/filepath"

    tea "github.com/charmbracelet/bubbletea"
    "github.com/charmbracelet/lipgloss"
    "github.com/spf13/cobra"

    "github.com/israoo/terrax/internal/stack"
    "github.com/israoo/terrax/internal/tui"
)
```

**Enforcement**: Run `goimports` or configure IDE to organize imports automatically.

### Package Structure

```text
terrax/
├── cmd/                    # CLI commands (thin wrappers)
├── internal/               # Private application code
│   ├── stack/              # Business logic (zero UI dependencies)
│   └── tui/                # Presentation layer (Bubble Tea)
├── pkg/                    # Public libraries (if needed)
└── main.go                 # Entry point
```

**Rules**:
- Keep packages focused on single responsibility.
- Avoid circular dependencies.
- Use `internal/` for code not intended for external use.
- Package names are singular, lowercase, no underscores.

## Naming Conventions

### General Rules

- **Exported identifiers**: `UpperCamelCase` (e.g., `Navigator`, `BuildTree`).
- **Unexported identifiers**: `lowerCamelCase` (e.g., `maxDepth`, `currentNode`).
- **Acronyms**: Keep consistent case (e.g., `HTTPServer` or `httpServer`, not `HttpServer`).
- **Package names**: Short, concise, lowercase, singular (e.g., `stack`, not `stacks`).

### Specific Conventions

**Interfaces**:
- Name by behavior: `Reader`, `Writer`, `Navigator`.
- Single-method interfaces often end in `-er`: `Renderer`, `Calculator`.

**Receivers**:
- Use short, consistent names: `n` for `Navigator`, `m` for `Model`.
- Be consistent within a type (don't mix `n` and `nav`).

**Variables**:
- Short names for local scope: `i`, `idx`, `err`, `ok`.
- Descriptive names for package scope: `maxVisibleNavColumns`, `rootDirectory`.

**Constants**:
- Exported: `UpperCamelCase` (e.g., `MaxDepth`).
- Unexported: `lowerCamelCase` (e.g., `defaultTimeout`).

## Error Handling

### Always Handle Errors

```go
// WRONG
result, _ := someFunction()

// CORRECT
result, err := someFunction()
if err != nil {
    return fmt.Errorf("failed to do something: %w", err)
}
```

### Wrap Errors with Context

```go
// Add context at each layer
if err := buildTree(path); err != nil {
    return fmt.Errorf("failed to build tree for %s: %w", path, err)
}
```

### Error Handling at Boundaries

```go
// Handle errors at application boundaries (main, CLI commands)
if err := rootCmd.Execute(); err != nil {
    fmt.Fprintf(os.Stderr, "Error: %v\n", err)
    os.Exit(1)
}
```

### Custom Error Types (When Needed)

```go
// For errors that need to be checked programmatically
type TreeBuildError struct {
    Path string
    Err  error
}

func (e *TreeBuildError) Error() string {
    return fmt.Sprintf("failed to build tree at %s: %v", e.Path, e.Err)
}

func (e *TreeBuildError) Unwrap() error {
    return e.Err
}
```

## Comments

### Comment Style (MANDATORY)

All comments must end with periods.

```go
// WRONG
// This function builds a tree

// CORRECT
// This function builds a tree.
```

### When to Comment

**DO comment**:
- Package documentation (package-level comment before `package`).
- Exported types, functions, methods (godoc).
- Non-obvious logic or algorithms.
- Edge cases, gotchas, or subtle behavior.
- Why something is done, not just what.
- TODO items with context.

**DON'T comment**:
- Obvious code (`// Increment counter` above `i++`).
- Duplicating function/variable names.
- Outdated or incorrect information.

### Package Documentation

```go
// Package stack provides tree building and navigation for Terragrunt stacks.
//
// The package implements core business logic for TerraX, including filesystem
// scanning, tree construction, and hierarchical navigation operations.
package stack
```

### Function Documentation

```go
// BuildTree scans the filesystem starting at rootPath and constructs a tree
// of Nodes representing the Terragrunt stack hierarchy. It identifies
// terragrunt.hcl files and organizes them by directory structure.
//
// Returns an error if rootPath does not exist or is not accessible.
func BuildTree(rootPath string) (*Node, error) {
    // Implementation
}
```

### Inline Comments

```go
// PropagateSelection updates selection indices when parent selection changes.
// This is necessary because children lists change when parent selection moves,
// so child indices must be reset to 0 to avoid out-of-bounds access.
func (n *Navigator) PropagateSelection(state *NavigationState) {
    // Implementation
}
```

## Function and Method Design

### Keep Functions Focused

- Single responsibility per function.
- Ideally under 50 lines; if longer, consider refactoring.
- Extract complex logic into helper functions.

### Function Signature Guidelines

```go
// Good: Clear parameters and return values
func BuildTree(rootPath string) (*Node, error)

// Good: Use struct for many parameters
type TreeOptions struct {
    RootPath    string
    MaxDepth    int
    ExcludeHidden bool
}

func BuildTreeWithOptions(opts TreeOptions) (*Node, error)

// Avoid: Too many parameters
func BuildTree(path string, maxDepth int, exclude bool, include bool, ...) (*Node, error)
```

### Receiver Types

```go
// Use pointer receivers for:
// 1. Methods that modify the receiver
// 2. Large structs (avoid copying)
// 3. Consistency (if any method has pointer receiver, all should)
func (n *Navigator) PropagateSelection(state *NavigationState) {
    // Modifies state
}

// Use value receivers for:
// 1. Small, immutable types
// 2. No state modification
func (n Node) IsLeaf() bool {
    return len(n.Children) == 0
}
```

## Concurrency (Future)

TerraX currently doesn't use concurrency, but when adding it:

### Use Goroutines Responsibly

```go
// Always handle goroutine lifecycle
func (m Model) startBackgroundTask() tea.Cmd {
    return func() tea.Msg {
        // Background work
        result := doWork()
        return WorkCompleteMsg{result}
    }
}
```

### Avoid Shared State

- Use channels for communication.
- Avoid mutex-protected shared state unless necessary.
- Prefer message passing (Bubble Tea commands).

## Testing

### Test File Naming

- Test files: `*_test.go` (e.g., `navigator_test.go`).
- Co-locate tests with implementation.

### Table-Driven Tests

```go
func TestNavigator_GetChildrenAtDepth(t *testing.T) {
    tests := []struct {
        name     string
        setup    func() (*Navigator, *NavigationState)
        depth    int
        expected int  // Expected number of children
    }{
        {
            name: "root level",
            setup: func() (*Navigator, *NavigationState) {
                // Setup
            },
            depth:    0,
            expected: 3,
        },
        // More test cases
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            nav, state := tt.setup()
            children := nav.GetChildrenAtDepth(state, tt.depth)
            if len(children) != tt.expected {
                t.Errorf("expected %d children, got %d", tt.expected, len(children))
            }
        })
    }
}
```

### Test Helpers

```go
// Helper functions for test setup
func createTestTree() *Node {
    return &Node{
        Name: "root",
        Children: []*Node{
            {Name: "child1"},
            {Name: "child2"},
        },
    }
}
```

## Performance Considerations

### Avoid Premature Optimization

- Write clear code first.
- Profile before optimizing.
- Optimize only hot paths.

### Common Optimizations

**String Building**:
```go
// WRONG: Inefficient for many concatenations
result := ""
for _, s := range strings {
    result += s
}

// CORRECT: Use strings.Builder
var builder strings.Builder
for _, s := range strings {
    builder.WriteString(s)
}
result := builder.String()
```

**Slice Allocation**:
```go
// Pre-allocate if size is known
nodes := make([]*Node, 0, expectedSize)

// Use append instead of index assignment for unknown sizes
nodes = append(nodes, newNode)
```

## Code Formatting

### Use `gofmt` or `goimports`

- All code must be formatted with `gofmt` before committing.
- Configure IDE to format on save.
- CI should enforce formatting.

```bash
# Format all code
go fmt ./...

# Or use goimports (handles imports + formatting)
goimports -w .
```

### Line Length

- No hard limit, but aim for ~100 characters.
- Break long lines for readability.
- Break before operators for clarity.

## Common Patterns

### Constructor Functions

```go
// Use New* functions for initialization
func NewNavigator(root *Node, maxDepth int) *Navigator {
    return &Navigator{
        root:     root,
        maxDepth: maxDepth,
    }
}
```

### Options Pattern (For Complex Initialization)

```go
type NavigatorOption func(*Navigator)

func WithMaxDepth(depth int) NavigatorOption {
    return func(n *Navigator) {
        n.maxDepth = depth
    }
}

func NewNavigator(root *Node, opts ...NavigatorOption) *Navigator {
    n := &Navigator{root: root}
    for _, opt := range opts {
        opt(n)
    }
    return n
}

// Usage
nav := NewNavigator(root, WithMaxDepth(10))
```

### Zero Values

```go
// Design types so zero value is useful
type Config struct {
    MaxDepth int  // Zero value (0) has sensible meaning
    Enabled  bool // Zero value (false) is valid
}

// No initialization needed for zero value usage
var cfg Config
```

## Linting

### Required Linters

Run before committing:

```bash
go vet ./...           # Official Go tool
golangci-lint run      # Comprehensive linter suite
```

### Recommended `golangci-lint` Configuration

```yaml
# .golangci.yml
linters:
  enable:
    - errcheck      # Check error handling
    - gofmt         # Check formatting
    - goimports     # Check imports
    - govet         # Official Go vet
    - ineffassign   # Detect ineffectual assignments
    - staticcheck   # Advanced static analysis
    - unused        # Detect unused code
```

## Commit Standards

See [Git Workflow](git-workflow.md) for commit message format and process.

## References

- [Effective Go](https://golang.org/doc/effective_go)
- [Go Code Review Comments](https://github.com/golang/go/wiki/CodeReviewComments)
- [Uber Go Style Guide](https://github.com/uber-go/guide/blob/master/style.md)
- [TerraX CLAUDE.md](../../CLAUDE.md)
