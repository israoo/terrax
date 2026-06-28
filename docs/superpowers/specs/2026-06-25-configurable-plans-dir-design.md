# Configurable Plans Directory Design

**Date:** 2026-06-25
**Status:** Approved

## Problem

The JSON plan output directory (`.terrax/plans`) is hardcoded as `config.DefaultJSONOutDir` throughout the codebase. Teams that use custom output paths, run multiple plan contexts in parallel, or store plan artifacts outside the repository root have no way to change it without modifying source code.

## Goal

Make the plans directory configurable via:
1. A `--plans-dir` CLI flag on commands that generate or read plan files (highest priority).
2. A `plan.json_out_dir` key in `.terrax.yaml` (second priority).
3. The existing default `".terrax/plans"` (fallback).

## Configuration Key

```yaml
plan:
  json_out_dir: ".terrax/plans"   # default; relative to repoRoot or absolute
```

**Priority order:** `--plans-dir` flag > `plan.json_out_dir` in yaml > `".terrax/plans"` default.

## Path Resolution

The stored value may be a relative path (resolved against `repoRoot`) or an absolute path (used as-is). Resolution logic applied at each consumer:

```go
dir := viper.GetString("plan.json_out_dir")
if !filepath.IsAbs(dir) {
    dir = filepath.Join(repoRoot, dir)
}
```

## Propagation Pattern

Each command that accepts `--plans-dir` applies it to Viper explicitly before invoking downstream logic:

```go
if plansDir, _ := cmd.Flags().GetString("plans-dir"); plansDir != "" {
    viper.Set("plan.json_out_dir", plansDir)
}
```

Downstream code (`executor`, `runPlanSummary`, `runPlanReview`, `runSummaryCmd`) reads `viper.GetString("plan.json_out_dir")` — no signature changes to any internal function.

## Commands Receiving `--plans-dir`

| Command | Role | Flag effect |
|---|---|---|
| `terrax [--plans-dir]` | Generator | Sets write dir before executor runs |
| `terrax run [--plans-dir]` | Generator | Sets write dir before executor runs |
| `terrax review [--plans-dir]` | Reader | Sets read dir before `runPlanReview` |
| `terrax summary [--plans-dir]` | Reader | Sets read dir before `plan.Summarize` |

`terrax last` and `terrax history` re-execution inherit the dir from Viper (yaml/default) — no flag, since the re-execution context belongs to the recorded project, not the invoker.

## File Changes

### `internal/config/defaults.go`
`DefaultJSONOutDir` stays as a constant — used only as the Viper default value, not consumed directly by executor or readers after this change.

### `cmd/root.go`
- `initConfig()`: add `viper.SetDefault("plan.json_out_dir", config.DefaultJSONOutDir)`.
- `init()`: register `--plans-dir` flag on `rootCmd`.
- `runTUI()`: apply flag to Viper before calling `executor.Run`.
- `runPlanSummary()`: replace `filepath.Join(repoRoot, config.DefaultJSONOutDir)` with resolved path from Viper.
- `runPlanReview()`: replace `filepath.Join(repoRoot, config.DefaultJSONOutDir)` — wait, `runPlanReview` uses `config.DefaultJSONOutDir` indirectly via `plan.CollectFromJSONDir`. Check: `jsonDir := filepath.Join(repoRoot, config.DefaultJSONOutDir)` → replace with resolved Viper value.

### `internal/executor/executor.go` (`buildFilterArgs`)
Replace:
```go
absJSONOutDir := filepath.Join(repoRoot, config.DefaultJSONOutDir)
```
With:
```go
jsonOutDir := viper.GetString("plan.json_out_dir")
var absJSONOutDir string
if filepath.IsAbs(jsonOutDir) {
    absJSONOutDir = jsonOutDir
} else {
    absJSONOutDir = filepath.Join(repoRoot, jsonOutDir)
}
```

### `cmd/run.go`
- `init()`: register `--plans-dir` flag.
- `runCommand()`: apply flag to Viper before executor call.

### `cmd/review.go`
- `init()`: register `--plans-dir` flag.
- `runReviewCmd()`: apply flag to Viper before `runPlanReview`.

### `cmd/summary.go`
- `init()`: register `--plans-dir` flag.
- `runSummaryCmd()`: apply flag to Viper before `plan.Summarize`; replace hardcoded `config.DefaultJSONOutDir` with Viper-resolved path.

### `.terrax.yaml`
Document `plan.json_out_dir` under the `plan:` section.

## Testing

- `executor_test.go` (or inline): verify `--json-out-dir` arg uses custom dir when `plan.json_out_dir` is set in Viper.
- `cmd/run_test.go`: verify `--plans-dir` flag sets Viper key.
- `cmd/review_test.go`: verify `--plans-dir` flag is wired.
- `cmd/summary_test.go`: verify `--plans-dir` flag makes `plan.Summarize` read from custom dir.

## Global Constraints

- No changes to `executor.Run` signature.
- All files in existing packages — no new packages.
- Three import groups: stdlib · third-party · `github.com/israoo/terrax/...` (alphabetical).
- All comments end with periods.
- Errors wrapped with `fmt.Errorf("context: %w", err)`.
- `task check` must pass.
