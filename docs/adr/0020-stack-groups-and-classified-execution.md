# ADR-0020: Stack Groups and Classified Execution

**Status**: Accepted

**Date**: 2026-06-21

**Deciders**: TerraX Core Team

**Related**:
- [ADR-0017: Filter-Based Execution Strategy](0017-filter-based-execution-strategy.md)
- [ADR-0015: Static HCL Dependency Graph](0015-static-hcl-dependency-graph.md)

## Context

TerraX users work with mixed stacks that have different execution requirements. A concrete example: `aurora-database` stacks use the Terragrunt PostgreSQL provider, which requires direct VPC-private network access to connect to Aurora. Running these stacks locally requires env vars that set up a tunnel (`TF_VAR_host=localhost TF_VAR_port=15432`), while in CI they must run on self-hosted runners inside the VPC.

With the flat filter list introduced in ADR-0017, TerraX executes all stacks in a single `terragrunt run` call. This made it impossible to:
1. Inject different env vars for different subsets of stacks.
2. Sequence execution so that one subset completes before another starts (e.g. base infrastructure before database configuration).
3. Give CI pipelines the structured metadata they need to dispatch different groups to different runners.

A secondary use case is exclusion: some stacks (e.g. deprecated ones) should simply not execute in certain contexts without removing them from the repository.

## Decision

Stacks are classified into named **groups** based on markers in their `stack.hcl` file. Each group runs as a **separate `terragrunt run`** in topological order. Groups expose their metadata as JSON for external consumers.

### Classification

`stack.hcl` markers are detected via exact string grep (same mechanism as the CI/CD pipeline that inspired this feature). TerraX reads each stack's `stack.hcl` at execution time using `stack.DetectGroup`. The first alphabetically-sorted group whose `detect` string is found in the file wins; stacks matching no group fall into the implicit `default` group.

### Configuration

```yaml
stack_groups:
  default:
    depends_on: []

  private_connection:
    detect: "require_private_connection = true"
    depends_on: [default]
    env:
      TF_VAR_host: "localhost"
      TF_VAR_port: "15432"

  deprecated:
    detect: "deprecated = true"
    skip: true   # excluded from local execution; still appears in JSON output
```

### Execution model

`cmd/root.go:buildGroupedExecution` assigns each filter path to a group, performs topological sort via Kahn's algorithm (`stack.TopologicalSort`), and returns `[]GroupExecution` in execution order. The caller loops over the slice and issues one `executor.Run` per group, with the group's `env` injected via `cmd.Env`.

```
collectTransitiveDeps(stackPath)
  → allFilterPaths

buildGroupedExecution(allFilterPaths, repoRoot)
  → [{Name:"default", Paths:[...], EnvVars:{}},
     {Name:"private_connection", Paths:[...], EnvVars:{TF_VAR_host:...}}]

for group in groups:
  if group.Skip → continue
  executor.Run(..., group.Paths, group.EnvVars)
```

The pre-execution `.terrax/` cleanup runs once before the loop, not per group, so JSON plan files from multiple groups accumulate for the combined summary and review.

### JSON endpoint

`terrax groups --json` outputs the classified groups without executing anything:

```json
{
  "groups": [
    {"name": "default", "depends_on": [], "filters": [...], "env": {}, "skip": false},
    {"name": "private_connection", "depends_on": ["default"], "filters": [...], "env": {...}, "skip": false}
  ],
  "repo_root": "/project"
}
```

CI pipelines consume this to dispatch each group to the appropriate runner. The `skip` field is present in the JSON regardless of value, allowing pipelines to implement their own exclusion logic independently of TerraX's local behavior.

### Exclusion

Groups with `skip: true` are excluded from local execution loops but remain in `terrax groups --json`. This cleanly separates the "TerraX shouldn't run this locally" concern from the "CI decides what to do with this group" concern.

## Consequences

### Positive

- **Per-group env vars** allow private-connection stacks to work locally without polluting all stacks with the same env vars.
- **Sequenced execution** guarantees base infrastructure completes before dependent stacks run.
- **JSON endpoint** provides structured metadata that CI pipelines can use for runner dispatch without coupling TerraX to any specific CI system.
- **Zero change for unconfigured repos** — if `stack_groups` is absent, an implicit `default` group is created and all stacks execute in a single group, identical to pre-existing behavior.
- **Static marker detection** requires no subprocess. `stack.DetectGroup` reads one file per stack.

### Negative

- **Multiple Terragrunt processes** — one per group — means multiple authentication flows, provider initializations, and cache warmups. For repos with many groups this adds latency.
- **Alphabetical tie-breaking** when a stack matches multiple groups is deterministic but non-obvious. Users must be aware that group detection order is alphabetical.
- **Flat env injection** — group env vars are passed to all units in the group's filter run. A group cannot apply different env vars to individual stacks within it.

## Alternatives Considered

### Option 1: Profile-based execution (local vs CI)

**Description**: Define per-profile behavior (`local`, `ci`) for each group, controlling env vars and exclusion. TerraX detects the active profile from a CLI flag, `CI` env var, or config.

**Pros**:

- Self-contained: one config file covers all environments.
- TerraX itself handles the CI/local distinction without needing external scripts.

**Cons**:

- TerraX becomes a CI orchestrator rather than a local execution tool.
- Profile logic must be maintained in TerraX as CI requirements evolve.
- The "run on a different runner" concern is fundamentally a CI scheduler problem, not a CLI problem.

**Why rejected**: The JSON endpoint approach is a cleaner separation of concerns — TerraX computes and exports the data, CI consumes it. This keeps TerraX focused on local developer workflows and avoids coupling it to specific CI pipeline behaviors.

### Option 2: Single-run with per-stack env vars

**Description**: Keep a single `terragrunt run` but inject env vars selectively per-stack by wrapping each unit's execution via `--terragrunt-source-update` or similar mechanisms.

**Pros**:

- No multiple Terragrunt processes.
- Simpler execution model.

**Cons**:

- Terragrunt does not natively support per-unit env var injection in a single `run` invocation.
- Would require parsing Terragrunt output to apply env vars per unit, reintroducing the fragile stdout-parsing problem rejected in ADR-0010.

**Why rejected**: Per-unit env var injection is not a supported Terragrunt primitive. Implementing it in TerraX would require deeply coupling to Terragrunt's execution internals.

### Option 3: Separate `terrax run --group <name>` subcommand

**Description**: Instead of automatic group-based execution, add a `--group` flag that users must explicitly pass to run a specific group.

**Pros**:

- Explicit — users control which group runs.
- No automatic sequencing surprises.

**Cons**:

- Users must know which groups to run and in what order.
- The topological ordering (run `default` before `private_connection`) must be remembered or documented per repo.
- Loses the convenience of a single `terrax` invocation running everything in the right order.

**Why rejected**: The goal is to make the complex case (mixed private/public stacks) transparent. Users should not need to manually orchestrate group execution order.

## Future Enhancements

**Potential Improvements**:

1. **Parallel group execution** — groups with no mutual `depends_on` relationship could run concurrently, reducing total execution time for repos with many independent groups.
2. **Per-group Terragrunt parallelism** — allow setting `--terragrunt-parallelism` per group, since private-connection groups may need lower parallelism than the default group.
3. **TUI visual markers** — display group membership badges in the navigation columns so users can see which stacks are classified before executing.

## References

- `internal/stack/markers.go` — `DetectGroup`, `TopologicalSort`, `GroupDetectConfig`
- `cmd/root.go` — `buildGroupedExecution`, `loadStackGroups`, `StackGroupConfig`, `GroupExecution`
- `cmd/groups.go` — `terrax groups --json` subcommand
- [ADR-0017: Filter-Based Execution Strategy](0017-filter-based-execution-strategy.md)
- [ADR-0015: Static HCL Dependency Graph](0015-static-hcl-dependency-graph.md)
- Reference CI/CD pipeline: `.github/actions/classify-and-build-matrices/action.yml` in `efex-tpl-do-pipeline-infrastructure-terragrunt`
