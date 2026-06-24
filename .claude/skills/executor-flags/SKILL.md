---
name: terrax-executor-flags
description: Use when adding, removing, or changing any Terragrunt or Terraform flag in TerraX — including new config keys in .terrax.yaml, new feature toggles, per-command flags, or first-class boolean options. Use before touching internal/executor/executor.go or internal/config/defaults.go for flag-related work.
---

# TerraX Executor Flags

## Arg Construction Order (never change this sequence)

```
buildFilterArgs(repoRoot, command, filterPaths) → []string

 1. ["run", "--filter", p1, "--filter", p2, ...]   ← pre-computed by collectTransitiveDeps
 2. appendLoggingFlags()              log_level, log_format, log_custom_format
 3. appendTerragruntFlags()           first-class booleans (non_interactive, no_color, parallelism…)
                                       NOTE: --queue-include-external is NOT emitted here — use include_dependencies
 4. appendFeatureFlags()              features.* shortcuts (tf_forward_stdout, summary_per_unit, report)
 5. appendExtraTerragruntFlags()      terragrunt.extra_flags  (global, arbitrary slice)
 6. appendCommandTerragruntFlags()    terragrunt.command_flags.<cmd>  (per-command slice)
 7. --json-out-dir=<abs>              plan only, when plan.summary_enabled OR plan.review_enabled
 8. "--"
 9. command
10. appendTerraformExtraFlags()       terraform.extra_flags  (global, arbitrary slice)
11. appendCommandTerraformFlags()     terraform.command_flags.<cmd>  (per-command slice)
```

Flags before `--` go to Terragrunt. Flags after `--` go to Terraform directly. There is no `-out=` binary injection — plan files come from `--json-out-dir`.

## Three Flag Categories

### Category 1 — Feature shortcuts (`features.*`)

For Terragrunt flags that benefit from a simple enable/disable toggle. A single key may expand to multiple flags (e.g. `report.enabled` → `--report-file <f> --report-format <fmt>`).

**Viper keys:** `features.<name>` (bool) or `features.<group>.<subkey>`

**Where to implement:** add a branch inside `appendFeatureFlags()` in `executor.go`.

```go
// In appendFeatureFlags:
if viper.GetBool("features.my_feature") {
    args = append(args, "--my-terragrunt-flag")
}
```

**Constants:** if the feature needs a default value (e.g. a file path), add it to `internal/config/defaults.go`:
```go
DefaultMyFeatureFile = ".terrax/my-output.json"
```

### Category 2 — First-class Terragrunt flags (`terragrunt.*`)

For flags with explicit typed config (bool toggles, int values). These go in `appendTerragruntFlags()`.

**Viper keys:** `terragrunt.<snake_case_name>` (bool or int)

```go
// In appendTerragruntFlags:
if viper.GetBool("terragrunt.my_flag") {
    args = append(args, "--my-terragrunt-flag")
}
```

**For arbitrary extra flags use the slices** (already wired — no code change needed):
- `terragrunt.extra_flags: ["--flag"]` → global, all commands
- `terragrunt.command_flags.plan: ["--flag"]` → plan only

### Category 3 — Terraform flags (`terraform.*`)

Flags passed after `--` directly to Terraform. Use the slices (already wired):
- `terraform.extra_flags: ["-flag"]` → global
- `terraform.command_flags.plan: ["-flag"]` → plan only

No new helper functions needed for these; just document in `.terrax.yaml`.

## Step-by-Step: Adding a New Flag

### A. New feature shortcut (e.g. `--new-feature-flag`)

- [ ] Add `DefaultNewFeatureXxx = "..."` to `internal/config/defaults.go` if it needs a default value.
- [ ] Add a branch to `appendFeatureFlags()` in `executor.go`.
- [ ] Add a test case to `TestBuildTerragruntArgs_FeatureFlags`.
- [ ] Document in `examples/terragrunt/.terrax.yaml` under `features:`.

### B. New first-class Terragrunt bool flag

- [ ] Add a branch to `appendTerragruntFlags()` in `executor.go`.
- [ ] Add a test case to `TestBuildTerragruntArgs_DynamicFlags`.
- [ ] Document in `examples/terragrunt/.terrax.yaml` under `terragrunt:`.

### C. New arbitrary flag (no code change)

Just document the viper key in `examples/terragrunt/.terrax.yaml`. The slices are already wired.

## Test Pattern (mandatory)

File: `internal/executor/executor_test.go` — package `executor` (no `_test` suffix).

```go
func TestBuildTerragruntArgs_MyFeatureFlags(t *testing.T) {
    tests := []struct {
        name    string
        command string
        myFlag  bool
        expected []string
    }{
        {
            name:     "my flag enabled",
            command:  "apply",
            myFlag:   true,
            expected: []string{"run", "--filter", "/path/to/stack", "--log-format", "pretty", "--my-flag", "--", "apply"},
        },
        {
            name:     "my flag disabled produces no extra args",
            command:  "apply",
            expected: []string{"run", "--filter", "/path/to/stack", "--log-format", "pretty", "--", "apply"},
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            resetViper()                         // mandatory — uses resetViper(), NOT viper.Reset()
            viper.Set("log_format", "pretty")    // mandatory — prevents log_format noise

            if tt.myFlag {
                viper.Set("features.my_flag", true)
            }

            args := buildFilterArgs("/repo", tt.command, []string{"/path/to/stack"})

            assert.Equal(t, tt.expected, args, "Arguments should match expected output.")
        })
    }
}
```

**Rules:**
- Use `resetViper()` (not bare `viper.Reset()`) — the helper exists in executor_test.go.
- Always set `log_format` to `"pretty"` after reset.
- Test function: `buildFilterArgs("/repo", tt.command, []string{tt.stackPath})`.
- Test function name: `TestBuildTerragruntArgs_<Category>Flags`.

## Existing Viper Keys (quick reference)

| Viper key | Flag emitted | Helper |
|---|---|---|
| `log_level` | `--log-level <v>` | appendLoggingFlags |
| `log_format` | `--log-format <v>` | appendLoggingFlags |
| `log_custom_format` | `--log-custom-format <v>` | appendLoggingFlags |
| `terragrunt.parallelism` | `--terragrunt-parallelism <n>` | appendTerragruntFlags |
| `terragrunt.no_color` | `--terragrunt-no-color` | appendTerragruntFlags |
| `terragrunt.non_interactive` | `--terragrunt-non-interactive` | appendTerragruntFlags |
| `terragrunt.ignore_dependency_errors` | `--terragrunt-ignore-dependency-errors` | appendTerragruntFlags |
| `terragrunt.ignore_external_dependencies` | `--terragrunt-ignore-external-dependencies` | appendTerragruntFlags |
| `terragrunt.include_external_dependencies` | `--terragrunt-include-external-dependencies` | appendTerragruntFlags |
| `features.tf_forward_stdout` | `--tf-forward-stdout` | appendFeatureFlags |
| `features.summary_per_unit` | `--summary-per-unit` | appendFeatureFlags |
| `features.report.enabled` | `--report-file <f> --report-format <fmt>` | appendFeatureFlags |
| `terragrunt.extra_flags` | (slice, verbatim) | appendExtraTerragruntFlags |
| `terragrunt.command_flags.<cmd>` | (slice, verbatim) | appendCommandTerragruntFlags |
| `terraform.extra_flags` | (slice, verbatim, after --) | appendTerraformExtraFlags |
| `terraform.command_flags.<cmd>` | (slice, verbatim, after --) | appendCommandTerraformFlags |
| `plan.summary_enabled` + `plan.review_enabled` | `--json-out-dir=<abs-repoRoot>/.terrax/plans` | buildFilterArgs (plan only) |

**Removed keys:** `terragrunt.queue_include_external` — replaced by top-level `include_dependencies` which controls TerraX's dependency BFS, not a Terragrunt flag.

## Common Mistakes

| Mistake | Fix |
|---|---|
| Using `buildTerragruntArgs` | That function no longer exists. Use `buildFilterArgs(repoRoot, command, filterPaths)`. |
| Using bare `viper.Reset()` in tests | Use `resetViper()` helper defined in executor_test.go. |
| Adding `--queue-include-external` | Do not add. Dependency discovery is done by TerraX via `collectTransitiveDeps`. |
| Adding `-out=` for plan binaries | Do not add. Plan data comes from `--json-out-dir`; binary approach was removed in ADR-0018. |
| Missing "flag disabled" test case | Always test that the flag is absent when the config key is not set. |
| Hardcoding a default inside the helper | Add the constant to `internal/config/defaults.go` and reference it. |
