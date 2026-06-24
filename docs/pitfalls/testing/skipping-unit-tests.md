# Pitfall: Skipping Unit Tests for Business Logic

**Category**: Testing

**Severity**: High

**Date Identified**: 2025-12-27

## Description

Not writing unit tests for business logic in `internal/stack/`, especially for Navigator methods, tree operations, and core algorithms, leaving critical code paths untested and vulnerable to regressions.

## Impact

Skipping unit tests for business logic creates severe problems:

- **Regressions go undetected**: Changes break existing functionality silently.
- **Refactoring becomes risky**: Can't safely improve code without tests.
- **Bugs reach production**: Issues discovered by users instead of tests.
- **Debugging is harder**: No test cases to reproduce bugs.
- **Documentation is missing**: Tests serve as executable documentation.
- **Confidence is low**: Fear of making changes slows development.
- **Technical debt accumulates**: Untested code is hard to maintain.

## Root Cause

Common reasons unit tests are skipped:

1. **Time pressure**: "We'll add tests later" (never happens).
2. **Lack of knowledge**: "I don't know how to test this."
3. **Wrong priorities**: "Testing isn't as important as features."
4. **Perceived difficulty**: "This is too complex to test."
5. **Manual testing mindset**: "I tested it manually, it works."
6. **Missing test culture**: Team doesn't value automated testing.
7. **Legacy code**: "This code doesn't have tests, so I won't add them."

## How to Avoid

### Do

- **Test business logic first**: Navigator, tree operations, selection propagation.
- **Write tests as you code**: Don't defer testing to later.
- **Test edge cases**: Null inputs, empty trees, boundary conditions.
- **Use table-driven tests**: Test multiple scenarios efficiently.
- **Test before refactoring**: Add tests to untested code before changing it.
- **Aim for 80%+ coverage**: For business logic in `internal/stack/`.
- **Make tests part of definition of done**: Feature isn't done without tests.

### Don't

- **Don't skip core logic**: Navigator methods MUST have tests.
- **Don't rely on manual testing**: Automate everything testable.
- **Don't test only happy paths**: Edge cases find the bugs.
- **Don't write tests later**: "Later" never comes.
- **Don't skip because it's "obvious"**: Obvious code still needs tests.
- **Don't test UI instead of logic**: Test Navigator, not TUI rendering.

## Detection

Warning signs of missing tests:

- **Low coverage**: `go test -cover` shows < 60% for `internal/stack/`.
- **No test files**: Missing `*_test.go` files next to implementation.
- **Only integration tests**: Testing through UI instead of unit tests.
- **Frequent regressions**: Same bugs keep coming back.
- **Fear of refactoring**: Developers avoid changing code.
- **Long debugging sessions**: No tests to reproduce issues.

### Code Smells

```go
// internal/stack/navigator.go exists with complex logic
// ❌ But internal/stack/navigator_test.go doesn't exist

// ❌ Test file exists but has no tests
package stack

import "testing"

// TODO: Add tests

// ❌ Only testing trivial cases
func TestNode_IsLeaf(t *testing.T) {
    node := &Node{}
    _ = node.IsLeaf()
    // No assertions!
}
```

## Remediation

If you have untested business logic, here's how to fix it:

### 1. Identify Untested Code

```bash
# Check coverage for business logic
go test ./internal/stack -cover

# Generate coverage report
go test ./internal/stack -coverprofile=coverage.out
go tool cover -html=coverage.out

# Find files without tests
for file in internal/stack/*.go; do
    test_file="${file%.go}_test.go"
    if [ ! -f "$test_file" ]; then
        echo "Missing tests: $file"
    fi
done
```

### 2. Start with Critical Paths

Prioritize testing:
1. **Navigator methods** (highest priority)
2. **Tree building logic**
3. **Selection propagation**
4. **Breadcrumb generation**
5. **Path resolution**

### 3. Write Table-Driven Tests

```go
// internal/stack/navigator_test.go
func TestNavigator_PropagateSelection(t *testing.T) {
    tests := []struct {
        name            string
        initialIndices  []int
        depth           int
        expectedIndices []int
    }{
        {
            name:            "selection at root resets children",
            initialIndices:  []int{0, 2, 1},
            depth:           0,
            expectedIndices: []int{0, 0, 0},
        },
        {
            name:            "selection at level 1 resets deeper",
            initialIndices:  []int{1, 2, 3},
            depth:           1,
            expectedIndices: []int{1, 2, 0},
        },
        {
            name:            "empty state",
            initialIndices:  []int{},
            depth:           0,
            expectedIndices: []int{0},
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            nav := NewNavigator(createTestTree(), 5)
            state := &NavigationState{
                SelectedIndices: tt.initialIndices,
            }

            nav.PropagateSelection(state, tt.depth)

            if !reflect.DeepEqual(state.SelectedIndices, tt.expectedIndices) {
                t.Errorf("expected %v, got %v", tt.expectedIndices, state.SelectedIndices)
            }
        })
    }
}
```

### 4. Test Edge Cases

```go
func TestNavigator_GetChildrenAtDepth_EdgeCases(t *testing.T) {
    nav := NewNavigator(createTestTree(), 5)

    tests := []struct {
        name      string
        depth     int
        state     *NavigationState
        shouldNil bool
    }{
        {
            name:      "negative depth",
            depth:     -1,
            state:     &NavigationState{},
            shouldNil: true,
        },
        {
            name:      "depth exceeds max",
            depth:     10,
            state:     &NavigationState{},
            shouldNil: true,
        },
        {
            name:      "nil state",
            depth:     0,
            state:     nil,
            shouldNil: true,
        },
        {
            name: "empty tree",
            depth: 0,
            state: &NavigationState{},
            // Setup with empty tree
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            result := nav.GetChildrenAtDepth(tt.state, tt.depth)
            if tt.shouldNil && result != nil {
                t.Error("expected nil result")
            }
        })
    }
}
```

### 5. Create Test Helpers

```go
// internal/stack/testing.go
// +build testing

package stack

// createTestTree returns a simple tree for testing.
func createTestTree() *Node {
    return &Node{
        Name: "root",
        Path: "/root",
        Children: []*Node{
            {
                Name: "child1",
                Path: "/root/child1",
                Children: []*Node{
                    {Name: "grandchild1", Path: "/root/child1/grandchild1"},
                },
            },
            {
                Name: "child2",
                Path: "/root/child2",
            },
        },
    }
}

// createDeepTestTree creates tree with specified depth.
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

## Related

- [Standard: Testing Strategy](../../standards/testing-strategy.md)
- [ADR-0002: Navigator Pattern](../../adr/0002-navigator-pattern.md)
- [ADR-0004: Separation of Concerns](../../adr/0004-separation-of-concerns.md)
- [Pitfall: Testing Wrong Layer](testing-wrong-layer.md) (planned)

## Examples

### Bad: No Unit Tests

```go
// internal/stack/navigator.go
package stack

// Complex business logic
func (n *Navigator) PropagateSelection(state *NavigationState, depth int) {
    // ... 50 lines of complex logic
    // Multiple edge cases
    // Critical business logic
}

// ❌ No internal/stack/navigator_test.go file exists!
// ❌ Or test file exists but is empty
// ❌ Or only trivial tests, no coverage of complex logic
```

**Problems**:
- No way to verify correctness
- Regressions won't be caught
- Can't refactor safely
- No documentation of expected behavior

### Good: Comprehensive Unit Tests

```go
// internal/stack/navigator_test.go
package stack

import (
    "reflect"
    "testing"
)

// Test happy path
func TestNavigator_PropagateSelection_HappyPath(t *testing.T) {
    nav := NewNavigator(createTestTree(), 5)
    state := &NavigationState{SelectedIndices: []int{0, 1}}

    nav.PropagateSelection(state, 0)

    expected := []int{0, 0}
    if !reflect.DeepEqual(state.SelectedIndices, expected) {
        t.Errorf("expected %v, got %v", expected, state.SelectedIndices)
    }
}

// Test edge cases
func TestNavigator_PropagateSelection_EdgeCases(t *testing.T) {
    tests := []struct {
        name     string
        setup    func() (*Navigator, *NavigationState)
        depth    int
        expected []int
    }{
        {
            name: "empty state",
            setup: func() (*Navigator, *NavigationState) {
                return NewNavigator(createTestTree(), 5), &NavigationState{}
            },
            depth:    0,
            expected: []int{0},
        },
        {
            name: "depth beyond tree",
            setup: func() (*Navigator, *NavigationState) {
                return NewNavigator(createTestTree(), 5),
                    &NavigationState{SelectedIndices: []int{0, 0, 0}}
            },
            depth:    5,
            expected: []int{0, 0, 0, 0, 0, 0},
        },
        // More edge cases
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            nav, state := tt.setup()
            nav.PropagateSelection(state, tt.depth)

            if !reflect.DeepEqual(state.SelectedIndices, tt.expected) {
                t.Errorf("expected %v, got %v", tt.expected, state.SelectedIndices)
            }
        })
    }
}

// Test error conditions
func TestNavigator_PropagateSelection_Errors(t *testing.T) {
    nav := NewNavigator(nil, 5)
    state := &NavigationState{}

    // Should handle nil tree gracefully
    nav.PropagateSelection(state, 0)
}
```

**Benefits**:
- Verifies correctness
- Catches regressions automatically
- Documents expected behavior
- Safe to refactor
- Quick feedback loop

### Bad: Only Manual Testing

```go
// Developer's approach:
// 1. Write Navigator method
// 2. Run app manually
// 3. Click through UI
// 4. "Looks good, ship it!"

// ❌ No automated tests
// ❌ Next change might break it
// ❌ Can't verify all edge cases manually
// ❌ No regression detection
```

### Good: Automated Unit Tests

```go
// Developer's approach:
// 1. Write test for expected behavior (TDD)
// 2. Write Navigator method to pass test
// 3. Run: go test ./internal/stack -v
// 4. Add more tests for edge cases
// 5. Commit with confidence

// ✅ Automated verification
// ✅ Fast feedback (seconds, not minutes)
// ✅ All edge cases covered
// ✅ Regression protection
// ✅ CI runs tests on every commit
```

## Testing Priority Matrix

### Must Test (Critical)

- ✅ Navigator.PropagateSelection()
- ✅ Navigator.GetChildrenAtDepth()
- ✅ Navigator.GenerateBreadcrumbs()
- ✅ Navigator.GetSelectedPath()
- ✅ BuildTree() with various directory structures
- ✅ Edge cases: nil, empty, max depth

### Should Test (High Priority)

- ✅ Tree traversal logic
- ✅ Selection validation
- ✅ Path resolution
- ✅ Error handling in business logic

### Can Test (Medium Priority)

- Layout calculations (if complex)
- Helper functions
- Utility methods

### Skip (Low Value)

- ❌ Bubble Tea rendering output (brittle)
- ❌ Terminal color codes (visual)
- ❌ Trivial getters/setters

## Coverage Goals

Per [Testing Strategy](../../standards/testing-strategy.md):

- **Business logic (`internal/stack/`)**: 80%+ coverage
- **UI logic (`internal/tui/`)**: 60%+ coverage
- **Overall**: 70%+ coverage

```bash
# Check coverage
go test ./internal/stack -cover

# Generate report
go test ./internal/stack -coverprofile=coverage.out
go tool cover -func=coverage.out

# View in browser
go tool cover -html=coverage.out
```

## Common Excuses (and Rebuttals)

### "I don't have time"

**Reality**: Writing tests saves time long-term.
- Find bugs earlier (cheaper to fix)
- Prevent regressions (no re-fixing same bugs)
- Enable safe refactoring (improve code faster)

### "It's too complex to test"

**Reality**: Complex code needs tests most.
- Break down into testable units
- Use table-driven tests
- Test one aspect at a time

### "I tested it manually"

**Reality**: Manual testing doesn't scale.
- Can't test all edge cases manually
- Can't run on every commit
- Doesn't prevent regressions

### "The code is obvious"

**Reality**: Obvious code still has bugs.
- Tests document expected behavior
- Tests catch integration issues
- Tests prevent future changes breaking it

### "I'll add tests later"

**Reality**: Later never comes.
- Technical debt accumulates
- Harder to test after code grows
- Never have "time" later either

## Enforcement

### CI Requirements

```yaml
# .github/workflows/ci.yml
- name: Run tests
  run: go test ./... -cover

- name: Check coverage
  run: |
    coverage=$(go test ./internal/stack -coverprofile=coverage.out -covermode=atomic | grep coverage | awk '{print $NF}' | sed 's/%//')
    if (( $(echo "$coverage < 80" | bc -l) )); then
      echo "Coverage $coverage% is below 80% threshold"
      exit 1
    fi
```

### Pre-commit Hook

```bash
#!/bin/bash
# .git/hooks/pre-commit

# Run tests before commit
if ! go test ./... ; then
    echo "Tests failed. Fix before committing."
    exit 1
fi

# Check coverage for business logic
coverage=$(go test ./internal/stack -coverprofile=/tmp/coverage.out | grep coverage | awk '{print $NF}' | sed 's/%//')
if (( $(echo "$coverage < 80" | bc -l) )); then
    echo "Coverage $coverage% is below 80% for internal/stack/"
    exit 1
fi
```

## Code Review Checklist

When reviewing code:

- [ ] New business logic has corresponding tests
- [ ] Tests cover happy path
- [ ] Tests cover edge cases (nil, empty, boundaries)
- [ ] Tests cover error conditions
- [ ] Tests use table-driven format where appropriate
- [ ] Test names are descriptive
- [ ] Coverage doesn't decrease
- [ ] Tests actually assert results (not just call functions)

## Migration Strategy

For existing untested code:

### 1. Assess Current State

```bash
go test ./internal/stack -cover
# Note current coverage percentage
```

### 2. Prioritize Critical Code

Start with most important:
1. Navigator methods
2. Tree building
3. Selection logic
4. Other business logic

### 3. Add Tests Incrementally

```bash
# Week 1: Test Navigator core methods (aim for 50% coverage)
# Week 2: Test tree operations (aim for 65% coverage)
# Week 3: Test edge cases (aim for 80% coverage)
# Week 4: Clean up and optimize tests
```

### 4. Make Tests Part of Workflow

- Require tests for all new code
- Add tests when fixing bugs
- Add tests before refactoring

## Quick Reference

| ❌ DON'T | ✅ DO |
|----------|-------|
| Ship without tests | Write tests first or alongside code |
| Only test happy path | Test edge cases and errors |
| Only manual testing | Automated unit tests |
| "I'll add tests later" | Tests are part of definition of done |
| Test through UI | Unit test business logic directly |
| Skip "obvious" code | Even simple code needs tests |
| < 60% coverage for business logic | Aim for 80%+ coverage |

## TerraX-Specific Rules

Per [Testing Strategy](../../standards/testing-strategy.md):

> **High Priority: Business Logic (internal/stack/)**
>
> Must test:
> - BuildTree()
> - Navigator methods
> - Selection propagation
> - Breadcrumb generation
> - Edge cases

This is **MANDATORY**. Business logic without tests should not be merged.
