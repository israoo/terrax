import * as vscode from 'vscode';
import { DependencyTreeProvider, DependentsTreeProvider } from './dependencyProvider';
import { runInTerminal } from './terminalRunner';
import { TerraXTreeProvider, StackNode } from './treeProvider';

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

  const runPlanCommand = vscode.commands.registerCommand(
    'terrax.runPlan',
    (node: StackNode) => {
      const config = vscode.workspace.getConfiguration('terrax');
      const binaryPath = config.get<string>('binaryPath', 'terrax');
      runInTerminal(binaryPath, node.path, 'plan');
    },
  );

  const workspaceRoot = vscode.workspace.workspaceFolders?.[0]?.uri.fsPath ?? '';
  const treeProvider = new TerraXTreeProvider(workspaceRoot);
  const depProvider = new DependencyTreeProvider();
  const dependentsProvider = new DependentsTreeProvider();

  const treeView = vscode.window.createTreeView('terrax.stackTree', {
    treeDataProvider: treeProvider,
    showCollapseAll: true,
  });

  const depTreeView = vscode.window.createTreeView('terrax.dependencyTree', {
    treeDataProvider: depProvider,
  });

  const dependentsTreeView = vscode.window.createTreeView('terrax.dependentsTree', {
    treeDataProvider: dependentsProvider,
  });

  treeView.onDidChangeSelection((e) => {
    const node = e.selection[0] ?? null;
    depProvider.setFocus(node);
    dependentsProvider.setFocus(node);
  });

  const doRefresh = (): void => {
    treeProvider.refresh();
    const tree = treeProvider.getTree();
    depProvider.setTree(tree);
    dependentsProvider.setTree(tree);
  };

  const refreshCommand = vscode.commands.registerCommand('terrax.refresh', doRefresh);

  const expandAllCommand = vscode.commands.registerCommand('terrax.expandAll', async () => {
    for (const node of treeProvider.getAllNodes()) {
      await treeView.reveal(node, { expand: true });
    }
  });

  const folderChangeListener = vscode.workspace.onDidChangeWorkspaceFolders(() => {
    const newRoot = vscode.workspace.workspaceFolders?.[0]?.uri.fsPath ?? '';
    treeProvider.updateWorkspaceRoot(newRoot);
    doRefresh();
  });

  context.subscriptions.push(
    openHereCommand,
    runPlanCommand,
    treeView,
    depTreeView,
    dependentsTreeView,
    refreshCommand,
    expandAllCommand,
    folderChangeListener,
  );

  doRefresh();
}

export function deactivate(): void {}
