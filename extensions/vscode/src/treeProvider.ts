import { spawnSync } from 'node:child_process';
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
  private parentMap = new Map<StackNode, StackNode>();

  constructor(private workspaceRoot: string) {}

  updateWorkspaceRoot(root: string): void {
    this.workspaceRoot = root;
  }

  refresh(): void {
    this.tree = null;
    this.hasError = false;
    this.parentMap.clear();

    if (!this.workspaceRoot) {
      this.hasError = true;
      this._onDidChangeTreeData.fire();
      return;
    }

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
      try {
        this.tree = JSON.parse(result.stdout) as StackNode;
        this.buildParentMap(this.tree);
      } catch {
        this.hasError = true;
        vscode.window.showErrorMessage('TerraX: Failed to parse tree output.');
      }
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

  getParent(node: StackNode): StackNode | undefined {
    return this.parentMap.get(node);
  }

  getRootChildren(): StackNode[] {
    return this.tree?.children ?? [];
  }

  private buildParentMap(node: StackNode): void {
    for (const child of node.children) {
      this.parentMap.set(child, node);
      this.buildParentMap(child);
    }
  }
}
