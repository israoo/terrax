# CLAUDE.md

Guidance for Claude Code when working in this repository.

## Project Overview

**TerraX** is a terminal UI (TUI) for interactive hierarchical navigation of Terragrunt stacks. Built with Go, Bubble Tea, and Lipgloss.

**Tech Stack:** Go 1.25.5 · Bubble Tea 1.3.10 · Lipgloss 1.1.0 · Cobra 1.10.2 · Viper 1.21.0 · Afero 1.15.0 · xdg 0.5.3

## Setup

This project uses [mise](https://mise.jdx.dev/) to manage Go, Task, golangci-lint, air, and goreleaser:

```bash
mise install   # Install all required tools
task init      # Install Go dependencies and build
```

## Essential Commands

```bash
task build          # Build to ./build/terrax
task run            # Build and run
task dev            # Build and run with hot reload (requires air)
task test           # Run tests with -race and coverage
task check          # fmt + vet + lint + test (full CI check)
task test-coverage  # Run tests and display per-file coverage
task clean          # Remove build artifacts

terrax --last       # Re-execute last command from history
terrax --history    # Open interactive history viewer
```

**Before committing:** `task check`

## Architecture

Strict Separation of Concerns — business logic, UI state, and rendering never mix.

```
terrax/
├── cmd/
│   └── root.go         # CLI orchestration only (Cobra/Viper)
├── internal/
│   ├── config/
│   │   └── defaults.go # Configuration defaults (commands, limits)
│   ├── history/
│   │   └── history.go  # Execution history (JSONL, XDG Base Directory)
│   ├── stack/
│   │   ├── tree.go     # Filesystem scanning, tree construction
│   │   └── navigator.go # Navigation logic — ZERO Bubble Tea dependencies
│   └── tui/
│       ├── model.go    # UI state only; delegates navigation to Navigator
│       ├── view.go     # Rendering: LayoutCalculator + Renderer (Lipgloss)
│       └── constants.go # Colors, key bindings, UI dimensions
└── main.go
```

### Layer Rules (MANDATORY)

- **`internal/stack/`** — pure business logic, no UI imports
- **`internal/tui/model.go`** — UI state only (focus, offsets, dimensions); delegates to Navigator
- **`internal/tui/view.go`** — pure rendering, never modifies state
- **`cmd/root.go`** — CLI glue only; injects TUI via `TUIRunner` interface (enables unit testing without a terminal)

## Architectural Patterns (MANDATORY)

### AppState Dual Mode

Model has two modes via `AppState`:
- `StateNavigation` — normal TUI tree navigation (`NewModel()`)
- `StateHistory` — history viewer, activated via `--history` (`NewHistoryModel()`)

Never add navigation logic to history mode or vice versa.

### Sliding Window

Deep hierarchies always show **max 3 nav columns** + 1 commands column. Window slides right as the user navigates deeper (`navigationOffset` tracks left edge). Configured via `max_navigation_columns` in `.terrax.yaml`.

### Per-Column Filtering

Press `/` in any navigation column to activate text filter. Filter state is per-column and persists across column switches.

### History Persistence

Execution history stored as JSONL in XDG Base Directory:
- macOS: `~/Library/Application Support/terrax/history.log`
- Linux: `~/.config/terrax/history.log`

Filtered by project root detection via `root_config_file` (default: `root.hcl`).

## Code Conventions (MANDATORY)

**Comments:** All comments must end with periods.

**Imports:** Three groups, alphabetically sorted:
1. Go stdlib
2. Third-party packages
3. `github.com/israoo/terrax/...` internal packages

**Lipgloss styles:** `Copy()` was removed in Lipgloss 1.x — define each style independently with `lipgloss.NewStyle()`.

**Errors:** Always wrap with context: `fmt.Errorf("failed to build tree: %w", err)`.

**Paths:** Always use `filepath.Join()`, never hardcoded `/` or `\`.

## Testing

Table-driven tests, Afero for filesystem mocking, no real terminal needed (TUIRunner interface).

```bash
go test ./...           # All tests
go test -v ./...        # Verbose
go test -cover ./...    # With coverage
```

Test files live alongside implementation (`model_test.go` next to `model.go`).

## Configuration

`.terrax.yaml` (searched in current dir, then `~/.terrax.yaml`):

```yaml
commands: [plan, apply, validate, fmt, init, output, refresh, destroy]
max_navigation_columns: 3
history:
  max_entries: 500
root_config_file: "root.hcl"
```

## Common Task Guides

**Modify TUI layout:** `LayoutCalculator` in `view.go` → layout math; `constants.go` → dimensions/colors.

**Add navigation feature:** Business logic in `navigator.go` → wire in `model.go` → render in `view.go`.

**Debug TUI issues:** Log to a temp file (never stdout/stderr in TUI). Bubble Tea has a built-in debug mode.
