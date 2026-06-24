# ADR-0018: Plan Analysis via --json-out-dir

**Status**: Accepted

**Date**: 2026-06-20

**Deciders**: TerraX Core Team

**Related**:
- [ADR-0010: Plan Analysis Mode via Binary Plan Generation](0010-plan-analysis-workflow.md) *(superseded)*
- [ADR-0014: Plan Summary Mode](0014-plan-summary-mode.md)
- [ADR-0017: Filter-Based Execution Strategy](0017-filter-based-execution-strategy.md)

## Context

ADR-0010 established a Binary-to-JSON workflow: TerraX injected `-out=tfplan.binary` into plan commands and ran `terraform show -json` on each binary file after execution. This worked correctly for single-stack runs but became incompatible with the filter-based execution strategy introduced in ADR-0017 for two reasons:

1. **`terraform show -json` per stack is expensive.** For N stacks, N subprocesses must initialize Terraform providers, read binary files, and serialize JSON. On large environments this is the primary source of latency in the plan review flow.

2. **The binary approach was coupled to `--working-dir`.** The old executor ran Terragrunt with `--working-dir <stack>` and scanned `.terragrunt-cache` for `terrax-tfplan-<ts>.binary` files. With ADR-0017, Terragrunt runs from the repository root with `--filter` flags — there is no single `--working-dir` to scope the `.terragrunt-cache` scan, and the timestamp-based file naming becomes unreliable across a heterogeneous filter list.

Terragrunt's `--json-out-dir` flag solves both problems: Terragrunt internally runs `terraform show -json` once per unit as part of its own execution, saving the output directly to a structured directory. TerraX reads these files with a plain `os.ReadFile` — no subprocess, no binary format, no per-stack process initialization.

## Decision

TerraX no longer injects `-out=tfplan.binary` or runs `terraform show -json` subprocesses. Instead:

1. **`--json-out-dir` injection** — `buildFilterArgs` injects `--json-out-dir=<abs-repoRoot>/.terrax/plans` when `plan.review_enabled` or `plan.summary_enabled` is active. Terragrunt writes one `tfplan.json` per unit to `<repoRoot>/.terrax/plans/<unit-rel-path>/tfplan.json`.

2. **`CollectFromJSONDir`** — `internal/plan/collector.go` exposes a single function that walks the output directory, reads each JSON file with `os.ReadFile`, and parses it into a `PlanReport` using the same `TerraformPlanJSON` struct used by the summary mode.

3. **`runPlanReview`** calls `plan.CollectFromJSONDir(ctx, jsonDir, stackPath)` to build the `PlanReport` and launches the `StatePlanReview` TUI. The goroutine + progress channel of the old Collector is eliminated — JSON reads are fast enough to run synchronously.

4. **Pre-execution reset** — `executor.Run` deletes `<repoRoot>/.terrax/` before invoking Terragrunt when `plan.summary_enabled` or `plan.review_enabled` is active, ensuring the output directory reflects only the current run.

**Shared infrastructure with summary mode.** Both `plan.review_enabled` (TUI) and `plan.summary_enabled` (terminal summary) read from the same `<repoRoot>/.terrax/plans/` directory using the same `TerraformPlanJSON` struct. The data is generated once by Terragrunt and consumed by whichever modes are active.

```
executor.Run
  ↓ os.RemoveAll(<repoRoot>/.terrax/)
  ↓ terragrunt run --filter ... --json-out-dir=<repoRoot>/.terrax/plans -- plan
  ↓
  ├─ plan.summary_enabled → runPlanSummary → plan.Summarize(jsonDir)
  └─ plan.review_enabled  → runPlanReview  → plan.CollectFromJSONDir(jsonDir, stackPath) → TUI
```

## Consequences

### Positive

- **No per-stack subprocesses.** Terragrunt handles the `terraform show -json` call internally for each unit. TerraX reads the output files directly.
- **Unified data source.** Summary mode and review TUI read from the same directory. There is no risk of the two modes seeing different data.
- **Simpler collector.** `internal/plan/collector.go` dropped from ~415 lines (`Collector`, goroutine, channel, binary scanning, `CleanupOldPlans`) to ~170 lines (`CollectFromJSONDir`, `processPlanJSONFile`, shared helpers).
- **Clean directory per run.** Pre-execution reset guarantees stale files from previous runs never pollute the analysis.

### Negative

- **Requires `--json-out-dir` support.** Terragrunt must support this flag. Older versions that do not will silently produce no JSON files, causing the review and summary to show empty results with no error.
- **Disk writes for every plan.** Every plan run writes JSON files to `.terrax/plans/` even when the user only wants the live console output. The directory is reset on the next run, but a single run with many stacks will temporarily occupy disk space proportional to the number of units.
- **Review TUI loses progress display.** The old `Collector` streamed progress messages through a channel, showing `[3/10] Processed workloads/dev/acm` during collection. `CollectFromJSONDir` reads synchronously and emits no per-file progress. For repositories with hundreds of stacks this may feel like a pause before the TUI appears.

## Alternatives Considered

### Option 1: Keep binary approach, adapt scan path for filter mode

**Description**: Keep `-out=terrax-tfplan-<ts>.binary` injection and `terraform show -json` per stack, but update the Collector to scan across all filter paths' `.terragrunt-cache` directories rather than a single working directory.

**Pros**:

- Minimal change to the existing plan review flow.
- No dependency on `--json-out-dir` availability.

**Cons**:

- Each `terraform show -json` invocation must locate and initialize the Terraform provider cache. For 10+ stacks this adds 5–30 seconds of latency per run.
- Scanning multiple `.terragrunt-cache` directories with timestamp filtering is fragile: parallel executions or retries can produce multiple binary files for the same unit.
- The binary format is opaque; any schema change in Terraform's plan binary requires Terraform itself for conversion, making offline analysis impossible.

**Why rejected**: The per-stack subprocess cost is the primary latency bottleneck the user wanted eliminated. A redesign that preserves it does not achieve the goal.

### Option 2: Parse terraform show -json output from --tf-forward-stdout

**Description**: Intercept the `terraform show -json` output streamed via `--tf-forward-stdout` and parse it inline during execution.

**Pros**:

- No temporary files on disk.
- Single pass: plan and parse happen simultaneously.

**Cons**:

- `--tf-forward-stdout` interleaves output from all units running in parallel. Demultiplexing unit boundaries from a single stream is fragile.
- The JSON output from `terraform show -json` is large (megabytes per unit); buffering it in memory for all parallel units simultaneously is impractical.

**Why rejected**: Already rejected in ADR-0010 for stdout parsing fragility. The interleaving problem is worse with multiple parallel units in filter mode.

## Future Enhancements

**Potential Improvements**:

1. **Progress display during collection.** Add a brief progress indicator (`Collecting plan results…`) when reading large numbers of JSON files to eliminate the perceived pause before the TUI appears.
2. **Apply from Plan.** Now that plan data is in a structured JSON directory, `terraform apply -auto-approve` can be targeted at specific units using the same filter list. This was deferred from ADR-0010.
3. **Offline review.** Because `.terrax/plans/` persists until the next run, a future `terrax review` subcommand could re-open the TUI for the last plan without re-running.

## References

- `internal/plan/collector.go` — `CollectFromJSONDir`, `processPlanJSONFile`
- `internal/executor/executor.go` — `buildFilterArgs` (`--json-out-dir` injection), pre-execution reset
- `cmd/root.go` — `runPlanReview`, `runPlanSummary`
- [ADR-0010: Plan Analysis Mode via Binary Plan Generation](0010-plan-analysis-workflow.md) *(superseded)*
- [ADR-0014: Plan Summary Mode](0014-plan-summary-mode.md)
- [ADR-0017: Filter-Based Execution Strategy](0017-filter-based-execution-strategy.md)
