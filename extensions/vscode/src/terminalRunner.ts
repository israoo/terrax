import * as fs from 'fs';
import * as path from 'path';
import * as vscode from 'vscode';

const TERMINAL_NAME = 'TerraX';

export function runInTerminal(binaryPath: string, itemPath: string): void {
  let stat: fs.Stats;
  try {
    stat = fs.statSync(itemPath);
  } catch {
    vscode.window.showErrorMessage(`TerraX: Cannot access path: ${itemPath}`);
    return;
  }

  const targetDir = stat.isDirectory() ? itemPath : path.dirname(itemPath);

  let command: string;
  if (process.platform === 'win32') {
    const escapedBinary = binaryPath.replace(/"/g, '\\"');
    const escapedDir = targetDir.replace(/"/g, '\\"');
    command = `"${escapedBinary}" --dir "${escapedDir}"`;
  } else {
    const escapedBinary = binaryPath.replace(/'/g, "'\\''");
    const escapedDir = targetDir.replace(/'/g, "'\\''");
    command = `'${escapedBinary}' --dir '${escapedDir}'`;
  }

  const existing = vscode.window.terminals.find((t) => t.name === TERMINAL_NAME);

  if (!existing) {
    const terminal = vscode.window.createTerminal(TERMINAL_NAME);
    terminal.show();
    terminal.sendText(command);
    return;
  }

  existing.show();

  if (existing.exitStatus !== undefined) {
    existing.sendText(command);
  } else {
    // terrax TUI is running — send Ctrl+C to exit it, then launch with new path.
    existing.sendText('\x03');
    setTimeout(() => existing.sendText(command), 300);
  }
}
