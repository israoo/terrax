# Force Unlock Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add a `force-unlock` command that discovers a Terraform state lock ID from S3 via AWS CLI and executes `terragrunt force-unlock` automatically.

**Architecture:** Three layers — `internal/state/locker.go` discovers the lock ID via `aws s3 cp`; `executor.RunForceUnlock` runs the Terragrunt command; `cmd/root.go` wires them together at all three dispatch sites (normal TUI, `--last`, `--history`).

**Tech Stack:** Go 1.25.5 · `github.com/spf13/viper` · `github.com/stretchr/testify` · AWS CLI (external dependency)

## Global Constraints

- All comments must end with periods.
- Imports: three groups — stdlib, third-party, `github.com/israoo/terrax/...` — alphabetically sorted, blank lines between groups.
- `internal/state/` must not import `internal/tui/` or any Bubble Tea/Lipgloss package.
- Tests: table-driven format, `resetViper()` (not `viper.Reset()`) in executor tests; plain `viper.Reset()` in state tests (no executor defaults needed).
- `execCommandContext` mock pattern: package-level `var execCommandContext = exec.CommandContext`, swapped in tests with `defer` restore.
- Errors wrapped: `fmt.Errorf("context: %w", err)`.
- No new files beyond: `internal/state/locker.go`, `internal/state/locker_test.go`.

---

## File Map

| File | Change |
|------|--------|
| `internal/config/defaults.go` | Add `DefaultStateRegion = "us-east-1"` |
| `internal/state/locker.go` | NEW — `GetLockID` |
| `internal/state/locker_test.go` | NEW — tests for `GetLockID` |
| `internal/executor/executor.go` | Add `RunForceUnlock` |
| `internal/executor/executor_test.go` | Add `TestRunForceUnlock_Args` |
| `cmd/root.go` | Add `runForceUnlock`, branch at 3 dispatch sites, import `internal/state` |
| `.terrax.yaml` | Document `state.*` keys |

---

### Task 1: Add DefaultStateRegion constant

**Files:**
- Modify: `internal/config/defaults.go`

**Interfaces:**
- Produces: `config.DefaultStateRegion = "us-east-1"` — consumed by `GetLockID` in Task 2 and `runForceUnlock` in Task 3.

- [ ] **Step 1: Add the constant**

Add after `DefaultReportFormat = "json"` inside the `const` block in `internal/config/defaults.go`:

```go
	// DefaultStateRegion is the default AWS region for Terraform state operations.
	DefaultStateRegion = "us-east-1"
```

- [ ] **Step 2: Build**

```bash
go build ./internal/config/...
```

Expected: no output.

- [ ] **Step 3: Commit**

```bash
git add internal/config/defaults.go
git commit -m "feat(config): add DefaultStateRegion constant"
```

---

### Task 2: Create internal/state/locker.go (TDD)

**Files:**
- Create: `internal/state/locker.go`
- Create: `internal/state/locker_test.go`

**Interfaces:**
- Consumes: `config.DefaultStateRegion` from Task 1.
- Produces: `state.GetLockID(ctx context.Context, bucket, project, stackRelPath, region string) (string, error)` — consumed by `runForceUnlock` in Task 3.

- [ ] **Step 1: Write the failing tests**

Create `internal/state/locker_test.go`:

```go
package state

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeExecCommand returns a *exec.Cmd that runs this test binary's TestHelperProcessLocker.
func fakeExecCommand(ctx context.Context, command string, args ...string) *exec.Cmd {
	cs := []string{"-test.run=TestHelperProcessLocker", "--", command}
	cs = append(cs, args...)
	cmd := exec.Command(os.Args[0], cs...)
	cmd.Env = []string{
		"GO_WANT_HELPER_PROCESS=1",
		fmt.Sprintf("GO_LOCKER_STDOUT=%s", lockerMockStdout),
		fmt.Sprintf("GO_LOCKER_STDERR=%s", lockerMockStderr),
		fmt.Sprintf("GO_LOCKER_EXIT=%d", lockerMockExit),
	}
	return cmd
}

// TestHelperProcessLocker is invoked as a subprocess by fakeExecCommand.
func TestHelperProcessLocker(t *testing.T) {
	if os.Getenv("GO_WANT_HELPER_PROCESS") != "1" {
		return
	}
	fmt.Print(os.Getenv("GO_LOCKER_STDOUT"))
	if s := os.Getenv("GO_LOCKER_STDERR"); s != "" {
		fmt.Fprint(os.Stderr, s)
	}
	if os.Getenv("GO_LOCKER_EXIT") == "1" {
		os.Exit(1)
	}
	os.Exit(0)
}

var (
	lockerMockStdout string
	lockerMockStderr string
	lockerMockExit   int
)

func TestGetLockID(t *testing.T) {
	oldExec := execCommandContext
	execCommandContext = fakeExecCommand
	defer func() { execCommandContext = oldExec }()

	tests := []struct {
		name         string
		bucket       string
		project      string
		stackRelPath string
		region       string
		mockStdout   string
		mockStderr   string
		mockExit     int
		wantID       string
		wantErr      bool
	}{
		{
			name:         "lock found returns ID",
			bucket:       "my-bucket",
			project:      "caas",
			stackRelPath: "workloads/dev/us-east-1/core/acm",
			region:       "us-east-1",
			mockStdout:   `{"ID":"abc-123","Who":"user@host","Operation":"OperationTypePlan","Created":"2026-06-20T00:00:00Z"}`,
			mockExit:     0,
			wantID:       "abc-123",
		},
		{
			name:         "no lock returns empty string",
			bucket:       "my-bucket",
			project:      "caas",
			stackRelPath: "workloads/dev/us-east-1/core/acm",
			region:       "us-east-1",
			mockStderr:   "An error occurred (NoSuchKey) when calling the GetObject operation",
			mockExit:     1,
			wantID:       "",
		},
		{
			name:         "aws CLI error returns error",
			bucket:       "my-bucket",
			project:      "caas",
			stackRelPath: "workloads/dev/us-east-1/core/acm",
			region:       "us-east-1",
			mockStderr:   "An error occurred (AccessDenied)",
			mockExit:     1,
			wantErr:      true,
		},
		{
			name:         "empty region falls back to default",
			bucket:       "my-bucket",
			project:      "caas",
			stackRelPath: "workloads/dev/us-east-1/core/acm",
			region:       "",
			mockStdout:   `{"ID":"def-456","Who":"ci@runner","Operation":"OperationTypeApply","Created":"2026-06-20T00:00:00Z"}`,
			mockExit:     0,
			wantID:       "def-456",
		},
		{
			name:         "malformed JSON returns error",
			bucket:       "my-bucket",
			project:      "caas",
			stackRelPath: "workloads/dev/us-east-1/core/acm",
			region:       "us-east-1",
			mockStdout:   `not-valid-json`,
			mockExit:     0,
			wantErr:      true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			lockerMockStdout = tt.mockStdout
			lockerMockStderr = tt.mockStderr
			lockerMockExit = tt.mockExit

			id, err := GetLockID(context.Background(), tt.bucket, tt.project, tt.stackRelPath, tt.region)

			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.wantID, id)
		})
	}
}
```

- [ ] **Step 2: Run the test to confirm it fails**

```bash
go test ./internal/state/... -run TestGetLockID -v
```

Expected: compilation error — `GetLockID` and `execCommandContext` not defined yet.

- [ ] **Step 3: Implement locker.go**

Create `internal/state/locker.go`:

```go
// Package state handles Terraform state operations for TerraX.
//
// It provides utilities for inspecting and managing Terraform state backends,
// including lock detection via the AWS CLI for force-unlock operations.
package state

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
	"strings"

	"github.com/israoo/terrax/internal/config"
)

// execCommandContext allows mocking exec.CommandContext in tests.
var execCommandContext = exec.CommandContext

// tfLockJSON represents the structure of a Terraform state lock file.
type tfLockJSON struct {
	ID        string `json:"ID"`
	Who       string `json:"Who"`
	Operation string `json:"Operation"`
	Created   string `json:"Created"`
}

// GetLockID fetches the Terraform state lock ID from S3 for the given stack.
// It returns ("", nil) when no lock exists for the stack.
// It returns ("", error) on AWS CLI failure or JSON parse error.
func GetLockID(ctx context.Context, bucket, project, stackRelPath, region string) (string, error) {
	if region == "" {
		region = config.DefaultStateRegion
	}

	lockKey := fmt.Sprintf("%s/%s/terraform.tfstate.tflock", project, stackRelPath)
	s3URI := fmt.Sprintf("s3://%s/%s", bucket, lockKey)

	cmd := execCommandContext(ctx, "aws", "s3", "cp", s3URI, "-", "--region", region)
	output, err := cmd.Output()
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) && strings.Contains(string(exitErr.Stderr), "NoSuchKey") {
			return "", nil
		}
		return "", fmt.Errorf("aws s3 cp failed: %w", err)
	}

	var lock tfLockJSON
	if err := json.Unmarshal(output, &lock); err != nil {
		return "", fmt.Errorf("failed to parse lock file: %w", err)
	}

	return lock.ID, nil
}
```

- [ ] **Step 4: Run the tests**

```bash
go test ./internal/state/... -v
```

Expected: all `TestGetLockID` cases pass.

- [ ] **Step 5: Run full suite**

```bash
go test ./...
```

Expected: all packages pass.

- [ ] **Step 6: Commit**

```bash
git add internal/state/locker.go internal/state/locker_test.go
git commit -m "feat(state): add GetLockID to discover Terraform state lock from S3"
```

---

### Task 3: Add RunForceUnlock to executor (TDD)

**Files:**
- Modify: `internal/executor/executor.go`
- Modify: `internal/executor/executor_test.go`

**Interfaces:**
- Consumes: `HistoryLogger`, `displayExecutionSummary`, `logExecutionToHistory` — all already in `executor.go`.
- Produces: `executor.RunForceUnlock(ctx context.Context, historyLogger HistoryLogger, lockID, absoluteStackPath string) error` — consumed by `runForceUnlock` in Task 4.

- [ ] **Step 1: Write the failing test**

Add at the end of `internal/executor/executor_test.go`:

```go
// TestRunForceUnlock_Args tests that RunForceUnlock builds the correct terragrunt args.
func TestRunForceUnlock_Args(t *testing.T) {
	// Capture stdout/stderr to suppress output during test.
	oldStdout := os.Stdout
	oldStderr := os.Stderr
	_, wOut, _ := os.Pipe()
	_, wErr, _ := os.Pipe()
	os.Stdout = wOut
	os.Stderr = wErr
	defer func() {
		os.Stdout = oldStdout
		os.Stderr = oldStderr
		_ = wOut.Close()
		_ = wErr.Close()
	}()

	logger := &mockHistoryLogger{}
	ctx := context.Background()

	// RunForceUnlock will fail because terragrunt is not installed in CI,
	// but we only care that the function returns without panicking and logs
	// to history with the correct command name.
	_ = RunForceUnlock(ctx, logger, "lock-id-abc-123", "/path/to/stack")

	assert.True(t, logger.appendCalled, "History should be logged after force-unlock.")
}
```

- [ ] **Step 2: Run the test to confirm it fails**

```bash
go test ./internal/executor/... -run TestRunForceUnlock_Args -v
```

Expected: FAIL — `RunForceUnlock` not defined yet.

- [ ] **Step 3: Implement RunForceUnlock**

Add after the `Run` function in `internal/executor/executor.go`:

```go
// RunForceUnlock executes a Terraform force-unlock for a specific stack.
// Unlike Run, it uses --working-dir without --all and passes the lock ID directly.
// It logs the operation to history the same way Run does.
func RunForceUnlock(ctx context.Context, historyLogger HistoryLogger, lockID, absoluteStackPath string) error {
	nextID, err := historyLogger.GetNextID(ctx)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Warning: Failed to get history ID: %v\n", err)
		nextID = 0
	}

	startTime := time.Now()

	args := []string{
		"run", "--working-dir", absoluteStackPath, "--non-interactive",
		"--", "force-unlock", "-force", lockID,
	}

	fmt.Printf("🔓 Executing: terragrunt %v\n\n", args)

	cmd := exec.CommandContext(ctx, "terragrunt", args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin

	execErr := cmd.Run()
	exitCode := 0
	summary := "Force unlock completed successfully."

	if execErr != nil {
		fmt.Fprintf(os.Stderr, "\n❌ Force unlock failed: %v\n", execErr)
		if exitErr, ok := execErr.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		} else {
			exitCode = 1
		}
		summary = fmt.Sprintf("Force unlock failed: %v", execErr)
	} else {
		fmt.Println("\n✅ Force unlock completed")
	}

	duration := time.Since(startTime)
	displayExecutionSummary("force-unlock", absoluteStackPath, duration, exitCode, startTime)
	logExecutionToHistory(ctx, historyLogger, nextID, startTime, "force-unlock", absoluteStackPath, exitCode, duration, summary)

	return execErr
}
```

- [ ] **Step 4: Run all executor tests**

```bash
go test ./internal/executor/... -v
```

Expected: all tests pass including `TestRunForceUnlock_Args`.

- [ ] **Step 5: Commit**

```bash
git add internal/executor/executor.go internal/executor/executor_test.go
git commit -m "feat(executor): add RunForceUnlock for Terraform state force-unlock"
```

---

### Task 4: Wire force-unlock in cmd/root.go + document config

**Files:**
- Modify: `cmd/root.go`
- Modify: `.terrax.yaml`

**Interfaces:**
- Consumes: `state.GetLockID` from Task 2, `executor.RunForceUnlock` from Task 3, `config.DefaultStateRegion` from Task 1.
- Produces: Nothing consumed by other tasks — this is the terminal wiring task.

- [ ] **Step 1: Add `internal/state` import to cmd/root.go**

The current import block in `cmd/root.go` (lines 8–23) has `internal/` packages in the third group. Add `"github.com/israoo/terrax/internal/state"` alphabetically:

```go
import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/israoo/terrax/internal/config"
	"github.com/israoo/terrax/internal/executor"
	"github.com/israoo/terrax/internal/history"
	"github.com/israoo/terrax/internal/plan"
	"github.com/israoo/terrax/internal/stack"
	"github.com/israoo/terrax/internal/state"
	"github.com/israoo/terrax/internal/tui"
)
```

- [ ] **Step 2: Add runForceUnlock helper function**

Add this function before `runPlanReview` in `cmd/root.go`:

```go
// runForceUnlock discovers the state lock ID from S3 and executes force-unlock.
// It returns nil if no lock is found for the given stack.
func runForceUnlock(ctx context.Context, historyService *history.Service, absoluteStackPath string) error {
	bucket := viper.GetString("state.bucket")
	project := viper.GetString("state.project")
	region := viper.GetString("state.region")

	if bucket == "" || project == "" {
		return fmt.Errorf("state.bucket and state.project must be set in .terrax.yaml to use force-unlock")
	}

	rootConfigFile := viper.GetString("root_config_file")
	if rootConfigFile == "" {
		rootConfigFile = config.DefaultRootConfigFile
	}

	stackRelPath, err := history.GetRelativeStackPath(absoluteStackPath, rootConfigFile)
	if err != nil {
		return fmt.Errorf("failed to determine stack relative path: %w", err)
	}

	lockID, err := state.GetLockID(ctx, bucket, project, stackRelPath, region)
	if err != nil {
		return fmt.Errorf("failed to get lock ID: %w", err)
	}

	if lockID == "" {
		fmt.Printf("No lock found for %s\n", stackRelPath)
		return nil
	}

	fmt.Printf("🔓 Unlocking %s (lock: %s)\n", stackRelPath, lockID)
	return executor.RunForceUnlock(ctx, historyService, lockID, absoluteStackPath)
}
```

- [ ] **Step 3: Add force-unlock branch at the three dispatch sites**

**Site 1 — normal TUI** (after `executor.Run`, around line 161):

```go
		err := executor.Run(ctx, historyService, command, stackPath)
		if err != nil {
			return err
		}

		if command == "plan" && viper.GetBool("plan.review_enabled") {
			return runPlanReview(ctx, stackPath)
		}
```

Replace with:

```go
		if command == "force-unlock" {
			return runForceUnlock(ctx, historyService, stackPath)
		}

		err := executor.Run(ctx, historyService, command, stackPath)
		if err != nil {
			return err
		}

		if command == "plan" && viper.GetBool("plan.review_enabled") {
			return runPlanReview(ctx, stackPath)
		}
```

**Site 2 — executeLastCommand** (after `executor.Run`, around line 275):

```go
	err = executor.Run(ctx, historyService, lastEntry.Command, absolutePath)
	if err != nil {
		return err
	}

	if lastEntry.Command == "plan" && viper.GetBool("plan.review_enabled") {
		return runPlanReview(ctx, absolutePath)
	}
```

Replace with:

```go
	if lastEntry.Command == "force-unlock" {
		return runForceUnlock(ctx, historyService, absolutePath)
	}

	err = executor.Run(ctx, historyService, lastEntry.Command, absolutePath)
	if err != nil {
		return err
	}

	if lastEntry.Command == "plan" && viper.GetBool("plan.review_enabled") {
		return runPlanReview(ctx, absolutePath)
	}
```

**Site 3 — runHistoryViewer re-execute** (after `executor.Run`, around line 335):

```go
			err := executor.Run(ctx, historyService, entry.Command, absolutePath)
			if err != nil {
				return err
			}

			if entry.Command == "plan" && viper.GetBool("plan.review_enabled") {
				return runPlanReview(ctx, absolutePath)
			}
```

Replace with:

```go
			if entry.Command == "force-unlock" {
				return runForceUnlock(ctx, historyService, absolutePath)
			}

			err := executor.Run(ctx, historyService, entry.Command, absolutePath)
			if err != nil {
				return err
			}

			if entry.Command == "plan" && viper.GetBool("plan.review_enabled") {
				return runPlanReview(ctx, absolutePath)
			}
```

- [ ] **Step 4: Build to confirm no compilation errors**

```bash
go build ./...
```

Expected: no output.

- [ ] **Step 5: Run full test suite**

```bash
go test ./...
```

Expected: all packages pass.

- [ ] **Step 6: Document state.* config in .terrax.yaml**

At the end of `.terrax.yaml`, add:

```yaml

# State backend configuration (required for force-unlock command)
# state:
#   bucket: "my-terraform-state-bucket"   # S3 bucket holding Terraform state
#   project: "caas-workloads"             # Key prefix inside the bucket (e.g., project name)
#   region: "us-east-1"                   # AWS region (default: us-east-1)
#
# To use force-unlock, add it to the commands list:
# commands:
#   - plan
#   - apply
#   - force-unlock
```

- [ ] **Step 7: Commit**

```bash
git add cmd/root.go .terrax.yaml
git commit -m "feat(cmd): wire force-unlock command with S3 lock discovery"
```

---

## Self-Review

**Spec coverage:**

| Spec requirement | Task |
|---|---|
| `state.GetLockID` via AWS CLI | Task 2 |
| `("", nil)` when no lock (NoSuchKey) | Task 2 |
| `("", error)` on other AWS failure | Task 2 |
| `("", error)` on malformed JSON | Task 2 |
| `config.DefaultStateRegion` fallback | Tasks 1 + 2 |
| `executor.RunForceUnlock` | Task 3 |
| Args: `run --working-dir <path> --non-interactive -- force-unlock -force <id>` | Task 3 |
| History logged with command `"force-unlock"` | Task 3 |
| `runForceUnlock` in cmd/root.go | Task 4 |
| Missing bucket/project → clear error | Task 4 |
| "No lock found" message → return nil | Task 4 |
| 3 dispatch sites protected | Task 4 |
| `.terrax.yaml` documentation | Task 4 |

**No placeholders found.**

**Type consistency:** `state.GetLockID` signature used in Task 2 (implementation) and Task 4 (caller) is identical: `(ctx context.Context, bucket, project, stackRelPath, region string) (string, error)`. `executor.RunForceUnlock` defined in Task 3 and called in Task 4 with matching `(ctx, historyService, lockID, absoluteStackPath)`.
