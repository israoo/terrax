import * as vscode from 'vscode';
import { runInTerminal } from './terminalRunner';

export function activate(context: vscode.ExtensionContext): void {
  const command = vscode.commands.registerCommand(
    'terrax.openHere',
    (uri?: vscode.Uri) => {
      const workspaceFolders = vscode.workspace.workspaceFolders;
      if (!workspaceFolders || workspaceFolders.length === 0) {
        vscode.window.showErrorMessage('TerraX: No workspace folder open.');
        return;
      }

      const targetPath = uri?.fsPath ?? workspaceFolders[0].uri.fsPath;
      const config = vscode.workspace.getConfiguration('terrax');
      const binaryPath = config.get<string>('binaryPath', 'terrax');

      runInTerminal(binaryPath, targetPath);
    }
  );

  context.subscriptions.push(command);
}

export function deactivate(): void {}
