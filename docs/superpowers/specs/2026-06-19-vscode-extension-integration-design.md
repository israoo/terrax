# Design: VS Code Extension Integration

**Date:** 2026-06-19
**Status:** Approved

## Context

`tg-runner` is a standalone VS Code extension that adds a right-click context menu in the file explorer to run Terragrunt commands. It currently uses a static `.vscode/tg-commands.json` config file to define available commands.

TerraX is a more powerful terminal TUI for Terragrunt stack navigation. The goal is to make the VS Code extension a thin launcher for TerraX, and consolidate both tools in the same repository.

## Goals

1. Move the VS Code extension into `terrax/extensions/vscode/` (monorepo structure).
2. Add a `--dir <path>` flag to TerraX so it can be invoked from any directory without `cd`.
3. Simplify the extension to be a zero-config launcher: right-click a folder → opens TerraX in VS Code's integrated terminal pointing at that folder.

## Non-Goals

- Keeping the `.vscode/tg-commands.json` config mechanism.
- Any bidirectional communication between VS Code and TerraX.
- Embedding TerraX output back into VS Code panels.

## Directory Structure

```
terrax/
├── cmd/root.go                    # +flag --dir
├── internal/
├── extensions/
│   └── vscode/
│       ├── src/
│       │   ├── extension.ts       # simplified — no config loading
│       │   └── terminalRunner.ts  # builds terrax --dir '<path>'
│       ├── package.json           # renamed to terrax-vscode
│       ├── tsconfig.json
│       └── vitest.config.ts
├── Taskfile.yml                   # +ext:install, ext:build, ext:package
└── .gitignore                     # +extensions/vscode/node_modules, out/, *.vsix
```

Deleted from tg-runner: `configLoader.ts`, `types.ts`, `__tests__/configLoader.test.ts`.

## TerraX: `--dir` Flag

Add `--dir <path>` to `cmd/root.go` as an optional persistent flag. When provided, `getWorkingDirectory()` returns this path instead of `os.Getwd()`. Without the flag, behavior is identical to today.

```
terrax --dir '/Users/isra/infra/modules/networking'
```

## VS Code Extension

### Command

Menu label: **"TerraX: Open here"**
VS Code command ID: `terrax.openHere`

### Settings

| Setting | Type | Default | Description |
|---|---|---|---|
| `terrax.binaryPath` | string | `"terrax"` | Path to terrax binary. Defaults to `terrax` on PATH. |

### Flow

```
Right-click folder/file → "TerraX: Open here"
  → resolve target directory (folder itself, or dirname if file)
  → read terrax.binaryPath from VS Code config
  → open/reuse terminal named "TerraX"
  → send: terrax --dir '<abs/path>'
```

### Removed Complexity

- No `.vscode/tg-commands.json` lookup.
- No QuickPick menu.
- No config traversal up the directory tree.
- No `configLoader.ts` / `types.ts`.

### `extension.ts`

Registers `terrax.openHere` command. Resolves target path from the URI argument (falls back to workspace root). Reads `terrax.binaryPath` from VS Code config. Calls `runInTerminal(binaryPath, targetPath)`.

### `terminalRunner.ts`

Builds command: `<binaryPath> --dir '<escaped-path>'`. Reuses existing terminal named `"TerraX"` if present, otherwise creates one. Shows and sends command.

## Taskfile Tasks

```yaml
ext:install:  pnpm install (in extensions/vscode)
ext:build:    pnpm compile (tsc)
ext:package:  pnpm vsce package → terrax-vscode-*.vsix
```

## `.gitignore` Additions

```
extensions/vscode/node_modules/
extensions/vscode/out/
extensions/vscode/*.vsix
```

## Testing

- Extension logic is minimal — no unit tests needed beyond what tg-runner already had (which are deleted since `configLoader` is removed).
- Manual test: right-click a folder → terminal opens → `terrax --dir` runs.
- TerraX `--dir` flag: add a unit test in `cmd/root_test.go` verifying `getWorkingDirectory()` returns the flag value when provided.
