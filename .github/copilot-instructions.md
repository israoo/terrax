# GitHub Copilot Instructions - TerraX

## Architecture Overview

**Strict 3-Layer Separation of Concerns:**
- `cmd/root.go` - CLI orchestration only (coordinates TUI, config, output). **No business logic.**
- `internal/stack/` - Pure domain logic (tree navigation, path resolution). **ZERO Bubble Tea dependencies.**
- `internal/tui/` - Presentation layer (Bubble Tea Model-Update-View). **Delegates to Navigator.**

## Critical Workflows

**Development:**
```bash
make build          # Build ./terrax binary
make run            # Run directly
make test           # Run all tests
make test-coverage  # Show per-file/function coverage stats (console only)
```

**Testing Strategy:**
- Use `afero.MemMapFs` for filesystem isolation in tests (see `internal/stack/tree_test.go`)
- Use `teatest` for Bubble Tea TUI testing (see `internal/tui/model_test_helpers.go`)
- Navigator tests must NOT import Bubble Tea - pure unit tests only

## Project-Specific Patterns

**Sliding Window Navigation:**
- Max **3 navigation columns** visible simultaneously (+ 1 commands column)
- See `internal/tui/model.go`: `navigationOffset` field implements sliding
- Dynamic columns: never show empty columns (see `calculateMaxVisibleLevel`)

**Navigator Pattern:**
- `stack.Navigator` encapsulates ALL tree traversal logic
- `stack.NavigationState` holds current selection state (columns, indices, nodes)
- TUI Model delegates to `navigator.PropagateSelection(navState)` - never manipulates tree directly
- See `internal/stack/navigator.go` for complete interface

**Bubble Tea Model-Update-View:**
- `Model` holds ONLY UI state (focus, offset, dimensions)
- Business state lives in `Navigator` + `NavigationState`
- `Update()` handles input, delegates to Navigator methods
- `View()` uses `LayoutCalculator` + `Renderer` for separation (see `internal/tui/view.go`)

**I/O Separation:**
- `stdout` - Data output (selected paths, JSON)
- `stderr` - UI rendering (Bubble Tea writes here)
- See `cmd/root.go`: `tea.NewProgram(model, tea.WithOutput(os.Stderr))`

## Code Conventions

**Imports:**
```go
// 1. Standard library
// 2. External packages
// 3. Internal packages (github.com/israoo/terrax/...)
```

**Cross-Platform Paths:**
- Always use `filepath.Join()` - never hardcode `/` or `\`
- See `pkg/pathutils/` for path manipulation utilities

**Context Usage:**
- First parameter for I/O operations: `func LoadStack(ctx context.Context, path string) (*Node, error)`

**Error Handling:**
- Wrap errors with context: `fmt.Errorf("failed to load stack: %w", err)`
- Return errors up to caller - no panics in library code

## Key References

- **CLAUDE.md** - Complete architectural patterns and testing strategies
- **internal/stack/navigator.go** - Business logic delegation example
- **internal/tui/model.go** - Bubble Tea Model pattern implementation
- **internal/tui/view.go** - LayoutCalculator + Renderer separation
- **.claude/agents/agent-developer.md** - Agent governance and creation patterns

---

**Read CLAUDE.md first** for deep architectural context. This file provides quick orientation.
