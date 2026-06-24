# ADR-0022: VS Code Context Menu Commands

**Status**: Accepted

**Date**: 2026-06-21

**Deciders**: TerraX Core Team

**Related**:
- [ADR-0013: VS Code Extension Integration](0013-vscode-extension-integration.md)
- [ADR-0019: History Panel and Leaf Stack Auto-Navigation](0019-history-panel-and-leaf-stack-navigation.md)

## Context

The VS Code extension's Stacks panel previously exposed only two interaction modes: clicking a directory node opened the interactive TUI, and an inline `$(play)` button ran `plan` directly. Engineers frequently need to run other Terragrunt commands (apply, validate, init, etc.) from the sidebar without opening the TUI. The only workaround was to use the terminal directly, losing the context of having already navigated to a specific stack in the panel.

Additionally, engineers navigating the stack tree often want to inspect the `terragrunt.hcl` file of a specific stack without leaving the editor.

## Decision

Add a right-click context menu to all Stacks tree nodes with the following items, in order:

**Group `terrax@1` — Terragrunt commands** (on both directory and stack nodes):
1. Run Plan
2. Run Apply
3. Run Validate
4. Run Fmt
5. Run Init
6. Run Output
7. Run Refresh
8. Run Destroy (icon: `$(warning)` — signals destructive operation)

**Group `terrax@2` — File actions** (on stack nodes only):
9. Open terragrunt.hcl — opens the file in the VS Code editor

VS Code inserts a visual separator between groups. The order follows the most common Terragrunt workflow: read-only commands first (plan, validate, fmt, init, output, refresh), write commands last (apply, destroy).

All run commands invoke `runInTerminal(binaryPath, node.path, commandName)` — the same terminal reuse pattern as the existing inline button. Open file uses `vscode.workspace.openTextDocument` + `showTextDocument`.

The command set is **static** — it mirrors `config.DefaultCommands` from the Go CLI. Custom commands configured in `.terrax.yaml` are not reflected in the menu without a VS Code reload.

**"Open terragrunt.hcl" is restricted to stack nodes** (`contextValue: 'terraxStack'`) because only stack directories are guaranteed to have `terragrunt.hcl`. Directory navigation nodes do not.

## Consequences

### Positive

- Engineers can run any Terragrunt command from the sidebar without opening the TUI — single click from panel to execution.
- `$(warning)` icon on Destroy creates a clear visual signal before destructive operations without blocking the engineer (no confirmation dialog).
- "Open terragrunt.hcl" makes it trivial to inspect a stack's configuration directly from the navigation panel.
- Consistent with VS Code conventions — context menus are the standard way to expose secondary actions on tree nodes.

### Negative

- The menu is static — custom commands added to `.terrax.yaml` do not appear without implementing a dynamic menu system. Engineers with non-default command configurations see items that may fail at runtime.
- "Run Plan" appears both as an inline `$(play)` button and in the context menu, creating minor redundancy. The inline button covers the primary use case; the context menu provides completeness.
- Eight commands in the context menu is verbose. Teams that only use a subset of commands cannot hide the others.

## Alternatives Considered

### Option 1: Dynamic commands from `.terrax.yaml`

**Description**: Read the configured command list from `terrax config --json` when the tree loads and register context menu items dynamically — either via a QuickPick triggered by a single "Run command..." entry, or by using `vscode.commands.setContext` to toggle visibility of pre-registered commands.

**Pros**:

- Menu exactly matches the project's configured commands.
- A project using only `plan` and `apply` would see only those two items.

**Cons**:

- VS Code's `package.json` menu contributions are static. True dynamic menus require either a QuickPick (adds an extra click) or `setContext` with one pre-registered command per possible command name (complex setup).
- `terrax config --json` is not yet implemented as a subcommand.
- The `when` clause approach requires knowing all possible command names upfront, defeating the purpose of dynamic configuration.

**Why rejected**: The added complexity — implementing a new `terrax config --json` subcommand, reading it at activation, and managing context keys — was disproportionate to the benefit for the current user base. The default command set covers the vast majority of use cases. Dynamic menus can be added in a future iteration once `terrax config --json` exists.

### Option 2: QuickPick triggered from a single context menu entry

**Description**: A single "Run command..." item in the context menu opens a VS Code QuickPick populated with the configured commands. One click to open the menu, one more to select the command.

**Pros**:

- Dynamic — reads the actual configured commands at invocation time.
- Single context menu item keeps the menu clean.

**Cons**:

- Two clicks instead of one defeats the purpose of the context menu as a shortcut.
- QuickPick is a modal interaction that interrupts the VS Code workflow more than a native submenu.
- Less discoverable — engineers do not know what commands are available until they click "Run command...".

**Why rejected**: The primary value of a context menu is direct access to actions in a single click. Adding a QuickPick introduces the same friction as opening the TUI selector. The inline `$(play)` button already covers the one-click fast path for the most common command (plan).

## References

- [`extensions/vscode/src/extension.ts`](../../extensions/vscode/src/extension.ts)
- [`extensions/vscode/package.json`](../../extensions/vscode/package.json)
- [ADR-0013: VS Code Extension Integration](0013-vscode-extension-integration.md)
