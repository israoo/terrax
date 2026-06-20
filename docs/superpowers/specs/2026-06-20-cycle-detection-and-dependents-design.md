# Design: Cycle Detection and Reverse Dependency Graph

**Date:** 2026-06-20
**Status:** Approved

## Context

The static HCL dependency graph (ADR-0015) populates `Dependencies []string` per stack node. Two logical extensions follow naturally from this data:

1. **Cycle detection** — circular dependency chains cause Terragrunt run-all to hang or error. Detecting them at scan time and surfacing them visually lets engineers fix them before execution.
2. **Reverse dependencies (dependents)** — "which stacks will break if I change vpc?" requires inverting the dependency graph. Currently impossible without manually tracing the graph.

Both operations are pure graph algorithms on data already present in the tree. No external tools or subprocesses needed.

## Goals

1. Compute `Dependents []string` (reverse dependency graph) and `InCycle bool` (cycle membership) for each stack node in Go, exposing them in `terrax tree --json`.
2. Show cycle indicators on affected nodes in the VS Code Stacks panel.
3. Add a VS Code "Dependents" panel that shows all stacks depending on the selected node, transitively.

## Non-Goals

- Reporting which specific cycle path exists (just membership flag for now).
- CLI command for querying dependents outside of `terrax tree --json`.
- Automatic cycle breaking or suggestions.

---

## Part 1 — Go: `internal/stack/graph.go`

### `stack.Node` additions

```go
type Node struct {
    Name         string   `json:"name"`
    Path         string   `json:"path"`
    IsStack      bool     `json:"isStack"`
    Children     []*Node  `json:"children"`
    Depth        int      `json:"depth"`
    Dependencies []string `json:"dependencies"`
    Dependents   []string `json:"dependents"` // absolute paths of direct dependents
    InCycle      bool     `json:"inCycle"`    // true if this node is part of any dependency cycle
}
```

Non-stack nodes always have `Dependents: []string{}` and `InCycle: false`.

### `AnalyzeGraph(root *Node)`

New file `internal/stack/graph.go`. Single exported function:

```go
// AnalyzeGraph computes Dependents and InCycle for all nodes in the tree.
// It must be called after FindAndBuildTree has populated Dependencies on all nodes.
func AnalyzeGraph(root *Node)
```

**Algorithm — three steps:**

**Step 1: Flatten.** Walk the tree recursively, building `nodeMap map[string]*Node` (path → node). O(V).

**Step 2: Build reverse graph.** For every node N with `Dependencies: [A, B]`, append N.Path to `A.Dependents` and `B.Dependents`. Sort each `Dependents` slice. O(E).

**Step 3: Detect cycles.** DFS with two sets — `visited map[string]bool` and `inStack map[string]bool`. When a node already in `inStack` is encountered, a cycle exists. All nodes in the current DFS path that are part of the cycle get `InCycle = true`. Only stack nodes are traversed (nodes with `len(Dependencies) > 0` or `IsStack == true`).

### Integration in `FindAndBuildTree`

Call `AnalyzeGraph(root)` as the last step before returning:

```go
func FindAndBuildTree(rootDir, rootConfigFile string) (*Node, int, error) {
    // ... existing scan + deps.ParseDependencies ...
    AnalyzeGraph(root)
    return root, maxDepth, nil
}
```

### Testing

`internal/stack/graph_test.go`:

1. `TestAnalyzeGraph_BuildsDependents` — two nodes A→B: B.Dependents contains A's path, A.Dependents is empty.
2. `TestAnalyzeGraph_DetectsCycle` — A→B→C→A: all three nodes have InCycle=true.
3. `TestAnalyzeGraph_NoCycle` — linear chain A→B→C: all InCycle=false.
4. `TestAnalyzeGraph_PartialCycle` — A→B→C→B (B and C in cycle, A not): A.InCycle=false, B.InCycle=true, C.InCycle=true.
5. `TestAnalyzeGraph_NonStackNodesUntouched` — non-stack nodes keep Dependents=[] and InCycle=false.

---

## Part 2 — VS Code

### `StackNode` interface update (`treeProvider.ts`)

```typescript
export interface StackNode {
  name: string;
  path: string;
  isStack: boolean;
  depth: number;
  children: StackNode[];
  dependencies?: string[];
  dependents?: string[];   // NEW
  inCycle?: boolean;       // NEW
}
```

### Cycle indicator in Stacks panel (`treeProvider.ts`)

In `getTreeItem`, stack nodes with `node.inCycle === true` get `$(warning)` as their icon instead of `$(package)`:

```typescript
if (node.isStack) {
  item.contextValue = 'terraxStack';
  item.iconPath = node.inCycle
    ? new vscode.ThemeIcon('warning')
    : new vscode.ThemeIcon('package');
}
```

### `DependentsTreeProvider` (`dependencyProvider.ts`)

Add `DependentsTreeProvider` to the existing `dependencyProvider.ts` file (alongside `DependencyTreeProvider`). Identical structure but reads `stackNode.dependents ?? []` instead of `stackNode.dependencies ?? []`:

```typescript
export class DependentsTreeProvider implements vscode.TreeDataProvider<DepNode> {
  // identical fields and methods to DependencyTreeProvider
  // getChildren reads dep.stackNode.dependents ?? []
  // getTreeItem identical
}
```

The `DepNode` interface, `MAX_DEPTH`, and `pathsToDepNodes` helper are shared within the file.

### `package.json` — third view

Add to `"views".terrax` array:

```json
{ "id": "terrax.dependentsTree", "name": "Dependents" }
```

### `extension.ts` updates

```typescript
const dependentsProvider = new DependentsTreeProvider();
vscode.window.createTreeView('terrax.dependentsTree', { treeDataProvider: dependentsProvider });

treeView.onDidChangeSelection((e) => {
  const node = e.selection[0] ?? null;
  depProvider.setFocus(node);
  dependentsProvider.setFocus(node);
});
```

Update `doRefresh()` to also call `dependentsProvider.setTree(treeProvider.getTree())`.

---

## Build verification

- `task check` — all Go tests pass.
- `task ext:build` — TypeScript compiles.
- `task ext:package` — `.vsix` builds.
