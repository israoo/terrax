# CLI Subcommand Reorganization Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Promote `--last`, `--history`, and `--review` flags to proper subcommands so flags only modify behavior, and convert `terrax history` to an interactive TUI with `--json` opt-in for external tool consumption.

**Architecture:** Each flag becomes its own `cmd/*.go` file in `package cmd`. Functions stay in the same package so they share helpers (`getHistoryService`, `runPlanReview`, etc.) without import cycles. `runTUI` in `root.go` sheds all flag-dispatching branches and becomes a pure TUI launcher. `runHistoryViewer` moves from `root.go` to `history.go`.

**Tech Stack:** Go · Cobra · Viper · Bubble Tea · testify

## Global Constraints

- All files in `package cmd` — no new packages.
- Three import groups, alphabetically sorted: stdlib · third-party · `github.com/israoo/terrax/...`.
- All comments end with periods.
- Errors wrapped with `fmt.Errorf("context: %w", err)`.
- No `--dir` flag on `terrax last` (execution path comes from stored history).
- `runPlanReview` stays in `root.go` (shared by post-plan flow and `terrax review`).
- Run `task check` before every commit.

---

## File Map

| File | Action | Responsibility |
|---|---|---|
| `cmd/last.go` | Create | `terrax last` subcommand + `executeLastCommand` (moved from root) |
| `cmd/review.go` | Create | `terrax review` subcommand |
| `cmd/history.go` | Modify | Add `--json` flag; add TUI path; move `runHistoryViewer` from root |
| `cmd/root.go` | Modify | Remove `--last`, `--history`, `--review` flags and their branches from `runTUI` |
| `extensions/vscode/src/historyProvider.ts` | Modify | Call `terrax history --json` instead of `terrax history` |

---

### Task 1: Create `terrax last` subcommand

**Files:**
- Create: `cmd/last.go`
- Create: `cmd/last_test.go`
- Modify: `cmd/root.go` (remove `--last` flag + branch, remove `executeLastCommand`)

**Interfaces:**
- Consumes: `getHistoryService() (*history.Service, error)` from `root.go`; `executeLastCommand(ctx, svc)` moved here.
- Produces: `runLastCmd(cmd *cobra.Command, args []string) error` — called by Cobra.

- [ ] **Step 1: Write the failing test**

Create `cmd/last_test.go`:

```go
package cmd

import (
	"bytes"
	"io"
	"os"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLastCmd_NoHistory(t *testing.T) {
	// Capture stdout.
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
	err := runLastCmd(cmd, []string{})

	_ = w.Close()
	os.Stdout = oldStdout
	output := <-done

	require.NoError(t, err)
	assert.Contains(t, output, "No execution history found")
}
```

- [ ] **Step 2: Run test to verify it fails**

```bash
cd /path/to/terrax && go test ./cmd/ -run TestLastCmd_NoHistory -v
```

Expected: FAIL — `runLastCmd` undefined.

- [ ] **Step 3: Create `cmd/last.go` and move `executeLastCommand` from `root.go`**

Create `cmd/last.go` with the full content below. Then **delete** `executeLastCommand` from `root.go` (lines 349–410 in the current file).

```go
package cmd

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/israoo/terrax/internal/config"
	"github.com/israoo/terrax/internal/executor"
)

var lastCmd = &cobra.Command{
	Use:   "last",
	Short: "Re-execute the last command from history",
	Long:  `Re-execute the most recent command from the execution history for the current project.`,
	RunE:  runLastCmd,
}

func init() {
	rootCmd.AddCommand(lastCmd)
}

func runLastCmd(cmd *cobra.Command, args []string) error {
	ctx := context.Background()

	historyService, err := getHistoryService()
	if err != nil {
		return err
	}

	return executeLastCommand(ctx, historyService)
}

// executeLastCommand retrieves and executes the most recent command from history for the current project.
func executeLastCommand(ctx context.Context, historyService *historyService_iface) error {
	lastEntry, err := historyService.GetLastExecutionForProject(ctx)
	if err != nil {
		return fmt.Errorf("failed to get last execution: %w", err)
	}

	if lastEntry == nil {
		fmt.Println("⚠️  No execution history found for this project")
		fmt.Println("Run terrax interactively first to build history")
		return nil
	}

	fmt.Println("═══════════════════════════════════════")
	fmt.Println("  🔄 Re-executing last command")
	fmt.Println("═══════════════════════════════════════")
	fmt.Printf("Command:    %s\n", lastEntry.Command)
	fmt.Printf("Stack Path: %s\n", lastEntry.StackPath)
	fmt.Printf("Previous:   %s (exit code: %d)\n", lastEntry.Timestamp.Format("2006-01-02 15:04:05"), lastEntry.ExitCode)
	fmt.Println("═══════════════════════════════════════")
	fmt.Println()

	absolutePath := lastEntry.AbsolutePath
	if absolutePath == "" {
		absolutePath = lastEntry.StackPath
	}

	if lastEntry.Command == "force-unlock" {
		return runForceUnlock(ctx, historyService, absolutePath)
	}

	repoRoot, filterPaths := collectTransitiveDeps(absolutePath)

	if lastEntry.Command == "plan" && (viper.GetBool("plan.summary_enabled") || viper.GetBool("plan.review_enabled")) {
		_ = os.RemoveAll(filepath.Join(repoRoot, config.DefaultOutputDir))
	}

	groups, err := buildGroupedExecution(filterPaths, repoRoot)
	if err != nil {
		return fmt.Errorf("failed to build group execution plan: %w", err)
	}
	for _, group := range groups {
		if group.Skip {
			continue
		}
		if err := executor.Run(ctx, historyService, lastEntry.Command, absolutePath, repoRoot, group.Paths, group.EnvVars); err != nil {
			return err
		}
	}
	if lastEntry.Command == "plan" && viper.GetBool("plan.summary_enabled") {
		if err := runPlanSummary(ctx, absolutePath, repoRoot); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: plan summary failed: %v\n", err)
		}
	}
	if lastEntry.Command == "plan" && viper.GetBool("plan.review_enabled") {
		return runPlanReview(ctx, absolutePath)
	}

	return nil
}
```

> **Note:** The `historyService_iface` placeholder above is wrong — use the concrete `*history.Service` type, same as in the original `root.go`. Copy the exact function body from `root.go:349-410` verbatim; do not paraphrase. Add the missing imports (`history`, `executor`, etc.) matching the three-group convention.

The actual signature is:
```go
func executeLastCommand(ctx context.Context, historyService *history.Service) error {
```

Imports for `cmd/last.go`:
```go
import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/israoo/terrax/internal/config"
	"github.com/israoo/terrax/internal/executor"
	"github.com/israoo/terrax/internal/history"
)
```

- [ ] **Step 4: Remove `--last` flag and its branch from `root.go`**

In `cmd/root.go`:

Remove from `init()`:
```go
rootCmd.Flags().BoolP("last", "l", false, "Execute the last command from history")
```

Remove from `runTUI()`:
```go
lastFlag, _ := cmd.Flags().GetBool("last")
if lastFlag {
    return executeLastCommand(ctx, historyService)
}
```

Also remove the `historyService` initialization from `runTUI` if it is no longer used there (check after also completing Tasks 2 and 3).

- [ ] **Step 5: Run tests**

```bash
task check
```

Expected: all tests pass, no compile errors.

- [ ] **Step 6: Commit**

```bash
git add cmd/last.go cmd/last_test.go cmd/root.go
git commit -m "feat: promote --last flag to terrax last subcommand"
```

---

### Task 2: Create `terrax review` subcommand

**Files:**
- Create: `cmd/review.go`
- Create: `cmd/review_test.go`
- Modify: `cmd/root.go` (remove `--review` flag + branch from `runTUI`)

**Interfaces:**
- Consumes: `runPlanReview(ctx context.Context, stackPath string) error` from `root.go`; `getWorkingDirectory(dir string) (string, error)` from `root.go`.
- Produces: `runReviewCmd(cmd *cobra.Command, args []string) error`.

- [ ] **Step 1: Write the failing test**

Create `cmd/review_test.go`:

```go
package cmd

import (
	"os"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestReviewCmd_NoPlanDir(t *testing.T) {
	tmpDir := t.TempDir()

	cmd := &cobra.Command{}
	cmd.Flags().String("dir", "", "")
	require.NoError(t, cmd.ParseFlags([]string{"--dir", tmpDir}))

	err := runReviewCmd(cmd, []string{})

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no plan results found")
}

func TestReviewCmd_UsesCwd(t *testing.T) {
	// When --dir is not set, runReviewCmd uses the current working directory.
	// A fresh temp dir has no plan output, so the same error is expected.
	originalWd, err := os.Getwd()
	require.NoError(t, err)
	t.Cleanup(func() { _ = os.Chdir(originalWd) })

	tmpDir := t.TempDir()
	require.NoError(t, os.Chdir(tmpDir))

	cmd := &cobra.Command{}
	cmd.Flags().String("dir", "", "")
	require.NoError(t, cmd.ParseFlags([]string{}))

	err = runReviewCmd(cmd, []string{})
	assert.Error(t, err)
}
```

- [ ] **Step 2: Run test to verify it fails**

```bash
go test ./cmd/ -run TestReviewCmd -v
```

Expected: FAIL — `runReviewCmd` undefined.

- [ ] **Step 3: Create `cmd/review.go`**

```go
package cmd

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"
)

var reviewCmd = &cobra.Command{
	Use:   "review",
	Short: "Open the plan review TUI from the last plan execution",
	Long:  `Open the plan review TUI without re-running the plan. Reads plan output from .terrax/plans/.`,
	RunE:  runReviewCmd,
}

func init() {
	reviewCmd.Flags().String("dir", "", "Working directory (overrides current directory)")
	rootCmd.AddCommand(reviewCmd)
}

func runReviewCmd(cmd *cobra.Command, args []string) error {
	ctx := context.Background()

	dirFlag, _ := cmd.Flags().GetString("dir")
	workDir, err := getWorkingDirectory(dirFlag)
	if err != nil {
		return fmt.Errorf("failed to get working directory: %w", err)
	}

	ensureConfigFromWorkDir(workDir)

	return runPlanReview(ctx, workDir)
}
```

- [ ] **Step 4: Remove `--review` flag and its branch from `root.go`**

Remove from `init()`:
```go
rootCmd.Flags().BoolP("review", "r", false, "Open the plan review TUI from the last plan execution without re-running")
```

Remove from `runTUI()`:
```go
reviewFlag, _ := cmd.Flags().GetBool("review")
```
And further down:
```go
if reviewFlag {
    return runPlanReview(ctx, workDir)
}
```

- [ ] **Step 5: Run tests**

```bash
task check
```

Expected: all tests pass.

- [ ] **Step 6: Commit**

```bash
git add cmd/review.go cmd/review_test.go cmd/root.go
git commit -m "feat: promote --review flag to terrax review subcommand"
```

---

### Task 3: Refactor `terrax history` — add `--json` flag and TUI mode

**Files:**
- Modify: `cmd/history.go` (add `--json` flag, add TUI path, move `runHistoryViewer` from root)
- Modify: `cmd/history_test.go` (add test for TUI/JSON routing)
- Modify: `cmd/root.go` (remove `--history` flag + branch; remove `runHistoryViewer`; remove `historyService` init if now unused in `runTUI`)

**Interfaces:**
- Consumes: `tui.NewHistoryModel`, `tui.Model` from `internal/tui`; `history.Service` from `internal/history`.
- Produces: `runHistoryCmdJSON` (existing behavior, renamed internally); `runHistoryCmdTUI` (new, from moved `runHistoryViewer`).

- [ ] **Step 1: Write the failing test**

Add to `cmd/history_test.go` (after the existing `TestHistoryCommand_OutputsValidJSON`):

```go
func TestHistoryCommand_JSONFlag_OutputsJSON(t *testing.T) {
	tmpDir := t.TempDir()

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
	cmd.Flags().Bool("json", false, "")
	_ = cmd.ParseFlags([]string{"--dir", tmpDir, "--json"})

	err := runHistoryCmd(cmd, []string{})

	_ = w.Close()
	os.Stdout = oldStdout
	output := <-done

	require.NoError(t, err)

	var entries []map[string]interface{}
	require.NoError(t,
		json.Unmarshal([]byte(output), &entries),
		"output with --json must be a valid JSON array, got: %s", output,
	)
}

func TestHistoryCommand_NoJSONFlag_DoesNotOutputJSON(t *testing.T) {
	// Without --json the command routes to the TUI path; since there is no
	// terminal in tests, we only verify that runHistoryCmd does not write
	// JSON to stdout (it writes nothing or writes to stderr).
	tmpDir := t.TempDir()

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
	cmd.Flags().Bool("json", false, "")
	_ = cmd.ParseFlags([]string{"--dir", tmpDir})

	// The TUI will fail without a real terminal — that is expected in tests.
	// We only assert that stdout does not contain a JSON array.
	_ = runHistoryCmd(cmd, []string{})

	_ = w.Close()
	os.Stdout = oldStdout
	output := <-done

	var entries []map[string]interface{}
	assert.Error(t, json.Unmarshal([]byte(output), &entries),
		"without --json the output must not be a JSON array")
}
```

- [ ] **Step 2: Run tests to verify they fail**

```bash
go test ./cmd/ -run TestHistoryCommand -v
```

Expected: the two new tests FAIL (flag `--json` not registered yet).

- [ ] **Step 3: Refactor `cmd/history.go`**

Replace the file content with:

```go
package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/israoo/terrax/internal/config"
	"github.com/israoo/terrax/internal/deps"
	"github.com/israoo/terrax/internal/executor"
	"github.com/israoo/terrax/internal/history"
	"github.com/israoo/terrax/internal/tui"
)

var historyCmd = &cobra.Command{
	Use:   "history",
	Short: "View or export command execution history",
	Long: `Without --json: opens the interactive TUI history viewer.
With --json: prints history for the current project as a JSON array for external tools such as editor extensions.`,
	RunE: runHistoryCmd,
}

func init() {
	historyCmd.Flags().String("dir", "", "Working directory (overrides current directory)")
	historyCmd.Flags().Bool("json", false, "Print history as JSON instead of opening the interactive TUI")
	rootCmd.AddCommand(historyCmd)
}

func runHistoryCmd(cmd *cobra.Command, args []string) error {
	jsonFlag, _ := cmd.Flags().GetBool("json")
	if jsonFlag {
		return runHistoryCmdJSON(cmd, args)
	}
	return runHistoryCmdTUI(cmd, args)
}

// runHistoryCmdJSON prints execution history for the current project as JSON.
// This is the original behavior of the history subcommand, used by external tools.
func runHistoryCmdJSON(cmd *cobra.Command, args []string) error {
	ctx := context.Background()

	dirFlag, _ := cmd.Flags().GetString("dir")
	workDir, err := getWorkingDirectory(dirFlag)
	if err != nil {
		return fmt.Errorf("failed to get working directory: %w", err)
	}

	historyService, err := getHistoryService()
	if err != nil {
		return fmt.Errorf("failed to initialize history service: %w", err)
	}

	entries, err := historyService.LoadAll(ctx)
	if err != nil {
		return fmt.Errorf("failed to load history: %w", err)
	}

	rootConfigFile := viper.GetString("root_config_file")
	if rootConfigFile == "" {
		rootConfigFile = config.DefaultRootConfigFile
	}

	repoRoot := deps.FindRepoRoot(workDir, rootConfigFile)
	if _, err := os.Stat(filepath.Join(repoRoot, rootConfigFile)); err != nil {
		if _, err := fmt.Fprintln(os.Stdout, "[]"); err != nil {
			return fmt.Errorf("failed to write output: %w", err)
		}
		return nil
	}

	originalDir, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get current directory: %w", err)
	}
	defer func() { _ = os.Chdir(originalDir) }()

	if err := os.Chdir(workDir); err != nil {
		return fmt.Errorf("failed to change directory: %w", err)
	}

	filtered, err := historyService.FilterByCurrentProject(entries)
	if err != nil {
		return fmt.Errorf("failed to filter history: %w", err)
	}

	if filtered == nil {
		filtered = []history.ExecutionLogEntry{}
	}

	data, err := json.Marshal(filtered)
	if err != nil {
		return fmt.Errorf("failed to serialize history: %w", err)
	}

	if _, err := fmt.Fprintln(os.Stdout, string(data)); err != nil {
		return fmt.Errorf("failed to write output: %w", err)
	}
	return nil
}

// runHistoryCmdTUI loads and displays the execution history in an interactive TUI.
// It filters the history to show only entries from the current project.
// If the user selects an entry and presses Enter, it re-executes that command.
func runHistoryCmdTUI(cmd *cobra.Command, args []string) error {
	ctx := context.Background()

	historyService, err := getHistoryService()
	if err != nil {
		return err
	}

	entries, err := historyService.LoadAll(ctx)
	if err != nil {
		return fmt.Errorf("failed to load history: %w", err)
	}

	filteredEntries, err := historyService.FilterByCurrentProject(entries)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Warning: Failed to filter history: %v\n", err)
		filteredEntries = entries
	}

	initialModel := tui.NewHistoryModel(filteredEntries)

	p := tea.NewProgram(
		initialModel,
		tea.WithAltScreen(),
		tea.WithOutput(os.Stderr),
	)

	finalModel, err := p.Run()
	if err != nil {
		return fmt.Errorf("history viewer error: %w", err)
	}

	model, ok := finalModel.(tui.Model)
	if !ok {
		return fmt.Errorf("unexpected model type")
	}

	if model.ShouldReExecuteFromHistory() {
		entry := model.GetSelectedHistoryEntry()
		if entry != nil {
			fmt.Fprintf(os.Stderr, "\n🔄 Re-executing command from history...\n")
			fmt.Fprintf(os.Stderr, "Command: %s\n", entry.Command)
			fmt.Fprintf(os.Stderr, "Path: %s\n\n", entry.StackPath)

			absolutePath := entry.AbsolutePath
			if absolutePath == "" {
				absolutePath = entry.StackPath
			}

			if entry.Command == "force-unlock" {
				return runForceUnlock(ctx, historyService, absolutePath)
			}

			repoRoot, filterPaths := collectTransitiveDeps(absolutePath)

			if entry.Command == "plan" && (viper.GetBool("plan.summary_enabled") || viper.GetBool("plan.review_enabled")) {
				_ = os.RemoveAll(filepath.Join(repoRoot, config.DefaultOutputDir))
			}

			groups, err := buildGroupedExecution(filterPaths, repoRoot)
			if err != nil {
				return fmt.Errorf("failed to build group execution plan: %w", err)
			}
			for _, group := range groups {
				if group.Skip {
					continue
				}
				if err := executor.Run(ctx, historyService, entry.Command, absolutePath, repoRoot, group.Paths, group.EnvVars); err != nil {
					return err
				}
			}
			if entry.Command == "plan" && viper.GetBool("plan.summary_enabled") {
				if err := runPlanSummary(ctx, absolutePath, repoRoot); err != nil {
					fmt.Fprintf(os.Stderr, "Warning: plan summary failed: %v\n", err)
				}
			}
			if entry.Command == "plan" && viper.GetBool("plan.review_enabled") {
				return runPlanReview(ctx, absolutePath)
			}
		}
	}

	return nil
}
```

- [ ] **Step 4: Remove `--history` flag, `runHistoryViewer`, and `historyService` init from `root.go`**

In `cmd/root.go`:

Remove from `init()`:
```go
rootCmd.Flags().Bool("history", false, "View execution history interactively")
```

Remove from `runTUI()` — the entire block:
```go
historyFlag, _ := cmd.Flags().GetBool("history")
if historyFlag {
    return runHistoryViewer(ctx, historyService)
}
```

Remove the function `runHistoryViewer` (lines 412–497 in the original file).

Also remove the `historyService` local variable initialization inside `runTUI` if it is no longer referenced there (verify after removing the `--last` and `--history` branches). If `historyService` is still used (e.g., for `force-unlock`), keep it.

- [ ] **Step 5: Run tests**

```bash
task check
```

Expected: all tests pass, including the two new `TestHistoryCommand_*` tests.

- [ ] **Step 6: Commit**

```bash
git add cmd/history.go cmd/history_test.go cmd/root.go
git commit -m "feat: promote --history flag to terrax history subcommand with --json opt-in"
```

---

### Task 4: Update VS Code extension

**Files:**
- Modify: `extensions/vscode/src/historyProvider.ts:61-63`

**Interfaces:**
- Consumes: `terrax history --json --dir <path>` CLI invocation.

- [ ] **Step 1: Update the `spawnSync` call**

In `extensions/vscode/src/historyProvider.ts`, line 63, change:

```typescript
// Before
['history', '--dir', this.workspaceRoot],

// After
['history', '--json', '--dir', this.workspaceRoot],
```

- [ ] **Step 2: Build the extension to verify it compiles**

```bash
task ext:build
```

Expected: TypeScript compiles without errors.

- [ ] **Step 3: Commit**

```bash
git add extensions/vscode/src/historyProvider.ts
git commit -m "fix: update VS Code extension to use terrax history --json"
```

---

## Self-Review

**Spec coverage:**
- ✅ `terrax last` subcommand without `--dir` — Task 1
- ✅ `terrax review` subcommand with `--dir` — Task 2
- ✅ `terrax history` → TUI by default, `--json` for JSON — Task 3
- ✅ `--last`, `--history`, `--review` removed from root — Tasks 1, 2, 3
- ✅ VS Code extension updated — Task 4

**Placeholder scan:** No TBDs. All steps include concrete code.

**Type consistency:** `executeLastCommand(ctx context.Context, historyService *history.Service)` used consistently. `runPlanReview(ctx context.Context, stackPath string) error` referenced in Tasks 2 and 3 — matches existing signature in `root.go`.
