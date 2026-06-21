# Stack Groups Design

**Date:** 2026-06-21
**Status:** Approved

---

## Goal

Allow users to classify stacks using markers in `stack.hcl`, group them into named execution buckets, and configure per-profile behavior (env var injection, exclusion) and inter-group dependencies. Each group executes as a separate `terragrunt run`, in topological order based on `depends_on`.

---

## Configuration Schema

```yaml
# Resolves via: --profile flag > CI env var > this key > "local"
active_profile: local

stack_groups:
  default:
    # No detect — captures all stacks without another marker.
    depends_on: []
    profiles:
      local: {}
      ci: {}

  private_connection:
    detect: "require_private_connection = true"
    depends_on: [default]
    profiles:
      local:
        env:
          TF_VAR_host: "localhost"
          TF_VAR_port: "15432"
      ci:
        exclude: true   # CI runs these on self-hosted runners; TerraX omits them.

  deprecated:
    detect: "deprecated = true"
    profiles:
      local:
        exclude: true
      ci:
        exclude: true
```

**Group assignment rules:**
- A stack's `stack.hcl` is scanned for each group's `detect` string (exact grep match).
- First matching group wins. If multiple groups could match, order is YAML definition order.
- Stacks matching no group go to `default`.
- If `default` is not defined in config, an implicit `default` group is created with no special behavior for any profile.

**Profile resolution (priority):**
1. `--profile <name>` CLI flag
2. `CI=true` environment variable → profile `"ci"`
3. `active_profile:` in `.terrax.yaml`
4. Default: `"local"`

---

## Architecture

### `internal/stack/markers.go` — NEW

Pure package, no viper, no UI imports.

```go
// GroupDetectConfig holds the detection pattern and dependencies for a single group.
type GroupDetectConfig struct {
    Detect    string   // exact string to grep in stack.hcl; empty = default group
    DependsOn []string // group names this group must wait for
}

// DetectGroup returns the name of the first group whose detect pattern is found
// in <stackPath>/stack.hcl. Returns "default" if no pattern matches or if no
// group has a non-empty detect string for this stack.
func DetectGroup(stackPath string, groups map[string]GroupDetectConfig) string

// TopologicalSort returns group names in a valid execution order respecting
// all depends_on relationships. Returns an error if a cycle is detected.
// Groups with no depends_on appear first; the implicit "default" group is
// always placed before any group that depends_on it.
func TopologicalSort(groups map[string]GroupDetectConfig) ([]string, error)
```

`DetectGroup` implementation:
1. Build `<stackPath>/stack.hcl` path.
2. If file does not exist, return `"default"`.
3. Read file content. For each group with a non-empty `Detect`, check `strings.Contains(content, detect)`.
4. Return first match's name, or `"default"` if none match.

`TopologicalSort` implementation: Kahn's algorithm (BFS). Nodes = group names. Edges = `depends_on` relationships. Returns error on cycle.

### `internal/stack/markers_test.go` — NEW

Table-driven tests:
- `DetectGroup`: stack.hcl with matching string, no match, missing file, multiple patterns (first wins).
- `TopologicalSort`: linear chain, diamond dependency, cycle detection.

### `internal/config/defaults.go`

Add: `DefaultActiveProfile = "local"`

### `cmd/root.go`

**New flag:**
```go
rootCmd.Flags().String("profile", "", "Execution profile (local, ci, or custom)")
```

**New types (local to cmd/root.go):**

```go
// GroupProfileConfig holds per-profile behavior.
type GroupProfileConfig struct {
    Exclude bool
    Env     map[string]string
}

// StackGroupConfig is the full config for one group.
type StackGroupConfig struct {
    Detect    string
    DependsOn []string
    Profiles  map[string]GroupProfileConfig
}

// GroupExecution is one resolved group ready for execution.
type GroupExecution struct {
    Name    string
    Paths   []string
    EnvVars map[string]string
}
```

**`resolveProfile(profileFlag string) string`:**
```go
func resolveProfile(profileFlag string) string {
    if profileFlag != "" { return profileFlag }
    if os.Getenv("CI") == "true" { return "ci" }
    if p := viper.GetString("active_profile"); p != "" { return p }
    return config.DefaultActiveProfile
}
```

**`loadStackGroups() map[string]StackGroupConfig`:**
Reads `stack_groups` from viper. Ensures an implicit `"default"` entry exists even if not configured.

**`buildGroupedExecution(filterPaths []string, profile string) ([]GroupExecution, error)`:**
1. Load group configs via `loadStackGroups()`.
2. Build detect configs: `map[string]markers.GroupDetectConfig`.
3. Assign each path to a group via `markers.DetectGroup(path, detectConfigs)`.
4. Apply profile: for each group, look up `profiles[profile]`.
   - If `Exclude: true` → drop all paths for this group entirely.
   - Collect `Env` map.
5. Remove groups with no paths remaining.
6. Topological sort remaining groups.
7. Return `[]GroupExecution` in execution order.

**Updated dispatch in all 3 execution sites (normal TUI, --last, --history):**

```go
// Before execution loop — clear output dir once for all groups.
if command == "plan" {
    _ = os.RemoveAll(filepath.Join(repoRoot, config.DefaultOutputDir))
}

groups, err := buildGroupedExecution(filterPaths, resolveProfile(profileFlag))
if err != nil {
    return fmt.Errorf("failed to build group execution plan: %w", err)
}

for _, group := range groups {
    if err := executor.Run(ctx, historyService, command, stackPath, repoRoot, group.Paths, group.EnvVars); err != nil {
        return err
    }
}

if command == "plan" && viper.GetBool("plan.summary_enabled") {
    if err := runPlanSummary(ctx, stackPath, repoRoot); err != nil { ... }
}
if command == "plan" && viper.GetBool("plan.review_enabled") {
    return runPlanReview(ctx, stackPath)
}
```

Note: the pre-execution `.terrax/` cleanup moves from `executor.Run` to here so it happens once before all groups, not before each group's run.

### `internal/executor/executor.go`

**`Run` signature change:**
```go
func Run(ctx context.Context, historyLogger HistoryLogger, command, absoluteStackPath, repoRoot string, filterPaths []string, envVars map[string]string) error
```

**Env var injection (in `Run`, before `cmd.Run()`):**
```go
if len(envVars) > 0 {
    env := os.Environ()
    for k, v := range envVars {
        env = append(env, fmt.Sprintf("%s=%s", k, v))
    }
    cmd.Env = env
}
```

**Remove** the `.terrax/` cleanup from `Run` (moved to `cmd/root.go`).

---

## Pre-execution Cleanup Change

Currently `executor.Run` clears `.terrax/` before each plan run. With grouped execution, clearing before each group would destroy previous groups' JSON files. **Move cleanup to `cmd/root.go`** — once, before the first group run.

---

## Testing

### `internal/stack/markers_test.go`

```go
func TestDetectGroup(t *testing.T) — table driven:
  - stack.hcl contains detect string → returns group name
  - stack.hcl does not contain detect string → returns "default"
  - stack.hcl does not exist → returns "default"
  - multiple groups, first defined one matches → returns that group

func TestTopologicalSort(t *testing.T) — table driven:
  - single group, no deps → [group]
  - A → [B] (A depends on B) → [B, A]
  - cycle A→B→A → error
  - diamond: C→[A,B], B→[A] → [A, B, C]
```

### `cmd/root.go` — existing tests

`TestInitConfig` already tests viper loading. Add:
- `TestResolveProfile` — tests all 4 priority levels.
- `TestBuildGroupedExecution` — tests group assignment, profile exclusion, topological order.

---

## Edge Cases

| Case | Behavior |
|---|---|
| No `stack_groups` configured | Single implicit `default` group; all stacks execute as before |
| Group has `exclude: true` for active profile | All paths for that group are silently dropped |
| Group has paths but all excluded | Group not created in execution list |
| Cycle in `depends_on` | `buildGroupedExecution` returns error; execution aborted with clear message |
| Stack matches no detect pattern | Assigned to `default` group |
| `default` group has `exclude: true` | All unclassified stacks are excluded; only explicitly grouped stacks run |
| Group referenced in `depends_on` does not exist | Treated as already-complete (skip, no error) |

---

## Out of Scope

- Parallel group execution (groups with independent `depends_on` could run concurrently — future enhancement).
- TUI visual markers per group (future enhancement).
- Per-group Terragrunt parallelism (`--terragrunt-parallelism`) — future enhancement.
- `RunForSummary` grouped execution — summary/review reads the combined `.terrax/plans/` output from all groups.
