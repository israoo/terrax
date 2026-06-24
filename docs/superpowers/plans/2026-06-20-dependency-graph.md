# Dependency Graph Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Parse Terragrunt `dependency` blocks statically from HCL files and expose them in `terrax tree --json`, plus a VS Code "Dependencies" panel that shows the dependency graph for the selected stack.

**Architecture:** Three sequential tasks. Task 1 builds an isolated `internal/deps` package with regex-based HCL parsing and include chain following. Task 2 enriches `stack.Node` with a `Dependencies []string` field and integrates the parser into `FindAndBuildTree`. Task 3 adds a VS Code `dependencyProvider.ts` and registers a second tree view that reacts to selection changes in the Stacks panel.

**Tech Stack:** Go 1.25.5 · regexp · TypeScript 5.9.3 · VS Code Extension API 1.125.0

## Global Constraints

- All Go comments must end with a period.
- Go imports: three groups (stdlib / third-party / `github.com/israoo/terrax/...`), alphabetically sorted.
- Errors always wrapped: `fmt.Errorf("context: %w", err)`.
- Run `task check` before each Go commit (fmt + vet + lint + test).
- Run `task ext:build` and `task ext:package` before each TypeScript commit.
- `internal/deps` package has no UI imports and no viper/cobra dependencies.
- Dependency parsing errors are silently ignored per-node — the tree must always build successfully.
- Non-stack nodes always have `dependencies: []` (empty array, never null) in JSON output.

---

### Task 1: `internal/deps` package — HCL parser and repo root detection

**Files:**
- Create: `internal/deps/parser.go`
- Create: `internal/deps/parser_test.go`

**Interfaces:**
- Produces: `deps.FindRepoRoot(startDir, rootConfigFile string) string`
- Produces: `deps.ParseDependencies(hclFilePath, repoRoot string) ([]string, error)` — returns sorted, deduplicated absolute paths of direct dependencies. Returns `[]string{}` on any error.

- [ ] **Step 1: Write the failing tests in `internal/deps/parser_test.go`**

```go
package deps

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseDependencies_StaticRelative(t *testing.T) {
	dir := t.TempDir()
	hclPath := filepath.Join(dir, "app", "terragrunt.hcl")
	require.NoError(t, os.MkdirAll(filepath.Dir(hclPath), 0755))
	require.NoError(t, os.WriteFile(hclPath, []byte(`
dependency "vpc" {
  config_path = "../vpc"
  mock_outputs = {
    id = "vpc-123"
  }
}
dependency "sg" {
  config_path = "../security-groups"
}
`), 0644))

	got, err := ParseDependencies(hclPath, dir)
	require.NoError(t, err)
	assert.Equal(t, []string{
		filepath.Join(dir, "security-groups"),
		filepath.Join(dir, "vpc"),
	}, got)
}

func TestParseDependencies_GetRepoRoot(t *testing.T) {
	dir := t.TempDir()
	hclPath := filepath.Join(dir, "infra", "s3", "terragrunt.hcl")
	require.NoError(t, os.MkdirAll(filepath.Dir(hclPath), 0755))
	require.NoError(t, os.WriteFile(hclPath, []byte(`
dependency "iam" {
  config_path = "${get_repo_root()}/management/global/iam"
}
`), 0644))

	got, err := ParseDependencies(hclPath, dir)
	require.NoError(t, err)
	assert.Equal(t, []string{filepath.Join(dir, "management", "global", "iam")}, got)
}

func TestParseDependencies_FollowsInclude(t *testing.T) {
	dir := t.TempDir()
	envcommonPath := filepath.Join(dir, "_envcommon", "app.hcl")
	require.NoError(t, os.MkdirAll(filepath.Dir(envcommonPath), 0755))
	require.NoError(t, os.WriteFile(envcommonPath, []byte(`
dependency "vpc" {
  config_path = "../vpc"
}
`), 0644))

	leafPath := filepath.Join(dir, "dev", "app", "terragrunt.hcl")
	require.NoError(t, os.MkdirAll(filepath.Dir(leafPath), 0755))
	require.NoError(t, os.WriteFile(leafPath, []byte(`
include "envcommon" {
  path = "${get_repo_root()}/_envcommon/app.hcl"
}
`), 0644))

	got, err := ParseDependencies(leafPath, dir)
	require.NoError(t, err)
	// Path resolved relative to _envcommon/, not dev/app/.
	assert.Equal(t, []string{filepath.Join(dir, "_envcommon", "vpc")}, got)
}

func TestParseDependencies_SkipsFindInParentFolders(t *testing.T) {
	dir := t.TempDir()
	hclPath := filepath.Join(dir, "app", "terragrunt.hcl")
	require.NoError(t, os.MkdirAll(filepath.Dir(hclPath), 0755))
	require.NoError(t, os.WriteFile(hclPath, []byte(`
include "root" {
  path = find_in_parent_folders("root.hcl")
}
`), 0644))

	got, err := ParseDependencies(hclPath, dir)
	require.NoError(t, err)
	assert.Empty(t, got)
}

func TestParseDependencies_MissingFile(t *testing.T) {
	got, err := ParseDependencies("/nonexistent/path/terragrunt.hcl", "/repo")
	require.NoError(t, err)
	assert.Equal(t, []string{}, got)
}

func TestFindRepoRoot_FindsRoot(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "root.hcl"), []byte(""), 0644))
	nested := filepath.Join(dir, "a", "b", "c")
	require.NoError(t, os.MkdirAll(nested, 0755))

	got := FindRepoRoot(nested, "root.hcl")
	assert.Equal(t, dir, got)
}

func TestFindRepoRoot_FallsBackToStartDir(t *testing.T) {
	dir := t.TempDir()
	got := FindRepoRoot(dir, "root.hcl")
	assert.Equal(t, dir, got)
}
```

- [ ] **Step 2: Run tests to verify they fail**

```bash
go test ./internal/deps/... -v
```

Expected: compile error — package `deps` does not exist.

- [ ] **Step 3: Create `internal/deps/parser.go`**

```go
// Package deps parses Terragrunt HCL files to extract static dependency declarations.
package deps

import (
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

var (
	// configPathRe matches config_path = "some/path" inside dependency blocks.
	configPathRe = regexp.MustCompile(`config_path\s*=\s*"([^"]+)"`)
	// includePathRe matches the path attribute inside include "name" { ... } blocks.
	// [^}] matches newlines in Go's regexp, handling multiline blocks correctly.
	includePathRe = regexp.MustCompile(`include\s+"[^"]+"\s*\{[^}]*\bpath\s*=\s*"([^"]+)"`)
)

// FindRepoRoot walks up from startDir until it finds a directory containing
// rootConfigFile, then returns that directory. Returns startDir if not found.
func FindRepoRoot(startDir, rootConfigFile string) string {
	current := startDir
	for {
		if _, err := os.Stat(filepath.Join(current, rootConfigFile)); err == nil {
			return current
		}
		parent := filepath.Dir(current)
		if parent == current {
			return startDir
		}
		current = parent
	}
}

// ParseDependencies reads a terragrunt.hcl file and returns the absolute paths
// of its direct dependencies, sorted and deduplicated. It follows include blocks
// with statically resolvable paths to envcommon files. Returns an empty slice if
// the file does not exist or cannot be read.
func ParseDependencies(hclFilePath, repoRoot string) ([]string, error) {
	raw := parseDepsFromFile(hclFilePath, repoRoot, 0)
	seen := make(map[string]bool, len(raw))
	result := make([]string, 0, len(raw))
	for _, p := range raw {
		if !seen[p] {
			seen[p] = true
			result = append(result, p)
		}
	}
	sort.Strings(result)
	return result, nil
}

// parseDepsFromFile extracts dependency paths from a single HCL file and follows
// statically resolvable include blocks recursively. depth prevents infinite loops.
func parseDepsFromFile(filePath, repoRoot string, depth int) []string {
	if depth > 5 {
		return nil
	}
	content, err := os.ReadFile(filePath)
	if err != nil {
		return nil
	}

	fileDir := filepath.Dir(filePath)
	var result []string

	for _, match := range configPathRe.FindAllStringSubmatch(string(content), -1) {
		if resolved := resolvePath(match[1], fileDir, repoRoot); resolved != "" {
			result = append(result, resolved)
		}
	}

	for _, match := range includePathRe.FindAllStringSubmatch(string(content), -1) {
		includePath := resolvePath(match[1], fileDir, repoRoot)
		if includePath == "" {
			continue
		}
		result = append(result, parseDepsFromFile(includePath, repoRoot, depth+1)...)
	}

	return result
}

// resolvePath converts a raw Terragrunt path expression to an absolute filesystem path.
// Returns an empty string for expressions that cannot be resolved statically.
func resolvePath(raw, baseDir, repoRoot string) string {
	if strings.Contains(raw, "find_in_parent_folders") || strings.Contains(raw, "get_terragrunt_dir") {
		return ""
	}
	resolved := strings.ReplaceAll(raw, "${get_repo_root()}", repoRoot)
	if strings.Contains(resolved, "${") {
		return ""
	}
	if filepath.IsAbs(resolved) {
		return filepath.Clean(resolved)
	}
	return filepath.Join(baseDir, resolved)
}
```

- [ ] **Step 4: Run tests to verify they pass**

```bash
go test ./internal/deps/... -v
```

Expected: all 7 tests PASS.

- [ ] **Step 5: Run full check**

```bash
task check
```

Expected: all checks pass, 0 lint issues.

- [ ] **Step 6: Commit**

```bash
git add internal/deps/parser.go internal/deps/parser_test.go
git commit -m "feat(deps): add static HCL dependency parser with include chain following"
```

---

### Task 2: Enrich `stack.Node` with dependencies

**Files:**
- Modify: `internal/stack/tree.go` — add `Dependencies []string` field to `Node`
- Modify: `internal/stack/builder.go` — detect repo root, populate Dependencies in stack nodes

**Interfaces:**
- Consumes: `deps.FindRepoRoot(startDir, rootConfigFile string) string` (Task 1)
- Consumes: `deps.ParseDependencies(hclFilePath, repoRoot string) ([]string, error)` (Task 1)
- Consumes: `config.DefaultRootConfigFile` — the string `"root.hcl"`
- Produces: `stack.Node.Dependencies []string` with json tag `"dependencies"`

- [ ] **Step 1: Add `Dependencies` field to `Node` in `internal/stack/tree.go`**

Replace the existing `Node` struct:

```go
// Node represents a directory node in the stack tree.
type Node struct {
	Name         string   `json:"name"`
	Path         string   `json:"path"`
	IsStack      bool     `json:"isStack"`
	Children     []*Node  `json:"children"`
	Depth        int      `json:"depth"`
	Dependencies []string `json:"dependencies"`
}
```

- [ ] **Step 2: Verify existing stack tests still pass**

```bash
go test ./internal/stack/... -v
```

Expected: all tests PASS (new field is additive, zero value `nil` marshals as `null` — Step 4 will fix this to `[]string{}`).

- [ ] **Step 3: Modify `FindAndBuildTree` in `internal/stack/builder.go`**

Change the function signature and body to detect the repo root and pass it to `buildTreeRecursive`:

```go
// FindAndBuildTree scans the filesystem starting from rootDir and builds a tree structure.
// It returns the root node, maximum depth, and any error encountered.
func FindAndBuildTree(rootDir string) (*Node, int, error) {
	if rootDir == "" {
		return nil, 0, fmt.Errorf("root directory cannot be empty")
	}

	absPath, err := filepath.Abs(rootDir)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to resolve absolute path: %w", err)
	}

	info, err := os.Stat(absPath)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to access directory: %w", err)
	}
	if !info.IsDir() {
		return nil, 0, fmt.Errorf("%s is not a directory", absPath)
	}

	repoRoot := deps.FindRepoRoot(absPath, config.DefaultRootConfigFile)

	root := &Node{
		Name:         filepath.Base(absPath),
		Path:         absPath,
		IsStack:      isStackDirectory(absPath),
		Children:     make([]*Node, 0),
		Dependencies: []string{},
		Depth:        0,
	}
	if root.IsStack {
		hclFile := filepath.Join(absPath, "terragrunt.hcl")
		root.Dependencies, _ = deps.ParseDependencies(hclFile, repoRoot)
	}

	maxDepth := 0
	if err := buildTreeRecursive(root, &maxDepth, repoRoot); err != nil {
		return nil, 0, fmt.Errorf("failed to build tree: %w", err)
	}

	return root, maxDepth, nil
}
```

- [ ] **Step 4: Update `buildTreeRecursive` to accept `repoRoot` and populate dependencies**

Replace the existing function:

```go
// buildTreeRecursive recursively builds the tree structure.
// Only includes directories that are stacks or contain stacks in their hierarchy.
func buildTreeRecursive(node *Node, maxDepth *int, repoRoot string) error {
	entries, err := os.ReadDir(node.Path)
	if err != nil {
		return nil
	}

	for _, entry := range entries {
		if !entry.IsDir() || strings.HasPrefix(entry.Name(), ".") {
			continue
		}
		if shouldSkipDirectory(entry.Name()) {
			continue
		}

		childPath := filepath.Join(node.Path, entry.Name())
		childNode := &Node{
			Name:         entry.Name(),
			Path:         childPath,
			IsStack:      isStackDirectory(childPath),
			Children:     make([]*Node, 0),
			Dependencies: []string{},
			Depth:        node.Depth + 1,
		}

		if childNode.IsStack {
			hclFile := filepath.Join(childPath, "terragrunt.hcl")
			childNode.Dependencies, _ = deps.ParseDependencies(hclFile, repoRoot)
		}

		if err := buildTreeRecursive(childNode, maxDepth, repoRoot); err != nil {
			return err
		}

		if childNode.IsStack || childNode.HasChildren() {
			node.Children = append(node.Children, childNode)
			if childNode.Depth > *maxDepth {
				*maxDepth = childNode.Depth
			}
		}
	}
	return nil
}
```

Note: update all recursive calls to `buildTreeRecursive` to pass `repoRoot`. Check the full file for any other calls.

- [ ] **Step 5: Add imports to `internal/stack/builder.go`**

The import block must be three groups, alphabetically sorted:

```go
import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/israoo/terrax/internal/config"
	"github.com/israoo/terrax/internal/deps"
)
```

- [ ] **Step 6: Run tests**

```bash
go test ./internal/stack/... ./internal/deps/... -v
```

Expected: all tests pass. Existing stack tests work because `deps.ParseDependencies` returns `[]string{}` when HCL files don't exist on the test filesystem (afero paths are virtual).

- [ ] **Step 7: Verify `terrax tree --json` output includes dependencies**

```bash
task build && ./build/terrax tree --dir . 2>/dev/null | head -c 500
```

Expected: JSON output includes `"dependencies":[]` on every node (empty for directories, populated for stack nodes if dependency HCL files exist).

- [ ] **Step 8: Run full check**

```bash
task check
```

Expected: all checks pass, 0 lint issues.

- [ ] **Step 9: Commit**

```bash
git add internal/stack/tree.go internal/stack/builder.go
git commit -m "feat(stack): enrich Node with static HCL dependency paths"
```

---

### Task 3: VS Code Dependencies panel

**Files:**
- Modify: `extensions/vscode/src/treeProvider.ts` — add `dependencies?: string[]` to `StackNode`, add `getTree()` method
- Create: `extensions/vscode/src/dependencyProvider.ts` — `DependencyTreeProvider`
- Modify: `extensions/vscode/src/extension.ts` — register second view, selection listener
- Modify: `extensions/vscode/package.json` — add `terrax.dependencyTree` view

**Interfaces:**
- Consumes: `StackNode.dependencies?: string[]` (populated by Task 2's JSON output)
- Consumes: `TerraXTreeProvider.getTree(): StackNode | null`
- Produces: `DependencyTreeProvider` with `setTree(root: StackNode | null): void` and `setFocus(node: StackNode | null): void`

- [ ] **Step 1: Update `StackNode` interface and add `getTree()` in `treeProvider.ts`**

Add `dependencies?: string[]` to the `StackNode` interface:

```typescript
export interface StackNode {
  name: string;
  path: string;
  isStack: boolean;
  depth: number;
  children: StackNode[];
  dependencies?: string[];
}
```

Add `getTree()` method to `TerraXTreeProvider` (after `getRootChildren()`):

```typescript
getTree(): StackNode | null {
  return this.tree;
}
```

- [ ] **Step 2: Create `extensions/vscode/src/dependencyProvider.ts`**

```typescript
import * as vscode from 'vscode';
import { StackNode } from './treeProvider';

interface DepNode {
  stackNode: StackNode;
  depth: number;
}

const MAX_DEPTH = 10;

export class DependencyTreeProvider implements vscode.TreeDataProvider<DepNode> {
  private _onDidChangeTreeData = new vscode.EventEmitter<void>();
  readonly onDidChangeTreeData = this._onDidChangeTreeData.event;

  private nodeMap = new Map<string, StackNode>();
  private focused: StackNode | null = null;

  setTree(root: StackNode | null): void {
    this.nodeMap.clear();
    if (root) {
      this.buildMap(root);
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
      if (!this.focused) {
        return [];
      }
      return this.pathsToDepNodes(this.focused.dependencies ?? [], 0);
    }
    if (dep.depth >= MAX_DEPTH) {
      return [];
    }
    return this.pathsToDepNodes(dep.stackNode.dependencies ?? [], dep.depth + 1);
  }

  private buildMap(node: StackNode): void {
    this.nodeMap.set(node.path, node);
    for (const child of node.children) {
      this.buildMap(child);
    }
  }

  private pathsToDepNodes(paths: string[], depth: number): DepNode[] {
    return paths.map((p) => {
      const node = this.nodeMap.get(p);
      if (node) {
        return { stackNode: node, depth };
      }
      // Dependency not in the scanned tree — show as an unresolved placeholder.
      return {
        stackNode: {
          name: p.split('/').pop() ?? p,
          path: p,
          isStack: true,
          depth: 0,
          children: [],
          dependencies: [],
        },
        depth,
      };
    });
  }
}
```

- [ ] **Step 3: Update `extension.ts`**

Add import at the top:

```typescript
import { DependencyTreeProvider } from './dependencyProvider';
```

Inside `activate`, after creating `treeView`:

```typescript
const depProvider = new DependencyTreeProvider();

vscode.window.createTreeView('terrax.dependencyTree', {
  treeDataProvider: depProvider,
});

treeView.onDidChangeSelection((e) => {
  depProvider.setFocus(e.selection[0] ?? null);
});
```

Update the `treeProvider.refresh()` call at the bottom to also feed the dependency provider:

```typescript
// Replace the single treeProvider.refresh() call with:
const doRefresh = (): void => {
  treeProvider.refresh();
  depProvider.setTree(treeProvider.getTree());
};

const refreshCommand = vscode.commands.registerCommand('terrax.refresh', doRefresh);
```

Also update the `folderChangeListener` to call `doRefresh()` instead of `treeProvider.refresh()`:

```typescript
const folderChangeListener = vscode.workspace.onDidChangeWorkspaceFolders(() => {
  const newRoot = vscode.workspace.workspaceFolders?.[0]?.uri.fsPath ?? '';
  treeProvider.updateWorkspaceRoot(newRoot);
  doRefresh();
});
```

At the end of `activate`, replace `treeProvider.refresh()` with `doRefresh()`.

The full updated `activate` function for reference:

```typescript
export function activate(context: vscode.ExtensionContext): void {
  const openHereCommand = vscode.commands.registerCommand(
    'terrax.openHere',
    async (uri?: vscode.Uri) => {
      const workspaceFolders = vscode.workspace.workspaceFolders;
      if (!workspaceFolders || workspaceFolders.length === 0) {
        vscode.window.showErrorMessage('TerraX: No workspace folder open.');
        return;
      }
      let targetPath: string;
      if (uri) {
        targetPath = uri.fsPath;
      } else if (workspaceFolders.length === 1) {
        targetPath = workspaceFolders[0].uri.fsPath;
      } else {
        const picked = await vscode.window.showWorkspaceFolderPick({
          placeHolder: 'Select a workspace folder to open in TerraX',
        });
        if (!picked) {
          return;
        }
        targetPath = picked.uri.fsPath;
      }
      const config = vscode.workspace.getConfiguration('terrax');
      const binaryPath = config.get<string>('binaryPath', 'terrax');
      runInTerminal(binaryPath, targetPath);
    },
  );

  const runPlanCommand = vscode.commands.registerCommand(
    'terrax.runPlan',
    (node: StackNode) => {
      const config = vscode.workspace.getConfiguration('terrax');
      const binaryPath = config.get<string>('binaryPath', 'terrax');
      runInTerminal(binaryPath, node.path, 'plan');
    },
  );

  const workspaceRoot = vscode.workspace.workspaceFolders?.[0]?.uri.fsPath ?? '';
  const treeProvider = new TerraXTreeProvider(workspaceRoot);
  const depProvider = new DependencyTreeProvider();

  const treeView = vscode.window.createTreeView('terrax.stackTree', {
    treeDataProvider: treeProvider,
    showCollapseAll: true,
  });

  vscode.window.createTreeView('terrax.dependencyTree', {
    treeDataProvider: depProvider,
  });

  treeView.onDidChangeSelection((e) => {
    depProvider.setFocus(e.selection[0] ?? null);
  });

  const doRefresh = (): void => {
    treeProvider.refresh();
    depProvider.setTree(treeProvider.getTree());
  };

  const refreshCommand = vscode.commands.registerCommand('terrax.refresh', doRefresh);

  const expandAllCommand = vscode.commands.registerCommand('terrax.expandAll', async () => {
    for (const node of treeProvider.getAllNodes()) {
      await treeView.reveal(node, { expand: true });
    }
  });

  const folderChangeListener = vscode.workspace.onDidChangeWorkspaceFolders(() => {
    const newRoot = vscode.workspace.workspaceFolders?.[0]?.uri.fsPath ?? '';
    treeProvider.updateWorkspaceRoot(newRoot);
    doRefresh();
  });

  context.subscriptions.push(
    openHereCommand, runPlanCommand, treeView, refreshCommand, expandAllCommand, folderChangeListener,
  );

  doRefresh();
}

export function deactivate(): void {}
```

- [ ] **Step 4: Update `package.json` — add `terrax.dependencyTree` view**

In the `"views"` object, replace:

```json
"views": {
  "terrax": [
    {
      "id": "terrax.stackTree",
      "name": "Stacks"
    }
  ]
}
```

With:

```json
"views": {
  "terrax": [
    {
      "id": "terrax.stackTree",
      "name": "Stacks"
    },
    {
      "id": "terrax.dependencyTree",
      "name": "Dependencies"
    }
  ]
}
```

- [ ] **Step 5: Build the extension**

```bash
task ext:build
```

Expected: TypeScript compiles with no errors. `out/dependencyProvider.js` created.

- [ ] **Step 6: Package and verify**

```bash
task ext:package
```

Expected: `terrax-vscode-0.1.0.vsix` created successfully.

- [ ] **Step 7: Rebuild the Go binary so it outputs `dependencies` in JSON**

```bash
task build
```

Expected: `./build/terrax` built with the enriched `Node` struct.

- [ ] **Step 8: Commit**

```bash
git add extensions/vscode/src/treeProvider.ts \
        extensions/vscode/src/dependencyProvider.ts \
        extensions/vscode/src/extension.ts \
        extensions/vscode/package.json
git commit -m "feat(vscode): add Dependencies panel showing selected stack's dependency graph"
```
