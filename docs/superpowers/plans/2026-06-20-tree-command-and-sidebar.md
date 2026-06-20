# Tree Command and VS Code Sidebar Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add `terrax tree --json` subcommand and a VS Code sidebar panel that calls it to display the Terragrunt stack tree.

**Architecture:** Two independent tasks. Task 1 adds json tags to `stack.Node` and a Cobra subcommand `tree` that marshals the scanned tree to stdout as JSON. Task 2 adds a `TreeDataProvider` to the VS Code extension that calls `terrax tree --dir <workspace>` via `spawnSync` and populates an Activity Bar panel; missing-binary errors surface both in the panel and as a notification banner.

**Tech Stack:** Go 1.25.5 · Cobra · encoding/json · TypeScript 5.9.3 · VS Code Extension API 1.125.0 · child_process.spawnSync

## Global Constraints

- All Go comments must end with a period.
- Go imports: three groups (stdlib / third-party / `github.com/israoo/terrax/...`), alphabetically sorted within each group.
- Errors always wrapped: `fmt.Errorf("context: %w", err)`.
- Run `task check` before each Go commit (fmt + vet + lint + test).
- Run `task ext:build` and `task ext:package` before each TypeScript commit.
- Extension lives at `extensions/vscode/` — never `.vscode/`.
- VS Code command IDs: `terrax.openHere` (existing), `terrax.refresh` (new).
- Tree view ID: `terrax.stackTree`. Activity Bar container ID: `terrax`.

---

### Task 1: `terrax tree --json` subcommand

**Files:**
- Modify: `internal/stack/tree.go` — add json tags to `Node` struct
- Create: `cmd/tree.go` — Cobra subcommand
- Create: `cmd/tree_test.go` — unit tests

**Interfaces:**
- Consumes: `stack.FindAndBuildTree(workDir string) (*stack.Node, int, error)` (existing)
- Consumes: `getWorkingDirectory(dir string) (string, error)` (existing in `cmd/root.go`)
- Produces: `terrax tree [--dir <path>]` — prints JSON to stdout, exits non-zero on error

- [ ] **Step 1: Add json tags to `internal/stack/tree.go`**

Replace the `Node` struct:

```go
// Node represents a directory node in the stack tree.
type Node struct {
	Name     string  `json:"name"`
	Path     string  `json:"path"`
	IsStack  bool    `json:"isStack"`
	Children []*Node `json:"children"`
	Depth    int     `json:"depth"`
}
```

- [ ] **Step 2: Verify existing tests still pass**

```bash
go test ./internal/stack/... -v
```

Expected: all tests pass (json tags are additive, no behavior change).

- [ ] **Step 3: Write the failing tests in `cmd/tree_test.go`**

Create `cmd/tree_test.go`:

```go
package cmd

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/israoo/terrax/internal/stack"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTreeCommand_OutputsValidJSON(t *testing.T) {
	tmpDir := t.TempDir()
	subDir := filepath.Join(tmpDir, "networking")
	require.NoError(t, os.MkdirAll(subDir, 0755))
	require.NoError(t, os.WriteFile(
		filepath.Join(subDir, "terragrunt.hcl"), []byte(""), 0644,
	))

	restore := captureStdout(t)
	rootCmd.SetArgs([]string{"tree", "--dir", tmpDir})
	err := rootCmd.Execute()
	output := restore()

	require.NoError(t, err)

	var root stack.Node
	require.NoError(t, json.Unmarshal([]byte(output), &root), "output must be valid JSON")
	assert.Equal(t, filepath.Base(tmpDir), root.Name)
	// Note: root.Path may differ from tmpDir on macOS due to symlink resolution (/private/tmp vs /tmp).
	assert.True(t, filepath.IsAbs(root.Path))
	require.Len(t, root.Children, 1)
	assert.Equal(t, "networking", root.Children[0].Name)
	assert.True(t, root.Children[0].IsStack)
}

func TestTreeCommand_InvalidDir(t *testing.T) {
	rootCmd.SetArgs([]string{"tree", "--dir", "/nonexistent/path-that-does-not-exist"})
	err := rootCmd.Execute()
	assert.Error(t, err)
}
```

- [ ] **Step 4: Run tests to verify they fail**

```bash
go test ./cmd/... -run TestTreeCommand -v
```

Expected: FAIL — `treeCmd` is not registered yet.

- [ ] **Step 5: Create `cmd/tree.go`**

```go
package cmd

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/israoo/terrax/internal/stack"
)

var treeCmd = &cobra.Command{
	Use:   "tree",
	Short: "Print the Terragrunt stack tree as JSON",
	Long:  `Print the Terragrunt stack tree as JSON for consumption by external tools such as editor extensions.`,
	RunE:  runTree,
}

func init() {
	treeCmd.Flags().String("dir", "", "Working directory (overrides current directory)")
	rootCmd.AddCommand(treeCmd)
}

func runTree(cmd *cobra.Command, args []string) error {
	dirFlag, _ := cmd.Flags().GetString("dir")
	workDir, err := getWorkingDirectory(dirFlag)
	if err != nil {
		return fmt.Errorf("failed to get working directory: %w", err)
	}

	root, _, err := stack.FindAndBuildTree(workDir)
	if err != nil {
		return fmt.Errorf("failed to build stack tree: %w", err)
	}

	data, err := json.Marshal(root)
	if err != nil {
		return fmt.Errorf("failed to serialize tree: %w", err)
	}

	fmt.Fprintln(os.Stdout, string(data))
	return nil
}
```

- [ ] **Step 6: Run tests to verify they pass**

```bash
go test ./cmd/... -run TestTreeCommand -v
```

Expected: both tests PASS.

- [ ] **Step 7: Run full check**

```bash
task check
```

Expected: all checks pass, 0 lint issues.

- [ ] **Step 8: Commit**

```bash
git add internal/stack/tree.go cmd/tree.go cmd/tree_test.go
git commit -m "feat: add terrax tree --json subcommand"
```

---

### Task 2: VS Code sidebar panel

**Files:**
- Create: `extensions/vscode/icons/terrax.svg` — Activity Bar icon
- Create: `extensions/vscode/src/treeProvider.ts` — `TerraXTreeProvider`
- Modify: `extensions/vscode/src/extension.ts` — register view and refresh command
- Modify: `extensions/vscode/package.json` — add viewsContainers, views, commands, menus

**Interfaces:**
- Consumes: `terrax tree --dir '<path>'` (produced by Task 1) via `spawnSync`
- Consumes: `runInTerminal(binaryPath, itemPath)` from `./terminalRunner` (existing)
- Produces: `TerraXTreeProvider` class with `refresh(): void` method

- [ ] **Step 1: Create `extensions/vscode/icons/terrax.svg`**

```svg
<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 16 16" fill="currentColor">
  <path d="M1 2h14v2H1V2zm2 4h10v2H3V6zm2 4h6v2H5v-2z"/>
</svg>
```

- [ ] **Step 2: Create `extensions/vscode/src/treeProvider.ts`**

```typescript
import { spawnSync } from 'child_process';
import * as vscode from 'vscode';

export interface StackNode {
  name: string;
  path: string;
  isStack: boolean;
  depth: number;
  children: StackNode[];
}

const ERROR_NODE: StackNode = {
  name: 'TerraX not found. Check settings or install TerraX.',
  path: '',
  isStack: false,
  depth: 0,
  children: [],
};

export class TerraXTreeProvider implements vscode.TreeDataProvider<StackNode> {
  private _onDidChangeTreeData =
    new vscode.EventEmitter<StackNode | undefined | null | void>();
  readonly onDidChangeTreeData = this._onDidChangeTreeData.event;

  private tree: StackNode | null = null;
  private hasError = false;

  constructor(private readonly workspaceRoot: string) {}

  refresh(): void {
    this.tree = null;
    this.hasError = false;

    const config = vscode.workspace.getConfiguration('terrax');
    const binaryPath = config.get<string>('binaryPath', 'terrax');

    const result = spawnSync(
      binaryPath,
      ['tree', '--dir', this.workspaceRoot],
      { timeout: 10000, encoding: 'utf-8' },
    );

    if (result.error || result.status !== 0) {
      this.hasError = true;
      vscode.window
        .showErrorMessage('TerraX binary not found.', 'Open Settings', 'Learn More')
        .then((selection) => {
          if (selection === 'Open Settings') {
            vscode.commands.executeCommand(
              'workbench.action.openSettings',
              'terrax.binaryPath',
            );
          } else if (selection === 'Learn More') {
            vscode.env.openExternal(
              vscode.Uri.parse('https://github.com/israoo/terrax/releases'),
            );
          }
        });
    } else {
      this.tree = JSON.parse(result.stdout) as StackNode;
    }

    this._onDidChangeTreeData.fire();
  }

  getTreeItem(node: StackNode): vscode.TreeItem {
    if (!node.path) {
      const item = new vscode.TreeItem(node.name, vscode.TreeItemCollapsibleState.None);
      item.iconPath = new vscode.ThemeIcon('warning');
      return item;
    }

    const item = new vscode.TreeItem(
      node.name,
      node.children.length > 0
        ? vscode.TreeItemCollapsibleState.Collapsed
        : vscode.TreeItemCollapsibleState.None,
    );
    item.command = {
      command: 'terrax.openHere',
      title: 'Open in TerraX',
      arguments: [vscode.Uri.file(node.path)],
    };
    if (node.isStack) {
      item.iconPath = new vscode.ThemeIcon('package');
    }
    return item;
  }

  getChildren(element?: StackNode): StackNode[] {
    if (this.hasError) {
      return [ERROR_NODE];
    }
    if (!this.tree) {
      return [];
    }
    return element ? element.children : this.tree.children;
  }
}
```

- [ ] **Step 3: Update `extensions/vscode/src/extension.ts`**

Replace the entire file:

```typescript
import * as vscode from 'vscode';
import { runInTerminal } from './terminalRunner';
import { TerraXTreeProvider } from './treeProvider';

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

  const workspaceRoot = vscode.workspace.workspaceFolders?.[0]?.uri.fsPath ?? '';
  const treeProvider = new TerraXTreeProvider(workspaceRoot);

  const treeView = vscode.window.createTreeView('terrax.stackTree', {
    treeDataProvider: treeProvider,
  });

  const refreshCommand = vscode.commands.registerCommand('terrax.refresh', () => {
    treeProvider.refresh();
  });

  context.subscriptions.push(openHereCommand, treeView, refreshCommand);

  treeProvider.refresh();
}

export function deactivate(): void {}
```

- [ ] **Step 4: Update `extensions/vscode/package.json`**

Add the following keys to the `"contributes"` object (merge into the existing contributes, do not replace it):

```json
"viewsContainers": {
  "activitybar": [
    {
      "id": "terrax",
      "title": "TerraX",
      "icon": "icons/terrax.svg"
    }
  ]
},
"views": {
  "terrax": [
    {
      "id": "terrax.stackTree",
      "name": "Stacks"
    }
  ]
},
```

Add to the existing `"commands"` array:

```json
{
  "command": "terrax.refresh",
  "title": "TerraX: Refresh",
  "icon": "$(refresh)"
}
```

Add to the existing `"menus"` object:

```json
"view/title": [
  {
    "command": "terrax.refresh",
    "when": "view == terrax.stackTree",
    "group": "navigation"
  }
]
```

The final `"contributes"` section of `package.json` should be:

```json
"contributes": {
  "viewsContainers": {
    "activitybar": [
      {
        "id": "terrax",
        "title": "TerraX",
        "icon": "icons/terrax.svg"
      }
    ]
  },
  "views": {
    "terrax": [
      {
        "id": "terrax.stackTree",
        "name": "Stacks"
      }
    ]
  },
  "commands": [
    {
      "command": "terrax.openHere",
      "title": "TerraX: Open here"
    },
    {
      "command": "terrax.refresh",
      "title": "TerraX: Refresh",
      "icon": "$(refresh)"
    }
  ],
  "menus": {
    "explorer/context": [
      {
        "command": "terrax.openHere",
        "group": "navigation"
      }
    ],
    "view/title": [
      {
        "command": "terrax.refresh",
        "when": "view == terrax.stackTree",
        "group": "navigation"
      }
    ]
  },
  "configuration": {
    "title": "TerraX",
    "properties": {
      "terrax.binaryPath": {
        "type": "string",
        "default": "terrax",
        "description": "Path to the terrax binary. Defaults to 'terrax' (assumes it's on PATH)."
      }
    }
  }
}
```

- [ ] **Step 5: Build the extension**

```bash
task ext:build
```

Expected: TypeScript compiles with no errors. `extensions/vscode/out/treeProvider.js` is created alongside `extension.js` and `terminalRunner.js`.

- [ ] **Step 6: Package and verify**

```bash
task ext:package
```

Expected: `terrax-vscode-0.1.0.vsix` created. The `.vsix` should include `icons/terrax.svg`.

Verify icon is included:

```bash
unzip -l extensions/vscode/terrax-vscode-0.1.0.vsix | grep icons
```

Expected: `extension/icons/terrax.svg` appears in the listing.

- [ ] **Step 7: Commit**

```bash
git add extensions/vscode/icons/ extensions/vscode/src/treeProvider.ts \
        extensions/vscode/src/extension.ts extensions/vscode/package.json
git commit -m "feat(vscode): add sidebar panel with Terragrunt stack tree"
```
