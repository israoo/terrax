# ADR-0024: CLI Subcommand Reorganization

**Status**: Accepted

**Date**: 2026-06-24

**Deciders**: TerraX Core Team

**Related**:
- [ADR-0019: History Panel and Leaf Stack Auto-Navigation](0019-history-panel-and-leaf-stack-navigation.md)
- [ADR-0010: Plan Analysis Workflow](0010-plan-analysis-workflow.md)

## Context

TerraX grew three behavioral modes triggered via root-level flags: `--last` (re-execute last command from history), `--history` (open interactive history TUI), and `--review` (open plan review TUI). These flags violated the POSIX convention that flags modify how a command runs rather than determine what it does.

Three concrete problems arose:

1. **Discoverability**: `terrax --help` listed `--last`, `--history`, and `--review` alongside modifiers like `--dir`, giving no signal that they invoke entirely different behaviors.
2. **Inconsistency**: `terrax history` already existed as a subcommand (JSON output for the VS Code extension), yet `--history` opened the interactive TUI — two unrelated behaviors under the same name in different positions.
3. **Testability**: `runHistoryViewer` (the `--history` implementation) lived inside `runTUI` alongside the navigation flow, making it impossible to test in isolation without also exercising the main TUI path.

## Decision

Promote `--last`, `--history`, and `--review` to proper subcommands. Convert `terrax history` from a JSON-only command to the interactive TUI by default, with `--json` as an explicit opt-in for machine-readable output.

**Resulting CLI surface:**

| Command | Flags | Description |
|---|---|---|
| `terrax` | `--dir` | Main TUI navigator |
| `terrax last` | — | Re-execute last command from history |
| `terrax review` | `--dir` | Open plan review TUI |
| `terrax history` | `--dir`, `--json` | TUI viewer (default) or JSON output (`--json`) |
| `terrax run <cmd>` | `--dir` | Execute command on stack (unchanged) |
| `terrax tree` | `--dir`, `--json` | Stack tree output (unchanged) |
| `terrax find` | `--dir`, `--base` | Changed stacks (unchanged) |
| `terrax groups` | `--dir` | Stack groups (unchanged) |

**`--dir` is omitted from `terrax last`** because the execution path always comes from the stored history entry (absolute path). Accepting `--dir` there would only affect which project's history is queried, which is misleading — the user would expect it to affect where execution happens.

**Implementation layout (`cmd/` package):**

```
cmd/
├── execute.go       # shared reExecuteHistoryEntry helper
├── history.go       # terrax history; HistoryTUIRunner injection pattern
├── last.go          # terrax last; executeLastCommand moved here
├── review.go        # terrax review; delegates to runPlanReview in root.go
└── root.go          # terrax (main TUI); runPlanReview stays here (shared by post-plan flow)
```

`runPlanReview` remains in `root.go` because it is called from both `terrax review` and the post-`plan` execution flow in `runTUI`.

The re-execution pipeline (AbsolutePath fallback → force-unlock branch → `collectTransitiveDeps` → `buildGroupedExecution` loop → plan summary/review) is extracted into `reExecuteHistoryEntry` in `execute.go` to eliminate the three independent copies that previously existed in `root.go`, `last.go`, and `history.go`.

**Testability pattern for history TUI:** `history.go` follows the `currentTUIRunner` / `setTUIRunner` pattern established in `root.go` by introducing `currentHistoryTUIRunner` / `setHistoryTUIRunner`. Tests inject a no-op runner to prevent Bubble Tea from opening a real terminal when `go test` inherits a TTY from an interactive shell (e.g., via lefthook pre-commit hooks).

**VS Code extension update:** `historyProvider.ts` is updated to call `terrax history --json --dir <path>` instead of `terrax history --dir <path>`.

## Consequences

### Positive

- Flags now exclusively modify behavior; subcommands determine what runs. `terrax --help` reflects this cleanly.
- `terrax history` matches user expectation: the primary use is interactive, with `--json` as an explicit machine-mode opt-in (consistent with `terrax tree --json`).
- `terrax last`, `terrax review`, and `terrax history` are each independently testable via their own `*_test.go` files with injected runners.
- The extracted `reExecuteHistoryEntry` eliminates a three-way code duplication that had already diverged (stdout vs. stderr for the re-execution banner).

### Negative

- **Breaking change for existing users**: Scripts or aliases using `terrax --last`, `terrax --history`, or `terrax --review` must be updated. There is no deprecation period — the flags are removed outright.
- **`terrax history` behavior change**: Any existing automation calling `terrax history` and parsing JSON output will break until updated to `terrax history --json`. The VS Code extension is updated in the same changeset; external consumers are not.
- **`--dir` absent from `terrax last`**: Users who expect all subcommands to accept `--dir` may find this inconsistent. The rationale (execution path from history, not cwd) requires understanding internal state.

## Alternatives Considered

### Option 1: Keep flags, add subcommands as aliases

**Description**: Retain `--last`, `--history`, and `--review` on the root command and add subcommands as aliases pointing to the same logic, so both forms work simultaneously.

**Pros**:

- No breaking change for existing users.
- Gradual migration path.

**Cons**:

- Two surfaces to maintain and document indefinitely.
- The core problem (flags that determine behavior rather than modify it) is never resolved — it becomes permanent.

**Why rejected**: The dual surface does not address the discoverability or conceptual problems; it adds maintenance overhead while preserving the confusion. TerraX has no stable external API contract for these flags, so the migration cost is low and the clean break is preferable.

### Option 2: Rename `terrax history` (JSON) to a different subcommand

**Description**: Keep `terrax --history` as a flag for the TUI; rename the existing `terrax history` (JSON) to `terrax log` or `terrax history export`, freeing the name for the TUI.

**Pros**:

- `--history` flag behavior is unchanged for existing users.
- The JSON subcommand gets a more explicit name.

**Cons**:

- The VS Code extension and any external tooling must be updated regardless.
- The `--history` flag problem persists — it still violates the flag-as-modifier principle.
- `terrax log` or `terrax history export` introduces new naming that has no precedent in the existing CLI.

**Why rejected**: Since the VS Code extension must be updated either way, there is no practical advantage to preserving the flag. Introducing `terrax log` or a nested `history export` adds surface without solving the root issue.

### Option 3: `terrax history` subcommand with `--interactive` flag

**Description**: Keep `terrax history` as JSON by default, add `--interactive` flag to open the TUI.

**Pros**:

- No behavior change for the existing JSON consumers.
- Explicit about when the TUI is opened.

**Cons**:

- `--interactive` on a command that is already a subcommand is redundant — it's a flag determining what happens, not modifying how.
- The default (JSON) is the less common use case for a human operator; the TUI should be the default for usability.

**Why rejected**: The constraint requires that flags modify behavior and subcommands determine it. `--interactive` repeats the same mistake in the opposite direction. Making JSON the default also optimizes for the machine consumer (VS Code extension) rather than the human operator, which inverts the intended ergonomics.

## Future Enhancements

**Potential Improvements**:

1. A `--last N` or `terrax last --nth 2` form could let users re-execute the Nth-most-recent command, not just the latest.
2. `terrax review --dir` currently passes the directory straight to `runPlanReview`. A future improvement could accept a stack path directly rather than requiring the user to know the repo root convention.

## References

- [CLI Subcommand Design Spec](../superpowers/specs/2026-06-24-cli-subcommands-design.md)
- [Implementation Plan](../superpowers/plans/2026-06-24-cli-subcommands.md)
- [`cmd/history.go`](../../cmd/history.go)
- [`cmd/last.go`](../../cmd/last.go)
- [`cmd/review.go`](../../cmd/review.go)
- [`cmd/execute.go`](../../cmd/execute.go)
