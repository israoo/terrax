# Plan Summary Mode Design

**Date:** 2026-06-20
**Status:** Approved

---

## Goal

Add a terminal summary mode for `plan` that reads pre-generated JSON plan files and prints a one-line-per-stack summary using `tf-summarize`, without launching the interactive TUI. Controlled by a new `plan.summary_enabled` flag independent of the existing `plan.review_enabled`.

## Context

TerraX's current plan review reads binary `.binary` files from `.terragrunt-cache`, spawns `terraform show -json` per stack (heavy, slow), and launches the `StatePlanReview` TUI. This is powerful but expensive for quick feedback on large stacks.

When `--json-out-dir` is configured, Terragrunt already generates `terraform show -json` output per stack as JSON files. Reading these pre-generated files with `tf-summarize` eliminates the per-stack subprocess and produces the same compact summary the CI/CD pipeline shows.

---

## Configuration

Two independent flags:

```yaml
plan:
  summary_enabled: false  # NEW — terminal summary via tf-summarize
  review_enabled: true    # existing — TUI plan review
```

**Behaviour matrix:**

| `summary_enabled` | `review_enabled` | Result |
|---|---|---|
| false | false | nothing |
| true | false | terminal summary only |
| false | true | TUI only (current) |
| true | true | terminal summary, then TUI |

Both default to their current values (`summary_enabled: false`, `review_enabled: true`).

---

## Architecture

### Arg injection (`internal/executor/executor.go`)

`appendPlanTerragruntOutputFlags` currently reads `plan.json_out_dir`. Replace with:

```go
func appendPlanTerragruntOutputFlags(args []string, command string) []string {
    if command != "plan" {
        return args
    }
    if viper.GetBool("plan.summary_enabled") {
        args = append(args, fmt.Sprintf("--json-out-dir=%s", config.DefaultJSONOutDir))
    }
    return args
}
```

`DefaultJSONOutDir = "./tmp/json-plans"` is added to `internal/config/defaults.go`.

### Summarizer (`internal/plan/summarizer.go`) — NEW

```go
// Summarize scans dir for JSON plan files, prints a count line per stack using
// tf-summarize, and returns the number of stacks with changes.
func Summarize(ctx context.Context, dir string) (int, error)
```

Internals:
1. Walk `dir` recursively for `*.json` files (sorted).
2. For each file, compute `stackName = filepath.Dir(strings.TrimPrefix(absPath, dir+"/"))`.
3. Run `tf-summarize -json-sum <file>` — captured stdout, no stderr noise.
4. If exit non-zero or output empty → `fmt.Fprintf(os.Stderr, "Warning: could not summarize %s\n", stackName)` and continue.
5. Parse JSON: `{"changes": {"add": N, "update": N, "delete": N, "recreate": N, "import": N, "moved": N}}`.
6. Print `"  <stackName>: +<add> ~<update> -<delete> ♻<recreate>"`.
7. If `total > 0`, increment `changedCount`.
8. After all files: print `"<changedCount> stack(s) with pending changes"` (or `"No changes detected across <total> stack(s)"`).
9. Return `changedCount, nil`.

**Error handling:**
- `dir` does not exist → return `0, nil` (not an error — plan may not have generated files yet).
- `tf-summarize` not in PATH → return `0, fmt.Errorf("tf-summarize not found: install from https://github.com/dineshba/tf-summarize")`.
- Per-file failures → warn and skip, continue with remaining files.

**Mockable exec:** `var execCommandContext = exec.CommandContext` (same pattern as `internal/plan/collector.go`).

### `cmd/root.go` wiring

New helper `runPlanSummary`:

```go
func runPlanSummary(ctx context.Context, stackPath string) error {
    fmt.Println()
    _, err := plan.Summarize(ctx, config.DefaultJSONOutDir)
    return err
}
```

`viper.SetDefault("plan.summary_enabled", false)` added in `initConfig`.

At each of the three dispatch sites (normal TUI, `--last`, `--history re-execute`), insert before `executor.Run`:

```go
if command == "force-unlock" {
    return runForceUnlock(ctx, historyService, stackPath)
}
```

And after `executor.Run`, before the `review_enabled` check:

```go
if command == "plan" && viper.GetBool("plan.summary_enabled") {
    if err := runPlanSummary(ctx, stackPath); err != nil {
        fmt.Fprintf(os.Stderr, "Warning: plan summary failed: %v\n", err)
    }
}
if command == "plan" && viper.GetBool("plan.review_enabled") {
    return runPlanReview(ctx, stackPath)
}
```

Summary errors are non-fatal (warn and continue) to avoid blocking the workflow.

---

## tf-summarize JSON schema

Input: Terraform plan JSON file (output of `terraform show -json`).
Output of `tf-summarize -json-sum`:

```json
{
  "changes": {
    "add": 1,
    "update": 0,
    "delete": 0,
    "recreate": 0,
    "import": 0,
    "moved": 0
  }
}
```

---

## Terminal output format

```
🔍 Scanning 12 JSON plan(s)...

  workloads/development/us-east-1/transversal/security-group-rules/alb: +0 ~0 -0 ♻0
  workloads/development/us-east-1/transversal/security-group-rules/ecs-tasks: +1 ~0 -0 ♻0
  workloads/development/us-east-1/transversal/vpc-peering/atlas: +0 ~0 -0 ♻0
  ...

1 stack(s) with pending changes
```

---

## Testing

### `internal/plan/summarizer_test.go`

Table-driven, mock `execCommandContext` (same subprocess pattern as `collector_test.go`).

Test cases:
- Directory does not exist → returns `(0, nil)`
- `tf-summarize` not in PATH → returns error with install URL
- Single stack with changes → prints correct count line, returns `(1, nil)`
- Single stack with no changes → prints `+0 ~0 -0 ♻0`, returns `(0, nil)`
- Multiple stacks, some with changes → correct `changedCount`
- Per-file failure (bad JSON output) → warns, continues, returns partial count

### `internal/executor/executor_test.go`

Add test `TestBuildTerragruntArgs_PlanSummaryEnabled`:
- `plan.summary_enabled: true` → `--json-out-dir=./tmp/json-plans` in args before `--`
- `plan.summary_enabled: false` → no `--json-out-dir` in args
- `plan.summary_enabled: true` + command != "plan" → no `--json-out-dir`

---

## Standards Compliance

- `internal/plan/summarizer.go` — no UI imports, pure business logic.
- All comments end with periods.
- Imports: 3 groups (stdlib, third-party, `github.com/israoo/terrax/...`).
- `execCommandContext` mockable var.
- Errors wrapped: `fmt.Errorf("...: %w", err)`.
- Summary errors are warnings, not fatal — workflow continues.

---

## Out of Scope

- Showing `import` and `moved` counts in the output line (counted internally but not displayed, matching workflow format).
- Markdown generation (`tf-summarize -md`).
- Modifying `PlanReport` models or `StatePlanReview` TUI.
- Removing the existing `plan.json_out_dir` viper key (it becomes unused but causes no harm).
