# ADR-0025: On-Demand Plan Summary Subcommand

**Status**: Accepted

**Date**: 2026-06-25

**Deciders**: TerraX Core Team

**Related**:
- [ADR-0010: Plan Analysis Workflow](0010-plan-analysis-workflow.md)
- [ADR-0018: Plan Analysis via --json-out-dir](0018-plan-analysis-via-json-out-dir.md)
- [ADR-0024: CLI Subcommand Reorganization](0024-cli-subcommand-reorganization.md)

## Context

`plan.Summarize` — the terminal summary that groups stacks into "no changes" and "pending changes" with per-stack resource counts — was only reachable automatically, triggered by `plan.summary_enabled: true` in `.terrax.yaml` after running `plan` from the TUI. Two gaps arose:

1. **No on-demand access**: a user who ran `plan` with `plan.summary_enabled: false` (the default), or who opened the existing plan files later, had no way to view the summary without re-running the full Terragrunt plan.
2. **Asymmetry with `terrax review`**: the interactive plan review TUI gained its own subcommand (`terrax review`) in ADR-0024. The terminal summary — a lighter, non-interactive counterpart — had no equivalent.

The existing `runPlanSummary` helper in `cmd/root.go` encapsulates the post-plan automatic flow and must not be exposed as a subcommand entry point directly, because its signature (`stackPath, repoRoot`) couples it to the automatic execution context.

## Decision

Add `terrax summary` as a standalone subcommand in `cmd/summary.go` that prints the terminal plan summary from existing `.terrax/plans/` JSON files without re-running the plan.

The subcommand follows the pattern established by `terrax review`:

```
terrax summary [--dir <path>]
```

- `--dir` points to the project root (same semantics as `terrax review --dir`).
- Without `--dir`, the current working directory is used.

Implementation flow in `runSummaryCmd`:

```go
func runSummaryCmd(cmd *cobra.Command, args []string) error {
    ctx := context.Background()

    dirFlag, _ := cmd.Flags().GetString("dir")
    workDir, err := getWorkingDirectory(dirFlag)
    // ...
    ensureConfigFromWorkDir(workDir)
    workDir = resolveWorkDir(workDir)

    rootConfigFile := viper.GetString("root_config_file")
    if rootConfigFile == "" {
        rootConfigFile = config.DefaultRootConfigFile
    }

    repoRoot := deps.FindRepoRoot(workDir, rootConfigFile)
    jsonDir := filepath.Join(repoRoot, config.DefaultJSONOutDir)

    _, err = plan.Summarize(ctx, jsonDir, repoRoot)
    return err
}
```

`runSummaryCmd` calls `plan.Summarize` directly rather than delegating to the existing `runPlanSummary` helper. The helper remains unchanged in `root.go` for the automatic post-plan flow. The three lines of logic duplication are intentional: the two contexts (automatic vs. on-demand) are invoked differently and should not share an internal entry point.

If `.terrax/plans/` does not exist or contains no JSON files, `plan.Summarize` returns `(0, nil)` — the subcommand exits with code 0 and no output.

## Consequences

### Positive

- Users can inspect plan results at any time without re-running `plan`, matching the ergonomics of `terrax review`.
- Enables CI pipelines to print the summary step as a separate stage after `terrax run plan --dir`.
- `plan.summary_enabled` can stay `false` (the default) for teams that prefer the summary on demand rather than automatic.

### Negative

- `runPlanSummary` in `root.go` and the body of `runSummaryCmd` share three lines of logic. The duplication is small but real; a future change to how `jsonDir` is derived must be applied in both places.
- `terrax summary` returns exit code 0 even when pending changes exist. Callers who want to fail CI on pending changes must inspect the output rather than checking the exit code.

## Alternatives Considered

### Option 1: Expose `runPlanSummary` as the subcommand entry point

**Description**: Register `runPlanSummary` directly as the `RunE` handler, adapting its `(stackPath, repoRoot)` signature to accept the working directory instead.

**Pros**:

- No duplication of the three-line setup.

**Cons**:

- `runPlanSummary` is called from the automatic post-plan flow with pre-computed `stackPath` and `repoRoot`. Changing its signature to accept a raw directory would require conditional logic or an overload, complicating a function that is currently simple and stable.

**Why rejected**: The automatic flow and the on-demand subcommand have different callers and different inputs. Merging them into a single function introduces coupling that would need to be untangled as the two use cases inevitably diverge.

### Option 2: Add `--summary` flag to `terrax review`

**Description**: `terrax review --summary` prints the terminal summary instead of opening the TUI.

**Pros**:

- No new subcommand; fewer entry points.

**Cons**:

- A flag that changes what the command does (TUI vs. terminal output) rather than how it does it violates the flag-as-modifier principle established in ADR-0024. The two outputs are different enough in kind (interactive TUI vs. plain text) that they warrant separate commands.

**Why rejected**: The project just completed a reorganization (ADR-0024) specifically to prevent flags from determining behavior. Adding `--summary` to `terrax review` would immediately reintroduce the anti-pattern.

## Future Enhancements

**Potential Improvements**:

1. A non-zero exit code when stacks with pending changes exist would make `terrax summary` directly usable as a CI gate without parsing output.
2. A `--json` flag on `terrax summary` (consistent with `terrax history --json` and `terrax tree --json`) would expose the summary data for external tooling and the VS Code extension.

## References

- [`cmd/summary.go`](../../cmd/summary.go)
- [`internal/plan/summarizer.go`](../../internal/plan/summarizer.go)
- [Summary Subcommand Design Spec](../superpowers/specs/2026-06-25-summary-subcommand-design.md)
