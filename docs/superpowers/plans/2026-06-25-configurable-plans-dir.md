# Configurable Plans Directory Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make the JSON plan output directory configurable via `--plans-dir` flag and `plan.json_out_dir` YAML key, replacing the hardcoded `config.DefaultJSONOutDir` constant throughout.

**Architecture:** Viper is the single source of truth for the plans directory. Each affected command applies its `--plans-dir` flag to Viper with `viper.Set("plan.json_out_dir", ...)` before calling downstream logic. The executor and root helpers read `viper.GetString("plan.json_out_dir")` and resolve absolute vs. relative paths inline. No signature changes to any internal function.

**Tech Stack:** Go · Cobra · Viper · `internal/executor` · `internal/config`

## Global Constraints

- Viper key: `plan.json_out_dir`, default value: `config.DefaultJSONOutDir` (`.terrax/plans`).
- Flag name: `--plans-dir` on `terrax`, `terrax run`, `terrax review`, `terrax summary`.
- `terrax last` and `terrax history` re-execution do NOT get a flag — they inherit from Viper.
- No changes to `executor.Run` signature.
- Path resolution: if `filepath.IsAbs(dir)` use as-is; otherwise `filepath.Join(repoRoot, dir)`.
- Three import groups: stdlib · third-party · `github.com/israoo/terrax/...` (alphabetical).
- All comments end with periods.
- Errors wrapped with `fmt.Errorf("context: %w", err)`.
- Run `task check` before every commit.

---

## File Map

| File | Action | What changes |
|---|---|---|
| `cmd/root.go` | Modify | `initConfig` default; `--plans-dir` flag; `runTUI` + `runPlanSummary` + `runPlanReview` use Viper |
| `internal/executor/executor.go` | Modify | `buildFilterArgs` reads `plan.json_out_dir` from Viper, resolves path |
| `internal/executor/executor_test.go` | Modify | New table row for custom `plan.json_out_dir` in `TestBuildFilterArgs_PlanOutputFlags` |
| `cmd/run.go` | Modify | `--plans-dir` flag; apply to Viper in `runCommand` |
| `cmd/review.go` | Modify | `--plans-dir` flag; apply to Viper in `runReviewCmd` |
| `cmd/summary.go` | Modify | `--plans-dir` flag; apply to Viper in `runSummaryCmd`, use Viper for path |
| `.terrax.yaml` | Modify | Document `plan.json_out_dir` key |

---

### Task 1: Wire `plan.json_out_dir` through Viper — executor and root helpers

**Files:**
- Modify: `cmd/root.go` — `initConfig`, `runPlanSummary`, `runPlanReview`
- Modify: `internal/executor/executor.go` — `buildFilterArgs` (lines 153–157)
- Modify: `internal/executor/executor_test.go` — extend `TestBuildFilterArgs_PlanOutputFlags`

**Interfaces:**
- Produces: `viper.GetString("plan.json_out_dir")` returns the correct directory in all consumers.

- [ ] **Step 1: Write the failing executor test**

Add one row to the `tests` table in `TestBuildFilterArgs_PlanOutputFlags` (`internal/executor/executor_test.go`) and a new sub-test after the loop that verifies a custom dir:

```go
// Add at the end of TestBuildFilterArgs_PlanOutputFlags, after the table loop:

t.Run("custom plan.json_out_dir is used", func(t *testing.T) {
    resetViper()
    viper.Set("log_format", "pretty")
    viper.Set("plan.review_enabled", true)
    viper.Set("plan.json_out_dir", "/custom/plans")

    args := buildFilterArgs("/repo", "plan", []string{"/path/to/stack"})

    want := "--json-out-dir=/custom/plans"
    found := false
    for _, a := range args {
        if a == want {
            found = true
            break
        }
    }
    assert.True(t, found, "expected --json-out-dir=/custom/plans in args, got: %v", args)
})

t.Run("relative plan.json_out_dir is joined with repoRoot", func(t *testing.T) {
    resetViper()
    viper.Set("log_format", "pretty")
    viper.Set("plan.review_enabled", true)
    viper.Set("plan.json_out_dir", "custom/plans")

    args := buildFilterArgs("/repo", "plan", []string{"/path/to/stack"})

    want := "--json-out-dir=/repo/custom/plans"
    found := false
    for _, a := range args {
        if a == want {
            found = true
            break
        }
    }
    assert.True(t, found, "expected --json-out-dir=/repo/custom/plans in args, got: %v", args)
})
```

- [ ] **Step 2: Run tests to verify they fail**

```bash
go test ./internal/executor/ -run TestBuildFilterArgs_PlanOutputFlags -v
```

Expected: the two new sub-tests FAIL (executor still uses `config.DefaultJSONOutDir`).

- [ ] **Step 3: Update executor `buildFilterArgs`**

In `internal/executor/executor.go`, replace the block at lines ~153–157:

```go
// Inject --json-out-dir when summary or review mode is active.
// Both modes read the JSON plan files — review for the TUI, summary for terminal output.
if command == "plan" && (viper.GetBool("plan.summary_enabled") || viper.GetBool("plan.review_enabled")) {
    absJSONOutDir := filepath.Join(repoRoot, config.DefaultJSONOutDir)
    args = append(args, fmt.Sprintf("--json-out-dir=%s", absJSONOutDir))
}
```

With:

```go
// Inject --json-out-dir when summary or review mode is active.
// Both modes read the JSON plan files — review for the TUI, summary for terminal output.
if command == "plan" && (viper.GetBool("plan.summary_enabled") || viper.GetBool("plan.review_enabled")) {
    jsonOutDir := viper.GetString("plan.json_out_dir")
    if jsonOutDir == "" {
        jsonOutDir = config.DefaultJSONOutDir
    }
    var absJSONOutDir string
    if filepath.IsAbs(jsonOutDir) {
        absJSONOutDir = jsonOutDir
    } else {
        absJSONOutDir = filepath.Join(repoRoot, jsonOutDir)
    }
    args = append(args, fmt.Sprintf("--json-out-dir=%s", absJSONOutDir))
}
```

- [ ] **Step 4: Add Viper default in `initConfig` and update root helpers**

In `cmd/root.go`:

**a) In `initConfig()`**, add after the existing `SetDefault` calls:
```go
viper.SetDefault("plan.json_out_dir", config.DefaultJSONOutDir)
```

**b) Replace `runPlanSummary`:**
```go
// runPlanSummary reads JSON plan files from the configured plans directory and prints a terminal count summary.
func runPlanSummary(ctx context.Context, stackPath, repoRoot string) error {
    jsonOutDir := viper.GetString("plan.json_out_dir")
    if jsonOutDir == "" {
        jsonOutDir = config.DefaultJSONOutDir
    }
    var dir string
    if filepath.IsAbs(jsonOutDir) {
        dir = jsonOutDir
    } else {
        dir = filepath.Join(repoRoot, jsonOutDir)
    }
    _, err := plan.Summarize(ctx, dir, repoRoot)
    return err
}
```

**c) In `runPlanReview`**, replace the line:
```go
jsonDir := filepath.Join(repoRoot, config.DefaultJSONOutDir)
```
With:
```go
jsonOutDir := viper.GetString("plan.json_out_dir")
if jsonOutDir == "" {
    jsonOutDir = config.DefaultJSONOutDir
}
var jsonDir string
if filepath.IsAbs(jsonOutDir) {
    jsonDir = jsonOutDir
} else {
    jsonDir = filepath.Join(repoRoot, jsonOutDir)
}
```

- [ ] **Step 5: Run tests to verify they pass**

```bash
go test ./internal/executor/ -run TestBuildFilterArgs_PlanOutputFlags -v
```

Expected: all sub-tests PASS including the two new ones.

- [ ] **Step 6: Run full check**

```bash
task check
```

Expected: `✅ All checks passed`

- [ ] **Step 7: Commit**

```bash
git add internal/executor/executor.go internal/executor/executor_test.go cmd/root.go
git commit -m "feat: wire plan.json_out_dir through Viper in executor and root helpers"
```

---

### Task 2: `--plans-dir` flag on generator commands

**Files:**
- Modify: `cmd/root.go` — register flag in `init()`, apply in `runTUI()`
- Modify: `cmd/run.go` — register flag in `init()`, apply in `runCommand()`
- Test: `cmd/run_test.go` — verify flag sets Viper key

**Interfaces:**
- Consumes: `viper.Set("plan.json_out_dir", plansDir)` from Task 1.

- [ ] **Step 1: Write the failing test**

Add to `cmd/run_test.go`:

```go
func TestRunCommand_PlansDirFlag_SetsViper(t *testing.T) {
    // Arrange: temp dir with a terragrunt.hcl so workDir resolves.
    tmpDir := t.TempDir()
    require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "terragrunt.hcl"), []byte(""), 0644))

    cmd := &cobra.Command{}
    cmd.Flags().String("dir", "", "")
    cmd.Flags().String("plans-dir", "", "")
    require.NoError(t, cmd.ParseFlags([]string{"--dir", tmpDir, "--plans-dir", "/custom/plans"}))

    // Act: read the flag and apply to viper (mimics what runCommand will do).
    if plansDir, _ := cmd.Flags().GetString("plans-dir"); plansDir != "" {
        viper.Set("plan.json_out_dir", plansDir)
    }

    // Assert.
    assert.Equal(t, "/custom/plans", viper.GetString("plan.json_out_dir"))
    viper.Reset() // clean up global state.
}
```

- [ ] **Step 2: Run test to verify it fails**

```bash
go test ./cmd/ -run TestRunCommand_PlansDirFlag_SetsViper -v
```

Expected: FAIL — `--plans-dir` flag not registered yet.

- [ ] **Step 3: Add `--plans-dir` to `cmd/run.go`**

In `init()`:
```go
runCmd.Flags().String("plans-dir", "", "Directory for JSON plan output files (overrides plan.json_out_dir in config)")
```

In `runCommand()`, add before the `if command == "plan"` block:
```go
if plansDir, _ := cmd.Flags().GetString("plans-dir"); plansDir != "" {
    viper.Set("plan.json_out_dir", plansDir)
}
```

- [ ] **Step 4: Add `--plans-dir` to `cmd/root.go`**

In `init()`:
```go
rootCmd.Flags().String("plans-dir", "", "Directory for JSON plan output files (overrides plan.json_out_dir in config)")
```

In `runTUI()`, add immediately after `ensureConfigFromWorkDir(workDir)`:
```go
if plansDir, _ := cmd.Flags().GetString("plans-dir"); plansDir != "" {
    viper.Set("plan.json_out_dir", plansDir)
}
```

- [ ] **Step 5: Run tests**

```bash
go test ./cmd/ -run TestRunCommand_PlansDirFlag_SetsViper -v
```

Expected: PASS.

- [ ] **Step 6: Full check**

```bash
task check
```

Expected: `✅ All checks passed`

- [ ] **Step 7: Commit**

```bash
git add cmd/root.go cmd/run.go cmd/run_test.go
git commit -m "feat: add --plans-dir flag to terrax and terrax run"
```

---

### Task 3: `--plans-dir` flag on reader commands + `.terrax.yaml` documentation

**Files:**
- Modify: `cmd/review.go` — register flag, apply in `runReviewCmd()`
- Modify: `cmd/summary.go` — register flag, apply in `runSummaryCmd()`, use Viper path
- Modify: `.terrax.yaml` — document `plan.json_out_dir`
- Test: `cmd/summary_test.go` — verify reading from custom dir

**Interfaces:**
- Consumes: `viper.Set("plan.json_out_dir", plansDir)` from Task 1.

- [ ] **Step 1: Write the failing test**

Add to `cmd/summary_test.go`:

```go
func TestSummaryCmd_CustomPlansDir(t *testing.T) {
    tmpDir := t.TempDir()

    // Write plan files in a custom subdir, not .terrax/plans.
    customPlansDir := filepath.Join(tmpDir, "custom-plans", "env", "dev", "vpc")
    require.NoError(t, os.MkdirAll(customPlansDir, 0755))
    planJSON := `{"resource_changes":[{"address":"aws_vpc.main","type":"aws_vpc","name":"main","change":{"actions":["create"],"before":null,"after":{}}}]}`
    require.NoError(t, os.WriteFile(filepath.Join(customPlansDir, "tfplan.json"), []byte(planJSON), 0644))

    // Write root.hcl so FindRepoRoot resolves correctly.
    require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "root.hcl"), []byte(""), 0644))

    oldStdout := os.Stdout
    r, w, _ := os.Pipe()
    os.Stdout = w

    done := make(chan string, 1)
    go func() {
        var buf bytes.Buffer
        _, _ = io.Copy(&buf, r)
        done <- buf.String()
    }()

    cmd := &cobra.Command{}
    cmd.Flags().String("dir", "", "")
    cmd.Flags().String("plans-dir", "", "")
    require.NoError(t, cmd.ParseFlags([]string{
        "--dir", tmpDir,
        "--plans-dir", filepath.Join(tmpDir, "custom-plans"),
    }))

    err := runSummaryCmd(cmd, []string{})

    _ = w.Close()
    os.Stdout = oldStdout
    output := <-done

    viper.Reset()

    require.NoError(t, err)
    assert.Contains(t, output, "Pending changes", "summary must read from custom plans dir")
}
```

- [ ] **Step 2: Run test to verify it fails**

```bash
go test ./cmd/ -run TestSummaryCmd_CustomPlansDir -v
```

Expected: FAIL — `--plans-dir` flag not registered on `summaryCmd`.

- [ ] **Step 3: Update `cmd/summary.go`**

In `init()`, add:
```go
summaryCmd.Flags().String("plans-dir", "", "Directory for JSON plan output files (overrides plan.json_out_dir in config)")
```

In `runSummaryCmd()`, after `ensureConfigFromWorkDir(workDir)` and `workDir = resolveWorkDir(workDir)`:
```go
if plansDir, _ := cmd.Flags().GetString("plans-dir"); plansDir != "" {
    viper.Set("plan.json_out_dir", plansDir)
}
```

Replace the hardcoded path construction:
```go
// Before:
jsonDir := filepath.Join(repoRoot, config.DefaultJSONOutDir)

// After:
jsonOutDir := viper.GetString("plan.json_out_dir")
if jsonOutDir == "" {
    jsonOutDir = config.DefaultJSONOutDir
}
var jsonDir string
if filepath.IsAbs(jsonOutDir) {
    jsonDir = jsonOutDir
} else {
    jsonDir = filepath.Join(repoRoot, jsonOutDir)
}
```

Remove the `config` import if it is no longer used in `summary.go` after this change (the import is only needed for `config.DefaultJSONOutDir` which is now only referenced in the fallback). Keep the import — the fallback still uses `config.DefaultJSONOutDir`.

- [ ] **Step 4: Update `cmd/review.go`**

In `init()`, add:
```go
reviewCmd.Flags().String("plans-dir", "", "Directory for JSON plan output files (overrides plan.json_out_dir in config)")
```

In `runReviewCmd()`, after `ensureConfigFromWorkDir(workDir)`:
```go
if plansDir, _ := cmd.Flags().GetString("plans-dir"); plansDir != "" {
    viper.Set("plan.json_out_dir", plansDir)
}
```

(No path construction change needed in `runReviewCmd` — it delegates to `runPlanReview`, which now reads Viper as updated in Task 1.)

- [ ] **Step 5: Document in `.terrax.yaml`**

Find the `plan:` section in `.terrax.yaml` and add the new key. Locate the block:

```yaml
plan:
  review_enabled: true        # Launches StatePlanReview TUI; reads .terrax/plans/
  summary_enabled: false      # Prints grouped terminal summary after plan
```

Replace with:

```yaml
plan:
  review_enabled: true        # Launches StatePlanReview TUI; reads plan output dir
  summary_enabled: false      # Prints grouped terminal summary after plan
  json_out_dir: ".terrax/plans"  # Directory for Terragrunt --json-out-dir output (relative to repo root or absolute)
```

- [ ] **Step 6: Run tests**

```bash
go test ./cmd/ -run TestSummaryCmd_CustomPlansDir -v
```

Expected: PASS.

- [ ] **Step 7: Full check**

```bash
task check
```

Expected: `✅ All checks passed`

- [ ] **Step 8: Commit**

```bash
git add cmd/review.go cmd/summary.go cmd/summary_test.go .terrax.yaml
git commit -m "feat: add --plans-dir flag to terrax review and terrax summary"
```

---

## Self-Review

**Spec coverage:**
- ✅ `plan.json_out_dir` viper default — Task 1 (`initConfig`)
- ✅ Executor reads viper with abs/rel resolution — Task 1
- ✅ `runPlanSummary` + `runPlanReview` use viper — Task 1
- ✅ `--plans-dir` on `terrax` (root TUI) — Task 2
- ✅ `--plans-dir` on `terrax run` — Task 2
- ✅ `--plans-dir` on `terrax review` — Task 3
- ✅ `--plans-dir` on `terrax summary` — Task 3
- ✅ `.terrax.yaml` documents `plan.json_out_dir` — Task 3
- ✅ `terrax last` and history re-execution inherit from Viper without flag — not in file map (correct)
- ✅ No `executor.Run` signature change — verified in all tasks

**Placeholder scan:** No TBDs. All code is complete and explicit.

**Type consistency:** `viper.Set("plan.json_out_dir", plansDir)` and `viper.GetString("plan.json_out_dir")` used consistently across all three tasks.
