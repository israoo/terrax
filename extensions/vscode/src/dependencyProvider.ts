import * as vscode from 'vscode';
import { StackNode } from './treeProvider';

interface DepNode {
  stackNode: StackNode;
  depth: number;
}

const MAX_DEPTH = 10;

// buildNodeMap recursively indexes all nodes by path for O(1) lookup.
function buildNodeMap(node: StackNode, map: Map<string, StackNode>): void {
  map.set(node.path, node);
  for (const child of node.children) {
    buildNodeMap(child, map);
  }
}

// makeDepNodes converts a list of paths to DepNodes, using placeholders for unknown paths.
function makeDepNodes(
  nodeMap: Map<string, StackNode>,
  paths: string[],
  depth: number,
  idPrefix: string,
): DepNode[] {
  return paths.map((p) => {
    const node = nodeMap.get(p);
    if (node) {
      return { stackNode: node, depth };
    }
    return {
      stackNode: {
        name: p.split('/').pop() ?? p,
        path: p,
        isStack: true,
        depth: 0,
        children: [],
        dependencies: [],
        dependents: [],
      },
      depth,
    };
  });
}

export class DependencyTreeProvider implements vscode.TreeDataProvider<DepNode> {
  private _onDidChangeTreeData = new vscode.EventEmitter<void>();
  readonly onDidChangeTreeData = this._onDidChangeTreeData.event;

  private nodeMap = new Map<string, StackNode>();
  private focused: StackNode | null = null;

  setTree(root: StackNode | null): void {
    this.nodeMap.clear();
    if (root) {
      buildNodeMap(root, this.nodeMap);
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
      if (!this.focused) return [];
      return makeDepNodes(this.nodeMap, this.focused.dependencies ?? [], 0, 'dep');
    }
    if (dep.depth >= MAX_DEPTH) return [];
    return makeDepNodes(this.nodeMap, dep.stackNode.dependencies ?? [], dep.depth + 1, 'dep');
  }
}

export class DependentsTreeProvider implements vscode.TreeDataProvider<DepNode> {
  private _onDidChangeTreeData = new vscode.EventEmitter<void>();
  readonly onDidChangeTreeData = this._onDidChangeTreeData.event;

  private nodeMap = new Map<string, StackNode>();
  private focused: StackNode | null = null;

  setTree(root: StackNode | null): void {
    this.nodeMap.clear();
    if (root) {
      buildNodeMap(root, this.nodeMap);
    }
    this._onDidChangeTreeData.fire();
  }

  setFocus(node: StackNode | null): void {
    this.focused = node;
    this._onDidChangeTreeData.fire();
  }

  getTreeItem(dep: DepNode): vscode.TreeItem {
    const deps = dep.stackNode.dependents ?? [];
    const hasExpandableChildren = deps.length > 0 && dep.depth < MAX_DEPTH;
    const item = new vscode.TreeItem(
      dep.stackNode.name,
      hasExpandableChildren
        ? vscode.TreeItemCollapsibleState.Collapsed
        : vscode.TreeItemCollapsibleState.None,
    );
    item.id = `dependent-${dep.stackNode.path}-${dep.depth}`;
    item.iconPath = new vscode.ThemeIcon('package');
    item.description = dep.stackNode.path;
    return item;
  }

  getChildren(dep?: DepNode): DepNode[] {
    if (!dep) {
      if (!this.focused) return [];
      return makeDepNodes(this.nodeMap, this.focused.dependents ?? [], 0, 'dependent');
    }
    if (dep.depth >= MAX_DEPTH) return [];
    return makeDepNodes(this.nodeMap, dep.stackNode.dependents ?? [], dep.depth + 1, 'dependent');
  }
}
