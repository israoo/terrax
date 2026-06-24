# Cycle Detection and Reverse Dependency Graph Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add cycle detection and reverse dependency (dependents) graph to TerraX, exposing them in `terrax tree --json` and surfacing them in two VS Code panels — cycle warnings in the Stacks tree and a new "Dependents" panel.

**Architecture:** Two tasks. Task 1 is pure Go: add `Dependents []string` and `InCycle bool` to `stack.Node`, implement `AnalyzeGraph(*Node)` in `internal/stack/graph.go`, and call it at the end of `FindAndBuildTree`. Task 2 is pure TypeScript: update the `StackNode` interface, add cycle icon to the Stacks tree, add `DependentsTreeProvider` to `dependencyProvider.ts`, register a third view, and wire selection and refresh.

**Tech Stack:** Go 1.25.5 · TypeScript 5.9.3 · VS Code Extension API 1.125.0

## Global Constraints

- All Go comments must end with a period.
- Go imports: three groups (stdlib / third-party / `github.com/israoo/terrax/...`), alphabetically sorted within each group.
- Errors always wrapped: `fmt.Errorf("context: %w", err)`.
- Run `task check` before each Go commit.
- Run `task ext:build` and `task ext:package` before each TypeScript commit.
- Non-stack nodes always have `Dependents: []string{}` (empty array, never null) and `InCycle: false` in JSON output.
- `DependentsTreeProvider` uses `item.id` format `dependent-${path}-${depth}` (not `dep-`).

---

### Task 1: Go — `AnalyzeGraph`, `stack.Node` fields, builder integration

**Files:**
- Modify: `internal/stack/tree.go` — add `Dependents []string` and `InCycle bool` to `Node`
- Create: `internal/stack/graph.go` — `AnalyzeGraph(*Node)` with DFS cycle detection and reverse graph
- Create: `internal/stack/graph_test.go` — 5 unit tests
- Modify: `internal/stack/builder.go` — initialize new fields, call `AnalyzeGraph(root)`

**Interfaces:**
- Produces: `stack.AnalyzeGraph(root *Node)` — populates `Dependents` and `InCycle` in-place on all nodes
- Produces: `stack.Node.Dependents []string` with json tag `"dependents"`
- Produces: `stack.Node.InCycle bool` with json tag `"inCycle"`

- [ ] **Step 1: Write the failing tests in `internal/stack/graph_test.go`**

```go
package stack

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

func makeTestNode(path string, isStack bool, deps []string) *Node {
	if deps == nil {
		deps = []string{}
	}
	return &Node{
		Name:         filepath.Base(path),
		Path:         path,
		IsStack:      isStack,
		Children:     []*Node{},
		Depth:        0,
		Dependencies: deps,
		Dependents:   []string{},
		InCycle:      false,
	}
}

func TestAnalyzeGraph_BuildsDependents(t *testing.T) {
	root := makeTestNode("/root", false, nil)
	a := makeTestNode("/a", true, []string{"/b"})
	b := makeTestNode("/b", true, nil)
	root.Children = []*Node{a, b}

	AnalyzeGraph(root)

	assert.Equal(t, []string{"/a"}, b.Dependents)
	assert.Empty(t, a.Dependents)
	assert.False(t, a.InCycle)
	assert.False(t, b.InCycle)
}

func TestAnalyzeGraph_DetectsCycle(t *testing.T) {
	root := makeTestNode("/root", false, nil)
	a := makeTestNode("/a", true, []string{"/b"})
	b := makeTestNode("/b", true, []string{"/c"})
	c := makeTestNode("/c", true, []string{"/a"})
	root.Children = []*Node{a, b, c}

	AnalyzeGraph(root)

	assert.True(t, a.InCycle)
	assert.True(t, b.InCycle)
	assert.True(t, c.InCycle)
}

func TestAnalyzeGraph_NoCycle(t *testing.T) {
	root := makeTestNode("/root", false, nil)
	a := makeTestNode("/a", true, []string{"/b"})
	b := makeTestNode("/b", true, []string{"/c"})
	c := makeTestNode("/c", true, nil)
	root.Children = []*Node{a, b, c}

	AnalyzeGraph(root)

	assert.False(t, a.InCycle)
	assert.False(t, b.InCycle)
	assert.False(t, c.InCycle)
}

func TestAnalyzeGraph_PartialCycle(t *testing.T) {
	// A → B → C → B (B and C form a cycle, A does not)
	root := makeTestNode("/root", false, nil)
	a := makeTestNode("/a", true, []string{"/b"})
	b := makeTestNode("/b", true, []string{"/c"})
	c := makeTestNode("/c", true, []string{"/b"})
	root.Children = []*Node{a, b, c}

	AnalyzeGraph(root)

	assert.False(t, a.InCycle)
	assert.True(t, b.InCycle)
	assert.True(t, c.InCycle)
}

func TestAnalyzeGraph_NonStackNodesUntouched(t *testing.T) {
	root := makeTestNode("/root", false, nil)
	dir := makeTestNode("/dir", false, nil)
	stack := makeTestNode("/dir/stack", true, nil)
	dir.Children = []*Node{stack}
	root.Children = []*Node{dir}

	AnalyzeGraph(root)

	assert.False(t, dir.InCycle)
	assert.Empty(t, dir.Dependents)
}
```

- [ ] **Step 2: Run tests to verify they fail**

```bash
go test ./internal/stack/... -run TestAnalyzeGraph -v
```

Expected: compile error — `AnalyzeGraph` undefined.

- [ ] **Step 3: Add `Dependents` and `InCycle` to `Node` in `internal/stack/tree.go`**

Replace the existing struct:

```go
// Node represents a directory node in the stack tree.
type Node struct {
	Name         string   `json:"name"`
	Path         string   `json:"path"`
	IsStack      bool     `json:"isStack"`
	Children     []*Node  `json:"children"`
	Depth        int      `json:"depth"`
	Dependencies []string `json:"dependencies"`
	Dependents   []string `json:"dependents"`
	InCycle      bool     `json:"inCycle"`
}
```

- [ ] **Step 4: Create `internal/stack/graph.go`**

```go
package stack

import "sort"

// AnalyzeGraph computes Dependents and InCycle for all nodes in the tree.
// It must be called after FindAndBuildTree has populated Dependencies on all nodes.
// Non-stack nodes are left with Dependents: []string{} and InCycle: false.
func AnalyzeGraph(root *Node) {
	nodeMap := make(map[string]*Node)
	flattenNodes(root, nodeMap)
	buildReverseGraph(nodeMap)
	detectCycles(nodeMap)
}

// flattenNodes recursively builds a path→node map for all nodes in the tree.
func flattenNodes(node *Node, nodeMap map[string]*Node) {
	nodeMap[node.Path] = node
	for _, child := range node.Children {
		flattenNodes(child, nodeMap)
	}
}

// buildReverseGraph populates Dependents by inverting the Dependencies edges.
// Nodes with no dependents keep their existing empty slice.
func buildReverseGraph(nodeMap map[string]*Node) {
	for _, node := range nodeMap {
		for _, depPath := range node.Dependencies {
			if dep, ok := nodeMap[depPath]; ok {
				dep.Dependents = append(dep.Dependents, node.Path)
			}
		}
	}
	for _, node := range nodeMap {
		sort.Strings(node.Dependents)
	}
}

// detectCycles runs DFS from every unvisited node to mark cycle membership.
// All nodes that are part of any dependency cycle have InCycle set to true.
func detectCycles(nodeMap map[string]*Node) {
	visited := make(map[string]bool)
	inStack := make(map[string]bool)
	stackPath := []string{}

	var dfs func(path string)
	dfs = func(path string) {
		visited[path] = true
		inStack[path] = true
		stackPath = append(stackPath, path)

		if node, ok := nodeMap[path]; ok {
			for _, depPath := range node.Dependencies {
				if inStack[depPath] {
					// Found a cycle — mark all nodes from depPath to current.
					marking := false
					for _, p := range stackPath {
						if p == depPath {
							marking = true
						}
						if marking {
							if n, ok2 := nodeMap[p]; ok2 {
								n.InCycle = true
							}
						}
					}
				} else if !visited[depPath] {
					dfs(depPath)
				}
			}
		}

		stackPath = stackPath[:len(stackPath)-1]
		inStack[path] = false
	}

	for path := range nodeMap {
		if !visited[path] {
			dfs(path)
		}
	}
}
```

- [ ] **Step 5: Run tests to verify they pass**

```bash
go test ./internal/stack/... -run TestAnalyzeGraph -v
```

Expected: all 5 tests PASS.

- [ ] **Step 6: Initialize new fields and call AnalyzeGraph in `internal/stack/builder.go`**

In `FindAndBuildTree`, add `Dependents: []string{}` and `InCycle: false` (InCycle is already the zero value but explicit is clearer) to the root node, and call `AnalyzeGraph` before returning:

```go
root := &Node{
    Name:         filepath.Base(absPath),
    Path:         absPath,
    IsStack:      isStackDirectory(absPath),
    Children:     make([]*Node, 0),
    Dependencies: []string{},
    Dependents:   []string{},
    Depth:        0,
}
```

In `buildTreeRecursive`, add `Dependents: []string{}` to the child node initializer:

```go
childNode := &Node{
    Name:         entry.Name(),
    Path:         childPath,
    IsStack:      isStackDirectory(childPath),
    Children:     make([]*Node, 0),
    Dependencies: []string{},
    Dependents:   []string{},
    Depth:        node.Depth + 1,
}
```

At the end of `FindAndBuildTree`, add `AnalyzeGraph(root)` before the return:

```go
if err := buildTreeRecursive(root, &maxDepth, repoRoot); err != nil {
    return nil, 0, fmt.Errorf("failed to build tree: %w", err)
}

AnalyzeGraph(root)
return root, maxDepth, nil
```

- [ ] **Step 7: Run full check**

```bash
task check
```

Expected: all checks pass, 0 lint issues. Verify `terrax tree --json` output includes `"dependents":[]` and `"inCycle":false` on every node.

- [ ] **Step 8: Commit**

```bash
git add internal/stack/tree.go internal/stack/graph.go internal/stack/graph_test.go internal/stack/builder.go
git commit -m "feat(stack): add AnalyzeGraph for cycle detection and reverse dependency graph"
```

---

### Task 2: VS Code — cycle indicators and Dependents panel

**Files:**
- Modify: `extensions/vscode/src/treeProvider.ts` — add `dependents?` and `inCycle?` to `StackNode`, add cycle icon in `getTreeItem`
- Modify: `extensions/vscode/src/dependencyProvider.ts` — extract `buildNodeMap` to module level, add `DependentsTreeProvider` class
- Modify: `extensions/vscode/src/extension.ts` — import `DependentsTreeProvider`, register third view, update selection listener and `doRefresh`
- Modify: `extensions/vscode/package.json` — add `terrax.dependentsTree` view

**Interfaces:**
- Consumes: `StackNode.dependents?: string[]` and `StackNode.inCycle?: boolean` (populated by Task 1 JSON output)
- Produces: `DependentsTreeProvider` with `setTree(root: StackNode | null): void` and `setFocus(node: StackNode | null): void`

- [ ] **Step 1: Update `StackNode` interface and `getTreeItem` in `treeProvider.ts`**

Add two optional fields to `StackNode`:

```typescript
export interface StackNode {
  name: string;
  path: string;
  isStack: boolean;
  depth: number;
  children: StackNode[];
  dependencies?: string[];
  dependents?: string[];
  inCycle?: boolean;
}
```

In `getTreeItem`, replace the stack icon block:

```typescript
if (node.isStack) {
  item.contextValue = 'terraxStack';
  item.iconPath = node.inCycle
    ? new vscode.ThemeIcon('warning')
    : new vscode.ThemeIcon('package');
}
```

- [ ] **Step 2: Refactor `dependencyProvider.ts` and add `DependentsTreeProvider`**

Replace the entire file with:

```typescript
import * as vscode from 'vscode';
import { StackNode } from './treeProvider';

interface DepNode {
  stackNode: StackNode;
  depth: number;
}

const MAX_DEPTH = 10;

// buildNodeMap recursively indexes all nodes by path for O(1) lookup.
function buildNodeMap(node: StackNode, map: Map<string, StackNode>): void {
  map.set(node.path, node);
  for (const child of node.children) {
    buildNodeMap(child, map);
  }
}

// makeDepNodes converts a list of paths to DepNodes, using placeholders for unknown paths.
function makeDepNodes(
  nodeMap: Map<string, StackNode>,
  paths: string[],
  depth: number,
  idPrefix: string,
): DepNode[] {
  return paths.map((p) => {
    const node = nodeMap.get(p);
    if (node) {
      return { stackNode: node, depth };
    }
    return {
      stackNode: {
        name: p.split('/').pop() ?? p,
        path: p,
        isStack: true,
        depth: 0,
        children: [],
        dependencies: [],
        dependents: [],
      },
      depth,
    };
  });
}

export class DependencyTreeProvider implements vscode.TreeDataProvider<DepNode> {
  private _onDidChangeTreeData = new vscode.EventEmitter<void>();
  readonly onDidChangeTreeData = this._onDidChangeTreeData.event;

  private nodeMap = new Map<string, StackNode>();
  private focused: StackNode | null = null;

  setTree(root: StackNode | null): void {
    this.nodeMap.clear();
    if (root) {
      buildNodeMap(root, this.nodeMap);
    }
    this._onDidChangeTreeData.fire();
  }

  setFocus(node: StackNode | null): void {
    this.focused = node;
    this._onDidChangeTreeData.fire();
  }

  getTreeItem(dep: DepNode): vscode.TreeItem {
    const deps = dep.stackNode.dependencies ?? [];
    const hasExpandableChildren = deps.length > 0 && dep.depth < MAX_DEPTH;
    const item = new vscode.TreeItem(
      dep.stackNode.name,
      hasExpandableChildren
        ? vscode.TreeItemCollapsibleState.Collapsed
        : vscode.TreeItemCollapsibleState.None,
    );
    item.id = `dep-${dep.stackNode.path}-${dep.depth}`;
    item.iconPath = new vscode.ThemeIcon('package');
    item.description = dep.stackNode.path;
    return item;
  }

  getChildren(dep?: DepNode): DepNode[] {
    if (!dep) {
      if (!this.focused) return [];
      return makeDepNodes(this.nodeMap, this.focused.dependencies ?? [], 0, 'dep');
    }
    if (dep.depth >= MAX_DEPTH) return [];
    return makeDepNodes(this.nodeMap, dep.stackNode.dependencies ?? [], dep.depth + 1, 'dep');
  }
}

export class DependentsTreeProvider implements vscode.TreeDataProvider<DepNode> {
  private _onDidChangeTreeData = new vscode.EventEmitter<void>();
  readonly onDidChangeTreeData = this._onDidChangeTreeData.event;

  private nodeMap = new Map<string, StackNode>();
  private focused: StackNode | null = null;

  setTree(root: StackNode | null): void {
    this.nodeMap.clear();
    if (root) {
      buildNodeMap(root, this.nodeMap);
    }
    this._onDidChangeTreeData.fire();
  }

  setFocus(node: StackNode | null): void {
    this.focused = node;
    this._onDidChangeTreeData.fire();
  }

  getTreeItem(dep: DepNode): vscode.TreeItem {
    const deps = dep.stackNode.dependents ?? [];
    const hasExpandableChildren = deps.length > 0 && dep.depth < MAX_DEPTH;
    const item = new vscode.TreeItem(
      dep.stackNode.name,
      hasExpandableChildren
        ? vscode.TreeItemCollapsibleState.Collapsed
        : vscode.TreeItemCollapsibleState.None,
    );
    item.id = `dependent-${dep.stackNode.path}-${dep.depth}`;
    item.iconPath = new vscode.ThemeIcon('package');
    item.description = dep.stackNode.path;
    return item;
  }

  getChildren(dep?: DepNode): DepNode[] {
    if (!dep) {
      if (!this.focused) return [];
      return makeDepNodes(this.nodeMap, this.focused.dependents ?? [], 0, 'dependent');
    }
    if (dep.depth >= MAX_DEPTH) return [];
    return makeDepNodes(this.nodeMap, dep.stackNode.dependents ?? [], dep.depth + 1, 'dependent');
  }
}
```

- [ ] **Step 3: Update `extension.ts`**

Add import for `DependentsTreeProvider`:

```typescript
import { DependencyTreeProvider, DependentsTreeProvider } from './dependencyProvider';
```

After creating `depProvider`, add `dependentsProvider`:

```typescript
const dependentsProvider = new DependentsTreeProvider();

vscode.window.createTreeView('terrax.dependentsTree', {
  treeDataProvider: dependentsProvider,
});
```

Update the selection listener to also call `dependentsProvider.setFocus`:

```typescript
treeView.onDidChangeSelection((e) => {
  const node = e.selection[0] ?? null;
  depProvider.setFocus(node);
  dependentsProvider.setFocus(node);
});
```

Update `doRefresh` to also feed `dependentsProvider`:

```typescript
const doRefresh = (): void => {
  treeProvider.refresh();
  const tree = treeProvider.getTree();
  depProvider.setTree(tree);
  dependentsProvider.setTree(tree);
};
```

- [ ] **Step 4: Add `terrax.dependentsTree` view in `package.json`**

In `"views".terrax`, add the third view after `terrax.dependencyTree`:

```json
"views": {
  "terrax": [
    { "id": "terrax.stackTree",      "name": "Stacks" },
    { "id": "terrax.dependencyTree", "name": "Dependencies" },
    { "id": "terrax.dependentsTree", "name": "Dependents" }
  ]
}
```

- [ ] **Step 5: Build the extension**

```bash
task ext:build
```

Expected: TypeScript compiles with no errors.

- [ ] **Step 6: Package the extension**

```bash
task ext:package
```

Expected: `terrax-vscode-0.1.0.vsix` created.

- [ ] **Step 7: Rebuild the Go binary**

```bash
task build
```

Expected: binary at `./build/terrax` includes `dependents` and `inCycle` in JSON output.

- [ ] **Step 8: Commit**

```bash
git add extensions/vscode/src/treeProvider.ts \
        extensions/vscode/src/dependencyProvider.ts \
        extensions/vscode/src/extension.ts \
        extensions/vscode/package.json
git commit -m "feat(vscode): add cycle indicators and Dependents panel"
```
