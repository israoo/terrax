# Extensible Flags Configuration Design

**Date:** 2026-06-19
**Status:** Approved

---

## Goal

Allow users to configure three categories of extra flags via `.terrax.yaml`: TerraX feature shortcuts (enable/disable high-level behaviors), raw Terragrunt flags (global and per-command), and raw Terraform flags passed after the `--` separator (global and per-command).

## Context

The executor already supports several first-class Terragrunt flags (`non_interactive`, `no_color`, `parallelism`, etc.) and a generic `terragrunt.extra_flags` slice for global arbitrary flags. The missing pieces are:

1. **Feature shortcuts** — flags like `--tf-forward-stdout`, `--summary-per-unit`, and `--report-file + --report-format json` that come as a bundle and benefit from a simple `enabled: true` syntax.
2. **Per-command Terragrunt flags** — flags like `--out-dir` and `--json-out-dir` that only apply to specific commands (e.g. `plan`).
3. **Terraform flags** — flags passed after `--` to Terraform itself (global and per-command), such as `-no-color` globally or `-detailed-exitcode` only for `plan`.

---

## Configuration Schema

```yaml
# TerraX feature shortcuts
features:
  tf_forward_stdout: false      # --tf-forward-stdout
  summary_per_unit: false       # --summary-per-unit
  report:
    enabled: false              # --report-file <file> --report-format <format>
    file: "./tmp/report.json"
    format: "json"

# Terragrunt flags (all go before the -- separator)
terragrunt:
  # ... existing flags unchanged ...
  command_flags:                # Per-command extra terragrunt flags (NEW)
    plan:
      - "--out-dir=./plans"
      - "--json-out-dir=./json-plans"

# Terraform flags (all go after the -- separator)
terraform:
  extra_flags:                  # Global extra terraform flags (NEW)
    - "-no-color"
  command_flags:                # Per-command extra terraform flags (NEW)
    plan:
      - "-detailed-exitcode"
```

---

## Architecture

### Files modified

| File | Change |
|------|--------|
| `internal/config/defaults.go` | Add `DefaultReportFile`, `DefaultReportFormat` constants. |
| `internal/executor/executor.go` | Extend `buildTerragruntArgs`; add 4 new helper functions; rename inline extra_flags block to `appendExtraTerragruntFlags`. |
| `internal/executor/executor_test.go` | Add table-driven tests for every new helper. |
| `examples/terragrunt/.terrax.yaml` | Document new config sections with examples. |

No new files are created. `executor.go` stays well under 300 lines.

### Arg construction order (updated)

```
buildTerragruntArgs(stackPath, command) → []string

  1. ["run", "--all", "--working-dir", stackPath]
  2. appendLoggingFlags()                         ← unchanged
  3. appendTerragruntFlags()                      ← unchanged (non_interactive, no_color, etc.)
  4. appendFeatureFlags()                         ← NEW: features.*
  5. appendExtraTerragruntFlags()                 ← renamed from inline extra_flags block
  6. appendCommandTerragruntFlags(command)        ← NEW: terragrunt.command_flags.<command>
  7. "--"
  8. command
  9. appendTerraformExtraFlags()                  ← NEW: terraform.extra_flags
  10. appendCommandTerraformFlags(command)        ← NEW: terraform.command_flags.<command>
  11. plan binary -out=... (existing behavior)    ← unchanged, appended last for plan
```

### New helper signatures

All helpers follow the existing `append(args []string) []string` pattern.

```go
// appendFeatureFlags appends flags derived from the features.* configuration section.
func appendFeatureFlags(args []string) []string

// appendExtraTerragruntFlags appends global extra terragrunt flags from terragrunt.extra_flags.
func appendExtraTerragruntFlags(args []string) []string

// appendCommandTerragruntFlags appends per-command terragrunt flags from terragrunt.command_flags.<command>.
func appendCommandTerragruntFlags(args []string, command string) []string

// appendTerraformExtraFlags appends global extra terraform flags from terraform.extra_flags.
func appendTerraformExtraFlags(args []string) []string

// appendCommandTerraformFlags appends per-command terraform flags from terraform.command_flags.<command>.
func appendCommandTerraformFlags(args []string, command string) []string
```

### Feature flags expansion

`appendFeatureFlags` maps each shortcut to its underlying flag(s):

| Config key | Terragrunt flags emitted |
|-----------|--------------------------|
| `features.tf_forward_stdout: true` | `--tf-forward-stdout` |
| `features.summary_per_unit: true` | `--summary-per-unit` |
| `features.report.enabled: true` | `--report-file <file> --report-format <format>` |

`features.report.file` defaults to `"./tmp/report.json"` (via `config.DefaultReportFile`).
`features.report.format` defaults to `"json"` (via `config.DefaultReportFormat`).

---

## Standards Compliance

All implementation follows project standards from `docs/standards/`:

**Naming:**
- Unexported helpers: `lowerCamelCase` (e.g., `appendFeatureFlags`).
- Constants: exported `UpperCamelCase` (e.g., `DefaultReportFile`).

**Comments:**
- All comments end with periods.
- Package-level and exported-function doc comments required.
- Comments explain WHY, not WHAT.

**Imports:** Three groups — stdlib, third-party, `github.com/israoo/terrax/...` — alphabetically sorted within each group.

**Tests:**
- Table-driven format with `name`, config fields, and `expected []string`.
- `viper.Reset()` at the start of every test case.
- `assert.Equal(t, tt.expected, args, "message")` assertion pattern.
- Test function names: `TestAppendFeatureFlags`, `TestAppendCommandTerragruntFlags`, etc.
- Co-located in `internal/executor/executor_test.go` (same package, not `_test` suffix).

**Error handling:** No new error paths — all helpers read from viper with zero-value defaults (no errors possible). Existing warning patterns for viper are preserved.

**Viper keys:**
- `features.tf_forward_stdout` → `viper.GetBool`
- `features.summary_per_unit` → `viper.GetBool`
- `features.report.enabled` → `viper.GetBool`
- `features.report.file` → `viper.GetString`
- `features.report.format` → `viper.GetString`
- `terragrunt.command_flags.<cmd>` → `viper.GetStringSlice(fmt.Sprintf("terragrunt.command_flags.%s", command))`
- `terraform.extra_flags` → `viper.GetStringSlice`
- `terraform.command_flags.<cmd>` → `viper.GetStringSlice(fmt.Sprintf("terraform.command_flags.%s", command))`

---

## Example .terrax.yaml (full pipeline-quality config)

```yaml
features:
  tf_forward_stdout: true
  summary_per_unit: true
  report:
    enabled: true
    file: "./tmp/report.json"
    format: "json"

terragrunt:
  non_interactive: true
  command_flags:
    plan:
      - "--out-dir=./tmp/plans"
      - "--json-out-dir=./tmp/json-plans"

terraform:
  extra_flags:
    - "-input=false"
  command_flags:
    plan:
      - "-detailed-exitcode"
```

This produces the exact same `terragrunt run ...` invocation as the GitHub Actions pipeline analyzed in the reference implementation.

---

## Out of Scope

- Viper default registration for the new keys (zero-value defaults are sufficient — disabled by default).
- Validation of flag syntax (user's responsibility).
- Removal or replacement of the existing `-out=terrax-tfplan-{timestamp}.binary` behavior for the plan analysis feature.
