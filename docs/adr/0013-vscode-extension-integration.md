# ADR-0013: VS Code Extension Integration

**Status**: Accepted

**Date**: 2026-06-20

**Deciders**: TerraX Core Team

**Related**:
- [ADR-0005: Filesystem Tree Building Strategy](0005-filesystem-tree-building-strategy.md)
- [ADR-0009: Executor Isolation Pattern](0009-executor-isolation-pattern.md)

## Context

TerraX is a terminal TUI, but engineers often work inside VS Code where switching to a separate terminal to launch the TUI creates friction. A companion VS Code extension would integrate TerraX into the IDE workflow without duplicating its logic.

Four design questions needed answers:

1. Where should the extension live — separate repository or monorepo with TerraX?
2. How should the extension discover the Terragrunt stack tree without reimplementing the scanner?
3. How should direct command execution (bypassing the TUI) be exposed to the extension?
4. How should a single persistent terminal be managed when switching between stacks?

## Decision

### 1. Monorepo under `extensions/vscode/`

The extension lives at `extensions/vscode/` inside the TerraX repository, built with TypeScript and packaged as a `.vsix`. Taskfile tasks (`ext:install`, `ext:build`, `ext:package`) manage the extension lifecycle alongside the Go build.

```
terrax/
├── cmd/
├── internal/
├── extensions/
│   └── vscode/          ← companion extension
│       ├── src/
│       │   ├── extension.ts
│       │   ├── terminalRunner.ts
│       │   └── treeProvider.ts
│       └── package.json
└── main.go
```

### 2. `terrax tree --json` for stack tree discovery

The extension calls `terrax tree --dir <workspace>` via `spawnSync` to populate a VS Code `TreeDataProvider`. The Go binary returns the full `stack.Node` tree as JSON:

```json
{
  "name": "root", "path": "/abs/path", "isStack": false, "depth": 0,
  "children": [
    { "name": "networking", "path": "/abs/.../networking", "isStack": true, "depth": 1, "children": [] }
  ]
}
```

Node struct json tags were added to `internal/stack/tree.go`. The extension never scans the filesystem directly.

### 3. `terrax run <command> --dir <path>` for direct execution

A new `cmd/run.go` subcommand allows the extension to execute any configured Terragrunt command without opening the TUI:

```bash
terrax run plan --dir '/path/to/module'
```

It reuses `executor.Run()`, `getHistoryService()`, and `getWorkingDirectory()` verbatim — the same code path as post-TUI confirmation. History logging and flag configuration are identical to TUI execution.

Directory nodes in the sidebar show an inline `$(play)` button that triggers `terrax run plan`. Clicking a directory node opens the full TUI via `terrax --dir <path>`. Stack nodes (leaf directories with `terragrunt.hcl`) show the plan button but do not open the TUI on click, since TerraX requires subdirectories to navigate.

### 4. Single terminal reuse with `q` + Ctrl+U

All TerraX invocations use a single persistent terminal named `"TerraX"`. When the terminal exists and its process has not exited (shell is alive), the extension:

1. Sends `q` without newline — the TerraX quit key, exits the TUI cleanly if running.
2. Waits 300 ms.
3. Sends `Ctrl+U` (`\x15`) — clears the readline buffer in case the shell received `q` at a prompt.
4. Sends the new `terrax` command.

`Ctrl+C` and `Escape` were rejected: `Ctrl+C` triggered terrax's cleanup code, causing unintended command execution; bare `Escape` was corrupted by zsh's escape-sequence buffering after terrax exited.

## Consequences

### Positive

- Extension code is versioned and released with TerraX, preventing API drift.
- Stack tree is always consistent with what the TUI sees — single scanning implementation in Go.
- `terrax run` is available as a standalone CLI primitive, useful outside VS Code (CI, scripts).
- History logging captures extension-triggered executions identically to TUI-triggered ones.
- Single terminal avoids spawning multiple `TerraX` tabs on rapid node clicks.

### Negative

- Monorepo adds TypeScript/pnpm toolchain to a Go project — contributors must have both installed to build the extension.
- `spawnSync` with a 10-second timeout blocks the extension host thread during tree refresh on very large repositories.
- The 300 ms delay between `q` and the new command is a heuristic — if TerraX takes longer than 300 ms to exit (e.g. during a slow alt-screen restore), the new command may arrive before the shell regains control.
- The extension cannot detect whether the terminal is showing a TUI or a shell prompt; `exitStatus` is always `undefined` while the shell process lives.

## Alternatives Considered

### Option 1: Separate repository for the extension

**Description**: Publish the VS Code extension as `israoo/terrax-vscode`, independent of the main TerraX repo. It would import no Go code and call the `terrax` binary as a subprocess.

**Pros**:

- Clean separation of concerns between Go and TypeScript toolchains.
- Extension can be released on its own cadence.

**Cons**:

- Two repositories to keep in sync when CLI flags or JSON formats change.
- No shared CI — a breaking change in `terrax tree` output would only surface when the extension breaks in production.

**Why rejected**: The extension has no logic of its own — it is purely a launcher for the TerraX binary. Keeping it in the same repository ties its releases to the binary and makes format changes immediately visible in the same PR.

### Option 2: Extension scans the filesystem directly

**Description**: Implement a TypeScript filesystem scanner that searches for `terragrunt.hcl` files and builds the tree, without calling `terrax tree`.

**Pros**:

- Extension works even if the `terrax` binary is not installed.
- No new Go subcommand needed.

**Cons**:

- Duplicates the scanning logic in `internal/stack/builder.go`.
- Any future change to stack detection (new marker files, ignore patterns) must be mirrored in TypeScript.
- The extension's tree and the TUI's tree could diverge, confusing users.

**Why rejected**: The stack scanner is the core competency of TerraX. Duplicating it creates two sources of truth that will diverge. The `terrax tree --json` approach keeps the extension as a thin client.

### Option 3: Dispose and recreate the terminal on each invocation

**Description**: Each time a node is clicked, call `terminal.dispose()` to kill the existing terminal (and any running process) and create a fresh one.

**Pros**:

- No timing issues — the new terminal always starts from a clean state.
- No need to detect TUI vs shell state.

**Cons**:

- The terminal tab closes and reopens visually on every click, which is disorienting.
- Any scrollback history in the terminal is lost.
- The user cannot track consecutive executions in a single terminal.

**Why rejected**: The user experience of a terminal that disappears and reappears on every click was unacceptable. A persistent terminal that updates in place was explicitly requested.

## Future Enhancements

**Potential Improvements**:

1. Replace the 300 ms heuristic delay with a `vscode.window.onDidWriteTerminalData` listener that waits for the shell prompt sequence before sending the new command.
2. Extend `terrax run` to accept additional commands beyond `plan` as inline buttons (e.g. `apply`, `validate`) configurable via `.terrax.yaml`.
3. Add a `terrax.refreshOnSave` VS Code setting that automatically refreshes the sidebar tree when `terragrunt.hcl` files change.

## References

- [`extensions/vscode/src/treeProvider.ts`](../../extensions/vscode/src/treeProvider.ts)
- [`extensions/vscode/src/terminalRunner.ts`](../../extensions/vscode/src/terminalRunner.ts)
- [`cmd/run.go`](../../cmd/run.go)
- [`cmd/tree.go`](../../cmd/tree.go)
- [ADR-0005: Filesystem Tree Building Strategy](0005-filesystem-tree-building-strategy.md)
- [ADR-0009: Executor Isolation Pattern](0009-executor-isolation-pattern.md)
