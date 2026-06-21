# ADR-0016: Cycle Detection and Reverse Dependency Graph

**Status**: Accepted

**Date**: 2026-06-20

**Deciders**: TerraX Core Team

**Related**:
- [ADR-0015: Static HCL Dependency Graph](0015-static-hcl-dependency-graph.md)
- [ADR-0013: VS Code Extension Integration](0013-vscode-extension-integration.md)

## Context

Once `terrax tree --json` provides `dependencies` per stack node (ADR-0015), two derived operations become immediately useful:

1. **Cycle detection** — circular dependency chains cause `terragrunt run-all` to hang or error with cryptic messages. Engineers need to identify cycles before running commands, not discover them mid-execution. Without proactive detection, cycles silently corrupt the dependency ordering that Terragrunt relies on.

2. **Reverse dependency lookup** — "which stacks will break if I change vpc?" requires inverting the forward dependency graph. Without this, an engineer must manually trace every `dependencies` list in the repo to find all transitive consumers of a module — impractical in repositories with hundreds of stacks.

Both operations are pure graph algorithms on data already present in the `stack.Node` tree after `FindAndBuildTree` populates `Dependencies`. The question was where to perform this computation and how to surface it.

Requirements:
1. Logic must live in Go so the VS Code extension remains a thin client with no graph computation of its own.
2. Results must be available in `terrax tree --json` so any consumer (extension, scripts, CI) can use them without additional subprocesses.
3. Cycle detection must correctly mark only the nodes that participate in a cycle — not entry points that merely lead into a cycle.
4. The VS Code extension must surface both pieces of data visually without requiring additional API calls.

## Decision

### Go: `internal/stack/graph.go`

A new `AnalyzeGraph(*Node)` function is called at the end of `FindAndBuildTree`, after all `Dependencies` are populated. It performs two passes over the tree:

**Pass 1 — Reverse graph.** Flattens all nodes into a `map[string]*Node`, then inverts every dependency edge: for each node N with `Dependencies: [A, B]`, appends N's path to `A.Dependents` and `B.Dependents`. Sorts each `Dependents` slice for stable output.

**Pass 2 — Cycle detection.** Standard DFS with `visited` and `inStack` sets, plus an explicit `stackPath []string` to track the current DFS path. When a node already in `inStack` is encountered, all nodes from that node's first appearance in `stackPath` to the current position are marked `InCycle = true`. This correctly handles partial cycles: in A→B→C→B, only B and C are marked — A is an entry point, not a cycle participant.

`stack.Node` gains two fields with proper JSON tags:

```go
Dependents []string `json:"dependents"` // absolute paths of direct dependents
InCycle    bool     `json:"inCycle"`     // true if part of any dependency cycle
```

All nodes — stack and non-stack — are initialized with `Dependents: []string{}` (empty array, never null in JSON) and `InCycle: false` before `AnalyzeGraph` runs.

### VS Code: cycle indicators and Dependents panel

The extension consumes the enriched JSON with no additional subprocess calls:

- **Stacks panel:** `getTreeItem` shows `$(warning)` instead of `$(package)` for stack nodes where `node.inCycle === true`, making cycles immediately visible in the tree.

- **Dependents panel:** A new `terrax.dependentsTree` view (third in the `terrax` Activity Bar container) is backed by `DependentsTreeProvider` in `dependencyProvider.ts`. It mirrors `DependencyTreeProvider` but reads `stackNode.dependents ?? []` instead of `stackNode.dependencies ?? []`. Both providers share module-level helpers `buildNodeMap` and `makeDepNodes`. Selecting a node in the Stacks panel updates both the Dependencies and Dependents panels simultaneously via `treeView.onDidChangeSelection`.

## Consequences

### Positive

- Cycle detection and reverse graph are computed in a single O(V+E) pass over data already in memory — negligible overhead on top of the existing tree scan.
- Engineers see cycle warnings instantly when the panel loads, before attempting any `run-all` operation.
- "Impact analysis" (what breaks if I change X?) is now a single click: select a node in the Stacks panel, read the Dependents panel.
- All graph data is in `terrax tree --json`, so scripts and CI pipelines can use it without any new subprocesses or parsing.
- The VS Code extension makes zero additional calls to the `terrax` binary to display the graph — all three panels (Stacks, Dependencies, Dependents) are populated from one `spawnSync` result.

### Negative

- `AnalyzeGraph` adds a post-scan O(V+E) pass to every `FindAndBuildTree` call, including `terrax tree`, `terrax run`, and the CLI entry point. On repositories with thousands of stacks and dense dependency graphs, this adds measurable latency even when cycle/dependents data is not needed by the caller.
- The `detectCycles` function uses a closure-captured `stackPath` slice, making the inner DFS non-reentrant. Future parallelization of cycle detection across disconnected components would require converting `stackPath` to a parameter.
- `DependencyTreeProvider` and `DependentsTreeProvider` are near-identical classes differing only in which field they read (`dependencies` vs `dependents`). Any fix to one must be manually mirrored to the other until a shared abstraction is introduced.
- `InCycle` marks cycle membership but does not identify which specific cycle path exists. Engineers can see that vpc and alb-target-groups are in a cycle but must trace the graph manually to find the exact edge causing it.

## Alternatives Considered

### Option 1: Compute reverse graph and cycles in the VS Code extension (TypeScript)

**Description**: Keep `stack.Node` as-is and build the reverse graph client-side in `DependentsTreeProvider.setTree()` using the `dependencies` field already in the JSON. Run DFS cycle detection in TypeScript after each tree refresh.

**Pros**:

- No changes to the Go binary — the extension handles its own derived data.
- Cycle detection only runs when the extension is active, not on every CLI invocation.

**Cons**:

- Two implementations of the same graph algorithms — one in Go (if ever needed for CLI use) and one in TypeScript.
- `terrax tree --json` output would not include cycle or dependent information, making it unusable as a graph analysis primitive for scripts and CI.
- TypeScript DFS would re-run on every panel refresh, including selection changes, introducing latency in the extension host thread.

**Why rejected**: The stated design constraint was that logic lives in Go and the extension is a thin client. Cycles detected in TypeScript would be invisible to CLI users and unavailable for scripting. Duplicating graph algorithms across languages also creates a maintenance burden when the dependency schema evolves.

### Option 2: Lazy computation via a `terrax graph --dir <path>` subcommand

**Description**: Add a new `terrax graph` subcommand that computes and returns cycle and dependents data on demand, rather than embedding it in `terrax tree --json`. The extension would call `terrax graph` separately after loading the tree.

**Pros**:

- `FindAndBuildTree` is not affected — existing callers pay no overhead.
- Subcommand can accept additional flags (e.g. `--max-depth`, `--focus <path>`) for targeted analysis.

**Cons**:

- Two subprocesses per extension refresh: `terrax tree` followed by `terrax graph`. This doubles the blocking `spawnSync` calls in the extension host thread.
- The tree and graph data are derived from the same scan — scanning the filesystem twice wastes I/O.
- API surface grows without clear necessity: `terrax graph` would duplicate most of `terrax tree` internally to get the dependency data it needs.

**Why rejected**: The tree scan and graph analysis are inseparable — the graph is computed from the tree's `Dependencies` field, which is populated during the same scan. Splitting them into two subprocesses would require either re-scanning the filesystem or implementing a caching layer, both of which are more complex than embedding `AnalyzeGraph` in the existing tree-building pipeline.

## Future Enhancements

**Potential Improvements**:

1. Surface the specific cycle path (e.g. `vpc → alb-target-groups → vpc`) in the VS Code panel or CLI output, not just membership, to make cycles actionable without manual tracing.
2. Add a `terrax graph --cycles` CLI subcommand that lists all detected cycles with their full paths — useful in CI pipelines to block merges that introduce circular dependencies.
3. Extract a shared `RelationshipTreeProvider` base class in the VS Code extension to eliminate the duplication between `DependencyTreeProvider` and `DependentsTreeProvider`.
4. Add a `--skip-graph` flag to `FindAndBuildTree` callers that don't need cycle or dependent data, to skip the `AnalyzeGraph` pass when only the tree structure is needed.

## References

- [`internal/stack/graph.go`](../../internal/stack/graph.go)
- [`internal/stack/graph_test.go`](../../internal/stack/graph_test.go)
- [`extensions/vscode/src/dependencyProvider.ts`](../../extensions/vscode/src/dependencyProvider.ts)
- [ADR-0015: Static HCL Dependency Graph](0015-static-hcl-dependency-graph.md)
- [ADR-0013: VS Code Extension Integration](0013-vscode-extension-integration.md)
