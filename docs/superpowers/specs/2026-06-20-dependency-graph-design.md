# Design: Static HCL Dependency Graph

**Date:** 2026-06-20
**Status:** Approved

## Context

Terragrunt stacks declare dependencies via `dependency "name" { config_path = "..." }` blocks in their `terragrunt.hcl` files. Understanding the dependency chain is useful for planning and impact analysis. Running `terragrunt graph-dependencies` is correct but slow. Static parsing of HCL files covers 100% of the patterns found in the target repo and takes milliseconds.

## Goals

1. Parse dependency declarations from `terragrunt.hcl` files statically (no terragrunt subprocess).
2. Enrich `terrax tree --json` output with a `dependencies` field per stack node.
3. Add a VS Code "Dependencies" panel that shows the dependency graph for the selected stack.

## Non-Goals

- Dynamic expression evaluation beyond `${get_repo_root()}` substitution.
- Transitive dependency resolution in Go (the extension builds the transitive graph client-side from the flat data).
- `dependencies { paths = [...] }` block support (not used in the target repo).

---

## Part 1 — Go: `internal/deps` package

### Package responsibility

Single function: given a `terragrunt.hcl` file path and the repo root, return the absolute paths of its direct dependencies.

```go
// Package deps parses Terragrunt HCL files to extract static dependency declarations.
package deps

// ParseDependencies reads a terragrunt.hcl file and returns the absolute paths
// of all declared dependencies, following include blocks to _envcommon files.
func ParseDependencies(hclFilePath, repoRoot string) ([]string, error)
```

### Parsing strategy (regex, no new Go dependencies)

**Step 1 — Read the file.** If the file does not exist, return empty slice (no error).

**Step 2 — Extract `include` paths.** Regex:
```
include\s+"[^"]+"\s*\{[^}]*path\s*=\s*"([^"]+)"
```
Resolve `${get_repo_root()}` → `repoRoot`, `find_in_parent_folders(...)` → skip (root config, no deps).
Read each resolved include file and collect its content alongside the original.

**Step 3 — Extract `dependency` blocks from all collected content.** Regex:
```
dependency\s+"[^"]+"\s*\{[^}]*config_path\s*=\s*"([^"]+)"[^}]*\}
```
For each `config_path`:
- Relative path (starts with `../` or `./`) → resolve against the directory of the file that declared it.
- `${get_repo_root()}/...` → resolve against `repoRoot`.

**Step 4 — Deduplicate and return** absolute paths sorted lexicographically.

### Repo root detection

`FindRepoRoot(startDir, rootConfigFile string) string`:
- Walk up from `startDir` until a directory containing `rootConfigFile` (default: `root.hcl`) is found.
- If not found, return `startDir` as fallback.
- Called once per `FindAndBuildTree` invocation; result passed to all `ParseDependencies` calls.

### `stack.Node` change

Add `Dependencies` field with json tag:

```go
type Node struct {
    Name         string   `json:"name"`
    Path         string   `json:"path"`
    IsStack      bool     `json:"isStack"`
    Children     []*Node  `json:"children"`
    Depth        int      `json:"depth"`
    Dependencies []string `json:"dependencies"` // absolute paths of direct dependencies
}
```

Non-stack nodes always have `dependencies: []` (empty array, not null).

### Integration point in `internal/stack/builder.go`

`FindAndBuildTree` detects the repo root once, then populates `Dependencies` on each node in `buildTreeRecursive` when `node.IsStack == true`:

```go
// Inside buildTreeRecursive, after child.IsStack is set:
if child.IsStack {
    hclFile := filepath.Join(child.Path, "terragrunt.hcl")
    child.Dependencies, _ = deps.ParseDependencies(hclFile, repoRoot)
}
```

Errors are silently ignored per-node (missing file, unreadable include) — the tree still builds, the node just has no dependencies.

### Testing

`internal/deps/parser_test.go` using `afero.MemMapFs`-style temp directories:

1. `TestParseDependencies_StaticRelative` — single `dependency` block with `../path` → returns resolved absolute path.
2. `TestParseDependencies_GetRepoRoot` — `config_path` with `${get_repo_root()}/...` → resolved correctly.
3. `TestParseDependencies_FollowsInclude` — `include` pointing to a file that declares dependencies → those deps are returned.
4. `TestParseDependencies_MissingFile` — file does not exist → returns empty slice, no error.
5. `TestFindRepoRoot` — walks up from a nested dir to find `root.hcl`.

---

## Part 2 — VS Code: Dependencies panel

### `package.json` additions

Add second view to the `terrax` container:

```json
"views": {
  "terrax": [
    { "id": "terrax.stackTree", "name": "Stacks" },
    { "id": "terrax.dependencyTree", "name": "Dependencies" }
  ]
}
```

### `extensions/vscode/src/dependencyProvider.ts`

New file. `DependencyTreeProvider` implements `vscode.TreeDataProvider<DepNode>`.

```typescript
interface DepNode {
  stackNode: StackNode;
  depth: number;        // tracks recursion to prevent infinite loops
}
```

**State:**
- `nodeMap: Map<string, StackNode>` — path → StackNode, built from the full tree on refresh.
- `focused: StackNode | null` — currently selected node in the Stacks tree.

**`setTree(root: StackNode)`** — rebuilds `nodeMap` by walking the full tree. Called by `TerraXTreeProvider` after each refresh.

**`setFocus(node: StackNode | null)`** — sets `focused`, fires `_onDidChangeTreeData`.

**`getChildren(dep?: DepNode)`:**
- If no `dep`: return direct dependencies of `focused` as `DepNode[]` with `depth: 0`.
- If `dep`: return `dep.stackNode`'s dependencies as `DepNode[]` with `depth + 1`.
- Guard: if `depth >= 10`, return empty (prevents cycles / infinite expansion).
- If dependency path not found in `nodeMap`: still show as a leaf with the basename as label and a warning icon.

**`getTreeItem(dep: DepNode)`:**
- Label: `dep.stackNode.name`
- Icon: `$(package)` (same as stack nodes in main tree)
- Collapsible if the dep node itself has dependencies.

### `extension.ts` changes

1. Import `DependencyTreeProvider`.
2. Create instance: `const depProvider = new DependencyTreeProvider()`.
3. Register view: `vscode.window.createTreeView('terrax.dependencyTree', { treeDataProvider: depProvider })`.
4. After `treeProvider.refresh()`, call `depProvider.setTree(treeProvider.getTree())`.
5. Add `treeView.onDidChangeSelection` listener: calls `depProvider.setFocus(e.selection[0] ?? null)`.

### `treeProvider.ts` addition

Add `getTree(): StackNode | null` — returns the current root node (or null if not loaded). Used by `depProvider.setTree()`.

---

## Build verification

- `task check` — all Go tests pass.
- `task ext:build` — TypeScript compiles.
- `task ext:package` — `.vsix` builds.
