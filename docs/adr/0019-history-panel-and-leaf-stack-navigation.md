# ADR-0019: History Panel and Leaf Stack Auto-Navigation

**Status**: Accepted

**Date**: 2026-06-20

**Deciders**: TerraX Core Team

**Related**:
- [ADR-0013: VS Code Extension Integration](0013-vscode-extension-integration.md)
- [ADR-0015: Static HCL Dependency Graph](0015-static-hcl-dependency-graph.md)

## Context

Two related problems emerged from using the VS Code extension in practice:

**1. Execution history is invisible in VS Code.** TerraX records every command execution in a JSONL history log. The interactive TUI viewer (`terrax --history`) works in a terminal but requires switching context away from VS Code. Engineers frequently want to re-run a recent command or navigate back to a stack they previously used — neither is possible from within VS Code without opening a terminal and invoking `terrax --history` manually.

**2. Pointing `terrax --dir` at a leaf stack fails with a cryptic error.** A leaf stack is a directory that contains `terragrunt.hcl` but has no subdirectories that are also stacks. When the VS Code History panel (or any caller) passes a leaf stack's absolute path to `terrax --dir`, TerraX scans for subdirectories, finds none, and exits with "no terragrunt directories found." This affected any click-to-open-TUI action on history entries pointing to leaf stacks — the most common case in a typical Terragrunt project.

## Decision

### `terrax history [--dir <path>]`

A new Cobra subcommand outputs the execution history for the current project as a JSON array, most-recent-first. It reuses `getHistoryService()` and `FilterByCurrentProject()` from the existing CLI infrastructure, requiring an `os.Chdir(workDir)` before filtering (since `FilterByCurrentProject` uses `os.Getwd()` internally). A pre-filter check using `deps.FindRepoRoot` prevents dumping all history across projects when `--dir` points to a directory without a `root.hcl` ancestor.

Output: array of `ExecutionLogEntry` objects, never null on empty:

```json
[
  { "id": 42, "timestamp": "...", "stack_path": "workloads/dev/vpc",
    "absolute_path": "/abs/path/vpc", "command": "plan",
    "exit_code": 0, "duration_s": 12.3, "summary": "3 added" }
]
```

### VS Code History panel

A fourth view `terrax.historyTree` in the `terrax` Activity Bar container, backed by `HistoryTreeProvider` in `historyProvider.ts`. Each entry displays `<command> · <stack_name>` as the label, relative time and duration as the description, and `$(check)` or `$(error)` as the icon. Hovering shows the full `absolute_path` as a tooltip.

**Click** opens the TUI at the entry's absolute path (`terrax.openHere`). **Hover button** `$(run)` re-executes the same command directly (`terrax run <command> --dir <path>`). Both the Dependencies and History panels refresh together in `doRefresh()`.

### `resolveWorkDir` — leaf stack auto-navigation

A new `resolveWorkDir(dir string) string` function in `cmd/root.go` is called in `runTUI` after `getWorkingDirectory`. It detects leaf stacks by checking whether the directory has `terragrunt.hcl` (it is a stack) and whether any immediate subdirectory also has `terragrunt.hcl` (it has navigable children). If the directory is a leaf stack with no navigable children, `resolveWorkDir` returns `filepath.Dir(dir)` instead. Otherwise it returns `dir` unchanged.

```go
func resolveWorkDir(dir string) string {
    if _, err := os.Stat(filepath.Join(dir, "terragrunt.hcl")); err != nil {
        return dir // not a stack directory
    }
    entries, _ := os.ReadDir(dir)
    for _, e := range entries {
        if e.IsDir() {
            if _, err := os.Stat(filepath.Join(dir, e.Name(), "terragrunt.hcl")); err == nil {
                return dir // has sub-stacks, not a leaf
            }
        }
    }
    return filepath.Dir(dir) // leaf stack — use parent
}
```

`terrax run` does not call `resolveWorkDir` — direct command execution targets the stack path exactly as given.

## Consequences

### Positive

- History is accessible in VS Code without a terminal context switch — recent executions are one click away.
- `terrax history` is a usable CLI primitive for scripts and CI in addition to the extension.
- Leaf stack paths passed to `terrax --dir` (from history clicks, clipboard, shell completions) work correctly without the user knowing about the leaf/non-leaf distinction.
- `resolveWorkDir` is transparent: the TUI scans from the parent and immediately shows the leaf stack as a selectable node — the effective navigation depth is the same.

### Negative

- `os.Chdir(workDir)` in `runHistoryCmd` is a process-global side effect used to work around `FilterByCurrentProject`'s implicit `os.Getwd()` dependency. A `defer os.Chdir(originalDir)` call restores the cwd, but the workaround would be unnecessary if `FilterByCurrentProject` accepted an explicit directory parameter.
- `resolveWorkDir` performs one `os.ReadDir` on every `runTUI` invocation, even when `--dir` was not specified (in which case the cwd is checked). On slow filesystems, this adds a small latency to every TUI launch.
- `resolveWorkDir` only looks one level deep: if the directory has no immediate sub-stacks but has a nested `module/submodule/terragrunt.hcl`, it will still redirect to the parent. This edge case is rare but possible in non-standard layouts.
- The History panel uses `spawnSync` (synchronous subprocess), blocking the VS Code extension host thread for up to 10 seconds on slow invocations. This is consistent with the existing `treeProvider.ts` pattern but extends the total blocking exposure on each `doRefresh`.

## Alternatives Considered

### Option 1: Redirect to parent in the VS Code extension (TypeScript)

**Description**: Keep `resolveWorkDir` out of Go and have the extension call `path.dirname(entry.absolute_path)` before passing the URI to `terrax.openHere`.

**Pros**:

- No change to the Go binary — the extension handles the special case.
- `terrax --dir <leaf>` fails loudly from the CLI, making leaf paths obviously wrong in other contexts.

**Cons**:

- Any future VS Code extension feature that opens a TUI from a path (e.g., a context menu on a file, a keyboard shortcut) must independently apply the same rule.
- CLI users calling `terrax --dir <leaf>` from a script or shell completion still get an unhelpful error.

**Why rejected**: The fix belongs in the Go binary because the constraint — TerraX cannot navigate from a leaf stack — is a property of TerraX itself, not of the extension. Encoding the workaround in TypeScript would scatter knowledge of TerraX's internal requirements across two codebases. `resolveWorkDir` is a single, documented function that benefits all callers regardless of how `--dir` is set.

### Option 2: Read JSONL history directly from TypeScript

**Description**: Have `HistoryTreeProvider` read `~/Library/Application Support/terrax/history.log` (or the XDG equivalent) directly, parse the JSONL, and filter entries in TypeScript without calling the `terrax` binary.

**Pros**:

- History panel loads without spawning a subprocess, avoiding the spawnSync blocking issue.
- No changes to the Go binary for this feature.

**Cons**:

- The XDG data directory path is OS-specific. Hardcoding it in TypeScript requires duplicating the path-resolution logic already in `internal/history`.
- Project filtering (which entries belong to the current workspace) uses `FindProjectRoot`, implemented in Go. Reimplementing it in TypeScript creates a second copy of the root-detection logic with different fallback semantics.
- The JSONL format is an internal storage detail. Any future change (new fields, compression, rotation) must be coordinated across both parsers.

**Why rejected**: The thin-client model established in ADR-0013 requires that all logic live in Go and the extension consume structured JSON. Directly reading the JSONL file violates this boundary and tightly couples the TypeScript extension to an internal implementation detail of the history subsystem.

## Future Enhancements

**Potential Improvements**:

1. Replace `os.Chdir` in `runHistoryCmd` with an explicit `dir string` parameter on `FilterByCurrentProject`, removing the process-global side effect entirely.
2. Make `resolveWorkDir` configurable — a `--no-resolve` flag for users who explicitly want to scan from a leaf directory and see the "no subdirectories" error rather than be silently redirected.
3. Replace `spawnSync` with async `execFile` in both `treeProvider.ts` and `historyProvider.ts` to avoid blocking the VS Code extension host thread.

## References

- [`cmd/history.go`](../../cmd/history.go)
- [`cmd/root.go` — `resolveWorkDir`](../../cmd/root.go)
- [`extensions/vscode/src/historyProvider.ts`](../../extensions/vscode/src/historyProvider.ts)
- [ADR-0013: VS Code Extension Integration](0013-vscode-extension-integration.md)
