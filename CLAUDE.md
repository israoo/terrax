# CLAUDE.md

Guidance for Claude Code when working with this repository.

## Project Overview

**TerraX** is a professional Terminal UI (TUI) for interactive and hierarchical navigation of Terragrunt stacks. Built with Go, Bubble Tea, and Lipgloss, it provides dynamic tree-based navigation for infrastructure management.

**Tech Stack:**

- Go 1.25.5
- Bubble Tea 1.3.10 (TUI framework)
- Lipgloss 1.1.0 (styling)
- Cobra 1.10.2 (CLI framework)

## Essential Commands

```bash
make build          # Build to ./build/terrax
make run            # Run directly
make clean          # Remove build artifacts
go build .          # Quick build
go test ./...       # Run all tests
```

## Architecture

TerraX follows strict **Separation of Concerns (SoC)** principles:

```text
terrax/
├── cmd/
│   └── root.go           # CLI entry point (lightweight wrapper)
├── internal/
│   ├── stack/
│   │   ├── tree.go       # Tree structure & filesystem scanning
│   │   └── navigator.go  # Business logic for tree navigation
│   └── tui/
│       ├── model.go      # Bubble Tea model (UI state only)
│       ├── view.go       # Rendering logic (Renderer pattern)
│       └── constants.go  # Constants and configuration
└── main.go               # Application entry point
```

### Layer Responsibilities

1. **`cmd/root.go`** - CLI wrapper
   - Minimal responsibility: parse args, coordinate initialization
   - Error handling at boundaries
   - No business logic

2. **`internal/stack/`** - Business logic
   - `tree.go`: Filesystem scanning, tree construction
   - `navigator.go`: Navigation logic, selection propagation
   - No UI concerns

3. **`internal/tui/`** - Presentation layer
   - `model.go`: UI state management (delegates to Navigator)
   - `view.go`: Rendering with LayoutCalculator and Renderer
   - `constants.go`: UI configuration
   - No business logic

## Architectural Patterns (MANDATORY)

### Bubble Tea Architecture (MANDATORY)

TerraX uses **Elm Architecture** via Bubble Tea. Strict adherence to Model-Update-View pattern:

**Model** (`internal/tui/model.go`):

- Holds UI state only (selections, focus, offsets, dimensions)
- Delegates business logic to `stack.Navigator`
- Never contains rendering logic

**Update** (`internal/tui/model.go`):

- Processes messages (key presses, window resize)
- Updates state via pure functions
- Returns updated model and optional commands

**View** (`internal/tui/view.go`):

- Pure rendering functions
- Uses `LayoutCalculator` for layout logic
- Uses `Renderer` for styled output
- Never modifies state

**CRITICAL**: Never mix concerns. UI state in Model, business logic in Navigator, rendering in View.

### Separation of Concerns (MANDATORY)

**Business Logic** (`internal/stack/navigator.go`):

- Tree traversal algorithms
- Selection propagation
- Breadcrumb generation
- Path resolution
- **ZERO Bubble Tea dependencies**

**UI State** (`internal/tui/model.go`):

- Focus management
- Viewport offsets
- Command selection
- Window dimensions
- Delegates navigation to Navigator

**Rendering** (`internal/tui/view.go`):

```go
// LayoutCalculator: Pure layout calculations
type LayoutCalculator struct{}
func (lc LayoutCalculator) CalculateVisibleColumns(...) (int, int) { ... }

// Renderer: Styled rendering with Lipgloss
type Renderer struct{}
func (r Renderer) RenderColumn(...) string { ... }
```

### Navigator Pattern (MANDATORY)

Navigator encapsulates all tree navigation logic:

```go
// Navigator provides tree navigation operations.
type Navigator struct {
    root     *Node
    maxDepth int
}

// Core operations
func (n *Navigator) GetChildrenAtDepth(state *NavigationState, depth int) []*Node
func (n *Navigator) PropagateSelection(state *NavigationState)
func (n *Navigator) GenerateBreadcrumbs(state *NavigationState) []string
func (n *Navigator) GetSelectedPath(state *NavigationState) string
```

**Benefits**:

- Business logic isolated from UI
- Testable without Bubble Tea
- Reusable across different UIs
- Clear contracts via interfaces

### Sliding Window Pattern (MANDATORY)

TerraX implements a **sliding window** for deep hierarchies (maxDepth > 3):

**Concept**:

- Always show **max 3 navigation columns** + 1 commands column
- Window slides right as user navigates deeper
- Left-most visible level = `navigationOffset`
- Visible range: `[offset, offset+3)`

**Implementation** (`internal/tui/model.go`):

```go
// When moving right beyond window
if m.focusedColumn >= maxVisibleNavColumns {
    m.navigationOffset++
    m.focusedColumn = maxVisibleNavColumns - 1
}

// When moving left before window
if m.focusedColumn < 1 && m.navigationOffset > 0 {
    m.navigationOffset--
    m.focusedColumn = 1
}
```

**Layout Calculation** (`internal/tui/view.go`):

```go
func (lc LayoutCalculator) CalculateVisibleColumns(
    maxDepth, offset int,
) (startDepth, endDepth int) {
    startDepth = offset
    endDepth = min(offset+maxVisibleNavColumns, maxDepth)
    return
}
```

### Dynamic Column Display (MANDATORY)

Columns appear/disappear based on navigation context:

**Rules**:

1. Commands column always visible (column 0)
2. Navigation columns appear when children exist at that depth
3. Empty columns never rendered
4. Breadcrumbs show full path (not limited by sliding window)

**Benefits**:

- No visual clutter from empty columns
- Efficient use of terminal space
- Clear indication of navigation depth

## Code Patterns & Conventions

### Comment Style (MANDATORY)

All comments must end with periods.

### Comment Preservation (MANDATORY)

**NEVER delete existing comments without a very strong reason.** Comments document why/how/what/where.

**Guidelines**: Preserve helpful comments, update to match code, refactor for clarity, add context when modifying.

**Acceptable removals**: Factually incorrect, code removed, duplicates obvious code, outdated TODO completed.

### Import Organization (MANDATORY)

Three groups separated by blank lines, sorted alphabetically:

1. Go stdlib
2. Third-party packages
3. TerraX internal packages (`github.com/israoo/terrax/...`)

**Example**:

```go
import (
    "fmt"
    "os"
    
    tea "github.com/charmbracelet/bubbletea"
    "github.com/charmbracelet/lipgloss"
    "github.com/spf13/cobra"
    
    "github.com/israoo/terrax/internal/stack"
    "github.com/israoo/terrax/internal/tui"
)
```

### File Organization (MANDATORY)

- Keep files focused and reasonably sized (<500 lines ideally)
- One major concept per file
- Co-locate tests with implementation (`model_test.go` next to `model.go`)
- Use `constants.go` for shared constants

## Bubble Tea Best Practices

### Model Design (MANDATORY)

**DO**:

- Keep Model struct simple and focused on UI state
- Delegate business logic to separate packages
- Use clear field names with comments
- Initialize with constructor function (`NewModel()`)

**DON'T**:

- Mix business logic in Update methods
- Include rendering logic in Model
- Use complex nested structs in Model
- Expose internal fields unnecessarily

### Update Method (MANDATORY)

**Pattern**:

```go
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
    switch msg := msg.(type) {
    case tea.KeyMsg:
        return m.handleKeyPress(msg)
    case tea.WindowSizeMsg:
        return m.handleWindowResize(msg), nil
    }
    return m, nil
}
```

**Rules**:

- Handle each message type in separate method
- Return updated model (immutable pattern)
- Return commands for side effects (none in TerraX currently)
- Keep Update method clean and delegating

### View Rendering (MANDATORY)

**Separation**:

- `View()` orchestrates rendering
- `LayoutCalculator` handles layout math
- `Renderer` handles Lipgloss styling

**Pattern**:

```go
func (m Model) View() string {
    if !m.ready {
        return "Initializing..."
    }
    
    calc := LayoutCalculator{}
    renderer := Renderer{}
    
    // Calculate layout
    startDepth, endDepth := calc.CalculateVisibleColumns(...)
    
    // Render components
    columns := renderer.RenderColumns(...)
    breadcrumbs := renderer.RenderBreadcrumbs(...)
    
    return lipgloss.JoinVertical(...)
}
```

### Lipgloss Styling (MANDATORY)

**Principles**:

- Define base styles as package-level variables
- Use `Copy()` for variations
- Apply styles functionally (don't mutate)
- Keep color palette in `constants.go`

**Example** (`internal/tui/constants.go`):

```go
var (
    ColorPrimary   = lipgloss.Color("86")   // Cyan
    ColorSecondary = lipgloss.Color("213")  // Pink
    ColorMuted     = lipgloss.Color("241")  // Gray
    
    BaseColumnStyle = lipgloss.NewStyle().
        Width(ColumnWidth).
        Padding(0, 1)
    
    FocusedStyle = BaseColumnStyle.Copy().
        BorderStyle(lipgloss.RoundedBorder()).
        BorderForeground(ColorPrimary)
)
```

## Testing Strategy

### Unit Tests (MANDATORY)

**Focus areas**:

1. Navigator logic (selection, propagation, breadcrumbs)
2. Tree building from filesystem
3. Layout calculations
4. Message handling logic

**Pattern**:

```go
func TestNavigator_PropagateSelection(t *testing.T) {
    tests := []struct {
        name     string
        setup    func() (*Navigator, *NavigationState)
        expected []int
    }{
        // Table-driven test cases
    }
    
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            nav, state := tt.setup()
            nav.PropagateSelection(state)
            // Assertions
        })
    }
}
```

### Integration Tests

TerraX currently uses manual testing with sample directory structures. Consider adding:

- Fixture-based tests with known directory trees
- Golden file tests for rendering output
- E2E tests with Bubble Tea test helpers

## Development Workflow

### Before Coding

1. **Understand the architecture**: Review relevant files in `internal/`
2. **Check existing patterns**: Search for similar implementations
3. **Plan the change**: Identify which layer(s) are affected

### While Coding

1. **Maintain separation**: Business logic ≠ UI logic ≠ Rendering
2. **Follow conventions**: Imports, comments, file organization
3. **Write tests**: As you code, not after
4. **Compile frequently**: `go build .` to catch errors early

### Before Committing

```bash
go build .              # Must compile
go test ./...           # All tests pass
go fmt ./...            # Format code
make build              # Final build verification
```

## Common Tasks

### Adding a New Command

Currently TerraX has a simple structure with one command (the TUI itself). To add new commands:

1. Create new file in `cmd/` (e.g., `cmd/validate.go`)
2. Define Cobra command
3. Add to `rootCmd` in `cmd/root.go`
4. Implement business logic in `internal/`
5. Add tests

### Modifying TUI Layout

1. **Layout changes**: Modify `LayoutCalculator` in `internal/tui/view.go`
2. **Style changes**: Update constants in `internal/tui/constants.go`
3. **New UI elements**: Add rendering method to `Renderer`
4. **Test**: Run `make run` with various terminal sizes

### Adding Navigation Features

1. **Business logic**: Implement in `internal/stack/navigator.go`
2. **UI integration**: Update `Model` and `Update` in `internal/tui/model.go`
3. **Rendering**: Add rendering in `internal/tui/view.go` if needed
4. **Test**: Unit test Navigator, manual test TUI

### Debugging TUI Issues

**Common issues**:

- **Layout broken**: Check `LayoutCalculator` logic and window size handling
- **Selection not working**: Debug `Navigator.PropagateSelection`
- **Rendering glitches**: Review Lipgloss style application
- **Performance**: Profile with `go test -bench` or `pprof`

**Debugging tips**:

- Add temporary log file output (avoid stdout/stderr in TUI)
- Use Bubble Tea's built-in debugging mode
- Test with different terminal sizes
- Verify breadcrumbs match actual selection

## Critical Requirements

### Cross-Platform (MANDATORY)

- Use `filepath.Join()` for paths, never hardcoded `/` or `\`
- Test on Linux, macOS, and Windows
- Use Go stdlib for filesystem operations
- Handle terminal differences gracefully

### Error Handling (MANDATORY)

**Pattern**:

```go
// Wrap errors with context
if err != nil {
    return fmt.Errorf("failed to build tree: %w", err)
}

// Handle at boundaries
if err := cmd.Execute(); err != nil {
    fmt.Fprintf(os.Stderr, "Error: %v\n", err)
    os.Exit(1)
}
```

**Rules**:

- Always wrap errors with context
- Handle errors at function boundaries
- Never ignore errors silently
- Provide actionable error messages

### Performance

**Current optimizations**:

- Single filesystem scan on startup (no repeated I/O)
- Pre-calculated column widths
- Efficient tree traversal with NavigationState
- Minimal re-renders

**Watch out for**:

- Deep recursion in large directory trees
- Excessive string allocations in rendering
- Unnecessary state copies in Update

### Git Workflow (MANDATORY)

**Do commit**:

- Source code changes
- Test files
- Documentation updates
- Build configuration

**Don't commit**:

- Binary files (`terrax` executable)
- Temporary files
- Debug logs
- IDE-specific files
- Personal notes

**Commit messages**:

- Use conventional commits format
- Be descriptive and specific
- Reference issues when applicable

## Development Environment

**Prerequisites**:

- Go 1.25+ (check `go.mod` for exact version)
- Make (for build automation)
- Git

**Recommended**:

- VS Code with Go extension
- Terminal with true color support
- `golangci-lint` for linting

**Build**:

```bash
make build    # Production build
make run      # Run directly
make clean    # Clean artifacts
```

**Testing**:

```bash
go test ./...           # Run all tests
go test -v ./...        # Verbose output
go test -cover ./...    # With coverage
```

## Key Principles Summary

1. **Separation of Concerns** - Business logic, UI state, rendering are separate
2. **Bubble Tea Architecture** - Strict Model-Update-View pattern
3. **Navigator Pattern** - Encapsulate tree navigation logic
4. **Sliding Window** - Handle deep hierarchies efficiently
5. **Dynamic Columns** - Show only relevant columns
6. **Clear Comments** - Document why, not just what
7. **Test Coverage** - Unit tests for business logic
8. **Error Context** - Wrap errors with meaningful messages
9. **Cross-Platform** - Work on Linux, macOS, Windows
10. **Clean Commits** - Atomic, well-described changes
