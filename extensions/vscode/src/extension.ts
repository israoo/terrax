import * as vscode from 'vscode';
import { runInTerminal } from './terminalRunner';

export function activate(context: vscode.ExtensionContext): void {
  const command = vscode.commands.registerCommand(
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
    }
  );

  context.subscriptions.push(command);
}

export function deactivate(): void {}
