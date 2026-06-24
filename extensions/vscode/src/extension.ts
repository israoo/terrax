import * as vscode from 'vscode';
import * as path from 'node:path';
import { DependencyTreeProvider, DependentsTreeProvider } from './dependencyProvider';
import { HistoryTreeProvider, HistoryEntry } from './historyProvider';
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

  const historyProvider = new HistoryTreeProvider(workspaceRoot);
  const historyTreeView = vscode.window.createTreeView('terrax.historyTree', {
    treeDataProvider: historyProvider,
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
    historyProvider.refresh();
  };

  const refreshCommand = vscode.commands.registerCommand('terrax.refresh', doRefresh);

  const expandAllCommand = vscode.commands.registerCommand('terrax.expandAll', async () => {
    for (const node of treeProvider.getAllNodes()) {
      await treeView.reveal(node, { expand: true });
    }
  });

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

  const folderChangeListener = vscode.workspace.onDidChangeWorkspaceFolders(() => {
    const newRoot = vscode.workspace.workspaceFolders?.[0]?.uri.fsPath ?? '';
    treeProvider.updateWorkspaceRoot(newRoot);
    historyProvider.updateWorkspaceRoot(newRoot);
    doRefresh();
  });

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

  doRefresh();
}

export function deactivate(): void {}
