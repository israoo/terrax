# History Panel Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add `terrax history --dir <path>` subcommand and a VS Code "History" panel showing past executions with re-run and click-to-open-TUI actions.

**Architecture:** Two tasks. Task 1 adds a `cmd/history.go` Cobra subcommand that loads, filters, and reverses history entries as JSON — following the exact pattern of `cmd/tree.go`. Task 2 creates `historyProvider.ts`, wires it into the extension as a fourth panel, and registers two new commands (`terrax.refreshHistory`, `terrax.reRunHistory`).

**Tech Stack:** Go 1.25.5 · Cobra · encoding/json · TypeScript 5.9.3 · VS Code Extension API 1.125.0 · node:child_process.spawnSync

## Global Constraints

- All Go comments must end with a period.
- Go imports: three groups (stdlib / third-party / `github.com/israoo/terrax/...`), alphabetically sorted.
- Errors always wrapped: `fmt.Errorf("context: %w", err)`.
- Run `task check` before each Go commit.
- Run `task ext:build` and `task ext:package` before each TypeScript commit.
- `terrax history` always outputs JSON to stdout — no interactive mode.
- Output is `[]` (empty array) not `null` when no history exists.
- `historyTreeView` must be pushed to `context.subscriptions`.

---

### Task 1: Go — `terrax history` subcommand

**Files:**
- Create: `cmd/history.go`
- Create: `cmd/history_test.go`

**Interfaces:**
- Consumes: `getWorkingDirectory(dir string) (string, error)` — existing in `cmd/root.go`
- Consumes: `getHistoryService() (*history.Service, error)` — existing in `cmd/root.go`
- Consumes: `historyService.LoadAll(ctx context.Context) ([]history.ExecutionLogEntry, error)`
- Consumes: `historyService.FilterByCurrentProject(entries []history.ExecutionLogEntry) ([]history.ExecutionLogEntry, error)`
- Produces: `terrax history [--dir <path>]` — prints JSON array to stdout, exits non-zero on error

- [ ] **Step 1: Write the failing test in `cmd/history_test.go`**

```go
package cmd

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHistoryCommand_OutputsValidJSON(t *testing.T) {
	tmpDir := t.TempDir()
	restore := captureStdout(t)
	rootCmd.SetArgs([]string{"history", "--dir", tmpDir})
	err := rootCmd.Execute()
	output := restore()

	require.NoError(t, err)

	// Output must be a valid JSON array (may be empty if no history file exists).
	var entries []map[string]interface{}
	require.NoError(t,
		json.Unmarshal([]byte(strings.TrimSpace(output)), &entries),
		"output must be a valid JSON array, got: %s", output,
	)
}
```

- [ ] **Step 2: Run test to verify it fails**

```bash
go test ./cmd/... -run TestHistoryCommand -v
```

Expected: compile error — `historyCmd` is not registered yet.

- [ ] **Step 3: Create `cmd/history.go`**

```go
package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/israoo/terrax/internal/history"
)

var historyCmd = &cobra.Command{
	Use:   "history",
	Short: "Print command execution history as JSON",
	Long:  `Print command execution history for the current project as JSON for consumption by external tools such as editor extensions.`,
	RunE:  runHistoryCmd,
}

func init() {
	historyCmd.Flags().String("dir", "", "Working directory (overrides current directory)")
	rootCmd.AddCommand(historyCmd)
}

func runHistoryCmd(cmd *cobra.Command, args []string) error {
	ctx := context.Background()

	dirFlag, _ := cmd.Flags().GetString("dir")
	workDir, err := getWorkingDirectory(dirFlag)
	if err != nil {
		return fmt.Errorf("failed to get working directory: %w", err)
	}

	historyService, err := getHistoryService()
	if err != nil {
		return fmt.Errorf("failed to initialize history service: %w", err)
	}

	entries, err := historyService.LoadAll(ctx)
	if err != nil {
		return fmt.Errorf("failed to load history: %w", err)
	}

	// FilterByCurrentProject detects the project root from os.Getwd().
	// Change to workDir first so detection uses the --dir argument.
	if err := os.Chdir(workDir); err != nil {
		return fmt.Errorf("failed to change directory: %w", err)
	}

	filtered, err := historyService.FilterByCurrentProject(entries)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Warning: failed to filter history: %v\n", err)
		filtered = entries
	}

	// Reverse to most-recent-first order.
	for i, j := 0, len(filtered)-1; i < j; i, j = i+1, j-1 {
		filtered[i], filtered[j] = filtered[j], filtered[i]
	}

	// Ensure empty slice marshals as [] not null.
	if filtered == nil {
		filtered = []history.ExecutionLogEntry{}
	}

	data, err := json.Marshal(filtered)
	if err != nil {
		return fmt.Errorf("failed to serialize history: %w", err)
	}

	if _, err := fmt.Fprintln(os.Stdout, string(data)); err != nil {
		return fmt.Errorf("failed to write output: %w", err)
	}
	return nil
}
```

- [ ] **Step 4: Run test to verify it passes**

```bash
go test ./cmd/... -run TestHistoryCommand -v
```

Expected: PASS.

- [ ] **Step 5: Run full check**

```bash
task check
```

Expected: all checks pass, 0 lint issues.

- [ ] **Step 6: Commit**

```bash
git add cmd/history.go cmd/history_test.go
git commit -m "feat: add terrax history subcommand for JSON history output"
```

---

### Task 2: VS Code — History panel

**Files:**
- Create: `extensions/vscode/src/historyProvider.ts`
- Modify: `extensions/vscode/src/extension.ts`
- Modify: `extensions/vscode/package.json`

**Interfaces:**
- Consumes: `terrax history --dir <workspace>` CLI (produced by Task 1)
- Consumes: `runInTerminal(binaryPath, itemPath, subcommand)` from `./terminalRunner`
- Produces: `HistoryEntry` interface (exported)
- Produces: `HistoryTreeProvider` class with `refresh(): void` and `updateWorkspaceRoot(root: string): void`

- [ ] **Step 1: Create `extensions/vscode/src/historyProvider.ts`**

```typescript
import { spawnSync } from 'node:child_process';
import * as vscode from 'vscode';

export interface HistoryEntry {
  id: number;
  timestamp: string;
  user: string;
  stack_path: string;
  absolute_path: string;
  command: string;
  exit_code: number;
  duration_s: number;
  summary: string;
  isError?: boolean; // internal sentinel — never present in JSON from Go
}

function formatRelativeTime(isoTimestamp: string): string {
  const diffMs = Date.now() - new Date(isoTimestamp).getTime();
  const diffMin = Math.floor(diffMs / 60000);
  if (diffMin < 60) return `${diffMin}m ago`;
  const diffH = Math.floor(diffMin / 60);
  if (diffH < 24) return `${diffH}h ago`;
  return `${Math.floor(diffH / 24)}d ago`;
}

function formatDuration(seconds: number): string {
  if (seconds < 60) return `${Math.round(seconds)}s`;
  return `${Math.floor(seconds / 60)}m ${Math.round(seconds % 60)}s`;
}

const ERROR_ENTRY: HistoryEntry = {
  id: -1, timestamp: '', user: '', stack_path: '', absolute_path: '',
  command: '', exit_code: -1, duration_s: 0, summary: '', isError: true,
};

export class HistoryTreeProvider implements vscode.TreeDataProvider<HistoryEntry> {
  private _onDidChangeTreeData = new vscode.EventEmitter<void>();
  readonly onDidChangeTreeData = this._onDidChangeTreeData.event;

  private entries: HistoryEntry[] = [];
  private hasError = false;

  constructor(private workspaceRoot: string) {}

  updateWorkspaceRoot(root: string): void {
    this.workspaceRoot = root;
  }

  refresh(): void {
    this.entries = [];
    this.hasError = false;

    if (!this.workspaceRoot) {
      this._onDidChangeTreeData.fire();
      return;
    }

    const config = vscode.workspace.getConfiguration('terrax');
    const binaryPath = config.get<string>('binaryPath', 'terrax');

    const result = spawnSync(
      binaryPath,
      ['history', '--dir', this.workspaceRoot],
      { timeout: 10000, encoding: 'utf-8' },
    );

    if (result.error || result.status !== 0) {
      this.hasError = true;
    } else {
      try {
        this.entries = JSON.parse(result.stdout) as HistoryEntry[];
      } catch {
        this.hasError = true;
      }
    }

    this._onDidChangeTreeData.fire();
  }

  getTreeItem(entry: HistoryEntry): vscode.TreeItem {
    if (entry.isError) {
      const item = new vscode.TreeItem('Failed to load history', vscode.TreeItemCollapsibleState.None);
      item.iconPath = new vscode.ThemeIcon('warning');
      return item;
    }

    const stackName = entry.stack_path.split('/').pop() ?? entry.stack_path;
    const item = new vscode.TreeItem(
      `${entry.command} · ${stackName}`,
      vscode.TreeItemCollapsibleState.None,
    );
    item.id = `history-${entry.id}`;
    item.description = `${formatRelativeTime(entry.timestamp)} · ${formatDuration(entry.duration_s)} · ${entry.exit_code === 0 ? '✓' : '✗'}`;
    item.iconPath = entry.exit_code === 0 ? new vscode.ThemeIcon('check') : new vscode.ThemeIcon('error');
    item.contextValue = 'terraxHistoryEntry';
    item.command = {
      command: 'terrax.openHere',
      title: 'Open in TerraX',
      arguments: [vscode.Uri.file(entry.absolute_path)],
    };
    return item;
  }

  getChildren(): HistoryEntry[] {
    if (this.hasError) {
      return [ERROR_ENTRY];
    }
    return this.entries;
  }
}
```

- [ ] **Step 2: Update `extensions/vscode/src/extension.ts`**

Add import at the top:

```typescript
import { HistoryTreeProvider, HistoryEntry } from './historyProvider';
```

After `const dependentsProvider = new DependentsTreeProvider();`, add:

```typescript
const historyProvider = new HistoryTreeProvider(workspaceRoot);
const historyTreeView = vscode.window.createTreeView('terrax.historyTree', {
  treeDataProvider: historyProvider,
});
```

Register `terrax.refreshHistory` and `terrax.reRunHistory` commands — add after `expandAllCommand`:

```typescript
const refreshHistoryCommand = vscode.commands.registerCommand(
  'terrax.refreshHistory',
  () => historyProvider.refresh(),
);

const reRunHistoryCommand = vscode.commands.registerCommand(
  'terrax.reRunHistory',
  (entry: HistoryEntry) => {
    const config = vscode.workspace.getConfiguration('terrax');
    const binaryPath = config.get<string>('binaryPath', 'terrax');
    runInTerminal(binaryPath, entry.absolute_path, entry.command);
  },
);
```

Update `doRefresh` to also call `historyProvider.refresh()`:

```typescript
const doRefresh = (): void => {
  treeProvider.refresh();
  const tree = treeProvider.getTree();
  depProvider.setTree(tree);
  dependentsProvider.setTree(tree);
  historyProvider.refresh();
};
```

Update `folderChangeListener` to also update `historyProvider`:

```typescript
const folderChangeListener = vscode.workspace.onDidChangeWorkspaceFolders(() => {
  const newRoot = vscode.workspace.workspaceFolders?.[0]?.uri.fsPath ?? '';
  treeProvider.updateWorkspaceRoot(newRoot);
  historyProvider.updateWorkspaceRoot(newRoot);
  doRefresh();
});
```

Update `context.subscriptions.push(...)` to include the new disposables:

```typescript
context.subscriptions.push(
  openHereCommand,
  runPlanCommand,
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

- [ ] **Step 3: Update `extensions/vscode/package.json`**

Add the fourth view after `terrax.dependentsTree` in the `"views".terrax` array:

```json
{ "id": "terrax.historyTree", "name": "History" }
```

Add two commands to the `"commands"` array:

```json
{
  "command": "terrax.refreshHistory",
  "title": "TerraX: Refresh History",
  "icon": "$(refresh)"
},
{
  "command": "terrax.reRunHistory",
  "title": "Re-run",
  "icon": "$(run)"
}
```

Add to `"view/title"` in the `"menus"` object:

```json
{
  "command": "terrax.refreshHistory",
  "when": "view == terrax.historyTree",
  "group": "navigation"
}
```

Add a new `"view/item/context"` entry (the existing one already has `terraxDirectory` and `terraxStack`):

```json
{
  "command": "terrax.reRunHistory",
  "when": "view == terrax.historyTree && viewItem == terraxHistoryEntry",
  "group": "inline"
}
```

- [ ] **Step 4: Build the extension**

```bash
task ext:build
```

Expected: TypeScript compiles with no errors. `out/historyProvider.js` created.

- [ ] **Step 5: Package the extension**

```bash
task ext:package
```

Expected: `terrax-vscode-0.1.0.vsix` created.

- [ ] **Step 6: Rebuild Go binary**

```bash
task build
```

Expected: binary at `./build/terrax`, `terrax history --help` shows the subcommand.

- [ ] **Step 7: Commit**

```bash
git add extensions/vscode/src/historyProvider.ts \
        extensions/vscode/src/extension.ts \
        extensions/vscode/package.json
git commit -m "feat(vscode): add History panel with re-run and click-to-open-TUI"
```
