# Stack Groups Design

**Date:** 2026-06-21
**Status:** Approved

---

## Goal

Allow users to classify stacks using markers in `stack.hcl`, group them into named execution buckets, and configure per-group env var injection and inter-group dependencies. TerraX exposes the grouped filter lists as JSON for external consumers (CI pipelines). Locally, TerraX executes all groups in order.

---

## Configuration Schema

```yaml
stack_groups:
  default:
    # No detect — captures all stacks without another marker.
    depends_on: []

  private_connection:
    detect: "require_private_connection = true"
    depends_on: [default]
    env:
      TF_VAR_host: "localhost"
      TF_VAR_port: "15432"

  deprecated:
    detect: "deprecated = true"
    # No env, no depends_on needed — these stacks are simply grouped separately.
    depends_on: []
```

**Group assignment rules:**
- A stack's `stack.hcl` is scanned for each group's `detect` string (exact grep match).
- First matching group wins (YAML definition order).
- Stacks matching no group go to `default`.
- If `default` is not defined in config, an implicit `default` group is created with no env vars.

---

## Two Modes of Operation

### Local execution (default)

```
terrax [--dir <path>]
```

TerraX computes groups, executes each as a separate `terragrunt run` in topological order, injecting the group's `env` into each run. Simple, no configuration needed beyond `.terrax.yaml`.

### JSON endpoint (for CI orchestration)

```
terrax groups --json [--dir <path>]
```

Outputs the groups and their filter lists as JSON. The CI pipeline consumes this to orchestrate execution across runners, decide which groups to run where, etc. TerraX has no knowledge of CI — it just computes and exports the data.

```json
{
  "groups": [
    {
      "name": "default",
      "depends_on": [],
      "filters": ["workloads/dev/acm", "workloads/dev/ecr"],
      "env": {}
    },
    {
      "name": "private_connection",
      "depends_on": ["default"],
      "filters": ["workloads/dev/aurora-database"],
      "env": {"TF_VAR_host": "localhost", "TF_VAR_port": "15432"}
    }
  ],
  "repo_root": "/Users/isra/Repos/npb/hip-iac-cl-aws-caas"
}
```

---

## Architecture

### `internal/stack/markers.go` — NEW

Pure package, no viper, no UI imports.

```go
// GroupDetectConfig holds the detection pattern and dependencies for a single group.
type GroupDetectConfig struct {
    Detect    string   // exact string to grep in stack.hcl; empty = default group
    DependsOn []string // group names this group must complete before
    Env       map[string]string
}

// DetectGroup returns the name of the first group whose detect pattern matches
// a string in <stackPath>/stack.hcl. Returns "default" if no pattern matches
// or stack.hcl does not exist.
func DetectGroup(stackPath string, groups map[string]GroupDetectConfig) string

// TopologicalSort returns group names in a valid execution order respecting
// all depends_on relationships. Returns an error if a cycle is detected.
func TopologicalSort(groups map[string]GroupDetectConfig) ([]string, error)
```

`DetectGroup` implementation:
1. Read `<stackPath>/stack.hcl`. If file does not exist, return `"default"`.
2. For each group with a non-empty `Detect`, check `strings.Contains(content, detect)`.
3. Return first match's name; or `"default"` if none match.

`TopologicalSort` implementation: Kahn's algorithm (BFS). Returns error on cycle.

### `internal/stack/markers_test.go` — NEW

Table-driven tests:
- `DetectGroup`: matching, no match, missing file, multiple groups (first wins).
- `TopologicalSort`: linear chain, diamond, cycle detection.

### `internal/config/defaults.go`

No new defaults needed (groups section is empty by default = single implicit default group).

### `cmd/groups.go` — NEW

New subcommand `terrax groups --json [--dir <path>]`:

```go
// GroupOutput is the JSON structure for terrax groups --json.
type GroupOutput struct {
    Groups   []GroupEntry `json:"groups"`
    RepoRoot string       `json:"repo_root"`
}

type GroupEntry struct {
    Name      string            `json:"name"`
    DependsOn []string          `json:"depends_on"`
    Filters   []string          `json:"filters"`
    Env       map[string]string `json:"env"`
}
```

Logic:
1. Resolve `workDir` from `--dir` flag or CWD.
2. Call `collectTransitiveDeps(workDir)` → `repoRoot, allPaths`.
3. Load group configs from viper (`loadStackGroups()`).
4. Assign each path to a group via `markers.DetectGroup`.
5. Topological sort groups.
6. Marshal `GroupOutput` to JSON, print to stdout.

### `cmd/root.go`

**New types:**

```go
type StackGroupConfig struct {
    Detect    string
    DependsOn []string
    Env       map[string]string
}

type GroupExecution struct {
    Name    string
    Paths   []string
    EnvVars map[string]string
}
```

**`loadStackGroups() map[string]StackGroupConfig`:**
Reads `stack_groups` from viper. Ensures implicit `"default"` exists.

**`buildGroupedExecution(filterPaths []string) ([]GroupExecution, error)`:**
1. Load group configs.
2. Assign each path to a group via `markers.DetectGroup`.
3. Topological sort.
4. Return `[]GroupExecution` in execution order.

**Updated dispatch in all 3 execution sites:**

```go
// Clear output dir once before all groups (not per-group).
if command == "plan" && (viper.GetBool("plan.summary_enabled") || viper.GetBool("plan.review_enabled")) {
    _ = os.RemoveAll(filepath.Join(repoRoot, config.DefaultOutputDir))
}

groups, err := buildGroupedExecution(filterPaths)
if err != nil {
    return fmt.Errorf("failed to build group execution plan: %w", err)
}

for _, group := range groups {
    if err := executor.Run(ctx, historyService, command, stackPath, repoRoot, group.Paths, group.EnvVars); err != nil {
        return err
    }
}

// Plan summary/review runs once after all groups complete.
if command == "plan" && viper.GetBool("plan.summary_enabled") { ... }
if command == "plan" && viper.GetBool("plan.review_enabled") { ... }
```

### `internal/executor/executor.go`

**`Run` signature change:**

```go
func Run(ctx context.Context, historyLogger HistoryLogger, command, absoluteStackPath, repoRoot string, filterPaths []string, envVars map[string]string) error
```

**Env var injection (before `cmd.Run()`):**

```go
if len(envVars) > 0 {
    env := os.Environ()
    for k, v := range envVars {
        env = append(env, fmt.Sprintf("%s=%s", k, v))
    }
    cmd.Env = env
}
```

**Remove** the `.terrax/` cleanup from `Run` — moved to `cmd/root.go`.

---

## Pre-execution Cleanup Change

The `.terrax/` cleanup moves from `executor.Run` to `cmd/root.go`, executed once before the first group run. This ensures JSON files from multiple groups accumulate correctly in `.terrax/plans/` for the combined plan summary/review.

---

## Files Changed

| File | Change |
|---|---|
| `internal/stack/markers.go` | NEW — `DetectGroup`, `TopologicalSort`, `GroupDetectConfig` |
| `internal/stack/markers_test.go` | NEW |
| `cmd/groups.go` | NEW — `terrax groups --json` subcommand |
| `cmd/root.go` | `buildGroupedExecution`, grouped execution loop, remove cleanup from Run |
| `internal/executor/executor.go` | `Run` + `envVars` param, remove cleanup |
| `internal/executor/executor_test.go` | update for new signature |
| `.terrax.yaml` | document `stack_groups` |

---

## Edge Cases

| Case | Behavior |
|---|---|
| No `stack_groups` configured | Single implicit `default` group; all stacks execute as before — zero behavior change |
| Group with no matching stacks | Group omitted from execution and JSON output |
| Cycle in `depends_on` | Error returned; execution aborted with clear message |
| Stack matches no detect pattern | Assigned to `default` |
| Group referenced in `depends_on` not defined | Treated as already-complete (no error) |
| `terrax groups --json` with no groups configured | Returns single `default` group with all stacks |

---

## Out of Scope

- Per-group Terragrunt parallelism (`--terragrunt-parallelism`).
- Parallel group execution (groups without mutual dependencies running concurrently).
- TUI visual markers per group.
- `exclude` behavior — CI handles exclusion by simply ignoring groups from the JSON output.
