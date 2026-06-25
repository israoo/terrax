# CLI Subcommand Reorganization

**Date:** 2026-06-24
**Status:** Approved

## Problem

`terrax --last`, `terrax --history`, and `terrax --review` are flags on the root command that trigger entirely different modes of operation rather than modifying behavior. Flags should modify how a command runs, not determine what it does. This makes the CLI surface harder to discover and violates the principle of least surprise.

## Goal

Promote `--last`, `--history`, and `--review` to proper subcommands. Update `terrax history` (currently JSON-only) to be the interactive TUI viewer by default, with `--json` as an opt-in flag for external tool consumption (VS Code extension).

## Resulting CLI Surface

| Command | Flags | Description |
|---|---|---|
| `terrax` | `--dir` | Main TUI navigator |
| `terrax last` | — | Re-executes the last command from history |
| `terrax review` | `--dir` | Opens the plan review TUI |
| `terrax history` | `--dir`, `--json` | No `--json`: interactive TUI viewer · `--json`: JSON output |
| `terrax run <cmd>` | `--dir` | Run command on stack (unchanged) |
| `terrax tree` | `--dir`, `--json` | Stack tree output (unchanged) |
| `terrax find` | `--dir`, `--base` | Changed stacks (unchanged) |
| `terrax groups` | `--dir` | Stack groups (unchanged) |

### `--dir` rationale per command

- `terrax last` — no `--dir`. The execution path always comes from the stored history entry (absolute path). Adding `--dir` would only affect which project's history is queried, which is confusing and not a real use case.
- `terrax review` — has `--dir`. The plan JSON files are discovered relative to `repoRoot`, which is computed from the working directory. `--dir` lets the VS Code extension point to the correct project.
- `terrax history` — has `--dir`. Used to filter history entries by project root, same as today.

## File Changes

### `cmd/root.go`

- Remove flags: `--last`, `--history`, `--review`.
- Remove flag-checking branches from `runTUI` (`if lastFlag`, `if historyFlag`, `if reviewFlag`). The function becomes a pure TUI launcher.
- `executeLastCommand` moves to `cmd/last.go`.
- `runHistoryViewer` moves to `cmd/history.go`.
- `runPlanReview` stays in `root.go` (shared by `terrax review` subcommand and the post-plan flow in `runTUI`).

### `cmd/last.go` (new)

- Registers subcommand `terrax last`.
- No flags.
- Calls `executeLastCommand` (moved here from `root.go`).

### `cmd/review.go` (new)

- Registers subcommand `terrax review`.
- Flag: `--dir`.
- Calls `runPlanReview` (stays in `root.go`, called from here).

### `cmd/history.go` (modified)

- Adds flag `--json` (bool, default false).
- With `--json`: existing JSON output behavior (unchanged).
- Without `--json`: calls `runHistoryViewer` (moved here from `root.go`).

### `extensions/vscode/src/historyProvider.ts` (modified)

- Updates the `terrax history` call to `terrax history --json`.

## Testing

- Existing `cmd/history_test.go`, `cmd/root_test.go`, `cmd/run_test.go` must pass unchanged.
- New test files: `cmd/last_test.go`, `cmd/review_test.go` covering the new subcommands with the injected TUI runner pattern already used in the codebase.
- The `--json` flag path in `history.go` is covered by the existing history tests.

## Breaking Changes

- `terrax --last` → `terrax last` (flag removed)
- `terrax --history` → `terrax history` (flag removed)
- `terrax --review` → `terrax review` (flag removed)
- `terrax history` (JSON output) → `terrax history --json` (flag required for JSON)
- VS Code extension updated in the same change set; no external consumers outside this repo.
