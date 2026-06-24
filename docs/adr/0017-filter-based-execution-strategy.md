# ADR-0017: Filter-Based Execution Strategy

**Status**: Accepted

**Date**: 2026-06-20

**Deciders**: TerraX Core Team

**Related**:
- [ADR-0009: Executor Isolation Pattern](0009-executor-isolation-pattern.md)
- [ADR-0011: Extensible Flags Configuration](0011-extensible-flags-configuration.md)
- [ADR-0014: Plan Summary Mode](0014-plan-summary-mode.md)
- [ADR-0015: Static HCL Dependency Graph](0015-static-hcl-dependency-graph.md)

## Context

TerraX originally executed Terragrunt commands using:

```
terragrunt run --all --working-dir <selected-stack> --queue-include-external [flags] -- <command>
```

This approach had two interconnected problems that became critical when introducing `--json-out-dir` for plan analysis:

1. **Path corruption for external dependencies.** Terragrunt computes the subdirectory within `--json-out-dir` as the path of each unit *relative to `--working-dir`*. When an external dependency (pulled in via `--queue-include-external`) resides outside the `--working-dir` subtree, its relative path contains `../../..` segments that escape the `--json-out-dir` directory. The resulting files land at incorrect locations and are unreadable by TerraX.

2. **`--queue-include-external` is opaque.** Passing this flag to Terragrunt delegates dependency discovery to the Terragrunt process. TerraX cannot know in advance which stacks will actually run, making it impossible to pre-configure output paths or validate the scope of execution.

Both problems share the same root cause: Terragrunt resolves paths relative to `--working-dir`, and when that working directory is a leaf stack rather than the project root, sibling-tree dependencies produce paths that escape the intended directory.

## Decision

All Terragrunt commands are now executed using explicit `--filter` flags, with the execution scope pre-computed by TerraX using static HCL analysis:

```
terragrunt run \
  --filter workloads/dev/acm \
  --filter management/global/core/organizations \
  --filter infrastructure/shared/ca-central-1/core/ecr \
  [flags] \
  --json-out-dir=<abs-repoRoot>/.terrax/plans \
  -- <command>
```

**Key decisions:**

**1. Execution from repoRoot (`cmd.Dir = repoRoot`).**
Terragrunt runs from the repository root. All `--filter` paths are relative to `repoRoot`, and `--json-out-dir` uses an absolute path anchored at `repoRoot`. This produces consistent, predictable file locations for all units regardless of where the selected stack is in the tree.

**2. Pre-computed filter list via `collectTransitiveDeps`.**
Before any command execution, `cmd/root.go` calls `collectTransitiveDeps(stackPath)`, which:
- Uses `deps.FindRepoRoot` to locate the repository root.
- Seeds the queue with the selected stack (leaf) or all leaf stacks under a directory selection.
- When `include_dependencies: true`, performs a BFS over the dependency graph using `deps.ParseDependencies`, adding each discovered dependency to the filter list.
- Deduplicates via a visited map.

The resulting `filterPaths []string` is passed to `executor.Run`, which builds the `--filter` args.

**3. `include_dependencies` replaces `--queue-include-external`.**
`terragrunt.queue_include_external` was removed from Terragrunt's command-line arguments entirely. Its role is now fulfilled by the `include_dependencies` top-level config key, which controls TerraX's BFS traversal. Terragrunt no longer discovers dependencies autonomously — TerraX passes the exact list.

```yaml
# Default: true — TerraX resolves transitive deps via static HCL analysis.
# include_dependencies: false
```

**4. Pre-execution output directory reset.**
Because `--json-out-dir` accumulates files across runs, `executor.Run` deletes `<repoRoot>/.terrax/` before invoking Terragrunt whenever `plan.summary_enabled` or `plan.review_enabled` is active. This ensures the TUI and summary always reflect only the current run.

**Arg shape after this decision:**

```
run
  --filter <rel-path-1>
  --filter <rel-path-2>
  ...
  [logging flags]
  [terragrunt flags — no --queue-include-external]
  [feature flags]
  [--json-out-dir=<abs-repoRoot>/.terrax/plans]  ← plan only
  --
  <command>
  [terraform flags]
```

## Consequences

### Positive

- **Predictable file paths.** `--json-out-dir` files always land at `<repoRoot>/.terrax/plans/<unit-path>/tfplan.json` regardless of which stack is selected or where its dependencies are in the tree.
- **Transparent execution scope.** TerraX knows exactly which stacks will run before Terragrunt starts. The filter list is visible in the `🚀 Executing:` log line.
- **No subprocess for dependency discovery.** `deps.ParseDependencies` resolves dependencies in milliseconds via static regex parsing — no `terragrunt graph-dependencies` call needed.
- **Unified code path.** All commands (plan, apply, validate, destroy, …) use the same `buildFilterArgs` function. There is no special case for different commands.
- **Directory-level selection.** When the user selects a non-leaf directory, `stack.CollectStackPaths` finds all leaf stacks within it. Each becomes a seed in the BFS, and their transitive dependencies are included if `include_dependencies: true`.

### Negative

- **Static analysis limitations.** `deps.ParseDependencies` covers 98.6% of the patterns in the target repository (static relative paths). Dynamically computed `config_path` expressions (e.g., `"${local.env}/vpc"`) are silently skipped — their dependent stacks will not appear in the filter list unless `include_dependencies: false` and the user selects them explicitly.
- **repoRoot detection required.** Every command execution now requires locating the repository root via `deps.FindRepoRoot`. If the selected stack is outside the repository tree (no `root.hcl` ancestor), the function falls back to `stackPath` itself, which may cause Terragrunt to fail to resolve the filters.
- **Filter list can be large.** On repositories with deep dependency chains, the filter list may contain dozens of paths. This is printed verbatim in the `🚀 Executing:` line, which becomes noisy for highly-connected stacks.

## Alternatives Considered

### Option 1: Keep --all --working-dir with absolute --json-out-dir

**Description**: Keep the existing `--all --working-dir <stack>` execution but set `--json-out-dir` to an absolute path rooted at the project root rather than the stack directory.

**Pros**:

- Minimal change to existing execution logic.
- No need for static HCL analysis.

**Cons**:

- Terragrunt v1.0.0 still computes the subdirectory within `--json-out-dir` as the path of each unit *relative to `--working-dir`*. External dependencies still generate `../../..` paths that escape the output directory — the absolute path fixes the base but not the relative traversal. Testing confirmed this: a dependency at `management/global/core/organizations` still produced a file outside `.terrax/plans/` when the working-dir was `management/ca-central-1/core/identity-center`.

**Why rejected**: The fundamental issue is path traversal from `--working-dir`, not the base of `--json-out-dir`. This option does not solve the problem.

### Option 2: Use project root as --working-dir without --filter

**Description**: Set `--working-dir` to the project root, which gives all units a clean relative path. Add no `--filter` — let Terragrunt discover everything.

**Pros**:

- Simple: one flag change, no static analysis.
- All units have clean paths relative to project root.

**Cons**:

- `--all --working-dir <repoRoot>` runs every stack in the entire repository — potentially hundreds of stacks.
- With `--filter <relPath>` added to restrict scope, Terragrunt v1.0.0 testing showed that `--filter` without explicit `--all` runs all stacks, and `--all --working-dir <root> --filter <path>` also runs all stacks rather than filtering correctly.

**Why rejected**: Terragrunt v1.0.0 does not restrict execution scope with `--filter` when `--all --working-dir` is also set. Every combination tested in this version resulted in running all stacks rather than the filtered subset.

### Option 3: Run terragrunt find to pre-compute the list

**Description**: Call `terragrunt find --all --working-dir <stack> --queue-include-external` to obtain the list of units that would run, then pass each as an explicit `--filter`.

**Pros**:

- 100% accurate — Terragrunt resolves all dynamic expressions and include chains.
- No static parser to maintain.

**Cons**:

- Requires an additional subprocess call before every command execution.
- `terragrunt find` initializes providers on some backends, adding latency.
- Adds a dependency on the Terragrunt subprocess for what is essentially metadata retrieval.

**Why rejected**: `deps.ParseDependencies` covers 100% of patterns in the target repository (ADR-0015). A subprocess for the same result contradicts TerraX's goal of fast, offline dependency resolution.

## Future Enhancements

**Potential Improvements**:

1. **Collapse the filter list in the execution log** — when the filter list exceeds a threshold (e.g., 10 entries), display a summary (`--filter <N stacks>`) rather than all paths verbatim.
2. **Validate repoRoot before execution** — if `deps.FindRepoRoot` falls back to `stackPath` (no root config file found), emit a warning rather than silently running with an incorrect working directory.
3. **Incremental filter refinement** — allow the user to interactively remove stacks from the filter list before confirming execution, useful for large dependency fans.

## References

- `internal/executor/executor.go` — `Run`, `buildFilterArgs`
- `cmd/root.go` — `collectTransitiveDeps`
- `internal/deps/parser.go` — `ParseDependencies`, `FindRepoRoot`
- [ADR-0015: Static HCL Dependency Graph](0015-static-hcl-dependency-graph.md)
- [ADR-0011: Extensible Flags Configuration](0011-extensible-flags-configuration.md)
