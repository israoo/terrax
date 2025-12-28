# File Organization Standards

**Status**: Active

**Last Updated**: 2025-12-27

## Overview

This document defines how files and packages should be organized in the TerraX project to maintain clarity, scalability, and separation of concerns.

## Directory Structure

```text
terrax/
├── cmd/                        # CLI commands and entry points
│   └── root.go                 # Cobra root command
├── internal/                   # Private application code
│   ├── stack/                  # Business logic layer (zero UI deps)
│   │   ├── tree.go             # Tree structure, filesystem scanning
│   │   ├── tree_test.go        # Tests for tree.go
│   │   ├── navigator.go        # Navigation business logic
│   │   └── navigator_test.go   # Tests for navigator.go
│   └── tui/                    # Presentation layer (Bubble Tea)
│       ├── model.go            # Bubble Tea Model and Update
│       ├── model_test.go       # Tests for model.go
│       ├── view.go             # Rendering (View, LayoutCalculator, Renderer)
│       ├── view_test.go        # Tests for view.go
│       └── constants.go        # UI constants and configuration
├── docs/                       # Documentation
│   ├── adr/                    # Architecture Decision Records
│   ├── standards/              # Coding and design standards
│   └── pitfalls/               # Lessons learned and common mistakes
├── build/                      # Build outputs (gitignored)
├── .github/                    # GitHub workflows and config
├── .gitignore                  # Git ignore rules
├── go.mod                      # Go module definition
├── go.sum                      # Go module checksums
├── Makefile                    # Build automation
├── CLAUDE.md                   # Project guidance for Claude Code
├── README.md                   # Project overview and usage
├── LICENSE                     # License file
└── main.go                     # Application entry point
```

## Package Organization Principles

### 1. Separation of Concerns (MANDATORY)

Each package has a clear, single responsibility:

- **`cmd/`**: CLI coordination, argument parsing, error handling at boundaries.
- **`internal/stack/`**: Business logic, tree operations, navigation algorithms.
- **`internal/tui/`**: UI state management, rendering, user input handling.

**Rule**: Business logic must never mix with UI code. UI code delegates to business logic.

### 2. Internal vs. Public Packages

- **`internal/`**: Private to TerraX, cannot be imported by external projects.
- **`pkg/`**: Public libraries (currently unused, add only if sharing code externally).

**Rule**: Default to `internal/` unless explicitly creating a reusable library.

### 3. Avoid Deep Nesting

Keep package hierarchy shallow (2-3 levels max):

```text
✅ Good:
internal/stack/tree.go
internal/tui/model.go

❌ Avoid:
internal/core/domain/business/logic/stack/tree/builder/tree.go
```

### 4. Package Naming

- **Singular**: `stack`, not `stacks`.
- **Lowercase**: `tui`, not `TUI` or `Tui`.
- **No underscores**: `navigator`, not `tree_navigator`.
- **Concise**: `stack` instead of `stackmanagement`.

## File Organization Within Packages

### File Naming Conventions

- **Implementation files**: Lowercase, descriptive (e.g., `navigator.go`, `tree.go`).
- **Test files**: Match implementation with `_test.go` suffix (e.g., `navigator_test.go`).
- **Constants/config**: `constants.go`, `config.go`.

### File Size Guidelines

- **Target**: 200-500 lines per file.
- **Maximum**: 800 lines (consider splitting if exceeded).
- **Minimum**: No hard minimum, but avoid overly fragmented files.

**When to split**:
- Multiple unrelated types in one file.
- File exceeds 800 lines.
- Different concerns mixed (e.g., models + rendering in same file).

### File Responsibilities

Each file should focus on one major concept:

#### `internal/stack/tree.go`

**Owns**:
- `Node` type definition.
- Tree building from filesystem (`BuildTree`).
- Filesystem scanning logic.

**Does NOT own**:
- Navigation logic (that's in `navigator.go`).
- UI state (that's in `tui/model.go`).

#### `internal/stack/navigator.go`

**Owns**:
- `Navigator` type and constructor.
- `NavigationState` type.
- Navigation methods (traversal, selection, breadcrumbs).

**Does NOT own**:
- Tree building (that's in `tree.go`).
- UI rendering (that's in `tui/view.go`).

#### `internal/tui/model.go`

**Owns**:
- `Model` type (Bubble Tea model).
- `Init()` and `Update()` methods.
- UI state management (focus, offsets, selections).
- Key press handling.

**Does NOT own**:
- Business logic (delegates to `Navigator`).
- Rendering (that's in `view.go`).

#### `internal/tui/view.go`

**Owns**:
- `View()` method.
- `LayoutCalculator` type and methods.
- `Renderer` type and methods.
- Rendering logic and styling.

**Does NOT own**:
- UI state management (reads from `Model`).
- Business logic (uses `Navigator` indirectly through `Model`).

#### `internal/tui/constants.go`

**Owns**:
- UI configuration constants.
- Lipgloss style definitions.
- Color palette.
- Layout dimensions.

**Does NOT own**:
- Dynamic state or logic.

## Test File Organization

### Co-location

Tests live next to implementation:

```text
internal/stack/
├── navigator.go
└── navigator_test.go      # Tests for navigator.go

internal/tui/
├── model.go
└── model_test.go          # Tests for model.go
```

### Test Helpers

If test helpers are reusable across files, create `testing.go`:

```text
internal/stack/
├── navigator.go
├── navigator_test.go
├── tree.go
├── tree_test.go
└── testing.go             # Shared test utilities
```

**Example `testing.go`**:

```go
// +build testing

package stack

// CreateTestTree builds a sample tree for testing.
func CreateTestTree() *Node {
    return &Node{
        Name: "root",
        Children: []*Node{
            {Name: "child1"},
            {Name: "child2"},
        },
    }
}
```

## Import Organization

See [Go Coding Standards](go-coding-standards.md#import-organization-mandatory) for detailed import organization rules.

**Summary**: Three groups (stdlib, third-party, internal), alphabetically sorted, separated by blank lines.

## Documentation Files

### README Files

- **Root `README.md`**: User-facing documentation (installation, usage, features).
- **Package README** (optional): Add `README.md` in packages with complex setup.

### CLAUDE.md

- **Location**: Root directory.
- **Purpose**: Guide Claude Code when working on the project.
- **Audience**: AI assistants and developers using AI tools.

### docs/ Directory

- **`docs/adr/`**: Architecture Decision Records.
- **`docs/standards/`**: Coding and design standards.
- **`docs/pitfalls/`**: Lessons learned and common mistakes.

## Build Artifacts

### Build Directory

```text
build/
└── terrax             # Compiled binary (gitignored)
```

**Rule**: Never commit build artifacts. Keep in `build/` and gitignore.

### .gitignore

```gitignore
# Build outputs
build/
terrax
*.exe

# IDE files
.vscode/
.idea/
*.swp
*.swo

# OS files
.DS_Store
Thumbs.db

# Test coverage
*.out
coverage.html
```

## Adding New Files

### Checklist: Adding a File

1. **Determine responsibility**: Business logic, UI, or CLI?
2. **Choose package**: `internal/stack/`, `internal/tui/`, or `cmd/`.
3. **Name appropriately**: Lowercase, descriptive, single concept.
4. **Add package comment**: Document package purpose (first file only).
5. **Follow structure**: Imports, types, constructors, methods.
6. **Add tests**: Create corresponding `*_test.go` file.
7. **Update documentation**: Add to relevant README or ADR if significant.

### Example: Adding New Business Logic

**Scenario**: Adding command execution logic.

**Steps**:

1. **Create file**: `internal/stack/executor.go` (or `internal/executor/` if complex).

2. **Add package comment**:

   ```go
   // Package executor handles Terragrunt command execution.
   package executor
   ```

3. **Define types**:

   ```go
   type Executor struct {
       workingDir string
   }
   ```

4. **Add constructor**:

   ```go
   func NewExecutor(workingDir string) *Executor {
       return &Executor{workingDir: workingDir}
   }
   ```

5. **Create tests**: `internal/executor/executor_test.go`.

6. **Update docs**: Mention in ADR or CLAUDE.md if architecturally significant.

## File Structure Template

### Go Source File Template

```go
// Package <name> provides <brief description>.
//
// <Optional longer description>
package <name>

import (
    // Standard library
    "fmt"
    "os"

    // Third-party
    tea "github.com/charmbracelet/bubbletea"

    // Internal
    "github.com/israoo/terrax/internal/stack"
)

// Constants
const (
    defaultValue = 42
)

// Type definitions
type Example struct {
    field string
}

// Constructor
func NewExample(field string) *Example {
    return &Example{field: field}
}

// Methods
func (e *Example) Method() {
    // Implementation
}

// Package-level functions
func HelperFunction() {
    // Implementation
}
```

### Test File Template

```go
package <name>

import (
    "testing"
)

func TestExample_Method(t *testing.T) {
    tests := []struct {
        name     string
        input    string
        expected string
    }{
        {
            name:     "basic case",
            input:    "test",
            expected: "result",
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            e := NewExample(tt.input)
            result := e.Method()
            if result != tt.expected {
                t.Errorf("expected %s, got %s", tt.expected, result)
            }
        })
    }
}
```

## Anti-Patterns to Avoid

### ❌ God Packages

Don't create packages that do everything:

```text
❌ internal/util/
    ├── everything.go          # 5000 lines of unrelated code
```

### ❌ Circular Dependencies

Don't create import cycles:

```go
// ❌ BAD
// internal/stack/tree.go imports internal/tui/model.go
// internal/tui/model.go imports internal/stack/tree.go
```

### ❌ Mixing Concerns in Files

Don't mix business logic and UI in one file:

```go
// ❌ BAD: internal/tui/model.go
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
    // UI logic mixed with tree traversal, filesystem scanning, etc.
}
```

### ❌ Deep Nesting

Don't create deeply nested package structures:

```text
❌ internal/core/business/domain/logic/stack/operations/tree/builder/
```

## References

- [ADR-0004: Separation of Concerns](../adr/0004-separation-of-concerns.md)
- [Go Coding Standards](go-coding-standards.md)
- [TerraX CLAUDE.md](../../CLAUDE.md)
