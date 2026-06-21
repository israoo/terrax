# Design: Context Menu Commands

**Date:** 2026-06-20
**Status:** Approved

## Context

The VS Code extension currently exposes one inline button (`$(play)` Run Plan) on each node. Engineers frequently need to run other Terragrunt commands (apply, validate, init, etc.) without opening the TUI. Right-click context menus in VS Code are the standard way to expose secondary actions on tree nodes.

## Goals

1. Add right-click context menu to Stacks tree nodes with all 8 default Terragrunt commands.
2. Add "Open terragrunt.hcl" action for stack nodes (those with `contextValue: 'terraxStack'`).

## Non-Goals

- Dynamic commands based on `.terrax.yaml` — menu is static with the 8 defaults.
- Inline buttons for any command other than Plan (already exists).
- Context menu on History or Dependency tree nodes.

---

## Design

### Commands registered

| Command ID | Title | Icon | Applies to |
|---|---|---|---|
| `terrax.runPlan` | Run Plan | `$(play)` | directory + stack (already exists) |
| `terrax.runApply` | Run Apply | `$(play)` | directory + stack |
| `terrax.runValidate` | Run Validate | `$(play)` | directory + stack |
| `terrax.runInit` | Run Init | `$(play)` | directory + stack |
| `terrax.runOutput` | Run Output | `$(play)` | directory + stack |
| `terrax.runRefresh` | Run Refresh | `$(play)` | directory + stack |
| `terrax.runFmt` | Run Fmt | `$(play)` | directory + stack |
| `terrax.runDestroy` | Run Destroy | `$(warning)` | directory + stack |
| `terrax.openFile` | Open terragrunt.hcl | `$(go-to-file)` | stack only |

`terrax.runDestroy` uses `$(warning)` to visually signal it is destructive.

### Menu structure

All run commands appear in group `"terrax@1"`. Open file appears in group `"terrax@2"` — VS Code inserts a separator between groups.

```json
"view/item/context": [
  // existing inline plan button (group: "inline")
  { "command": "terrax.runPlan",     "when": "...terraxDirectory || ...terraxStack", "group": "terrax@1" },
  { "command": "terrax.runApply",    "when": "...terraxDirectory || ...terraxStack", "group": "terrax@1" },
  { "command": "terrax.runValidate", "when": "...terraxDirectory || ...terraxStack", "group": "terrax@1" },
  { "command": "terrax.runInit",     "when": "...terraxDirectory || ...terraxStack", "group": "terrax@1" },
  { "command": "terrax.runOutput",   "when": "...terraxDirectory || ...terraxStack", "group": "terrax@1" },
  { "command": "terrax.runRefresh",  "when": "...terraxDirectory || ...terraxStack", "group": "terrax@1" },
  { "command": "terrax.runFmt",      "when": "...terraxDirectory || ...terraxStack", "group": "terrax@1" },
  { "command": "terrax.runDestroy",  "when": "...terraxDirectory || ...terraxStack", "group": "terrax@1" },
  { "command": "terrax.openFile",    "when": "...terraxStack",                       "group": "terrax@2" }
]
```

Full `when` clause: `view == terrax.stackTree && viewItem == terraxDirectory` (or `terraxStack`).

### `extension.ts` changes

Seven new command registrations, all identical pattern:

```typescript
vscode.commands.registerCommand('terrax.runApply', (node: StackNode) => {
  const binaryPath = vscode.workspace.getConfiguration('terrax').get<string>('binaryPath', 'terrax');
  runInTerminal(binaryPath, node.path, 'apply');
});
```

One additional command for file opening:

```typescript
vscode.commands.registerCommand('terrax.openFile', async (node: StackNode) => {
  const filePath = path.join(node.path, 'terragrunt.hcl');
  const doc = await vscode.workspace.openTextDocument(vscode.Uri.file(filePath));
  await vscode.window.showTextDocument(doc);
});
```

All new commands pushed to `context.subscriptions`.

## Build verification

- `task ext:build` — TypeScript compiles.
- `task ext:package` — `.vsix` builds.
