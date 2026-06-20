# Design: `terrax tree --json` + VS Code Sidebar Panel

**Date:** 2026-06-20
**Status:** Approved

## Context

TerraX has a VS Code extension at `extensions/vscode/` that currently launches `terrax --dir <path>` from the file explorer context menu. The goal is to add a sidebar panel that displays the Terragrunt stack tree, using TerraX itself as the data source via a new `tree --json` subcommand.

## Goals

1. Add a `terrax tree --json` subcommand that prints the stack tree as JSON to stdout.
2. Add a VS Code sidebar panel (Activity Bar) that calls this subcommand to populate a tree view.
3. Handle missing `terrax` binary gracefully: panel message + notification banner.

## Non-Goals

- Commands (plan, apply, etc.) accessible from the tree nodes — click opens `terrax --dir <path>` only.
- Watch mode / auto-refresh on filesystem changes.
- Streaming output from `terrax tree`.

---

## Part 1 — `terrax tree --json` subcommand

### Files

- **Modify:** `internal/stack/tree.go` — add json tags to `Node` struct
- **Create:** `cmd/tree.go` — new Cobra subcommand

### Node JSON tags

Add json tags to the existing `Node` struct (non-breaking change, no behavior impact):

```go
type Node struct {
    Name     string  `json:"name"`
    Path     string  `json:"path"`
    IsStack  bool    `json:"isStack"`
    Children []*Node `json:"children"`
    Depth    int     `json:"depth"`
}
```

### Subcommand

`terrax tree [--dir <path>]`

- Registered as a Cobra subcommand in `cmd/tree.go`, added to `rootCmd` in `cmd/root.go`.
- Accepts `--dir <path>` (same semantics as the root command — overrides `os.Getwd()`).
- Calls `stack.FindAndBuildTree(workDir)` to build the tree.
- Marshals the root `*stack.Node` to JSON with `encoding/json` and prints to stdout.
- On error (scan failure, no stacks found), prints to stderr and exits non-zero.
- No progress messages on stdout (stdout is for JSON only); stderr is free for diagnostics.

### JSON output format

```json
{
  "name": "root",
  "path": "/abs/path/to/project",
  "isStack": false,
  "depth": 0,
  "children": [
    {
      "name": "networking",
      "path": "/abs/path/to/project/networking",
      "isStack": true,
      "depth": 1,
      "children": []
    }
  ]
}
```

### Testing

Unit test in `cmd/tree_test.go`: build a temp filesystem with known structure, call `terrax tree --json --dir <tmpdir>`, verify JSON output matches expected tree shape.

---

## Part 2 — VS Code Sidebar Panel

### Files

- **Create:** `extensions/vscode/src/treeProvider.ts` — `TerraXTreeProvider` (implements `vscode.TreeDataProvider`)
- **Modify:** `extensions/vscode/src/extension.ts` — register the tree view and refresh command
- **Modify:** `extensions/vscode/package.json` — add `viewsContainers`, `views`, commands, menus

### `package.json` additions

```json
"viewsContainers": {
  "activitybar": [
    { "id": "terrax", "title": "TerraX", "icon": "$(server-process)" }
  ]
},
"views": {
  "terrax": [
    { "id": "terrax.stackTree", "name": "Stacks" }
  ]
},
"commands": [
  { "command": "terrax.refresh", "title": "TerraX: Refresh", "icon": "$(refresh)" }
],
"menus": {
  "view/title": [
    {
      "command": "terrax.refresh",
      "when": "view == terrax.stackTree",
      "group": "navigation"
    }
  ]
}
```

### `TerraXTreeProvider`

Implements `vscode.TreeDataProvider<StackNode>`.

**State:** holds the parsed tree from `terrax tree --json` output, or an error state.

**`refresh()`:** re-runs `terrax tree --json --dir <workspaceRoot>`, updates state, fires `_onDidChangeTreeData`.

**`getChildren(element?)`:**
- If no element: returns root's children (top-level nodes).
- If element: returns that node's children.

**`getTreeItem(node)`:** returns a `vscode.TreeItem` with:
- `label`: `node.name`
- `collapsibleState`: `Collapsed` if has children, `None` if leaf
- `command`: `{ command: 'terrax.openHere', arguments: [vscode.Uri.file(node.path)] }` — clicking a node opens it in TerraX

### Missing binary handling (combined)

On `refresh()`, if the `terrax` binary is not found or the command fails:

1. **Panel message:** `getChildren()` returns a single `TreeItem` with label *"TerraX not found. Check settings or install TerraX."* and no command.
2. **Notification banner:** `vscode.window.showErrorMessage('TerraX binary not found.', 'Open Settings', 'Learn More')`:
   - **Open Settings** → `vscode.commands.executeCommand('workbench.action.openSettings', 'terrax.binaryPath')`
   - **Learn More** → `vscode.env.openExternal(vscode.Uri.parse('https://github.com/israoo/terrax/releases'))`

The banner is shown only once per `refresh()` call, not repeatedly on every `getChildren()` call.

### `extension.ts` changes

- Register `TerraXTreeProvider` via `vscode.window.createTreeView('terrax.stackTree', { treeDataProvider })`.
- Register `terrax.refresh` command that calls `treeProvider.refresh()`.
- Call `treeProvider.refresh()` once on activation.

### Error handling

- `execSync` throws if the binary is not found or exits non-zero → caught, triggers the combined error state above.
- Timeout: `execSync` called with `{ timeout: 10000 }` (10 seconds) to avoid hanging VS Code on large repos.

---

## Build & Packaging

After implementing Part 2:
- `task ext:build` must pass (TypeScript compiles cleanly).
- `task ext:package` must produce a valid `.vsix`.
- Re-install with: `code --install-extension extensions/vscode/terrax-vscode-0.1.0.vsix`
