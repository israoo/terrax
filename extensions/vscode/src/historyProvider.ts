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
      ['history', '--json', '--dir', this.workspaceRoot],
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
    item.description = `${formatRelativeTime(entry.timestamp)} · ${formatDuration(entry.duration_s)}`;
    item.tooltip = entry.absolute_path;
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
