# ADR-0014: Plan Summary Mode with Native JSON Parsing

**Status**: Accepted

**Date**: 2026-06-20

**Deciders**: TerraX Core Team

**Related**:
- [ADR-0010: Plan Analysis Workflow](0010-plan-analysis-workflow.md)
- [ADR-0011: Extensible Flags Configuration](0011-extensible-flags-configuration.md)

## Context

TerraX's plan review feature (ADR-0010) reads binary `.binary` plan files from `.terragrunt-cache`, spawns `terraform show -json` per stack as a subprocess, and launches the full `StatePlanReview` TUI. This is powerful but heavy: on large stacks the per-stack subprocess cost is significant, and the TUI requires an interactive terminal.

Users working in CI/CD pipelines or wanting quick feedback after `plan` need a lighter alternative: a one-line-per-stack terminal summary showing counts by change type (+add ~update -delete ♻recreate), equivalent to what the reference pipeline produces with `--summary-per-unit` and a post-processing step.

Three constraints shaped the design:

1. The summary must require zero external tools — `tf-summarize` was initially evaluated but would be a hard runtime dependency with its own installation requirement.
2. The summary must be independent of the existing `plan.review_enabled` TUI toggle — users should be able to enable summary without launching the interactive viewer.
3. Generated files (JSON plans, report) must live in a predictable, gitignore-friendly directory, not scattered under `./tmp/`.

## Decision

We introduce `plan.summary_enabled` as a second, independent plan flag alongside `plan.review_enabled`. When enabled:

1. **Arg injection** — `appendPlanTerragruntOutputFlags` adds `--json-out-dir=.terrax/plans` before the `--` separator in the Terragrunt command. Terragrunt writes a `plan.json` file per stack into `.terrax/plans/<stack-path>/plan.json`.

2. **Native parsing** — `internal/plan/summarizer.go` reads these JSON files directly using Go's standard library. It reuses `TerraformPlanJSON` (already defined in `collector.go`) and counts resource changes by type via `countChanges`:
   - `add`: `actions: ["create"]` without `importing`
   - `update`: `actions: ["update"]`
   - `delete`: `actions: ["delete"]`
   - `recreate`: `actions: ["delete", "create"]` (or reversed)
   - `import`: `actions: ["create"]` with `importing != nil`

3. **Terminal output** — one line per stack, final count line:
   ```
   🔍 Scanning 12 JSON plan(s)...

     workloads/dev/us-east-1/acm: +0 ~0 -0 ♻0
     workloads/dev/us-east-1/ecs: +1 ~0 -0 ♻0

   1 stack(s) with pending changes
   ```

4. **Output directory** — all TerraX-generated files go under `.terrax/` at the stack working directory:
   - `.terrax/plans/` — JSON plan files per stack
   - `.terrax/report.json` — Terragrunt run report (when `features.report.enabled`)

5. **Cleanup** — `plan.cleanup_enabled: true` removes the entire `.terrax/` directory after the summary via `os.RemoveAll`.

**Behaviour matrix:**

| `summary_enabled` | `review_enabled` | Result |
|---|---|---|
| false | false | nothing |
| true | false | terminal summary only |
| false | true | TUI only (existing) |
| true | true | terminal summary, then TUI |

**Configuration:**

```yaml
plan:
  summary_enabled: true    # terminal summary after plan
  cleanup_enabled: true    # delete .terrax/ after summary
  review_enabled: false    # TUI plan review (existing)
```

**`.gitignore` entry:**

```
.terrax/
```

## Consequences

### Positive

- **Zero external dependencies** — native Go JSON parsing requires no installed tools. `tf-summarize` was removed entirely; the same Terraform plan JSON format already parsed by `collector.go` is reused.
- **Fast** — reads pre-generated JSON files with `os.ReadFile` + `json.Unmarshal`; no subprocess per stack.
- **Independent toggle** — `summary_enabled` and `review_enabled` are orthogonal; each can be activated without the other.
- **Clean output directory** — `.terrax/` is predictable, single-entry gitignore, and contains only generated artifacts.
- **Optional cleanup** — `cleanup_enabled` leaves no trace of generated files when enabled.

### Negative

- **Requires `--json-out-dir` support** — Terragrunt must support this flag (available since Terragrunt 1.x). Older versions silently ignore it, producing no JSON files and an empty summary.
- **Summary depends on summary_enabled being true** — cleanup only runs inside `runPlanSummary`, so `cleanup_enabled` alone (without `summary_enabled`) has no effect. The `report.json` is not cleaned unless summary is also active.
- **Stack path derivation is dirname-based** — stack name is `filepath.Dir(rel)` where `rel` is the path of the JSON file relative to `.terrax/plans/`. If Terragrunt names files differently across versions, the displayed stack name may change.

## Alternatives Considered

### Option 1: tf-summarize subprocess

**Description**: Run `tf-summarize -json-sum <file>` as a subprocess per JSON file, same as the reference CI/CD pipeline. The counts come from `tf-summarize`'s output JSON.

**Pros**:

- Mirrors the pipeline exactly, including `import` and `moved` counts.
- Delegates format changes in Terraform plan JSON to an actively maintained tool.

**Cons**:

- Requires `tf-summarize` to be installed and in `PATH`. Adding a runtime dependency contradicts TerraX's self-contained design.
- The `exec.ErrNotFound` detection for missing binary is unreliable — `cmd.Output()` returns `*os.PathError`, not `*exec.Error`, requiring a separate `exec.LookPath` pre-check.
- Adds subprocess overhead per file.

**Why rejected**: TerraX already parses the Terraform plan JSON format natively in `collector.go`. Reimplementing the four-field count logic (add/update/delete/recreate) is trivial and avoids a new runtime dependency. The self-contained binary is a core design goal.

### Option 2: Read from binary plan files (existing collector path)

**Description**: Reuse the existing `plan.Collector` which scans `.terragrunt-cache` for `terrax-tfplan-*.binary` files and runs `terraform show -json` on each, then display counts in the terminal instead of the TUI.

**Pros**:

- No new flags or directories — reuses the existing binary capture mechanism.
- Consistent with the `review_enabled` path; same data source for both summary and TUI.

**Cons**:

- The `terraform show -json` subprocess per stack is the bottleneck the user was trying to avoid. Summary mode exists precisely because this path is slow.
- Binary files in `.terragrunt-cache` are interleaved with Terraform's own cache files; scanning is slower and must filter by timestamp.

**Why rejected**: The performance concern that motivated summary mode is the per-stack `terraform show -json` subprocess. Reusing the binary path would not solve it.

### Option 3: Unified output directory under `./tmp/`

**Description**: Keep plan files, report, and other generated artifacts under `./tmp/` with subdirectories (`./tmp/json-plans/`, `./tmp/report.json`).

**Pros**:

- Convention familiar to users who already have `./tmp/` in their gitignore.

**Cons**:

- `./tmp/` is a generic name shared with other tools and test artifacts. A single gitignore entry for `./tmp/` may suppress non-TerraX files.
- Provides no discovery signal that the directory is TerraX-specific; harder to document and explain.

**Why rejected**: A TerraX-specific directory (`.terrax/`) is unambiguous — one gitignore entry covers all generated artifacts, and the directory name communicates ownership clearly.

## Future Enhancements

**Potential Improvements**:

1. **`moved` change type** — Terraform plan JSON represents moved resources as `no-op` with additional context that is non-trivial to detect reliably. Adding `moved` count would require deeper inspection of the plan JSON structure.
2. **Report-driven summary** — When `features.report.enabled` is true, `report.json` already contains per-unit result data. A future optimization could read from `report.json` instead of per-stack JSON files to avoid scanning `.terrax/plans/`.
3. **Standalone `cleanup` command** — A dedicated `terrax cleanup` command to remove `.terrax/` regardless of whether summary ran, useful for manual housekeeping.
4. **`cleanup_enabled` independent of `summary_enabled`** — Currently cleanup only runs inside `runPlanSummary`. A future refactor could run cleanup as a separate post-plan hook, activated independently.

## References

- `internal/plan/summarizer.go` — `Summarize` and `countChanges` implementation
- `internal/plan/collector.go` — `TerraformPlanJSON` struct (shared with summary mode)
- `internal/executor/executor.go` — `appendPlanTerragruntOutputFlags`
- `internal/config/defaults.go` — `DefaultOutputDir`, `DefaultJSONOutDir`, `DefaultPlanSummaryEnabled`, `DefaultPlanCleanupEnabled`
- `cmd/root.go` — `runPlanSummary` with cleanup logic
- [ADR-0010: Plan Analysis Workflow](0010-plan-analysis-workflow.md)
- [ADR-0011: Extensible Flags Configuration](0011-extensible-flags-configuration.md)
