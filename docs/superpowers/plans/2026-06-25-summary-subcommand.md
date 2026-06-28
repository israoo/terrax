# Summary Subcommand Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add `terrax summary` subcommand that prints the terminal plan summary from existing `.terrax/plans/` JSON files.

**Architecture:** Single new file `cmd/summary.go` following the exact pattern of `cmd/review.go`. `runSummaryCmd` derives `repoRoot` via `deps.FindRepoRoot` and delegates to `plan.Summarize` directly. `runPlanSummary` in `root.go` is not modified.

**Tech Stack:** Go · Cobra · Viper · `internal/plan` · `internal/deps` · `internal/config`

## Global Constraints

- File in `package cmd` — no new packages.
- Three import groups alphabetically sorted: stdlib · third-party · `github.com/israoo/terrax/...`.
- All comments end with periods.
- Errors wrapped with `fmt.Errorf("context: %w", err)`.
- `--dir` flag present; no other flags.
- `runPlanSummary` in `root.go` must not be modified.
- Run `task check` (fmt + vet + lint + test) before every commit.

---

## File Map

| File | Action | Responsibility |
|---|---|---|
| `cmd/summary.go` | Create | `terrax summary` subcommand |
| `cmd/summary_test.go` | Create | Two tests: no-plan-dir and with-plan-files |

---

### Task 1: `terrax summary` subcommand

**Files:**
- Create: `cmd/summary.go`
- Create: `cmd/summary_test.go`

**Interfaces:**
- Consumes: `getWorkingDirectory(dir string) (string, error)` — `cmd/root.go`
- Consumes: `resolveWorkDir(dir string) string` — `cmd/root.go`
- Consumes: `ensureConfigFromWorkDir(workDir string)` — `cmd/root.go`
- Consumes: `deps.FindRepoRoot(workDir, rootConfigFile string) string` — `internal/deps`
- Consumes: `plan.Summarize(ctx context.Context, dir, projectRoot string) (int, error)` — `internal/plan`
- Consumes: `viper.GetString("root_config_file")` — returns the root sentinel filename
- Consumes: `config.DefaultRootConfigFile` — fallback value `"root.hcl"`
- Consumes: `config.DefaultJSONOutDir` — value `".terrax/plans"`
- Produces: `runSummaryCmd(cmd *cobra.Command, args []string) error` — registered as `RunE` on `summaryCmd`

- [ ] **Step 1: Write the failing tests**

Create `cmd/summary_test.go`:

```go
package cmd

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSummaryCmd_NoPlanDir(t *testing.T) {
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
	require.NoError(t, cmd.ParseFlags([]string{"--dir", tmpDir}))

	err := runSummaryCmd(cmd, []string{})

	_ = w.Close()
	os.Stdout = oldStdout
	output := <-done

	require.NoError(t, err)
	assert.Empty(t, output, "no plan dir must produce no output")
}

func TestSummaryCmd_WithPlanFiles(t *testing.T) {
	tmpDir := t.TempDir()

	// Write a minimal root config file so FindRepoRoot finds this dir as root.
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "root.hcl"), []byte(""), 0644))

	// Write a minimal plan JSON with one "create" resource change.
	planDir := filepath.Join(tmpDir, ".terrax", "plans", "env", "dev", "vpc")
	require.NoError(t, os.MkdirAll(planDir, 0755))
	planJSON := `{
		"resource_changes": [
			{
				"address": "aws_vpc.main",
				"type": "aws_vpc",
				"name": "main",
				"change": {
					"actions": ["create"],
					"before": null,
					"after": {}
				}
			}
		]
	}`
	require.NoError(t, os.WriteFile(filepath.Join(planDir, "tfplan.json"), []byte(planJSON), 0644))

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
	require.NoError(t, cmd.ParseFlags([]string{"--dir", tmpDir}))

	err := runSummaryCmd(cmd, []string{})

	_ = w.Close()
	os.Stdout = oldStdout
	output := <-done

	require.NoError(t, err)
	assert.Contains(t, output, "Pending changes", "output must report pending changes")
	assert.Contains(t, output, "env/dev/vpc", "output must include the stack path")
}
```

- [ ] **Step 2: Run tests to verify they fail**

```bash
go test ./cmd/ -run TestSummaryCmd -v
```

Expected: FAIL — `runSummaryCmd` undefined.

- [ ] **Step 3: Create `cmd/summary.go`**

```go
package cmd

import (
	"context"
	"fmt"
	"path/filepath"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/israoo/terrax/internal/config"
	"github.com/israoo/terrax/internal/deps"
	"github.com/israoo/terrax/internal/plan"
)

var summaryCmd = &cobra.Command{
	Use:   "summary",
	Short: "Print a terminal summary of pending plan changes",
	Long:  `Print a grouped terminal summary of pending vs. no-change stacks from existing plan files in .terrax/plans/.`,
	RunE:  runSummaryCmd,
}

func init() {
	summaryCmd.Flags().String("dir", "", "Working directory (overrides current directory)")
	rootCmd.AddCommand(summaryCmd)
}

func runSummaryCmd(cmd *cobra.Command, args []string) error {
	ctx := context.Background()

	dirFlag, _ := cmd.Flags().GetString("dir")
	workDir, err := getWorkingDirectory(dirFlag)
	if err != nil {
		return fmt.Errorf("failed to get working directory: %w", err)
	}

	ensureConfigFromWorkDir(workDir)
	workDir = resolveWorkDir(workDir)

	rootConfigFile := viper.GetString("root_config_file")
	if rootConfigFile == "" {
		rootConfigFile = config.DefaultRootConfigFile
	}

	repoRoot := deps.FindRepoRoot(workDir, rootConfigFile)
	jsonDir := filepath.Join(repoRoot, config.DefaultJSONOutDir)

	_, err = plan.Summarize(ctx, jsonDir, repoRoot)
	return err
}
```

- [ ] **Step 4: Run tests to verify they pass**

```bash
go test ./cmd/ -run TestSummaryCmd -v
```

Expected:
```
=== RUN   TestSummaryCmd_NoPlanDir
--- PASS: TestSummaryCmd_NoPlanDir (0.00s)
=== RUN   TestSummaryCmd_WithPlanFiles
--- PASS: TestSummaryCmd_WithPlanFiles (0.00s)
PASS
```

- [ ] **Step 5: Run full check**

```bash
task check
```

Expected: `✅ All checks passed`

- [ ] **Step 6: Commit**

```bash
git add cmd/summary.go cmd/summary_test.go
git commit -m "feat: add terrax summary subcommand"
```

---

## Self-Review

**Spec coverage:**
- ✅ `terrax summary` subcommand with `--dir` — Task 1
- ✅ Calls `plan.Summarize` directly, not `runPlanSummary` — Task 1
- ✅ `runPlanSummary` in `root.go` untouched — not in file map
- ✅ `TestSummaryCmd_NoPlanDir` — Task 1
- ✅ `TestSummaryCmd_WithPlanFiles` with create resource — Task 1

**Placeholder scan:** No TBDs. All code is complete.

**Type consistency:** `runSummaryCmd(cmd *cobra.Command, args []string) error` used in both the implementation and test file consistently.
