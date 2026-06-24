# Context Menu Commands Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add right-click context menu to Stacks tree nodes with all 8 Terragrunt commands plus "Open terragrunt.hcl" for stack nodes.

**Architecture:** Single TypeScript task — no Go changes. Register 8 new commands in `extension.ts` (all using `runInTerminal` or `showTextDocument`), add them to `package.json` commands array and `view/item/context` menus in two groups. `terrax.runPlan` already exists and gets a context menu entry added (it keeps its inline button too).

**Tech Stack:** TypeScript 5.9.3 · VS Code Extension API 1.125.0 · node:path

## Global Constraints

- Run `task ext:build` and `task ext:package` before committing.
- All new commands pushed to `context.subscriptions`.
- `terrax.runDestroy` uses icon `$(warning)` to signal destructive action.
- Run commands use `runInTerminal(binaryPath, node.path, '<command>')` — same pattern as `terrax.runPlan`.
- `terrax.openFile` only appears for `viewItem == terraxStack` (those have `terragrunt.hcl`).
- All run commands appear for both `viewItem == terraxDirectory` and `viewItem == terraxStack`.
- Group `"terrax@1"` for run commands; group `"terrax@2"` for Open file (VS Code inserts separator between groups).

---

### Task 1: Context menu commands — `extension.ts` + `package.json`

**Files:**
- Modify: `extensions/vscode/src/extension.ts` — register 8 new commands + `terrax.openFile`
- Modify: `extensions/vscode/package.json` — add commands + menu entries

**Interfaces:**
- Consumes: `runInTerminal(binaryPath: string, itemPath: string, subcommand?: string): void` from `./terminalRunner`
- Consumes: `StackNode.path: string` from `./treeProvider`
- Consumes: `StackNode` type from `./treeProvider`

- [ ] **Step 1: Read both files before editing**

```bash
cat extensions/vscode/src/extension.ts
cat extensions/vscode/package.json
```

- [ ] **Step 2: Add 8 new commands + `terrax.openFile` to `extension.ts`**

Add `import * as path from 'node:path';` to the imports if not already present.

After the existing `runPlanCommand` registration (around line 38), add:

```typescript
const runApplyCommand = vscode.commands.registerCommand(
  'terrax.runApply',
  (node: StackNode) => {
    const binaryPath = vscode.workspace.getConfiguration('terrax').get<string>('binaryPath', 'terrax');
    runInTerminal(binaryPath, node.path, 'apply');
  },
);

const runValidateCommand = vscode.commands.registerCommand(
  'terrax.runValidate',
  (node: StackNode) => {
    const binaryPath = vscode.workspace.getConfiguration('terrax').get<string>('binaryPath', 'terrax');
    runInTerminal(binaryPath, node.path, 'validate');
  },
);

const runInitCommand = vscode.commands.registerCommand(
  'terrax.runInit',
  (node: StackNode) => {
    const binaryPath = vscode.workspace.getConfiguration('terrax').get<string>('binaryPath', 'terrax');
    runInTerminal(binaryPath, node.path, 'init');
  },
);

const runOutputCommand = vscode.commands.registerCommand(
  'terrax.runOutput',
  (node: StackNode) => {
    const binaryPath = vscode.workspace.getConfiguration('terrax').get<string>('binaryPath', 'terrax');
    runInTerminal(binaryPath, node.path, 'output');
  },
);

const runRefreshCommand = vscode.commands.registerCommand(
  'terrax.runRefresh',
  (node: StackNode) => {
    const binaryPath = vscode.workspace.getConfiguration('terrax').get<string>('binaryPath', 'terrax');
    runInTerminal(binaryPath, node.path, 'refresh');
  },
);

const runFmtCommand = vscode.commands.registerCommand(
  'terrax.runFmt',
  (node: StackNode) => {
    const binaryPath = vscode.workspace.getConfiguration('terrax').get<string>('binaryPath', 'terrax');
    runInTerminal(binaryPath, node.path, 'fmt');
  },
);

const runDestroyCommand = vscode.commands.registerCommand(
  'terrax.runDestroy',
  (node: StackNode) => {
    const binaryPath = vscode.workspace.getConfiguration('terrax').get<string>('binaryPath', 'terrax');
    runInTerminal(binaryPath, node.path, 'destroy');
  },
);

const openFileCommand = vscode.commands.registerCommand(
  'terrax.openFile',
  async (node: StackNode) => {
    const filePath = path.join(node.path, 'terragrunt.hcl');
    const doc = await vscode.workspace.openTextDocument(vscode.Uri.file(filePath));
    await vscode.window.showTextDocument(doc);
  },
);
```

Add all 9 new commands to `context.subscriptions.push(...)`:

```typescript
context.subscriptions.push(
  openHereCommand,
  runPlanCommand,
  runApplyCommand,
  runValidateCommand,
  runInitCommand,
  runOutputCommand,
  runRefreshCommand,
  runFmtCommand,
  runDestroyCommand,
  openFileCommand,
  treeView,
  depTreeView,
  dependentsTreeView,
  historyTreeView,
  refreshCommand,
  expandAllCommand,
  refreshHistoryCommand,
  reRunHistoryCommand,
  folderChangeListener,
);
```

- [ ] **Step 3: Add commands to `package.json` commands array**

Add after the existing `terrax.runPlan` entry in the `"commands"` array:

```json
{
  "command": "terrax.runApply",
  "title": "Run Apply",
  "icon": "$(play)"
},
{
  "command": "terrax.runValidate",
  "title": "Run Validate",
  "icon": "$(play)"
},
{
  "command": "terrax.runInit",
  "title": "Run Init",
  "icon": "$(play)"
},
{
  "command": "terrax.runOutput",
  "title": "Run Output",
  "icon": "$(play)"
},
{
  "command": "terrax.runRefresh",
  "title": "Run Refresh",
  "icon": "$(play)"
},
{
  "command": "terrax.runFmt",
  "title": "Run Fmt",
  "icon": "$(play)"
},
{
  "command": "terrax.runDestroy",
  "title": "Run Destroy",
  "icon": "$(warning)"
},
{
  "command": "terrax.openFile",
  "title": "Open terragrunt.hcl",
  "icon": "$(go-to-file)"
}
```

- [ ] **Step 4: Add menu entries to `package.json` `view/item/context`**

The full `view/item/context` array after this change (replace the existing one):

```json
"view/item/context": [
  {
    "command": "terrax.runPlan",
    "when": "view == terrax.stackTree && viewItem == terraxDirectory",
    "group": "inline"
  },
  {
    "command": "terrax.runPlan",
    "when": "view == terrax.stackTree && viewItem == terraxStack",
    "group": "inline"
  },
  {
    "command": "terrax.reRunHistory",
    "when": "view == terrax.historyTree && viewItem == terraxHistoryEntry",
    "group": "inline"
  },
  {
    "command": "terrax.runPlan",
    "when": "view == terrax.stackTree && (viewItem == terraxDirectory || viewItem == terraxStack)",
    "group": "terrax@1"
  },
  {
    "command": "terrax.runApply",
    "when": "view == terrax.stackTree && (viewItem == terraxDirectory || viewItem == terraxStack)",
    "group": "terrax@1"
  },
  {
    "command": "terrax.runValidate",
    "when": "view == terrax.stackTree && (viewItem == terraxDirectory || viewItem == terraxStack)",
    "group": "terrax@1"
  },
  {
    "command": "terrax.runInit",
    "when": "view == terrax.stackTree && (viewItem == terraxDirectory || viewItem == terraxStack)",
    "group": "terrax@1"
  },
  {
    "command": "terrax.runOutput",
    "when": "view == terrax.stackTree && (viewItem == terraxDirectory || viewItem == terraxStack)",
    "group": "terrax@1"
  },
  {
    "command": "terrax.runRefresh",
    "when": "view == terrax.stackTree && (viewItem == terraxDirectory || viewItem == terraxStack)",
    "group": "terrax@1"
  },
  {
    "command": "terrax.runFmt",
    "when": "view == terrax.stackTree && (viewItem == terraxDirectory || viewItem == terraxStack)",
    "group": "terrax@1"
  },
  {
    "command": "terrax.runDestroy",
    "when": "view == terrax.stackTree && (viewItem == terraxDirectory || viewItem == terraxStack)",
    "group": "terrax@1"
  },
  {
    "command": "terrax.openFile",
    "when": "view == terrax.stackTree && viewItem == terraxStack",
    "group": "terrax@2"
  }
]
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

- [ ] **Step 7: Commit**

```bash
git add extensions/vscode/src/extension.ts extensions/vscode/package.json
git commit -m "feat(vscode): add right-click context menu with all Terragrunt commands and Open file"
```
