# Summary Subcommand Design

**Date:** 2026-06-25
**Status:** Approved

## Problem

`plan.Summarize` — the terminal summary of pending vs. no-change stacks — is only reachable automatically after running `plan` from the TUI when `plan.summary_enabled: true` is set in `.terrax.yaml`. There is no way to invoke the summary on already-generated plan files without re-running the full plan.

## Goal

Add `terrax summary` as a standalone subcommand that prints the terminal plan summary from existing `.terrax/plans/` JSON files, following the same `--dir` pattern as `terrax review`.

## CLI Surface

```
terrax summary [--dir <path>]
```

- Without `--dir`: uses the current working directory to locate the project root.
- With `--dir`: points to a specific project directory (same behavior as `terrax review --dir`).

**Output** (produced by the existing `plan.Summarize`):

```
🔍 Scanning 5 JSON plan(s)...

✅ No changes (3):
  env/dev/vpc
  env/dev/rds
  env/dev/alb

⚠️  Pending changes (2):
  env/prod/app: +2 ~1 -0 ♻0
  env/prod/db:  +0 ~3 -1 ♻0

2 stack(s) with pending changes
```

If `.terrax/plans/` does not exist or contains no JSON files, `plan.Summarize` returns `(0, nil)` with no output. Exit code is 0 in all cases unless a filesystem error occurs.

## Architecture

Single new file `cmd/summary.go` in `package cmd`. No changes to `internal/plan/`, `root.go`, or any existing subcommand.

Execution flow:

```
terrax summary [--dir]
  └── getWorkingDirectory(dirFlag)        // "" → os.Getwd()
  └── resolveWorkDir(workDir)             // redirects leaf stack path to parent
  └── ensureConfigFromWorkDir(workDir)    // loads .terrax.yaml from project root
  └── deps.FindRepoRoot(workDir, rootConfigFile)  // derives repoRoot
  └── plan.Summarize(ctx, jsonDir, repoRoot)       // jsonDir = repoRoot/.terrax/plans
```

`runPlanSummary` in `root.go` is unchanged — it continues to serve the automatic post-plan flow triggered by `plan.summary_enabled`.

## File Changes

| File | Action | Responsibility |
|---|---|---|
| `cmd/summary.go` | Create | `terrax summary` subcommand |
| `cmd/summary_test.go` | Create | Two table-driven tests |

## Test Cases

**`TestSummaryCmd_NoPlanDir`**
- Setup: `t.TempDir()` with no `.terrax/plans/` directory.
- Assert: `runSummaryCmd` returns `nil`, stdout is empty.

**`TestSummaryCmd_WithPlanFiles`**
- Setup: `t.TempDir()` with a minimal valid plan JSON at `.terrax/plans/env/dev/vpc/tfplan.json` containing one `create` resource change.
- Assert: `runSummaryCmd` returns `nil`, stdout contains `"Pending changes"` and the stack name.

## Global Constraints

- File in `package cmd` — no new packages.
- Three import groups: stdlib · third-party · `github.com/israoo/terrax/...` (alphabetical within each).
- All comments end with periods.
- Errors wrapped with `fmt.Errorf("context: %w", err)`.
- `--dir` flag present; no other flags.
- `runPlanSummary` in `root.go` must not be modified.
- `task check` must pass.
