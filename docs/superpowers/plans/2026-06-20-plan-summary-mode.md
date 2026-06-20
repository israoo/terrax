# Plan Summary Mode Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add `plan.summary_enabled` flag that injects `--json-out-dir` into plan commands and prints a terminal summary via `tf-summarize` after execution, independent of the existing `plan.review_enabled` TUI flag.

**Architecture:** Four layers — constants in `config/defaults.go`; arg injection in `executor.go` via a new `appendPlanTerragruntOutputFlags` helper; a new `internal/plan/summarizer.go` that runs `tf-summarize -json-sum` per JSON file; wiring in `cmd/root.go` at all three dispatch sites.

**Tech Stack:** Go 1.25.5 · `github.com/spf13/viper` · `tf-summarize` (external CLI, not a Go dep)

## Global Constraints

- All comments must end with periods.
- Imports: three groups — stdlib, third-party, `github.com/israoo/terrax/...` — alphabetically sorted, blank lines between groups.
- `internal/plan/summarizer.go` must not import `internal/tui/` or any charmbracelet package.
- Tests: table-driven, `resetViper()` in executor tests; plain `viper.Reset()` not used directly in new tests.
- New exec mock var in `summarizer.go` named `execSummarizerContext` (not `execCommandContext` — that name is already taken by `collector.go` in the same package).
- New TestHelper function named `TestHelperProcessSummarizer` (not `TestHelperProcess` — already taken in `collector_test.go`).
- Errors wrapped: `fmt.Errorf("context: %w", err)`.
- No new files beyond: `internal/plan/summarizer.go`, `internal/plan/summarizer_test.go`.

---

## File Map

| File | Change |
|------|--------|
| `internal/config/defaults.go` | Add `DefaultJSONOutDir`, `DefaultPlanSummaryEnabled` |
| `internal/executor/executor.go` | Add `appendPlanTerragruntOutputFlags`; call it in `buildTerragruntArgs` before `--` |
| `internal/executor/executor_test.go` | Add `TestBuildTerragruntArgs_PlanSummaryEnabled` |
| `internal/plan/summarizer.go` | NEW — `Summarize(ctx, dir) (int, error)` |
| `internal/plan/summarizer_test.go` | NEW — tests with mocked `tf-summarize` |
| `cmd/root.go` | Add `runPlanSummary`; `viper.SetDefault`; 3 dispatch site branches |
| `.terrax.yaml` | Document `plan.summary_enabled` |

---

### Task 1: Add constants to config/defaults.go

**Files:**
- Modify: `internal/config/defaults.go`

**Interfaces:**
- Produces: `config.DefaultJSONOutDir = "./tmp/json-plans"` and `config.DefaultPlanSummaryEnabled = false` — consumed by executor (Task 2), summarizer (Task 3), and root.go (Task 4).

- [ ] **Step 1: Add two constants after `DefaultPlanReviewEnabled`**

In `internal/config/defaults.go`, add after the `DefaultPlanReviewEnabled` line:

```go
	// DefaultJSONOutDir is the default output directory for Terragrunt JSON plan files.
	DefaultJSONOutDir = "./tmp/json-plans"

	// DefaultPlanSummaryEnabled controls whether the terminal plan summary is shown after plan execution.
	DefaultPlanSummaryEnabled = false
```

- [ ] **Step 2: Build**

```bash
go build ./internal/config/...
```

Expected: no output.

- [ ] **Step 3: Commit**

```bash
git add internal/config/defaults.go
git commit -m "feat(config): add DefaultJSONOutDir and DefaultPlanSummaryEnabled constants"
```

---

### Task 2: Add appendPlanTerragruntOutputFlags to executor (TDD)

**Files:**
- Modify: `internal/executor/executor.go`
- Modify: `internal/executor/executor_test.go`

**Interfaces:**
- Consumes: `config.DefaultJSONOutDir` from Task 1.
- Produces: `appendPlanTerragruntOutputFlags(args []string, command string) []string` — called in `buildTerragruntArgs` before the `--` separator.

- [ ] **Step 1: Write the failing test**

Append to `internal/executor/executor_test.go`:

```go
// TestBuildTerragruntArgs_PlanSummaryEnabled tests plan.summary_enabled via buildTerragruntArgs.
func TestBuildTerragruntArgs_PlanSummaryEnabled(t *testing.T) {
	tests := []struct {
		name           string
		stackPath      string
		command        string
		summaryEnabled bool
		expected       []string
	}{
		{
			name:           "summary enabled injects --json-out-dir before separator",
			stackPath:      "/path/to/stack",
			command:        "plan",
			summaryEnabled: true,
			expected:       []string{"run", "--all", "--working-dir", "/path/to/stack", "--log-format", "pretty", "--json-out-dir=./tmp/json-plans", "--", "plan"},
		},
		{
			name:           "summary disabled produces no --json-out-dir",
			stackPath:      "/path/to/stack",
			command:        "plan",
			summaryEnabled: false,
			expected:       []string{"run", "--all", "--working-dir", "/path/to/stack", "--log-format", "pretty", "--", "plan"},
		},
		{
			name:           "summary enabled ignored for non-plan commands",
			stackPath:      "/path/to/stack",
			command:        "apply",
			summaryEnabled: true,
			expected:       []string{"run", "--all", "--working-dir", "/path/to/stack", "--log-format", "pretty", "--", "apply"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resetViper()
			viper.Set("log_format", "pretty")
			viper.Set("plan.summary_enabled", tt.summaryEnabled)
			viper.Set("plan.review_enabled", false) // isolate from binary -out= injection

			args := buildTerragruntArgs(tt.stackPath, tt.command)

			assert.Equal(t, tt.expected, args, "Arguments should match expected output.")
		})
	}
}
```

- [ ] **Step 2: Run test to confirm it fails**

```bash
go test ./internal/executor/... -run TestBuildTerragruntArgs_PlanSummaryEnabled -v
```

Expected: FAIL — `appendPlanTerragruntOutputFlags` not defined yet.

- [ ] **Step 3: Add the helper and wire it into buildTerragruntArgs**

In `internal/executor/executor.go`, replace:

```go
	args = appendCommandTerragruntFlags(args, command)

	args = append(args, "--", command)
```

With:

```go
	args = appendCommandTerragruntFlags(args, command)
	args = appendPlanTerragruntOutputFlags(args, command)

	args = append(args, "--", command)
```

Then add the new function after `appendCommandTerragruntFlags`:

```go
// appendPlanTerragruntOutputFlags injects --json-out-dir for plan commands when summary mode is enabled.
// This flag must appear before the -- separator so Terragrunt processes it.
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

- [ ] **Step 4: Run all executor tests**

```bash
go test ./internal/executor/... -v
```

Expected: all tests pass including `TestBuildTerragruntArgs_PlanSummaryEnabled` and all pre-existing tests.

- [ ] **Step 5: Commit**

```bash
git add internal/executor/executor.go internal/executor/executor_test.go
git commit -m "feat(executor): inject --json-out-dir when plan.summary_enabled is true"
```

---

### Task 3: Create internal/plan/summarizer.go (TDD)

**Files:**
- Create: `internal/plan/summarizer.go`
- Create: `internal/plan/summarizer_test.go`

**Interfaces:**
- Consumes: nothing from prior tasks at the Go level (reads from filesystem and runs `tf-summarize`).
- Produces: `plan.Summarize(ctx context.Context, dir string) (int, error)` — consumed by `runPlanSummary` in Task 4.

- [ ] **Step 1: Write the failing tests**

Create `internal/plan/summarizer_test.go`:

```go
package plan

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// summarizer mock controls
var (
	summarizerMockStdout string
	summarizerMockExit   int
)

// fakeExecSummarizer returns a *exec.Cmd running TestHelperProcessSummarizer.
func fakeExecSummarizer(ctx context.Context, command string, args ...string) *exec.Cmd {
	cs := []string{"-test.run=TestHelperProcessSummarizer", "--", command}
	cs = append(cs, args...)
	cmd := exec.Command(os.Args[0], cs...)
	cmd.Env = []string{
		"GO_WANT_HELPER_PROCESS=1",
		fmt.Sprintf("GO_SUMMARIZER_STDOUT=%s", summarizerMockStdout),
		fmt.Sprintf("GO_SUMMARIZER_EXIT=%d", summarizerMockExit),
	}
	return cmd
}

// TestHelperProcessSummarizer is invoked as a subprocess by fakeExecSummarizer.
func TestHelperProcessSummarizer(t *testing.T) {
	if os.Getenv("GO_WANT_HELPER_PROCESS") != "1" {
		return
	}
	fmt.Print(os.Getenv("GO_SUMMARIZER_STDOUT"))
	if code := os.Getenv("GO_SUMMARIZER_EXIT"); code != "" && code != "0" {
		exitCode, _ := strconv.Atoi(code)
		os.Exit(exitCode)
	}
	os.Exit(0)
}

func TestSummarize_DirectoryNotExist(t *testing.T) {
	count, err := Summarize(context.Background(), "/nonexistent/path/xyz")
	require.NoError(t, err)
	assert.Equal(t, 0, count)
}

func TestSummarize_EmptyDirectory(t *testing.T) {
	dir := t.TempDir()
	count, err := Summarize(context.Background(), dir)
	require.NoError(t, err)
	assert.Equal(t, 0, count)
}

func TestSummarize_TFSummarizeNotFound(t *testing.T) {
	oldExec := execSummarizerContext
	execSummarizerContext = func(ctx context.Context, name string, args ...string) *exec.Cmd {
		// Running a nonexistent binary triggers exec.ErrNotFound.
		return exec.CommandContext(ctx, "tf-summarize-not-installed-xyz123")
	}
	defer func() { execSummarizerContext = oldExec }()

	dir := t.TempDir()
	stackDir := filepath.Join(dir, "workloads", "dev", "acm")
	require.NoError(t, os.MkdirAll(stackDir, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(stackDir, "plan.json"), []byte("{}"), 0644))

	_, err := Summarize(context.Background(), dir)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "tf-summarize not found")
}

func TestSummarize_StackWithChanges(t *testing.T) {
	oldExec := execSummarizerContext
	execSummarizerContext = fakeExecSummarizer
	defer func() { execSummarizerContext = oldExec }()

	summarizerMockStdout = `{"changes":{"add":2,"update":1,"delete":0,"recreate":0,"import":0,"moved":0}}`
	summarizerMockExit = 0

	dir := t.TempDir()
	stackDir := filepath.Join(dir, "workloads", "dev", "acm")
	require.NoError(t, os.MkdirAll(stackDir, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(stackDir, "plan.json"), []byte("{}"), 0644))

	count, err := Summarize(context.Background(), dir)
	require.NoError(t, err)
	assert.Equal(t, 1, count)
}

func TestSummarize_StackNoChanges(t *testing.T) {
	oldExec := execSummarizerContext
	execSummarizerContext = fakeExecSummarizer
	defer func() { execSummarizerContext = oldExec }()

	summarizerMockStdout = `{"changes":{"add":0,"update":0,"delete":0,"recreate":0,"import":0,"moved":0}}`
	summarizerMockExit = 0

	dir := t.TempDir()
	stackDir := filepath.Join(dir, "workloads", "dev", "acm")
	require.NoError(t, os.MkdirAll(stackDir, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(stackDir, "plan.json"), []byte("{}"), 0644))

	count, err := Summarize(context.Background(), dir)
	require.NoError(t, err)
	assert.Equal(t, 0, count)
}

func TestSummarize_MultipleStacksPartialChanges(t *testing.T) {
	oldExec := execSummarizerContext
	execSummarizerContext = fakeExecSummarizer
	defer func() { execSummarizerContext = oldExec }()

	// First call has changes, second does not (both use same mock stdout — fine for count test).
	summarizerMockStdout = `{"changes":{"add":1,"update":0,"delete":0,"recreate":0,"import":0,"moved":0}}`
	summarizerMockExit = 0

	dir := t.TempDir()
	for _, stack := range []string{"workloads/dev/acm", "workloads/dev/ecr"} {
		stackDir := filepath.Join(dir, filepath.FromSlash(stack))
		require.NoError(t, os.MkdirAll(stackDir, 0755))
		require.NoError(t, os.WriteFile(filepath.Join(stackDir, "plan.json"), []byte("{}"), 0644))
	}

	count, err := Summarize(context.Background(), dir)
	require.NoError(t, err)
	assert.Equal(t, 2, count) // both stacks have add:1
}
```

Note: `TestHelperProcessSummarizer` uses `strconv.Atoi` — add `"strconv"` to the import block.

- [ ] **Step 2: Run tests to confirm they fail**

```bash
go test ./internal/plan/... -run "TestSummarize" -v
```

Expected: compilation error — `Summarize` and `execSummarizerContext` not defined yet.

- [ ] **Step 3: Implement summarizer.go**

Create `internal/plan/summarizer.go`:

```go
// Package plan provides plan analysis utilities for TerraX.
package plan

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
)

// execSummarizerContext allows mocking exec.CommandContext in tests.
// Named separately from execCommandContext (used by collector.go) to avoid collision.
var execSummarizerContext = exec.CommandContext

// tfSummarizeJSON represents the output of tf-summarize -json-sum.
type tfSummarizeJSON struct {
	Changes struct {
		Add      int `json:"add"`
		Update   int `json:"update"`
		Delete   int `json:"delete"`
		Recreate int `json:"recreate"`
		Import   int `json:"import"`
		Moved    int `json:"moved"`
	} `json:"changes"`
}

// Summarize scans dir for JSON plan files, prints a count line per stack via
// tf-summarize -json-sum, and returns the number of stacks with changes.
// Returns (0, nil) when dir does not exist or contains no JSON files.
// Returns (0, error) when tf-summarize is not installed.
func Summarize(ctx context.Context, dir string) (int, error) {
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		return 0, nil
	}

	var jsonFiles []string
	err := filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil // Skip inaccessible paths.
		}
		if !d.IsDir() && strings.HasSuffix(d.Name(), ".json") {
			jsonFiles = append(jsonFiles, path)
		}
		return nil
	})
	if err != nil {
		return 0, fmt.Errorf("failed to scan plan directory: %w", err)
	}

	if len(jsonFiles) == 0 {
		return 0, nil
	}

	sort.Strings(jsonFiles)
	fmt.Printf("🔍 Scanning %d JSON plan(s)...\n\n", len(jsonFiles))

	changedCount := 0
	for _, planFile := range jsonFiles {
		rel, _ := filepath.Rel(dir, planFile)
		stackName := filepath.ToSlash(filepath.Dir(rel))

		cmd := execSummarizerContext(ctx, "tf-summarize", "-json-sum", planFile)
		output, err := cmd.Output()
		if err != nil {
			// Distinguish "not installed" from per-file failure.
			var execErr *exec.Error
			if errors.As(err, &execErr) && errors.Is(execErr.Err, exec.ErrNotFound) {
				return 0, fmt.Errorf("tf-summarize not found: install from https://github.com/dineshba/tf-summarize")
			}
			fmt.Fprintf(os.Stderr, "Warning: could not summarize %s\n", stackName)
			continue
		}

		if len(output) == 0 {
			fmt.Fprintf(os.Stderr, "Warning: could not summarize %s\n", stackName)
			continue
		}

		var summary tfSummarizeJSON
		if err := json.Unmarshal(output, &summary); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: could not summarize %s\n", stackName)
			continue
		}

		c := summary.Changes
		fmt.Printf("  %s: +%d ~%d -%d ♻%d\n", stackName, c.Add, c.Update, c.Delete, c.Recreate)

		total := c.Add + c.Update + c.Delete + c.Recreate + c.Import + c.Moved
		if total > 0 {
			changedCount++
		}
	}

	fmt.Println()
	if changedCount > 0 {
		fmt.Printf("%d stack(s) with pending changes\n", changedCount)
	} else {
		fmt.Printf("No changes detected across %d stack(s)\n", len(jsonFiles))
	}

	return changedCount, nil
}
```

- [ ] **Step 4: Run all plan tests**

```bash
go test ./internal/plan/... -v
```

Expected: all tests pass including the new `TestSummarize_*` cases and all pre-existing plan tests.

- [ ] **Step 5: Run full suite**

```bash
go test ./...
```

Expected: all packages pass.

- [ ] **Step 6: Commit**

```bash
git add internal/plan/summarizer.go internal/plan/summarizer_test.go
git commit -m "feat(plan): add Summarize function using tf-summarize for terminal plan summary"
```

---

### Task 4: Wire runPlanSummary in cmd/root.go + document .terrax.yaml

**Files:**
- Modify: `cmd/root.go`
- Modify: `.terrax.yaml`

**Interfaces:**
- Consumes: `plan.Summarize` from Task 3, `config.DefaultJSONOutDir` and `config.DefaultPlanSummaryEnabled` from Task 1.
- Produces: nothing consumed by other tasks — terminal wiring.

- [ ] **Step 1: Add viper default for plan.summary_enabled in initConfig**

In `cmd/root.go`, inside `initConfig()`, after the `plan.review_enabled` default line:

```go
	viper.SetDefault("plan.summary_enabled", config.DefaultPlanSummaryEnabled)
```

- [ ] **Step 2: Add runPlanSummary helper**

Add before `runPlanReview` in `cmd/root.go`:

```go
// runPlanSummary reads JSON plan files from the default output directory and
// prints a one-line count summary per stack via tf-summarize.
func runPlanSummary(ctx context.Context) error {
	fmt.Println()
	_, err := plan.Summarize(ctx, config.DefaultJSONOutDir)
	return err
}
```

- [ ] **Step 3: Add summary branch at all three dispatch sites**

**Site 1 — normal TUI** (after `executor.Run`, before `review_enabled` check):

Replace:

```go
		if command == "plan" && viper.GetBool("plan.review_enabled") {
			return runPlanReview(ctx, stackPath)
		}
```

With:

```go
		if command == "plan" && viper.GetBool("plan.summary_enabled") {
			if err := runPlanSummary(ctx); err != nil {
				fmt.Fprintf(os.Stderr, "Warning: plan summary failed: %v\n", err)
			}
		}
		if command == "plan" && viper.GetBool("plan.review_enabled") {
			return runPlanReview(ctx, stackPath)
		}
```

**Site 2 — executeLastCommand** (same pattern, uses `lastEntry.Command` and `absolutePath`):

Replace:

```go
	if lastEntry.Command == "plan" && viper.GetBool("plan.review_enabled") {
		return runPlanReview(ctx, absolutePath)
	}
```

With:

```go
	if lastEntry.Command == "plan" && viper.GetBool("plan.summary_enabled") {
		if err := runPlanSummary(ctx); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: plan summary failed: %v\n", err)
		}
	}
	if lastEntry.Command == "plan" && viper.GetBool("plan.review_enabled") {
		return runPlanReview(ctx, absolutePath)
	}
```

**Site 3 — runHistoryViewer re-execute** (same pattern, uses `entry.Command` and `absolutePath`):

Replace:

```go
			if entry.Command == "plan" && viper.GetBool("plan.review_enabled") {
				return runPlanReview(ctx, absolutePath)
			}
```

With:

```go
			if entry.Command == "plan" && viper.GetBool("plan.summary_enabled") {
				if err := runPlanSummary(ctx); err != nil {
					fmt.Fprintf(os.Stderr, "Warning: plan summary failed: %v\n", err)
				}
			}
			if entry.Command == "plan" && viper.GetBool("plan.review_enabled") {
				return runPlanReview(ctx, absolutePath)
			}
```

- [ ] **Step 4: Build**

```bash
go build ./...
```

Expected: no output.

- [ ] **Step 5: Run full test suite**

```bash
go test ./...
```

Expected: all packages pass.

- [ ] **Step 6: Document in .terrax.yaml**

In the `plan:` section of `.terrax.yaml`, add `summary_enabled` after `review_enabled`:

```yaml
plan:
  review_enabled: false   # Enable or disable plan file scanning and review TUI after running plan
  # Enable terminal summary mode via tf-summarize after plan execution.
  # When enabled, --json-out-dir=./tmp/json-plans is automatically injected and
  # tf-summarize prints a count line per stack.
  # Requires tf-summarize: https://github.com/dineshba/tf-summarize
  # Default: false
  # summary_enabled: true
```

- [ ] **Step 7: Commit**

```bash
git add cmd/root.go .terrax.yaml
git commit -m "feat(cmd): wire plan.summary_enabled with runPlanSummary at all dispatch sites"
```

---

## Self-Review

**Spec coverage:**

| Spec requirement | Task |
|---|---|
| `DefaultJSONOutDir = "./tmp/json-plans"` | Task 1 |
| `DefaultPlanSummaryEnabled = false` | Task 1 |
| `appendPlanTerragruntOutputFlags` injects `--json-out-dir` when `summary_enabled` | Task 2 |
| `Summarize(ctx, dir) (int, error)` | Task 3 |
| Returns `(0, nil)` for nonexistent dir | Task 3 |
| Returns error for `tf-summarize` not found | Task 3 |
| Warns and continues on per-file failure | Task 3 |
| Prints `+add ~update -delete ♻recreate` per stack | Task 3 |
| Prints final "N stack(s) with pending changes" | Task 3 |
| `viper.SetDefault("plan.summary_enabled", ...)` | Task 4 |
| `runPlanSummary` non-fatal (warn on error) | Task 4 |
| 3 dispatch sites updated | Task 4 |
| `.terrax.yaml` documented | Task 4 |
| Summary errors are warnings, not fatal | Task 4 |

**No placeholders found.**

**Type consistency:** `plan.Summarize` defined in Task 3 as `(ctx context.Context, dir string) (int, error)`; called in Task 4 as `plan.Summarize(ctx, config.DefaultJSONOutDir)` — consistent. `config.DefaultJSONOutDir` defined in Task 1, used in Tasks 2 and 4 — consistent.
