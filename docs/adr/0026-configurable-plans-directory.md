# ADR-0026: Configurable Plans Directory

**Status**: Accepted

**Date**: 2026-06-25

**Deciders**: TerraX Core Team

**Related**:
- [ADR-0018: Plan Analysis via --json-out-dir](0018-plan-analysis-via-json-out-dir.md)
- [ADR-0025: On-Demand Plan Summary Subcommand](0025-summary-subcommand.md)
- [ADR-0011: Extensible Flags Configuration](0011-extensible-flags-configuration.md)

## Context

The JSON plan output directory — the path passed to Terragrunt's `--json-out-dir` and read back by `terrax review` and `terrax summary` — was hardcoded as `config.DefaultJSONOutDir` (`.terrax/plans`) throughout the codebase. This created three practical problems:

1. **Parallel CI runs**: teams running multiple plan contexts simultaneously in CI must write to distinct directories to avoid cross-contamination. With a hardcoded path, every job writes to `.terrax/plans` and overwrites the others.
2. **Repository-external storage**: organizations that store Terraform plan artifacts outside the repository (e.g., in a shared network mount or a CI artifact store) had no way to redirect output without forking the binary.
3. **Asymmetry with other flags**: `terrax run` already accepted `--dir` to override the working directory; `terrax review` and `terrax summary` accepted `--dir`; but no command offered a way to override where plan files are written or read.

## Decision

Make the plans directory configurable via a `plan.json_out_dir` Viper key with three-tier priority: CLI flag (`--plans-dir`) > YAML config > default (`.terrax/plans`).

**Viper propagation pattern:** each affected command applies its `--plans-dir` flag to Viper explicitly before invoking downstream logic:

```go
if plansDir, _ := cmd.Flags().GetString("plans-dir"); plansDir != "" {
    viper.Set("plan.json_out_dir", plansDir)
}
```

Downstream consumers (`executor.buildFilterArgs`, `runPlanSummary`, `runPlanReview`, `runSummaryCmd`) read `viper.GetString("plan.json_out_dir")` and apply a consistent path resolution rule:

```go
jsonOutDir := viper.GetString("plan.json_out_dir")
if jsonOutDir == "" {
    jsonOutDir = config.DefaultJSONOutDir
}
var absDir string
if filepath.IsAbs(jsonOutDir) {
    absDir = jsonOutDir
} else {
    absDir = filepath.Join(repoRoot, jsonOutDir)
}
```

**Commands receiving `--plans-dir`:**

| Command | Role |
|---|---|
| `terrax [--plans-dir]` | Generator — TUI plan execution |
| `terrax run [--plans-dir]` | Generator — direct plan execution |
| `terrax review [--plans-dir]` | Reader — plan review TUI |
| `terrax summary [--plans-dir]` | Reader — terminal summary |

`terrax last` and `terrax history` re-execution do not receive a `--plans-dir` flag. They inherit the configured value from Viper (yaml or default). If a custom directory is needed for re-execution, it must be set in `.terrax.yaml` rather than passed as a session-scoped flag.

**Pre-run cleanup** (the `os.RemoveAll` that clears stale plan files before each plan run) was updated in all three sites (`cmd/run.go`, `cmd/root.go`, `cmd/execute.go`) to remove the configured plans directory instead of the hardcoded `.terrax/` parent. This ensures custom directories are cleared between runs just as the default is.

**No signature changes** to `executor.Run` or any other internal function. All propagation is through the Viper global, consistent with how `plan.summary_enabled` and `plan.review_enabled` are already propagated.

## Consequences

### Positive

- Parallel CI runs can isolate plan output by passing distinct `--plans-dir` values.
- External artifact stores are reachable via absolute paths in `plan.json_out_dir`.
- The `--plans-dir` flag follows the same convention as `--dir` across all relevant commands.
- The default behavior is unchanged: users who do not set `plan.json_out_dir` get `.terrax/plans` as before.

### Negative

- The `abs/rel` path resolution block is duplicated in four places (`buildFilterArgs`, `runPlanSummary`, `runPlanReview`, `runSummaryCmd`). A future change to path resolution (e.g., home-directory expansion) must be applied in all four.
- `terrax last` and history re-execution inherit the Viper value but cannot override it per-invocation. A user who ran `terrax --plans-dir /ci/plans plan` and then runs `terrax last` in a new shell session will get plan review from `.terrax/plans` (the default) unless `plan.json_out_dir` is also set in `.terrax.yaml`. This is a session-boundary limitation of flag-based overrides.

## Alternatives Considered

### Option 1: Thread `plansDir` as a parameter to `executor.Run`

**Description**: Add a `plansDir string` parameter to `executor.Run`. Each caller computes the resolved path and passes it explicitly. Readers receive `--plans-dir` directly and pass it to the plan collection functions.

**Pros**:

- No global state; the plans directory is explicit at every call site.
- Easier to trace where the value originates.

**Cons**:

- Changes the signature of `executor.Run`, which has four call sites (`runTUI`, `runCommand`, `reExecuteHistoryEntry`, `runForceUnlock`). Force-unlock does not use plan output at all and would carry a meaningless parameter.
- All callers must compute the resolved path before calling `Run`, duplicating resolution logic.

**Why rejected**: The project already uses Viper to propagate feature flags (`plan.summary_enabled`, `plan.review_enabled`, `terragrunt.parallelism`) without touching `executor.Run`'s signature. Adding a parameter would break the established pattern for a case that is architecturally identical to the existing flags. The global-state concern is real but bounded — Viper is process-scoped, and each command invocation is a single process.

### Option 2: Config YAML only (no CLI flag)

**Description**: Expose `plan.json_out_dir` only in `.terrax.yaml`, with no `--plans-dir` flag on any command.

**Pros**:

- Simpler surface — one configuration channel instead of two.
- Enforces that the plans directory is stable across a project rather than varying per invocation.

**Cons**:

- CI pipelines that want per-job isolation cannot set the directory without maintaining separate config files per job.
- Inconsistent with `--dir`, which allows the working directory to be overridden per invocation without modifying config.

**Why rejected**: The primary motivation is CI parallelism, which requires per-invocation control. A config-only approach forces teams to maintain multiple `.terrax.yaml` variants or use per-job config file overrides — more operational complexity than a single flag.

## Future Enhancements

**Potential Improvements**:

1. Extract the abs/rel resolution logic into a shared helper (e.g., `resolvePlansDir(repoRoot string) string` in `cmd/root.go`) to eliminate the four-site duplication.
2. Support `~` expansion in `plan.json_out_dir` so users can write `~/terrax-plans` without needing an absolute path.
3. Store the `plansDir` value used during a plan run in the history entry, so `terrax last` can restore it automatically when re-executing a plan command.

## References

- [`internal/executor/executor.go`](../../internal/executor/executor.go)
- [`cmd/root.go`](../../cmd/root.go)
- [`cmd/run.go`](../../cmd/run.go)
- [`cmd/review.go`](../../cmd/review.go)
- [`cmd/summary.go`](../../cmd/summary.go)
- [`cmd/execute.go`](../../cmd/execute.go)
- [Configurable Plans Directory Design Spec](../superpowers/specs/2026-06-25-configurable-plans-dir-design.md)
