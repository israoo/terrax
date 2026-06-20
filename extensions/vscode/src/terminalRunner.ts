import * as fs from 'fs';
import * as path from 'path';
import * as vscode from 'vscode';

const TERMINAL_NAME = 'TerraX';

export function runInTerminal(binaryPath: string, itemPath: string, subcommand?: string): void {
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
    command = subcommand
      ? `"${escapedBinary}" run "${subcommand}" --dir "${escapedDir}"`
      : `"${escapedBinary}" --dir "${escapedDir}"`;
  } else {
    const escapedBinary = binaryPath.replace(/'/g, "'\\''");
    const escapedDir = targetDir.replace(/'/g, "'\\''");
    command = subcommand
      ? `'${escapedBinary}' run '${subcommand}' --dir '${escapedDir}'`
      : `'${escapedBinary}' --dir '${escapedDir}'`;
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
    // Send 'q' (TerraX quit key, no newline) to close the TUI if running.
    // Then Ctrl+U clears any stale chars from the readline buffer (e.g. the 'q'
    // itself if the shell was already at a prompt), before sending the new command.
    existing.sendText('q', false);
    setTimeout(() => {
      existing.sendText('\x15', false); // Ctrl+U — clear readline buffer
      existing.sendText(command);
    }, 300);
  }
}
