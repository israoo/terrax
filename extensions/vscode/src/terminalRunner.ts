import * as fs from 'fs';
import * as path from 'path';
import * as vscode from 'vscode';

const TERMINAL_NAME = 'TerraX';

export function runInTerminal(binaryPath: string, itemPath: string): void {
  const stat = fs.statSync(itemPath);
  const targetDir = stat.isDirectory() ? itemPath : path.dirname(itemPath);
  const escaped = targetDir.replace(/'/g, "'\\''");
  const command = `${binaryPath} --dir '${escaped}'`;

  const existing = vscode.window.terminals.find((t) => t.name === TERMINAL_NAME);
  const terminal = existing ?? vscode.window.createTerminal(TERMINAL_NAME);

  terminal.show();
  terminal.sendText(command);
}
