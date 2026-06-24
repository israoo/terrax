# Design: VS Code History Panel

**Date:** 2026-06-20
**Status:** Approved

## Context

TerraX stores every command execution in a JSONL history log (XDG data directory). The interactive TUI history viewer (`terrax --history`) works well in a terminal session but is inaccessible from VS Code without switching context. The VS Code extension already has three panels (Stacks, Dependencies, Dependents) but no way to see past executions or re-run them.

## Goals

1. Add `terrax history [--dir <path>]` subcommand that outputs execution history filtered to the current project as a JSON array.
2. Add a "History" panel in the VS Code extension showing the most recent executions for the workspace.
3. Click an entry → open TUI with that stack preselected (`terrax --dir <absolute_path>`).
4. Hover button → re-run the same command directly (`terrax run <command> --dir <absolute_path>`).

## Non-Goals

- Pagination or infinite scroll (show all entries for the project; the history is capped by `history.max_entries` config).
- Filtering or search within the panel.
- Clearing history from the extension.

---

## Part 1 — Go: `cmd/history.go`

### Subcommand

```bash
terrax history [--dir <path>]
```

New file `cmd/history.go` following the exact same pattern as `cmd/tree.go` and `cmd/run.go`.

- Registers `historyCmd` with `rootCmd.AddCommand(historyCmd)` in `init()`.
- Accepts `--dir <path>` flag (same semantics as all other subcommands).
- Always outputs JSON to stdout — this subcommand has no interactive mode.

### Implementation

```go
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
        // Log warning but continue with unfiltered entries.
        fmt.Fprintf(os.Stderr, "Warning: failed to filter history: %v\n", err)
        filtered = entries
    }

    // Reverse to most-recent-first order.
    for i, j := 0, len(filtered)-1; i < j; i, j = i+1, j-1 {
        filtered[i], filtered[j] = filtered[j], filtered[i]
    }

    data, err := json.Marshal(filtered)
    if err != nil {
        return fmt.Errorf("failed to serialize history: %w", err)
    }

    fmt.Fprintln(os.Stdout, string(data))
    return nil
}
```

Note: `FilterByCurrentProject` uses the CWD of the process to detect the project root. Since `getWorkingDirectory(dirFlag)` is called first and the history service is initialized from the same root detection, the filtering is consistent.

### JSON output

Array of `history.ExecutionLogEntry` objects, most recent first. Returns `[]` (empty array) when no history exists for the project — never null or error.

```json
[
  {
    "id": 42,
    "timestamp": "2026-06-20T10:30:00Z",
    "user": "isra",
    "stack_path": "workloads/dev/vpc",
    "absolute_path": "/abs/path/workloads/dev/vpc",
    "command": "plan",
    "exit_code": 0,
    "duration_s": 12.3,
    "summary": "3 added, 0 changed"
  }
]
```

### Testing

`cmd/history_test.go` — one test: build a temp dir with a known history JSONL, call the subcommand with `--dir <tmpdir>`, verify the JSON output contains the expected entries in reverse chronological order.

---

## Part 2 — VS Code: History panel

### `package.json` additions

Add fourth view to the `terrax` container:

```json
{ "id": "terrax.historyTree", "name": "History" }
```

Add two commands:

```json
{ "command": "terrax.refreshHistory", "title": "TerraX: Refresh History", "icon": "$(refresh)" },
{ "command": "terrax.reRunHistory",   "title": "Re-run",                   "icon": "$(run)" }
```

Add menus:

```json
"view/title": [
  { "command": "terrax.refreshHistory", "when": "view == terrax.historyTree", "group": "navigation" }
],
"view/item/context": [
  { "command": "terrax.reRunHistory", "when": "view == terrax.historyTree && viewItem == terraxHistoryEntry", "group": "inline" }
]
```

### `HistoryEntry` interface

```typescript
export interface HistoryEntry {
  id: number;
  timestamp: string;        // ISO 8601
  user: string;
  stack_path: string;       // relative display path
  absolute_path: string;    // used for command execution
  command: string;
  exit_code: number;
  duration_s: number;
  summary: string;
}
```

### `HistoryTreeProvider` (`historyProvider.ts`)

New file. `HistoryTreeProvider implements vscode.TreeDataProvider<HistoryEntry>`.

**`refresh()`:**
- Reads `terrax.binaryPath` from VS Code config.
- Calls `spawnSync(binaryPath, ['history', '--dir', workspaceRoot], { timeout: 10000, encoding: 'utf-8' })`.
- On error (binary not found, non-zero exit): sets error state, shows `$(warning)` item with message.
- On success: parses JSON array of `HistoryEntry[]`, stores in `this.entries`.
- Fires `_onDidChangeTreeData`.

**`getTreeItem(entry)`:**
- Label: `${entry.command} · ${entry.stack_path.split('/').pop()}` (e.g. `plan · vpc`)
- Description: relative time + duration + result indicator (e.g. `2h ago · 12s · ✓`)
- Icon: `$(check)` when `exit_code === 0`, `$(error)` otherwise
- `contextValue: 'terraxHistoryEntry'` (triggers inline re-run button)
- `command`: `{ command: 'terrax.openHere', arguments: [vscode.Uri.file(entry.absolute_path)] }` (click opens TUI)

**`getChildren()`:** returns flat `this.entries` array (no nesting).

### `extension.ts` additions

```typescript
import { HistoryTreeProvider } from './historyProvider';

const historyProvider = new HistoryTreeProvider(workspaceRoot);
const historyTreeView = vscode.window.createTreeView('terrax.historyTree', {
  treeDataProvider: historyProvider,
});

vscode.commands.registerCommand('terrax.refreshHistory', () => historyProvider.refresh());

vscode.commands.registerCommand('terrax.reRunHistory', (entry: HistoryEntry) => {
  const config = vscode.workspace.getConfiguration('terrax');
  const binaryPath = config.get<string>('binaryPath', 'terrax');
  runInTerminal(binaryPath, entry.absolute_path, entry.command);
});
```

`doRefresh` also calls `historyProvider.refresh()`.
`historyTreeView` pushed to `context.subscriptions`.

---

## Build verification

- `task check` — all Go tests pass.
- `task ext:build` — TypeScript compiles.
- `task ext:package` — `.vsix` builds.
