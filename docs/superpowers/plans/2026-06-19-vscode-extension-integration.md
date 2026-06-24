# VS Code Extension Integration Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Integrate tg-runner as `extensions/vscode/` inside the TerraX monorepo, simplified to a zero-config launcher that opens `terrax --dir <path>` from VS Code's file explorer context menu.

**Architecture:** Two independent changes shipped in sequence. First, a `--dir` flag is added to the TerraX CLI so it can target any directory without `cd`. Second, the VS Code extension is scaffolded fresh under `extensions/vscode/` with all config-loading code removed — it reads only a `terrax.binaryPath` VS Code setting and sends `terrax --dir '<path>'` to an integrated terminal. No shared state between Go and TypeScript; the extension communicates exclusively via CLI arguments.

**Tech Stack:** Go 1.25.5 · Cobra · testify v1.11.1 · TypeScript 5.3 · VS Code Extension API 1.85+ · pnpm · @vscode/vsce

## Global Constraints

- All Go comments must end with a period.
- Go imports: three groups (stdlib / third-party / `github.com/israoo/terrax/...`), alphabetically sorted within each group.
- Errors always wrapped: `fmt.Errorf("context: %w", err)`.
- Run `task check` before each Go commit (fmt + vet + lint + test).
- Extension lives at `extensions/vscode/` — never inside `.vscode/`.
- Do NOT copy files from `tg-runner` — create all extension files from scratch as specified below.

---

### Task 1: Add `--dir` flag to TerraX

**Files:**
- Modify: `cmd/root.go` — add flag, update `getWorkingDirectory` signature and its call site
- Modify: `cmd/root_test.go` — add two unit tests for `getWorkingDirectory`

**Interfaces:**
- Produces: `getWorkingDirectory(dir string) (string, error)` — returns `dir` when non-empty, otherwise `os.Getwd()`

- [ ] **Step 1: Write the failing tests**

In `cmd/root_test.go`, add these two functions (testify is already imported in the file):

```go
func TestGetWorkingDirectory_WithExplicitDir(t *testing.T) {
    got, err := getWorkingDirectory("/tmp/custom-path")
    require.NoError(t, err)
    assert.Equal(t, "/tmp/custom-path", got)
}

func TestGetWorkingDirectory_DefaultsToCwd(t *testing.T) {
    expected, err := os.Getwd()
    require.NoError(t, err)

    got, err := getWorkingDirectory("")
    require.NoError(t, err)
    assert.Equal(t, expected, got)
}
```

- [ ] **Step 2: Run tests to verify they fail**

```bash
go test ./cmd/... -run TestGetWorkingDirectory -v
```

Expected: compile error — `getWorkingDirectory` does not accept arguments.

- [ ] **Step 3: Update `getWorkingDirectory` in `cmd/root.go`**

Replace the existing function:

```go
// getWorkingDirectory returns the current working directory.
func getWorkingDirectory() (string, error) {
    workDir, err := os.Getwd()
    if err != nil {
        return "", err
    }
    return workDir, nil
}
```

With:

```go
// getWorkingDirectory returns dir if non-empty, otherwise the current working directory.
func getWorkingDirectory(dir string) (string, error) {
    if dir != "" {
        return dir, nil
    }
    workDir, err := os.Getwd()
    if err != nil {
        return "", err
    }
    return workDir, nil
}
```

- [ ] **Step 4: Register the `--dir` flag**

In `cmd/root.go`, inside `init()`, add after the existing flag registrations:

```go
rootCmd.Flags().String("dir", "", "Working directory (overrides current directory)")
```

- [ ] **Step 5: Update the `runTUI` call site**

In `cmd/root.go`, inside `runTUI`, replace:

```go
workDir, err := getWorkingDirectory()
```

With:

```go
dirFlag, _ := cmd.Flags().GetString("dir")
workDir, err := getWorkingDirectory(dirFlag)
```

- [ ] **Step 6: Run tests to verify they pass**

```bash
go test ./cmd/... -run TestGetWorkingDirectory -v
```

Expected: both tests PASS.

- [ ] **Step 7: Run full check**

```bash
task check
```

Expected: all checks pass, 0 lint issues.

- [ ] **Step 8: Commit**

```bash
git add cmd/root.go cmd/root_test.go
git commit -m "feat: add --dir flag to override working directory"
```

---

### Task 2: Scaffold VS Code extension under `extensions/vscode/`

**Files:**
- Create: `extensions/vscode/package.json`
- Create: `extensions/vscode/tsconfig.json`
- Create: `extensions/vscode/src/extension.ts`
- Create: `extensions/vscode/src/terminalRunner.ts`
- Modify: `.gitignore` — add extension build artifacts
- Modify: `Taskfile.yml` — add `ext:install`, `ext:build`, `ext:package` tasks

**Interfaces:**
- Consumes: `terrax --dir '<path>'` CLI (produced by Task 1)
- Produces: `runInTerminal(binaryPath: string, itemPath: string): void`

- [ ] **Step 1: Create `extensions/vscode/package.json`**

```json
{
  "name": "terrax-vscode",
  "displayName": "TerraX",
  "description": "Open TerraX from the VS Code file explorer",
  "version": "0.1.0",
  "engines": {
    "vscode": "^1.85.0"
  },
  "activationEvents": [],
  "main": "./out/extension.js",
  "contributes": {
    "commands": [
      {
        "command": "terrax.openHere",
        "title": "TerraX: Open here"
      }
    ],
    "menus": {
      "explorer/context": [
        {
          "command": "terrax.openHere",
          "group": "navigation"
        }
      ]
    },
    "configuration": {
      "title": "TerraX",
      "properties": {
        "terrax.binaryPath": {
          "type": "string",
          "default": "terrax",
          "description": "Path to the terrax binary. Defaults to 'terrax' (assumes it's on PATH)."
        }
      }
    }
  },
  "scripts": {
    "compile": "tsc -p ./",
    "watch": "tsc -watch -p ./",
    "vscode:prepublish": "pnpm run compile"
  },
  "devDependencies": {
    "@types/node": "^18.0.0",
    "@types/vscode": "^1.85.0",
    "@vscode/vsce": "^3.9.1",
    "typescript": "^5.3.0"
  }
}
```

- [ ] **Step 2: Create `extensions/vscode/tsconfig.json`**

```json
{
  "compilerOptions": {
    "module": "commonjs",
    "target": "ES2020",
    "outDir": "out",
    "lib": ["ES2020"],
    "sourceMap": true,
    "rootDir": "src",
    "strict": true
  },
  "exclude": ["node_modules", "out"]
}
```

- [ ] **Step 3: Create `extensions/vscode/src/terminalRunner.ts`**

```typescript
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
```

- [ ] **Step 4: Create `extensions/vscode/src/extension.ts`**

```typescript
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
```

- [ ] **Step 5: Update `.gitignore`**

Add these three lines at the end of `.gitignore`:

```
extensions/vscode/node_modules/
extensions/vscode/out/
extensions/vscode/*.vsix
```

- [ ] **Step 6: Add tasks to `Taskfile.yml`**

In `Taskfile.yml`, add after the `clean` task (before `init`):

```yaml
  ext:install:
    desc: Install VS Code extension dependencies
    dir: extensions/vscode
    cmds:
      - pnpm install

  ext:build:
    desc: Compile VS Code extension TypeScript
    dir: extensions/vscode
    cmds:
      - pnpm compile

  ext:package:
    desc: Package VS Code extension as .vsix
    dir: extensions/vscode
    cmds:
      - pnpm vsce package
```

- [ ] **Step 7: Install dependencies and verify the build**

```bash
task ext:install
task ext:build
```

Expected: `extensions/vscode/out/extension.js` created, no TypeScript errors.

- [ ] **Step 8: Package the extension**

```bash
task ext:package
```

Expected: `extensions/vscode/terrax-vscode-0.1.0.vsix` created.

- [ ] **Step 9: Commit**

```bash
git add extensions/vscode/ .gitignore Taskfile.yml
git commit -m "feat: add VS Code extension as TerraX launcher"
```
