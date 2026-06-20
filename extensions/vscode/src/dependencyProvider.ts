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
