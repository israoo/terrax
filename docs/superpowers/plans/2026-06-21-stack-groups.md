# Stack Groups Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Classify stacks via markers in `stack.hcl`, execute each group as a separate `terragrunt run` in topological order with per-group env vars, and expose group data as JSON via `terrax groups --json`.

**Architecture:** New `internal/stack/markers.go` provides `DetectGroup` + `TopologicalSort` (pure, no viper). `executor.Run` gains an `envVars` parameter. `cmd/root.go` adds `buildGroupedExecution` which replaces single `executor.Run` calls with a per-group loop. `cmd/groups.go` is a new subcommand mirroring `cmd/tree.go`.

**Tech Stack:** Go 1.25.5 · `github.com/spf13/viper` · `github.com/stretchr/testify`

## Global Constraints

- All comments must end with periods.
- Imports: 3 groups (stdlib, third-party, `github.com/israoo/terrax/...`), alphabetically sorted.
- `internal/stack/markers.go` must not import viper, cobra, or any UI package.
- Tests: table-driven, `viper.Reset()` where needed.
- Errors wrapped: `fmt.Errorf("context: %w", err)`.
- `stack_groups` config key is parsed via `viper.UnmarshalKey` using `mapstructure` tags.

---

## File Map

| File | Change |
|------|--------|
| `internal/stack/markers.go` | NEW — `GroupDetectConfig`, `DetectGroup`, `TopologicalSort` |
| `internal/stack/markers_test.go` | NEW |
| `internal/executor/executor.go` | Add `envVars map[string]string` to `Run`, add env injection, remove `.terrax/` cleanup |
| `cmd/root.go` | Add `StackGroupConfig`, `GroupExecution`, `loadStackGroups`, `buildGroupedExecution`; replace 3 dispatch sites with grouped loop; move cleanup |
| `cmd/run.go` | Replace `executor.Run` call with grouped loop |
| `cmd/groups.go` | NEW — `terrax groups --json` subcommand |
| `.terrax.yaml` | Document `stack_groups` section |

---

### Task 1: internal/stack/markers.go (TDD)

**Files:**
- Create: `internal/stack/markers.go`
- Create: `internal/stack/markers_test.go`

**Interfaces:**
- Produces:
  - `type GroupDetectConfig struct { Detect string; DependsOn []string; Env map[string]string }`
  - `DetectGroup(stackPath string, groups map[string]GroupDetectConfig) string`
  - `TopologicalSort(groups map[string]GroupDetectConfig) ([]string, error)`

- [ ] **Step 1: Write the failing tests**

Create `internal/stack/markers_test.go`:

```go
package stack

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDetectGroup(t *testing.T) {
	groups := map[string]GroupDetectConfig{
		"private": {Detect: "require_private_connection = true"},
		"deprecated": {Detect: "deprecated = true"},
	}

	tests := []struct {
		name        string
		hclContent  string
		writeFile   bool
		expected    string
	}{
		{
			name:       "matches first group",
			hclContent: "locals {\n  require_private_connection = true\n}",
			writeFile:  true,
			expected:   "private",
		},
		{
			name:       "matches second group",
			hclContent: "locals {\n  deprecated = true\n}",
			writeFile:  true,
			expected:   "deprecated",
		},
		{
			name:       "no match returns default",
			hclContent: "locals {\n  enabled_providers = [\"aws\"]\n}",
			writeFile:  true,
			expected:   "default",
		},
		{
			name:      "missing stack.hcl returns default",
			writeFile: false,
			expected:  "default",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			if tt.writeFile {
				require.NoError(t, os.WriteFile(filepath.Join(dir, "stack.hcl"), []byte(tt.hclContent), 0644))
			}
			assert.Equal(t, tt.expected, DetectGroup(dir, groups))
		})
	}
}

func TestTopologicalSort(t *testing.T) {
	tests := []struct {
		name      string
		groups    map[string]GroupDetectConfig
		wantOrder []string
		wantErr   bool
	}{
		{
			name:      "single group no deps",
			groups:    map[string]GroupDetectConfig{"default": {}},
			wantOrder: []string{"default"},
		},
		{
			name: "private depends on default",
			groups: map[string]GroupDetectConfig{
				"default": {},
				"private": {DependsOn: []string{"default"}},
			},
			wantOrder: []string{"default", "private"},
		},
		{
			name: "cycle returns error",
			groups: map[string]GroupDetectConfig{
				"a": {DependsOn: []string{"b"}},
				"b": {DependsOn: []string{"a"}},
			},
			wantErr: true,
		},
		{
			name: "depends_on undefined group is ignored",
			groups: map[string]GroupDetectConfig{
				"private": {DependsOn: []string{"nonexistent"}},
			},
			wantOrder: []string{"private"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := TopologicalSort(tt.groups)
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.wantOrder, result)
		})
	}
}
```

- [ ] **Step 2: Run to confirm FAIL**

```bash
go test ./internal/stack/... -run "TestDetectGroup|TestTopologicalSort" -v
```

Expected: compilation error — `GroupDetectConfig`, `DetectGroup`, `TopologicalSort` undefined.

- [ ] **Step 3: Implement markers.go**

Create `internal/stack/markers.go`:

```go
package stack

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// GroupDetectConfig holds the classification and execution config for a single stack group.
type GroupDetectConfig struct {
	Detect    string            // Exact string to grep in stack.hcl; empty = default group.
	DependsOn []string          // Group names that must complete before this group.
	Env       map[string]string // Environment variables injected for this group's execution.
}

// DetectGroup returns the name of the first group whose detect pattern is found in
// <stackPath>/stack.hcl. Returns "default" if no pattern matches or the file does not exist.
func DetectGroup(stackPath string, groups map[string]GroupDetectConfig) string {
	hclPath := filepath.Join(stackPath, "stack.hcl")
	data, err := os.ReadFile(hclPath)
	if err != nil {
		return "default"
	}
	content := string(data)

	// Sort group names for deterministic matching order.
	names := make([]string, 0, len(groups))
	for name := range groups {
		names = append(names, name)
	}
	sort.Strings(names)

	for _, name := range names {
		cfg := groups[name]
		if cfg.Detect != "" && strings.Contains(content, cfg.Detect) {
			return name
		}
	}
	return "default"
}

// TopologicalSort returns group names in a valid execution order respecting all
// depends_on relationships. Groups referenced in depends_on but not defined in
// the map are treated as already complete. Returns an error if a cycle is detected.
func TopologicalSort(groups map[string]GroupDetectConfig) ([]string, error) {
	inDegree := make(map[string]int, len(groups))
	successors := make(map[string][]string, len(groups))

	for name := range groups {
		inDegree[name] = 0
	}

	for name, cfg := range groups {
		for _, dep := range cfg.DependsOn {
			if _, exists := groups[dep]; !exists {
				continue // Undefined group — treat as already complete.
			}
			inDegree[name]++
			successors[dep] = append(successors[dep], name)
		}
	}

	// Seed with groups that have no dependencies.
	queue := make([]string, 0, len(groups))
	for name := range groups {
		if inDegree[name] == 0 {
			queue = append(queue, name)
		}
	}
	sort.Strings(queue)

	result := make([]string, 0, len(groups))
	for len(queue) > 0 {
		node := queue[0]
		queue = queue[1:]
		result = append(result, node)

		succs := successors[node]
		sort.Strings(succs)
		for _, succ := range succs {
			inDegree[succ]--
			if inDegree[succ] == 0 {
				queue = append(queue, succ)
				sort.Strings(queue)
			}
		}
	}

	if len(result) != len(groups) {
		return nil, fmt.Errorf("cycle detected in stack_groups depends_on")
	}
	return result, nil
}
```

- [ ] **Step 4: Run tests**

```bash
go test ./internal/stack/... -run "TestDetectGroup|TestTopologicalSort" -v
```

Expected: all PASS.

- [ ] **Step 5: Run full suite**

```bash
go test ./...
```

Expected: all packages pass.

- [ ] **Step 6: Commit**

```bash
git add internal/stack/markers.go internal/stack/markers_test.go
git commit -m "feat(stack): add DetectGroup and TopologicalSort for stack group classification"
```

---

### Task 2: Add envVars to executor.Run, remove cleanup

**Files:**
- Modify: `internal/executor/executor.go`

**Interfaces:**
- Consumes: nothing new.
- Produces: `Run(ctx, historyLogger, command, absoluteStackPath, repoRoot string, filterPaths []string, envVars map[string]string) error` — consumed by all callers in Task 3.

Note: This task will break compilation until Task 3 updates all callers. That is expected — run only package-level tests in this task.

- [ ] **Step 1: Update Run signature, add env injection, remove cleanup**

In `internal/executor/executor.go`, make these exact changes:

**A) Change `Run` signature** (line 32):

```go
func Run(ctx context.Context, historyLogger HistoryLogger, command, absoluteStackPath, repoRoot string, filterPaths []string, envVars map[string]string) error {
```

**B) Remove the cleanup block** (lines 41–45, currently reads):

```go
	// Clear the output directory before plan execution so each run starts with a clean slate.
	// This ensures the TUI and summary always reflect only the current run's results.
	if command == "plan" && (viper.GetBool("plan.summary_enabled") || viper.GetBool("plan.review_enabled")) {
		_ = os.RemoveAll(filepath.Join(repoRoot, config.DefaultOutputDir))
	}
```

Delete those 4 lines entirely.

**C) Add env injection** after `args := buildFilterArgs(...)` and before `fmt.Printf("🚀 Executing...")`:

```go
	cmd := exec.CommandContext(ctx, "terragrunt", args...)
	cmd.Dir = repoRoot
	if len(envVars) > 0 {
		env := os.Environ()
		for k, v := range envVars {
			env = append(env, fmt.Sprintf("%s=%s", k, v))
		}
		cmd.Env = env
	}
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
```

The complete updated `Run` function body from `startTime` to end:

```go
	startTime := time.Now()

	args := buildFilterArgs(repoRoot, command, filterPaths)

	fmt.Printf("🚀 Executing: terragrunt %v\n\n", args)

	cmd := exec.CommandContext(ctx, "terragrunt", args...)
	cmd.Dir = repoRoot
	if len(envVars) > 0 {
		env := os.Environ()
		for k, v := range envVars {
			env = append(env, fmt.Sprintf("%s=%s", k, v))
		}
		cmd.Env = env
	}
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin

	execErr := cmd.Run()
	exitCode := 0
	summary := "Command completed successfully."

	if execErr != nil {
		fmt.Fprintf(os.Stderr, "\n❌ Command execution failed: %v\n", execErr)
		if exitErr, ok := execErr.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		} else {
			exitCode = 1
		}
		summary = fmt.Sprintf("Command failed: %v", execErr)
	} else {
		fmt.Println("\n✅ Command execution completed")
	}

	duration := time.Since(startTime)
	displayExecutionSummary(command, absoluteStackPath, duration, exitCode, startTime)
	logExecutionToHistory(ctx, historyLogger, nextID, startTime, command, absoluteStackPath, exitCode, duration, summary)

	return execErr
```

- [ ] **Step 2: Build only the executor package (callers not yet updated)**

```bash
go build ./internal/executor/...
```

Expected: no output (package compiles).

- [ ] **Step 3: Run executor tests**

```bash
go test ./internal/executor/... -v
```

Expected: all PASS (no executor test calls `Run` directly via the cmd layer).

- [ ] **Step 4: Commit**

```bash
git add internal/executor/executor.go
git commit -m "feat(executor): add envVars parameter to Run, remove pre-execution cleanup"
```

---

### Task 3: cmd/root.go — grouped execution + cmd/run.go

**Files:**
- Modify: `cmd/root.go`
- Modify: `cmd/run.go`

**Interfaces:**
- Consumes: `stack.GroupDetectConfig`, `stack.DetectGroup`, `stack.TopologicalSort` from Task 1; `executor.Run` new signature from Task 2.
- Produces: `type StackGroupConfig struct`, `type GroupExecution struct`, `loadStackGroups() map[string]StackGroupConfig`, `buildGroupedExecution(filterPaths []string, repoRoot string) ([]GroupExecution, error)` — consumed by Task 4 (`cmd/groups.go`).

- [ ] **Step 1: Add types and helpers at the end of cmd/root.go**

Append to `cmd/root.go` (before the final closing of the file):

```go
// StackGroupConfig holds the configuration for one stack group, loaded from stack_groups in .terrax.yaml.
type StackGroupConfig struct {
	Detect    string            `mapstructure:"detect"`
	DependsOn []string          `mapstructure:"depends_on"`
	Env       map[string]string `mapstructure:"env"`
}

// GroupExecution is one resolved group ready for sequential execution.
type GroupExecution struct {
	Name      string
	DependsOn []string
	Paths     []string
	EnvVars   map[string]string
}

// loadStackGroups reads the stack_groups section from viper config.
// Always ensures an implicit "default" group exists.
func loadStackGroups() map[string]StackGroupConfig {
	var groups map[string]StackGroupConfig
	if err := viper.UnmarshalKey("stack_groups", &groups); err != nil || groups == nil {
		groups = map[string]StackGroupConfig{}
	}
	if _, ok := groups["default"]; !ok {
		groups["default"] = StackGroupConfig{}
	}
	return groups
}

// buildGroupedExecution assigns each filter path to a stack group, applies topological
// sorting, and returns the groups in execution order.
func buildGroupedExecution(filterPaths []string, repoRoot string) ([]GroupExecution, error) {
	groups := loadStackGroups()

	detectConfigs := make(map[string]stack.GroupDetectConfig, len(groups))
	for name, cfg := range groups {
		detectConfigs[name] = stack.GroupDetectConfig{
			Detect:    cfg.Detect,
			DependsOn: cfg.DependsOn,
			Env:       cfg.Env,
		}
	}

	pathsByGroup := make(map[string][]string)
	for _, relPath := range filterPaths {
		absPath := filepath.Join(repoRoot, filepath.FromSlash(relPath))
		groupName := stack.DetectGroup(absPath, detectConfigs)
		pathsByGroup[groupName] = append(pathsByGroup[groupName], relPath)
	}

	order, err := stack.TopologicalSort(detectConfigs)
	if err != nil {
		return nil, err
	}

	var result []GroupExecution
	for _, name := range order {
		paths := pathsByGroup[name]
		if len(paths) == 0 {
			continue
		}
		deps := groups[name].DependsOn
		if deps == nil {
			deps = []string{}
		}
		env := groups[name].Env
		if env == nil {
			env = map[string]string{}
		}
		result = append(result, GroupExecution{
			Name:      name,
			DependsOn: deps,
			Paths:     paths,
			EnvVars:   env,
		})
	}
	return result, nil
}
```

Also add `"github.com/israoo/terrax/internal/stack"` to the imports in `cmd/root.go` if not already present.

- [ ] **Step 2: Update the 3 dispatch sites in cmd/root.go**

**Site 1 — normal TUI** (around line 198, currently reads):

```go
		repoRoot, filterPaths := collectTransitiveDeps(stackPath)
		if err := executor.Run(ctx, historyService, command, stackPath, repoRoot, filterPaths); err != nil {
			return err
		}
		if command == "plan" && viper.GetBool("plan.summary_enabled") {
			if err := runPlanSummary(ctx, stackPath, repoRoot); err != nil {
				fmt.Fprintf(os.Stderr, "Warning: plan summary failed: %v\n", err)
			}
		}
		if command == "plan" && viper.GetBool("plan.review_enabled") {
			return runPlanReview(ctx, stackPath)
		}
```

Replace with:

```go
		repoRoot, filterPaths := collectTransitiveDeps(stackPath)

		if command == "plan" && (viper.GetBool("plan.summary_enabled") || viper.GetBool("plan.review_enabled")) {
			_ = os.RemoveAll(filepath.Join(repoRoot, config.DefaultOutputDir))
		}

		groups, err := buildGroupedExecution(filterPaths, repoRoot)
		if err != nil {
			return fmt.Errorf("failed to build group execution plan: %w", err)
		}
		for _, group := range groups {
			if err := executor.Run(ctx, historyService, command, stackPath, repoRoot, group.Paths, group.EnvVars); err != nil {
				return err
			}
		}
		if command == "plan" && viper.GetBool("plan.summary_enabled") {
			if err := runPlanSummary(ctx, stackPath, repoRoot); err != nil {
				fmt.Fprintf(os.Stderr, "Warning: plan summary failed: %v\n", err)
			}
		}
		if command == "plan" && viper.GetBool("plan.review_enabled") {
			return runPlanReview(ctx, stackPath)
		}
```

**Site 2 — executeLastCommand** (around line 338, currently reads):

```go
	repoRoot, filterPaths := collectTransitiveDeps(absolutePath)
	if err := executor.Run(ctx, historyService, lastEntry.Command, absolutePath, repoRoot, filterPaths); err != nil {
		return err
	}
```

Replace with:

```go
	repoRoot, filterPaths := collectTransitiveDeps(absolutePath)

	if lastEntry.Command == "plan" && (viper.GetBool("plan.summary_enabled") || viper.GetBool("plan.review_enabled")) {
		_ = os.RemoveAll(filepath.Join(repoRoot, config.DefaultOutputDir))
	}

	groups, err := buildGroupedExecution(filterPaths, repoRoot)
	if err != nil {
		return fmt.Errorf("failed to build group execution plan: %w", err)
	}
	for _, group := range groups {
		if err := executor.Run(ctx, historyService, lastEntry.Command, absolutePath, repoRoot, group.Paths, group.EnvVars); err != nil {
			return err
		}
	}
```

**Site 3 — runHistoryViewer** (around line 407, currently reads):

```go
			repoRoot, filterPaths := collectTransitiveDeps(absolutePath)
			if err := executor.Run(ctx, historyService, entry.Command, absolutePath, repoRoot, filterPaths); err != nil {
				return err
			}
```

Replace with:

```go
			repoRoot, filterPaths := collectTransitiveDeps(absolutePath)

			if entry.Command == "plan" && (viper.GetBool("plan.summary_enabled") || viper.GetBool("plan.review_enabled")) {
				_ = os.RemoveAll(filepath.Join(repoRoot, config.DefaultOutputDir))
			}

			groups, err := buildGroupedExecution(filterPaths, repoRoot)
			if err != nil {
				return fmt.Errorf("failed to build group execution plan: %w", err)
			}
			for _, group := range groups {
				if err := executor.Run(ctx, historyService, entry.Command, absolutePath, repoRoot, group.Paths, group.EnvVars); err != nil {
					return err
				}
			}
```

- [ ] **Step 3: Update cmd/run.go**

In `cmd/run.go`, the `runCommand` function (around line 52):

Current:

```go
	repoRoot, filterPaths := collectTransitiveDeps(workDir)
	if err := executor.Run(ctx, historyService, command, workDir, repoRoot, filterPaths); err != nil {
		return err
	}

	if command == "plan" && viper.GetBool("plan.summary_enabled") {
		if err := runPlanSummary(ctx, workDir, repoRoot); err != nil {
			return err
		}
	}
	if command == "plan" && viper.GetBool("plan.review_enabled") {
		return runPlanReview(ctx, workDir)
	}
	return nil
```

Replace with:

```go
	repoRoot, filterPaths := collectTransitiveDeps(workDir)

	if command == "plan" && (viper.GetBool("plan.summary_enabled") || viper.GetBool("plan.review_enabled")) {
		_ = os.RemoveAll(filepath.Join(repoRoot, config.DefaultOutputDir))
	}

	groups, err := buildGroupedExecution(filterPaths, repoRoot)
	if err != nil {
		return fmt.Errorf("failed to build group execution plan: %w", err)
	}
	for _, group := range groups {
		if err := executor.Run(ctx, historyService, command, workDir, repoRoot, group.Paths, group.EnvVars); err != nil {
			return err
		}
	}

	if command == "plan" && viper.GetBool("plan.summary_enabled") {
		if err := runPlanSummary(ctx, workDir, repoRoot); err != nil {
			return err
		}
	}
	if command == "plan" && viper.GetBool("plan.review_enabled") {
		return runPlanReview(ctx, workDir)
	}
	return nil
```

Add `"path/filepath"` to `cmd/run.go` imports if not already present.

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

- [ ] **Step 6: Commit**

```bash
git add cmd/root.go cmd/run.go
git commit -m "feat(cmd): grouped stack execution with buildGroupedExecution and per-group env vars"
```

---

### Task 4: cmd/groups.go — terrax groups --json

**Files:**
- Create: `cmd/groups.go`
- Modify: `.terrax.yaml`

**Interfaces:**
- Consumes: `buildGroupedExecution` from Task 3, `collectTransitiveDeps` from `cmd/root.go`.
- Produces: `terrax groups --json` CLI subcommand.

- [ ] **Step 1: Create cmd/groups.go**

```go
package cmd

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"
)

var groupsCmd = &cobra.Command{
	Use:   "groups",
	Short: "Print stack groups and their filter lists as JSON",
	Long: `Compute stack groups from the stack_groups configuration and print them as JSON.
Useful for CI pipelines that need to orchestrate execution across different runners.`,
	RunE: runGroupsCmd,
}

func init() {
	groupsCmd.Flags().String("dir", "", "Working directory (overrides current directory)")
	rootCmd.AddCommand(groupsCmd)
}

// groupsOutput is the JSON structure for terrax groups.
type groupsOutput struct {
	Groups   []groupEntry `json:"groups"`
	RepoRoot string       `json:"repo_root"`
}

// groupEntry is one resolved group in the JSON output.
type groupEntry struct {
	Name      string            `json:"name"`
	DependsOn []string          `json:"depends_on"`
	Filters   []string          `json:"filters"`
	Env       map[string]string `json:"env"`
}

func runGroupsCmd(cmd *cobra.Command, args []string) error {
	dirFlag, _ := cmd.Flags().GetString("dir")
	workDir, err := getWorkingDirectory(dirFlag)
	if err != nil {
		return fmt.Errorf("failed to get working directory: %w", err)
	}
	ensureConfigFromWorkDir(workDir)

	repoRoot, filterPaths := collectTransitiveDeps(workDir)

	groups, err := buildGroupedExecution(filterPaths, repoRoot)
	if err != nil {
		return fmt.Errorf("failed to build groups: %w", err)
	}

	output := groupsOutput{
		RepoRoot: repoRoot,
		Groups:   make([]groupEntry, 0, len(groups)),
	}
	for _, g := range groups {
		output.Groups = append(output.Groups, groupEntry{
			Name:      g.Name,
			DependsOn: g.DependsOn,
			Filters:   g.Paths,
			Env:       g.EnvVars,
		})
	}

	data, err := json.MarshalIndent(output, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to serialize groups: %w", err)
	}
	fmt.Println(string(data))
	return nil
}
```

- [ ] **Step 2: Build**

```bash
go build ./...
```

Expected: no output.

- [ ] **Step 3: Run full test suite**

```bash
go test ./...
```

Expected: all packages pass.

- [ ] **Step 4: Document stack_groups in .terrax.yaml**

Add before the `# Extra Terraform flags` section in `.terrax.yaml`:

```yaml
# Stack group classification
# Stacks matching a group's detect string in their stack.hcl are assigned to that group.
# Each group runs as a separate terragrunt run, in depends_on order.
# The default group captures all stacks not matching any other group.
# Use `terrax groups --json` to see the computed groups for a given directory.
#
# stack_groups:
#   default:
#     depends_on: []
#
#   private_connection:
#     detect: "require_private_connection = true"
#     depends_on: [default]
#     env:
#       TF_VAR_host: "localhost"
#       TF_VAR_port: "15432"

```

- [ ] **Step 5: Commit**

```bash
git add cmd/groups.go .terrax.yaml
git commit -m "feat(cmd): add terrax groups --json subcommand for CI orchestration"
```

---

## Self-Review

**Spec coverage:**

| Spec requirement | Task |
|---|---|
| `GroupDetectConfig`, `DetectGroup`, `TopologicalSort` | Task 1 |
| Exact grep match in `stack.hcl` | Task 1 |
| Kahn's algorithm for topological sort | Task 1 |
| Cycle detection returns error | Task 1 |
| Undefined `depends_on` group = no error | Task 1 |
| `envVars` param on `executor.Run` | Task 2 |
| Env injection into `cmd.Env` | Task 2 |
| Cleanup removed from `executor.Run` | Task 2 |
| `StackGroupConfig` with mapstructure tags | Task 3 |
| `loadStackGroups` with implicit default | Task 3 |
| `buildGroupedExecution` assigns paths, sorts, returns ordered list | Task 3 |
| Absolute path used for `DetectGroup` (relPath → absPath via `filepath.Join(repoRoot, ...)`) | Task 3 |
| Cleanup moved to `cmd/root.go`, once before all groups | Task 3 |
| All 3 dispatch sites in root.go updated | Task 3 |
| `cmd/run.go` updated | Task 3 |
| `terrax groups --json` subcommand | Task 4 |
| JSON output: `groups[]`, `repo_root` | Task 4 |
| Each group entry: `name`, `depends_on`, `filters`, `env` | Task 4 |
| `.terrax.yaml` documentation | Task 4 |

**No placeholders found.**

**Type consistency:** `GroupExecution.DependsOn []string` defined in Task 3 and accessed in Task 4 — consistent. `buildGroupedExecution(filterPaths []string, repoRoot string)` defined in Task 3 and called in Task 4 — consistent.
