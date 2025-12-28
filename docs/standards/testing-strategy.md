# Testing Strategy

**Status**: Active

**Last Updated**: 2025-12-27

## Overview

This document defines the testing strategy for TerraX, including what to test, how to structure tests, and quality standards for test coverage.

## Testing Philosophy

1. **Test business logic extensively**: Focus testing effort on `internal/stack/`.
2. **UI logic can be tested**: Test `internal/tui/` where practical.
3. **Tests are documentation**: Tests show how code should be used.
4. **Fast tests enable confidence**: Quick feedback loop encourages frequent testing.
5. **Coverage is not the goal**: Meaningful tests matter more than coverage percentage.

## Test Types

### Unit Tests

**Definition**: Tests of individual functions, methods, or types in isolation.

**Scope**: Most TerraX tests are unit tests.

**Benefits**:
- Fast execution.
- Easy to debug.
- No external dependencies.
- Test specific behavior.

**Examples**:
- `Navigator.PropagateSelection()` with various selection states.
- `Node.IsLeaf()` with different tree structures.
- `LayoutCalculator.CalculateVisibleColumns()` with various depths.

### Integration Tests

**Definition**: Tests of multiple components working together.

**Scope**: Limited in TerraX currently, but useful for:
- Tree building from real filesystem.
- Navigator + tree interaction.
- TUI model + navigator integration.

**Examples**:
- Building tree from fixture directory structure.
- End-to-end navigation flow through model updates.

### Manual Testing

**Definition**: Human testing of the TUI application.

**Scope**: Primary testing method for UI/UX.

**Process**:
1. Build application: `make build`.
2. Run against sample directory structure.
3. Test navigation, selection, display.
4. Verify across terminal sizes.

## What to Test

### High Priority: Business Logic (internal/stack/)

**Must test**:
- `BuildTree()`: Tree construction from filesystem.
- `Navigator` methods: All navigation operations.
- Selection propagation logic.
- Breadcrumb generation.
- Path resolution.
- Edge cases (empty trees, maximum depth, invalid indices).

**Example**:
```go
func TestNavigator_PropagateSelection(t *testing.T) {
    tests := []struct {
        name            string
        initialState    *NavigationState
        depth           int
        expectedIndices []int
    }{
        {
            name: "selection at depth 0 resets deeper selections",
            initialState: &NavigationState{
                SelectedIndices: []int{0, 2, 1},
            },
            depth:           0,
            expectedIndices: []int{0, 0, 0},
        },
        // More test cases
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            nav := NewNavigator(createTestTree(), 5)
            nav.PropagateSelection(tt.initialState, tt.depth)

            if !reflect.DeepEqual(tt.initialState.SelectedIndices, tt.expectedIndices) {
                t.Errorf("expected %v, got %v", tt.expectedIndices, tt.initialState.SelectedIndices)
            }
        })
    }
}
```

### Medium Priority: UI Logic (internal/tui/)

**Should test**:
- Layout calculations (LayoutCalculator).
- Window sliding logic.
- Focus management.
- Key press handling logic.

**Can skip**:
- Actual rendering output (brittle, low value).
- Lipgloss styling (visual testing more appropriate).

**Example**:
```go
func TestLayoutCalculator_CalculateVisibleColumns(t *testing.T) {
    tests := []struct {
        name             string
        maxDepth         int
        offset           int
        expectedStart    int
        expectedEnd      int
    }{
        {
            name:          "shallow tree shows all",
            maxDepth:      2,
            offset:        0,
            expectedStart: 0,
            expectedEnd:   2,
        },
        {
            name:          "deep tree with offset",
            maxDepth:      10,
            offset:        3,
            expectedStart: 3,
            expectedEnd:   6,  // offset + maxVisibleNavColumns
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            calc := LayoutCalculator{}
            start, end := calc.CalculateVisibleColumns(tt.maxDepth, tt.offset)

            if start != tt.expectedStart || end != tt.expectedEnd {
                t.Errorf("expected (%d, %d), got (%d, %d)",
                    tt.expectedStart, tt.expectedEnd, start, end)
            }
        })
    }
}
```

### Low Priority: CLI (cmd/)

**Testing approach**:
- CLI is thin wrapper, minimal logic.
- Manual testing typically sufficient.
- Integration tests if CLI logic grows.

## Test Structure

### File Organization

Co-locate tests with implementation:

```text
internal/stack/
├── tree.go
├── tree_test.go
├── navigator.go
└── navigator_test.go

internal/tui/
├── model.go
├── model_test.go
├── view.go
└── view_test.go
```

### Test File Template

```go
package stack

import (
    "testing"
)

// Test helper functions at the top
func createTestTree() *Node {
    return &Node{
        Name: "root",
        Path: "/root",
        Children: []*Node{
            {Name: "child1", Path: "/root/child1"},
            {Name: "child2", Path: "/root/child2"},
        },
    }
}

// Table-driven tests
func TestNavigator_MethodName(t *testing.T) {
    tests := []struct {
        name     string
        input    interface{}
        expected interface{}
    }{
        {
            name:     "descriptive test case name",
            input:    /* test input */,
            expected: /* expected output */,
        },
        // More test cases
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            // Setup
            nav := NewNavigator(createTestTree(), 5)

            // Execute
            result := nav.MethodName(tt.input)

            // Assert
            if result != tt.expected {
                t.Errorf("expected %v, got %v", tt.expected, result)
            }
        })
    }
}

// Individual test functions for complex cases
func TestNavigator_ComplexScenario(t *testing.T) {
    // Setup
    tree := createComplexTestTree()
    nav := NewNavigator(tree, 10)

    // Execute multiple operations
    state := &NavigationState{SelectedIndices: []int{0}}
    nav.PropagateSelection(state, 0)
    children := nav.GetChildrenAtDepth(state, 1)

    // Assert multiple conditions
    if len(children) != 3 {
        t.Errorf("expected 3 children, got %d", len(children))
    }

    if children[0].Name != "expected-child" {
        t.Errorf("unexpected child name: %s", children[0].Name)
    }
}
```

### Table-Driven Tests (Preferred)

Use table-driven tests for functions with multiple cases:

```go
func TestNode_IsLeaf(t *testing.T) {
    tests := []struct {
        name     string
        node     *Node
        expected bool
    }{
        {
            name:     "node with children is not leaf",
            node:     &Node{Children: []*Node{{Name: "child"}}},
            expected: false,
        },
        {
            name:     "node without children is leaf",
            node:     &Node{Children: []*Node{}},
            expected: true,
        },
        {
            name:     "node with nil children is leaf",
            node:     &Node{Children: nil},
            expected: true,
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            result := tt.node.IsLeaf()
            if result != tt.expected {
                t.Errorf("expected %v, got %v", tt.expected, result)
            }
        })
    }
}
```

**Benefits**:
- Easy to add new test cases.
- Clear separation of test data and logic.
- Subtests provide granular failure reporting.

### Test Helpers

Create helper functions for common test setup:

```go
// testing.go (if shared across multiple test files)
// +build testing

package stack

// createTestTree returns a simple tree for testing.
func createTestTree() *Node {
    return &Node{
        Name: "root",
        Path: "/root",
        Children: []*Node{
            {Name: "child1", Path: "/root/child1"},
            {Name: "child2", Path: "/root/child2"},
        },
    }
}

// createDeepTestTree returns a tree with specified depth.
func createDeepTestTree(depth int) *Node {
    root := &Node{Name: "root", Path: "/root"}
    current := root

    for i := 1; i < depth; i++ {
        child := &Node{
            Name: fmt.Sprintf("level%d", i),
            Path: filepath.Join(current.Path, fmt.Sprintf("level%d", i)),
        }
        current.Children = []*Node{child}
        current = child
    }

    return root
}
```

**Usage**:
```go
func TestNavigator_DeepTree(t *testing.T) {
    tree := createDeepTestTree(10)
    nav := NewNavigator(tree, 10)
    // Test with deep tree
}
```

## Test Assertions

### Basic Assertions

```go
// Equality
if got != want {
    t.Errorf("expected %v, got %v", want, got)
}

// Nil checks
if result == nil {
    t.Error("expected non-nil result")
}

if err != nil {
    t.Errorf("unexpected error: %v", err)
}

// Boolean
if !condition {
    t.Error("expected condition to be true")
}
```

### Deep Equality

```go
import "reflect"

if !reflect.DeepEqual(got, want) {
    t.Errorf("expected %+v, got %+v", want, got)
}
```

### Error Checking

```go
import "errors"

// Check error occurs
if err == nil {
    t.Error("expected error, got nil")
}

// Check specific error
if !errors.Is(err, ErrExpected) {
    t.Errorf("expected ErrExpected, got %v", err)
}

// Check error type
var expectedErr *CustomError
if !errors.As(err, &expectedErr) {
    t.Errorf("expected CustomError, got %T", err)
}

// Check error message contains text
if err == nil || !strings.Contains(err.Error(), "expected text") {
    t.Errorf("expected error containing 'expected text', got %v", err)
}
```

### Testing Testify (Optional)

For more expressive assertions, consider `github.com/stretchr/testify`:

```go
import (
    "testing"
    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"
)

func TestExample(t *testing.T) {
    result, err := someFunction()

    // require stops test on failure
    require.NoError(t, err)

    // assert continues on failure
    assert.Equal(t, expected, result)
    assert.NotNil(t, result)
    assert.True(t, result.IsValid())
}
```

**Decision**: TerraX currently uses standard library assertions. Testify can be added if tests become verbose.

## Test Coverage

### Running Tests with Coverage

```bash
# Run tests with coverage
go test ./... -cover

# Generate coverage report
go test ./... -coverprofile=coverage.out

# View coverage in browser
go tool cover -html=coverage.out

# Coverage for specific package
go test ./internal/stack -cover -v
```

### Coverage Goals

**Target coverage**:
- **Business logic (`internal/stack/`)**: 80%+ coverage.
- **UI logic (`internal/tui/`)**: 60%+ coverage.
- **Overall**: 70%+ coverage.

**Focus areas**:
- Critical paths (tree building, navigation).
- Error handling.
- Edge cases.

**Not covered**:
- Pure rendering code (low ROI).
- Main/CLI wrappers (manual testing sufficient).

### Coverage is Not the Goal

High coverage doesn't guarantee good tests:

```go
// WRONG: High coverage, low value
func TestBuildTree_Calls(t *testing.T) {
    BuildTree("/some/path")  // Just calls function, asserts nothing
}

// CORRECT: Tests behavior
func TestBuildTree_ValidPath(t *testing.T) {
    tree, err := BuildTree("/valid/path")
    if err != nil {
        t.Fatalf("unexpected error: %v", err)
    }
    if tree == nil {
        t.Error("expected non-nil tree")
    }
    if tree.Name != "path" {
        t.Errorf("expected root name 'path', got %s", tree.Name)
    }
}
```

**Principle**: Write meaningful tests that verify behavior, not tests to hit coverage targets.

## Testing Edge Cases

### Boundary Conditions

```go
func TestNavigator_GetChildrenAtDepth_Boundaries(t *testing.T) {
    tree := createDeepTestTree(5)
    nav := NewNavigator(tree, 5)
    state := &NavigationState{SelectedIndices: []int{0, 0, 0}}

    tests := []struct {
        name  string
        depth int
        valid bool
    }{
        {name: "negative depth", depth: -1, valid: false},
        {name: "zero depth", depth: 0, valid: true},
        {name: "max depth", depth: 4, valid: true},
        {name: "beyond max depth", depth: 5, valid: false},
        {name: "way beyond max", depth: 100, valid: false},
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            children := nav.GetChildrenAtDepth(state, tt.depth)
            if tt.valid && children == nil {
                t.Error("expected children for valid depth")
            }
            if !tt.valid && children != nil {
                t.Error("expected nil for invalid depth")
            }
        })
    }
}
```

### Empty/Nil Inputs

```go
func TestNavigator_EmptyTree(t *testing.T) {
    emptyTree := &Node{Name: "empty", Children: nil}
    nav := NewNavigator(emptyTree, 5)

    children := nav.GetChildrenAtDepth(&NavigationState{}, 0)
    if children != nil {
        t.Errorf("expected nil for empty tree, got %v", children)
    }
}

func TestNavigator_NilState(t *testing.T) {
    nav := NewNavigator(createTestTree(), 5)

    // Should handle nil state gracefully (or panic if documented)
    defer func() {
        if r := recover(); r != nil {
            t.Errorf("should not panic on nil state: %v", r)
        }
    }()

    nav.PropagateSelection(nil, 0)
}
```

### Large Inputs

```go
func TestNavigator_DeepTree(t *testing.T) {
    tree := createDeepTestTree(100)
    nav := NewNavigator(tree, 100)

    // Should handle deep trees without stack overflow
    state := &NavigationState{SelectedIndices: make([]int, 100)}
    breadcrumbs := nav.GenerateBreadcrumbs(state)

    if len(breadcrumbs) != 100 {
        t.Errorf("expected 100 breadcrumbs, got %d", len(breadcrumbs))
    }
}
```

## Benchmark Tests

### When to Benchmark

Benchmark performance-critical code:
- Tree building (filesystem I/O).
- Navigation operations (called frequently).
- Rendering calculations (every frame).

### Benchmark Format

```go
func BenchmarkBuildTree(b *testing.B) {
    tempDir := createTempDirStructure(b)
    defer os.RemoveAll(tempDir)

    b.ResetTimer()  // Don't count setup time

    for i := 0; i < b.N; i++ {
        _, err := BuildTree(tempDir)
        if err != nil {
            b.Fatalf("benchmark failed: %v", err)
        }
    }
}

func BenchmarkNavigator_PropagateSelection(b *testing.B) {
    tree := createDeepTestTree(10)
    nav := NewNavigator(tree, 10)
    state := &NavigationState{SelectedIndices: []int{0, 1, 2}}

    b.ResetTimer()

    for i := 0; i < b.N; i++ {
        nav.PropagateSelection(state, 1)
    }
}
```

### Running Benchmarks

```bash
# Run all benchmarks
go test -bench=. ./...

# Run specific benchmark
go test -bench=BenchmarkBuildTree ./internal/stack

# With memory allocation stats
go test -bench=. -benchmem ./...

# Compare benchmarks
go test -bench=. -benchmem > old.txt
# Make changes
go test -bench=. -benchmem > new.txt
go install golang.org/x/perf/cmd/benchstat@latest
benchstat old.txt new.txt
```

## Testing Filesystem Operations

### Use Temporary Directories

```go
func TestBuildTree_RealFilesystem(t *testing.T) {
    // Create temporary directory structure
    tempDir := t.TempDir()  // Automatically cleaned up

    // Create test structure
    os.MkdirAll(filepath.Join(tempDir, "env", "dev"), 0755)
    os.WriteFile(filepath.Join(tempDir, "env", "terragrunt.hcl"), []byte(""), 0644)

    // Test against real filesystem
    tree, err := BuildTree(tempDir)
    if err != nil {
        t.Fatalf("failed to build tree: %v", err)
    }

    // Assertions
    if len(tree.Children) != 1 {
        t.Errorf("expected 1 child, got %d", len(tree.Children))
    }
}
```

### Fixture Directories

For complex structures, use fixtures:

```text
testdata/
├── simple-stack/
│   ├── terragrunt.hcl
│   ├── env/
│   │   └── terragrunt.hcl
│   └── modules/
│       └── vpc/
│           └── terragrunt.hcl
└── deep-stack/
    └── ... (deep structure)
```

```go
func TestBuildTree_FixtureDirectory(t *testing.T) {
    tree, err := BuildTree("testdata/simple-stack")
    if err != nil {
        t.Fatalf("failed to build tree: %v", err)
    }

    // Assert expected structure
    if tree.Name != "simple-stack" {
        t.Errorf("unexpected root name: %s", tree.Name)
    }

    // Validate children
    expectedChildren := []string{"env", "modules"}
    // ... assertions
}
```

## Testing TUI Components

### Test Model Logic, Not Rendering

```go
// GOOD: Test state changes
func TestModel_HandleRightKey(t *testing.T) {
    m := NewModel("/test/path")
    m.focusedColumn = 0

    // Simulate right arrow key
    updatedModel, _ := m.Update(tea.KeyMsg{Type: tea.KeyRight})
    m = updatedModel.(Model)

    if m.focusedColumn != 1 {
        t.Errorf("expected focusedColumn 1, got %d", m.focusedColumn)
    }
}

// SKIP: Testing rendered output (brittle, low value)
func TestModel_View_Output(t *testing.T) {
    m := NewModel("/test/path")
    output := m.View()

    // Don't test exact string output - it's brittle
    // Styling changes break tests frequently
}
```

### Test Layout Calculations

```go
func TestLayoutCalculator_CalculateVisibleColumns(t *testing.T) {
    calc := LayoutCalculator{}

    start, end := calc.CalculateVisibleColumns(10, 3)

    if start != 3 {
        t.Errorf("expected start 3, got %d", start)
    }

    if end != 6 {
        t.Errorf("expected end 6, got %d", end)
    }
}
```

## Test Naming

### Test Function Names

Format: `Test<Type>_<Method>_<Scenario>`

```go
// Testing function
func TestBuildTree_ValidPath(t *testing.T) {}
func TestBuildTree_NonexistentPath(t *testing.T) {}

// Testing method
func TestNavigator_PropagateSelection_SingleLevel(t *testing.T) {}
func TestNavigator_PropagateSelection_DeepTree(t *testing.T) {}

// Testing type behavior
func TestNode_IsLeaf_WithChildren(t *testing.T) {}
func TestNode_IsLeaf_WithoutChildren(t *testing.T) {}
```

### Table Test Case Names

Be descriptive:

```go
tests := []struct {
    name string
    // ...
}{
    {name: "empty tree returns nil"},
    {name: "single node tree"},
    {name: "deep tree with max depth"},
    {name: "invalid depth returns error"},
}
```

**DON'T**:
```go
{name: "test1"},  // What does test1 test?
{name: "case2"},  // Uninformative
```

## Running Tests

### Commands

```bash
# Run all tests
go test ./...

# Verbose output
go test -v ./...

# Specific package
go test ./internal/stack -v

# Specific test
go test -run TestNavigator_PropagateSelection ./internal/stack

# With coverage
go test ./... -cover

# With race detection
go test -race ./...

# Short mode (skip long tests)
go test -short ./...
```

### CI Integration

```yaml
# .github/workflows/test.yml
name: Tests
on: [push, pull_request]

jobs:
  test:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v3
      - uses: actions/setup-go@v4
        with:
          go-version: '1.25'

      - name: Run tests
        run: go test -v -race -coverprofile=coverage.out ./...

      - name: Check coverage
        run: go tool cover -func=coverage.out

      - name: Upload coverage
        uses: codecov/codecov-action@v3
        with:
          files: ./coverage.out
```

## Code Review Checklist

When reviewing tests:

- [ ] Tests are co-located with implementation (`*_test.go`).
- [ ] Test names are descriptive (`Test<Type>_<Method>_<Scenario>`).
- [ ] Table-driven tests used for multiple cases.
- [ ] Edge cases tested (nil, empty, boundaries).
- [ ] Error cases tested.
- [ ] Test helpers avoid duplication.
- [ ] Tests are independent (no shared mutable state).
- [ ] Tests are deterministic (no randomness, timing dependencies).
- [ ] Business logic has high coverage (80%+).
- [ ] Tests document expected behavior.

## Related Documentation

- [Go Coding Standards](go-coding-standards.md)
- [Error Handling Standards](error-handling.md)
- [ADR-0002: Navigator Pattern](../adr/0002-navigator-pattern.md)
- [ADR-0004: Separation of Concerns](../adr/0004-separation-of-concerns.md)

## References

- [Go Testing Package](https://golang.org/pkg/testing/)
- [Table Driven Tests](https://github.com/golang/go/wiki/TableDrivenTests)
- [Testify Framework](https://github.com/stretchr/testify)
- [Go Testing Best Practices](https://go.dev/doc/tutorial/add-a-test)
