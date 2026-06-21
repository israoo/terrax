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

task ext:install    # Install VS Code extension dependencies (pnpm)
task ext:build      # Compile extension TypeScript
task ext:package    # Package extension as .vsix
```

**Before committing:** `task check`

## Architecture

Strict Separation of Concerns — business logic, UI state, and rendering never mix.

```
terrax/
├── cmd/
│   ├── root.go              # CLI orchestration only (Cobra/Viper)
│   ├── tree.go              # terrax tree --json subcommand
│   ├── run.go               # terrax run <command> --dir subcommand
│   └── history.go           # terrax history --dir subcommand
├── internal/
│   ├── config/
│   │   └── defaults.go      # Configuration defaults (commands, limits)
│   ├── deps/
│   │   └── parser.go        # Static HCL dependency parser (stdlib only)
│   ├── executor/
│   │   └── executor.go      # Builds and runs Terragrunt CLI commands
│   ├── history/
│   │   └── history.go       # Execution history (JSONL, XDG Base Directory)
│   ├── plan/
│   │   ├── collector.go     # Runs `terragrunt plan -json`, parses output
│   │   ├── models.go        # PlanReport, StackResult, ChangeType types
│   │   └── tree.go          # Builds display tree from plan results
│   ├── stack/
│   │   ├── tree.go          # Node struct with Dependencies/Dependents/InCycle fields
│   │   ├── builder.go       # Filesystem scanning, FindAndBuildTree
│   │   ├── graph.go         # AnalyzeGraph: cycle detection + reverse dependency graph
│   │   └── navigator.go     # Navigation logic — ZERO Bubble Tea dependencies
│   └── tui/
│       ├── model.go         # UI state only; delegates navigation to Navigator
│       ├── update.go        # Bubble Tea Update: keyboard/mouse event handling
│       ├── update_plan.go   # Update logic specific to StatePlanReview
│       ├── view.go          # View entry point; dispatches to sub-renderers
│       ├── view_common.go   # Shared rendering helpers (headers, footers)
│       ├── view_history.go  # Renders StateHistory mode
│       ├── view_navigation.go # Renders StateNavigation mode (sliding window)
│       ├── view_plan.go     # Renders StatePlanReview mode
│       └── styles.go        # Lipgloss styles, colors, UI dimensions
├── extensions/
│   └── vscode/              # VS Code companion extension (TypeScript/pnpm)
│       └── src/
│           ├── extension.ts         # Activation, command registration
│           ├── treeProvider.ts      # StackNode interface + Stacks panel
│           ├── dependencyProvider.ts # Dependencies + Dependents panels
│           ├── historyProvider.ts   # History panel
│           └── terminalRunner.ts    # Terminal reuse + q+Ctrl+U pattern
└── main.go
```

### Layer Rules (MANDATORY)

- **`internal/deps/`** — stdlib only; no viper, cobra, or UI imports
- **`internal/stack/`** — pure business logic, no UI imports
- **`internal/tui/model.go`** — UI state only (focus, offsets, dimensions); delegates to Navigator
- **`internal/tui/view.go`** — pure rendering, never modifies state
- **`cmd/root.go`** — CLI glue only; injects TUI via `TUIRunner` interface (enables unit testing without a terminal)

## Architectural Patterns (MANDATORY)

### AppState Tri Mode

Model has three modes via `AppState`:
- `StateNavigation` — normal TUI tree navigation (`NewModel()`)
- `StateHistory` — history viewer, activated via `--history` (`NewHistoryModel()`)
- `StatePlanReview` — plan analysis view, activated after running `plan` command

Never mix logic between modes; each has its own `update_*.go` and `view_*.go` counterpart.

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

### CLI as Local API

`terrax tree --json`, `terrax history`, and `terrax run` form a JSON API consumed by the VS Code extension via `spawnSync`. All business logic lives in Go; the extension is a thin client. When adding features visible in VS Code, prefer a new Go subcommand over TypeScript logic.

### stack.Node JSON fields

`Node` (`internal/stack/tree.go`) outputs: `name`, `path`, `isStack`, `children`, `depth`, `dependencies` (direct dep absolute paths), `dependents` (reverse deps), `inCycle` (bool). `AnalyzeGraph` in `graph.go` populates `dependents` and `inCycle` after `FindAndBuildTree`.

### Leaf stack auto-navigation

`resolveWorkDir` in `cmd/root.go` redirects `--dir` to the parent when the target is a leaf stack (has `terragrunt.hcl` but no sub-stacks). Applied in `runTUI` only — `terrax run` targets the exact path given.

### VS Code extension pattern

Extension lives in `extensions/vscode/`. All calls use `spawnSync` with 10s timeout. Terminal reuse uses `q` (bare, no `\r`) + 300ms + `Ctrl+U` to close a live TUI and clear the readline buffer before sending the next command. Build/test cycle: `task ext:build` then `task ext:package` then `code --install-extension extensions/vscode/terrax-vscode-0.1.0.vsix`.

## Common Task Guides

**Modify TUI layout:** `LayoutCalculator` in `view.go` → layout math; `styles.go` → dimensions/colors.

**Add navigation feature:** Business logic in `navigator.go` → wire in `model.go` → render in `view.go`.

**Debug TUI issues:** Log to a temp file (never stdout/stderr in TUI). Bubble Tea has a built-in debug mode.
