# ADR-0011: Extensible Flags Configuration

**Status**: Accepted

**Date**: 2026-06-19

**Deciders**: TerraX Core Team

**Related**:
- [ADR-0007: Configuration Management Strategy](0007-configuration-management-strategy.md)
- [ADR-0009: Executor Isolation Pattern](0009-executor-isolation-pattern.md)

## Context

TerraX executes Terragrunt commands by constructing an argument list in `internal/executor/executor.go`. The initial implementation hardcoded a small set of commonly used flags (log level, log format, parallelism, non-interactive mode, etc.) as first-class Viper keys.

As TerraX is used alongside CI/CD pipelines, users need to pass additional Terragrunt and Terraform flags that are not covered by the hardcoded set. Three recurring patterns emerged from production pipeline analysis:

1. **High-level feature bundles** — flags like `--tf-forward-stdout`, `--summary-per-unit`, and `--report-file + --report-format` that always appear together and have clear enable/disable semantics. Users should not need to remember the exact flag syntax or that `report` requires two flags.
2. **Per-command Terragrunt flags** — flags that only apply to specific commands, such as `--out-dir` and `--json-out-dir` for `plan`. Passing these globally would break other commands.
3. **Terraform flags** — flags passed after the `--` separator directly to Terraform (e.g. `-input=false`, `-detailed-exitcode`), both globally and per-command.

Without a structured extension mechanism, users were forced to either modify TerraX source code or accept incomplete pipeline parity.

## Decision

We introduce three configuration layers in `.terrax.yaml`, each handled by a dedicated helper function in `internal/executor/executor.go`:

**Layer 1 — Feature shortcuts (`features.*`):**
Boolean toggles for high-level behaviors. Each key expands to one or more Terragrunt flags:

```yaml
features:
  tf_forward_stdout: true    # → --tf-forward-stdout
  summary_per_unit: true     # → --summary-per-unit
  report:
    enabled: true            # → --report-file <file> --report-format <format>
    file: "./tmp/report.json"
    format: "json"
```

**Layer 2 — Per-command Terragrunt flags (`terragrunt.command_flags.<cmd>`):**
Arbitrary Terragrunt flags applied only when running a specific command:

```yaml
terragrunt:
  command_flags:
    plan:
      - "--out-dir=./tmp/plans"
      - "--json-out-dir=./tmp/json-plans"
```

> **Warning — `--out-dir` conflicts with TerraX plan review.** When `--out-dir` is set, Terragrunt injects its own `-out=<dir>/<stack>.binary` into each Terraform invocation, overriding the `-out=terrax-tfplan-<ts>.binary` flag that TerraX appends internally. The `plan.Collector` searches for files named `terrax-tfplan-<ts>.binary` inside `.terragrunt-cache/` directories; when `--out-dir` redirects plan files elsewhere under different names, the Collector finds nothing and the plan review feature is non-functional. `--out-dir` and `--json-out-dir` are intended for CI/CD pipelines that consume plan artifacts in subsequent steps — do not use them in `terragrunt.command_flags.plan` for local TerraX usage.

**Layer 3 — Terraform flags (`terraform.extra_flags`, `terraform.command_flags.<cmd>`):**
Flags passed after the `--` separator to Terraform directly:

```yaml
terraform:
  extra_flags:
    - "-input=false"
  command_flags:
    plan:
      - "-detailed-exitcode"
```

The argument construction order is strictly defined:

```
1. run --all --working-dir <path>
2. appendLoggingFlags          (log_level, log_format)
3. appendTerragruntFlags       (first-class booleans)
4. appendFeatureFlags          (features.*)
5. appendExtraTerragruntFlags  (terragrunt.extra_flags)
6. appendCommandTerragruntFlags(terragrunt.command_flags.<cmd>)
7. --
8. <command>
9. appendTerraformExtraFlags   (terraform.extra_flags)
10. appendCommandTerraformFlags (terraform.command_flags.<cmd>)
11. -out=<binary>              (plan only)
```

Default values for report file and format are centralized in `internal/config/defaults.go` as `DefaultReportFile` and `DefaultReportFormat`.

## Consequences

### Positive

- **Pipeline parity** — users can replicate production CI/CD pipeline flags (e.g. `--tf-forward-stdout`, `--out-dir`, `-detailed-exitcode`) without modifying TerraX source code.
- **Additive, not breaking** — all new keys default to disabled/empty, preserving existing behavior for users who do not configure them.
- **Feature abstraction** — `features.report.enabled: true` is simpler than remembering `--report-file ./tmp/report.json --report-format json` and consistent across all TerraX users.
- **Testable** — each helper function is independently tested with table-driven tests following the existing `TestBuildTerragruntArgs_*` pattern.

### Negative

- **User-facing complexity** — three configuration layers with different semantics require documentation. A user unfamiliar with the Terragrunt CLI may not know which layer a given flag belongs to.
- **No validation** — flag syntax is not validated; an invalid flag value will fail at Terragrunt execution time, not at config load time.
- **Ordering is implicit** — the arg construction order is enforced by convention, not by the type system. A future contributor adding a new helper must know to insert it in the correct position.

## Alternatives Considered

### Option 1: Single universal extra_flags slice

**Description**: Extend the existing `terragrunt.extra_flags` slice to accept all additional flags globally, with no per-command or feature-abstraction support.

**Pros**:

- Simplest possible implementation — one config key, zero new code.
- Already partially implemented (the key existed before this ADR).

**Cons**:

- Cannot scope flags to specific commands; `--out-dir` would be passed to `apply` and `destroy`, which would fail.
- No abstraction for multi-flag features like `report`; users must remember both `--report-file` and `--report-format`.
- No distinction between Terragrunt flags and Terraform flags — position relative to `--` matters and cannot be expressed in a flat slice.

**Why rejected**: The flat slice cannot satisfy per-command scoping or Terraform-vs-Terragrunt placement, which are the two most common production requirements.

### Option 2: Template-based command construction

**Description**: Allow users to define the full Terragrunt command as a template string in `.terrax.yaml`, giving complete control over all flags and their order.

**Pros**:

- Maximum flexibility — any flag combination is expressible.
- No need for TerraX to model each flag category.

**Cons**:

- Requires users to replicate TerraX's internal argument logic (working-dir, logging flags, plan binary output) in their config, making misconfiguration likely.
- Breaks TerraX's history logging, plan analysis, and other features that depend on controlled argument construction.
- Impossible to maintain safely as Terragrunt's CLI evolves.

**Why rejected**: Template-based construction delegates too much responsibility to the user and undermines TerraX's value as an abstraction over Terragrunt's CLI.

### Option 3: Per-command configuration blocks

**Description**: Define a full configuration block per command, allowing any flag to be set independently for each command.

**Pros**:

- Maximum per-command control.
- Explicit and self-documenting.

**Cons**:

- High configuration verbosity — users must repeat common flags (like `--tf-forward-stdout`) in every command block.
- Significantly more complex to implement and test.
- Most flags apply globally; per-command blocks would mostly be copies of each other.

**Why rejected**: The majority of flags are global, making per-command blocks unnecessarily verbose. The three-layer model (global features, global extras, per-command extras) covers all observed production cases with minimal configuration.

## Future Enhancements

**Potential Improvements**:

1. **Flag validation at config load** — validate known flags against a schema and warn on unknown keys before execution begins.
2. **Environment-specific profiles** — allow defining multiple named profiles (e.g. `ci`, `local`) and activating one via CLI flag or environment variable.
3. **Flag documentation in TUI** — display the active flag configuration in the TUI before command execution, so users can confirm what will be run.

## References

- `internal/executor/executor.go` — `buildTerragruntArgs` and the five helper functions
- `internal/config/defaults.go` — `DefaultReportFile`, `DefaultReportFormat`
- `examples/terragrunt/.terrax.yaml` — annotated configuration reference
- [ADR-0007: Configuration Management Strategy](0007-configuration-management-strategy.md)
- [ADR-0009: Executor Isolation Pattern](0009-executor-isolation-pattern.md)
