# Extensible Flags Configuration Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Allow users to configure TerraX feature shortcuts, extra Terragrunt flags per command, and extra Terraform flags per command via `.terrax.yaml`.

**Architecture:** All changes are confined to `internal/config/defaults.go` (two new constants) and `internal/executor/executor.go` (five new helper functions, one inline refactor, `buildTerragruntArgs` wiring). Tests live in the existing `internal/executor/executor_test.go`. No new files created.

**Tech Stack:** Go 1.25.5 · `github.com/spf13/viper` · `github.com/stretchr/testify`

## Global Constraints

- All comments must end with periods.
- Imports: three groups — stdlib, third-party, `github.com/israoo/terrax/...` — alphabetically sorted, blank lines between groups.
- Tests: table-driven format, `viper.Reset()` at the start of every test case, `assert.Equal(t, tt.expected, args, "message.")` assertion pattern.
- Test function names follow `TestBuildTerragruntArgs_<Feature>` convention (matches existing tests).
- Tests live in `package executor` (no `_test` suffix — internal package tests, same as existing file).
- No new files. Modify only the four files listed below.

---

## File Map

| File | Change |
|------|--------|
| `internal/config/defaults.go` | Add `DefaultReportFile` and `DefaultReportFormat` constants. |
| `internal/executor/executor.go` | Add `appendFeatureFlags`, `appendExtraTerragruntFlags` (refactored from inline), `appendCommandTerragruntFlags`, `appendTerraformExtraFlags`, `appendCommandTerraformFlags`. Update `buildTerragruntArgs` to call them. |
| `internal/executor/executor_test.go` | Add three new test functions for the new helpers. |
| `examples/terragrunt/.terrax.yaml` | Add `features`, `terragrunt.command_flags`, and `terraform` sections. |

---

### Task 1: Add report constants to config/defaults.go

**Files:**
- Modify: `internal/config/defaults.go`

**Interfaces:**
- Produces: `config.DefaultReportFile = "./tmp/report.json"` and `config.DefaultReportFormat = "json"` — consumed by `appendFeatureFlags` in Task 2.

- [ ] **Step 1: Add two constants at the end of the existing `const` block**

Open `internal/config/defaults.go`. The current `const` block ends at line 36 with `DefaultNoColor = false`. Add the two new constants directly after it, before the closing `)`:

```go
	// DefaultReportFile is the default path for the Terragrunt report file.
	DefaultReportFile = "./tmp/report.json"

	// DefaultReportFormat is the default format for the Terragrunt report file.
	DefaultReportFormat = "json"
```

The complete updated `const` block becomes:

```go
// Default configuration values for TerraX.
const (
	// DefaultMaxNavigationColumns is the default number of navigation columns visible simultaneously.
	DefaultMaxNavigationColumns = 3

	// MinMaxNavigationColumns is the minimum allowed value for max navigation columns.
	MinMaxNavigationColumns = 1

	// DefaultHistoryMaxEntries is the default maximum number of history entries to keep.
	// When the history exceeds this limit, older entries are automatically trimmed.
	DefaultHistoryMaxEntries = 500

	// MinHistoryMaxEntries is the minimum allowed value for history max entries.
	MinHistoryMaxEntries = 10

	// DefaultRootConfigFile is the default name of the root configuration file
	// used to determine the project root directory.
	DefaultRootConfigFile = "root.hcl"

	// DefaultLogFormat is the default terragrunt log format.
	DefaultLogFormat = "pretty"

	// DefaultParallelism is the default number of modules to run in parallel.
	// 0 means use terragrunt's default.
	DefaultParallelism = 0

	// DefaultNoColor controls whether to disable colored output.
	DefaultNoColor = false

	// DefaultReportFile is the default path for the Terragrunt report file.
	DefaultReportFile = "./tmp/report.json"

	// DefaultReportFormat is the default format for the Terragrunt report file.
	DefaultReportFormat = "json"
)
```

- [ ] **Step 2: Build to confirm no syntax errors**

```bash
go build ./internal/config/...
```

Expected: no output (success).

- [ ] **Step 3: Commit**

```bash
git add internal/config/defaults.go
git commit -m "feat(config): add DefaultReportFile and DefaultReportFormat constants"
```

---

### Task 2: Feature flags shortcut (appendFeatureFlags)

**Files:**
- Modify: `internal/executor/executor_test.go`
- Modify: `internal/executor/executor.go`

**Interfaces:**
- Consumes: `config.DefaultReportFile`, `config.DefaultReportFormat` from Task 1.
- Produces: `appendFeatureFlags(args []string) []string`, `appendExtraTerragruntFlags(args []string) []string` — consumed by `buildTerragruntArgs` in this same task.

- [ ] **Step 1: Write the failing test**

Add this function at the end of `internal/executor/executor_test.go`:

```go
// TestBuildTerragruntArgs_FeatureFlags tests feature flag shortcuts via buildTerragruntArgs.
func TestBuildTerragruntArgs_FeatureFlags(t *testing.T) {
	tests := []struct {
		name            string
		stackPath       string
		command         string
		tfForwardStdout bool
		summaryPerUnit  bool
		reportEnabled   bool
		reportFile      string
		reportFormat    string
		expected        []string
	}{
		{
			name:            "tf_forward_stdout enabled",
			stackPath:       "/path/to/stack",
			command:         "apply",
			tfForwardStdout: true,
			expected:        []string{"run", "--all", "--working-dir", "/path/to/stack", "--log-format", "pretty", "--tf-forward-stdout", "--", "apply"},
		},
		{
			name:           "summary_per_unit enabled",
			stackPath:      "/path/to/stack",
			command:        "apply",
			summaryPerUnit: true,
			expected:       []string{"run", "--all", "--working-dir", "/path/to/stack", "--log-format", "pretty", "--summary-per-unit", "--", "apply"},
		},
		{
			name:          "report enabled uses defaults",
			stackPath:     "/path/to/stack",
			command:       "apply",
			reportEnabled: true,
			expected:      []string{"run", "--all", "--working-dir", "/path/to/stack", "--log-format", "pretty", "--report-file", "./tmp/report.json", "--report-format", "json", "--", "apply"},
		},
		{
			name:          "report enabled with custom file and format",
			stackPath:     "/path/to/stack",
			command:       "apply",
			reportEnabled: true,
			reportFile:    "./custom/report.json",
			reportFormat:  "key-value",
			expected:      []string{"run", "--all", "--working-dir", "/path/to/stack", "--log-format", "pretty", "--report-file", "./custom/report.json", "--report-format", "key-value", "--", "apply"},
		},
		{
			name:            "all features enabled for plan command",
			stackPath:       "/path/to/stack",
			command:         "plan",
			tfForwardStdout: true,
			summaryPerUnit:  true,
			reportEnabled:   true,
			expected:        []string{"run", "--all", "--working-dir", "/path/to/stack", "--log-format", "pretty", "--tf-forward-stdout", "--summary-per-unit", "--report-file", "./tmp/report.json", "--report-format", "json", "--", "plan", "-out=terrax-tfplan-0.binary"},
		},
		{
			name:      "no features enabled produces no extra flags",
			stackPath: "/path/to/stack",
			command:   "apply",
			expected:  []string{"run", "--all", "--working-dir", "/path/to/stack", "--log-format", "pretty", "--", "apply"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			viper.Reset()
			viper.Set("log_format", "pretty")

			if tt.tfForwardStdout {
				viper.Set("features.tf_forward_stdout", true)
			}
			if tt.summaryPerUnit {
				viper.Set("features.summary_per_unit", true)
			}
			if tt.reportEnabled {
				viper.Set("features.report.enabled", true)
				if tt.reportFile != "" {
					viper.Set("features.report.file", tt.reportFile)
				}
				if tt.reportFormat != "" {
					viper.Set("features.report.format", tt.reportFormat)
				}
			}

			args := buildTerragruntArgs(tt.stackPath, tt.command)

			assert.Equal(t, tt.expected, args, "Arguments should match expected output.")
		})
	}
}
```

- [ ] **Step 2: Run the test to confirm it fails**

```bash
go test ./internal/executor/... -run TestBuildTerragruntArgs_FeatureFlags -v
```

Expected: `FAIL` — `appendFeatureFlags` does not exist yet.

- [ ] **Step 3: Implement appendFeatureFlags and appendExtraTerragruntFlags, update buildTerragruntArgs**

Replace the entire `buildTerragruntArgs` function and the inline `extra_flags` block in `internal/executor/executor.go`. The section to replace is lines 74–97 (the `buildTerragruntArgs` function). Replace with:

```go
// buildTerragruntArgs constructs the full Terragrunt command arguments.
func buildTerragruntArgs(absoluteStackPath, command string) []string {
	args := []string{"run", "--all", "--working-dir", absoluteStackPath}

	args = appendLoggingFlags(args)
	args = appendTerragruntFlags(args)
	args = appendFeatureFlags(args)
	args = appendExtraTerragruntFlags(args)

	args = append(args, "--", command)

	// If command is "plan", output to a binary file for later analysis.
	if command == "plan" {
		timestamp := viper.GetInt64("terrax.session_timestamp")
		planFile := fmt.Sprintf("terrax-tfplan-%d.binary", timestamp)
		args = append(args, fmt.Sprintf("-out=%s", planFile))
	}

	return args
}

// appendFeatureFlags appends flags derived from the features.* configuration section.
// Each feature key maps to one or more Terragrunt flags, hiding multi-flag complexity
// behind a single boolean toggle.
func appendFeatureFlags(args []string) []string {
	if viper.GetBool("features.tf_forward_stdout") {
		args = append(args, "--tf-forward-stdout")
	}
	if viper.GetBool("features.summary_per_unit") {
		args = append(args, "--summary-per-unit")
	}
	if viper.GetBool("features.report.enabled") {
		reportFile := viper.GetString("features.report.file")
		if reportFile == "" {
			reportFile = config.DefaultReportFile
		}
		reportFormat := viper.GetString("features.report.format")
		if reportFormat == "" {
			reportFormat = config.DefaultReportFormat
		}
		args = append(args, "--report-file", reportFile, "--report-format", reportFormat)
	}
	return args
}

// appendExtraTerragruntFlags appends global extra Terragrunt flags from terragrunt.extra_flags.
func appendExtraTerragruntFlags(args []string) []string {
	return append(args, viper.GetStringSlice("terragrunt.extra_flags")...)
}
```

- [ ] **Step 4: Run all executor tests to confirm nothing regressed**

```bash
go test ./internal/executor/... -v
```

Expected: all tests pass, including the pre-existing `TestBuildTerragruntArgs` and `TestBuildTerragruntArgs_DynamicFlags`.

- [ ] **Step 5: Commit**

```bash
git add internal/executor/executor.go internal/executor/executor_test.go
git commit -m "feat(executor): add feature flags shortcut configuration"
```

---

### Task 3: Per-command Terragrunt flags (appendCommandTerragruntFlags)

**Files:**
- Modify: `internal/executor/executor_test.go`
- Modify: `internal/executor/executor.go`

**Interfaces:**
- Consumes: `buildTerragruntArgs` and `appendExtraTerragruntFlags` from Task 2 (both already in the file).
- Produces: `appendCommandTerragruntFlags(args []string, command string) []string` — wired into `buildTerragruntArgs`.

- [ ] **Step 1: Write the failing test**

Add this function at the end of `internal/executor/executor_test.go`:

```go
// TestBuildTerragruntArgs_CommandTerragruntFlags tests per-command Terragrunt flags via buildTerragruntArgs.
func TestBuildTerragruntArgs_CommandTerragruntFlags(t *testing.T) {
	tests := []struct {
		name         string
		stackPath    string
		command      string
		commandFlags map[string][]string
		expected     []string
	}{
		{
			name:      "plan-specific flags are appended before separator",
			stackPath: "/path/to/stack",
			command:   "plan",
			commandFlags: map[string][]string{
				"plan": {"--out-dir=./plans", "--json-out-dir=./json-plans"},
			},
			expected: []string{"run", "--all", "--working-dir", "/path/to/stack", "--log-format", "pretty", "--out-dir=./plans", "--json-out-dir=./json-plans", "--", "plan", "-out=terrax-tfplan-0.binary"},
		},
		{
			name:      "apply-specific flags are appended for apply command",
			stackPath: "/path/to/stack",
			command:   "apply",
			commandFlags: map[string][]string{
				"apply": {"--custom-flag"},
			},
			expected: []string{"run", "--all", "--working-dir", "/path/to/stack", "--log-format", "pretty", "--custom-flag", "--", "apply"},
		},
		{
			name:      "plan flags not applied when running apply",
			stackPath: "/path/to/stack",
			command:   "apply",
			commandFlags: map[string][]string{
				"plan": {"--out-dir=./plans"},
			},
			expected: []string{"run", "--all", "--working-dir", "/path/to/stack", "--log-format", "pretty", "--", "apply"},
		},
		{
			name:      "no command flags set produces no extra args",
			stackPath: "/path/to/stack",
			command:   "plan",
			expected:  []string{"run", "--all", "--working-dir", "/path/to/stack", "--log-format", "pretty", "--", "plan", "-out=terrax-tfplan-0.binary"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			viper.Reset()
			viper.Set("log_format", "pretty")

			for cmd, flags := range tt.commandFlags {
				viper.Set(fmt.Sprintf("terragrunt.command_flags.%s", cmd), flags)
			}

			args := buildTerragruntArgs(tt.stackPath, tt.command)

			assert.Equal(t, tt.expected, args, "Arguments should match expected output.")
		})
	}
}
```

- [ ] **Step 2: Run the test to confirm it fails**

```bash
go test ./internal/executor/... -run TestBuildTerragruntArgs_CommandTerragruntFlags -v
```

Expected: `FAIL` — `appendCommandTerragruntFlags` does not exist yet.

- [ ] **Step 3: Add appendCommandTerragruntFlags and update buildTerragruntArgs**

Replace the `buildTerragruntArgs` function (which currently ends after `appendExtraTerragruntFlags`) with this updated version, and add the new helper below `appendExtraTerragruntFlags`:

```go
// buildTerragruntArgs constructs the full Terragrunt command arguments.
func buildTerragruntArgs(absoluteStackPath, command string) []string {
	args := []string{"run", "--all", "--working-dir", absoluteStackPath}

	args = appendLoggingFlags(args)
	args = appendTerragruntFlags(args)
	args = appendFeatureFlags(args)
	args = appendExtraTerragruntFlags(args)
	args = appendCommandTerragruntFlags(args, command)

	args = append(args, "--", command)

	// If command is "plan", output to a binary file for later analysis.
	if command == "plan" {
		timestamp := viper.GetInt64("terrax.session_timestamp")
		planFile := fmt.Sprintf("terrax-tfplan-%d.binary", timestamp)
		args = append(args, fmt.Sprintf("-out=%s", planFile))
	}

	return args
}
```

Add the new helper directly after `appendExtraTerragruntFlags`:

```go
// appendCommandTerragruntFlags appends per-command Terragrunt flags from
// terragrunt.command_flags.<command>. Only the flags for the active command are added.
func appendCommandTerragruntFlags(args []string, command string) []string {
	return append(args, viper.GetStringSlice(fmt.Sprintf("terragrunt.command_flags.%s", command))...)
}
```

- [ ] **Step 4: Run all executor tests**

```bash
go test ./internal/executor/... -v
```

Expected: all tests pass.

- [ ] **Step 5: Commit**

```bash
git add internal/executor/executor.go internal/executor/executor_test.go
git commit -m "feat(executor): add per-command Terragrunt flags configuration"
```

---

### Task 4: Terraform flags (appendTerraformExtraFlags + appendCommandTerraformFlags)

**Files:**
- Modify: `internal/executor/executor_test.go`
- Modify: `internal/executor/executor.go`

**Interfaces:**
- Consumes: `buildTerragruntArgs` from Task 3.
- Produces: `appendTerraformExtraFlags(args []string) []string`, `appendCommandTerraformFlags(args []string, command string) []string` — both wired into `buildTerragruntArgs`.

- [ ] **Step 1: Write the failing test**

Add this function at the end of `internal/executor/executor_test.go`:

```go
// TestBuildTerragruntArgs_TerraformFlags tests Terraform flags (after --) via buildTerragruntArgs.
func TestBuildTerragruntArgs_TerraformFlags(t *testing.T) {
	tests := []struct {
		name             string
		stackPath        string
		command          string
		terraformExtra   []string
		terraformCommand map[string][]string
		expected         []string
	}{
		{
			name:           "global terraform flags appended after command",
			stackPath:      "/path/to/stack",
			command:        "apply",
			terraformExtra: []string{"-no-color"},
			expected:       []string{"run", "--all", "--working-dir", "/path/to/stack", "--log-format", "pretty", "--", "apply", "-no-color"},
		},
		{
			name:      "plan-specific terraform flag appended before binary out flag",
			stackPath: "/path/to/stack",
			command:   "plan",
			terraformCommand: map[string][]string{
				"plan": {"-detailed-exitcode"},
			},
			expected: []string{"run", "--all", "--working-dir", "/path/to/stack", "--log-format", "pretty", "--", "plan", "-detailed-exitcode", "-out=terrax-tfplan-0.binary"},
		},
		{
			name:      "plan command flags not applied to apply",
			stackPath: "/path/to/stack",
			command:   "apply",
			terraformCommand: map[string][]string{
				"plan": {"-detailed-exitcode"},
			},
			expected: []string{"run", "--all", "--working-dir", "/path/to/stack", "--log-format", "pretty", "--", "apply"},
		},
		{
			name:           "global and per-command terraform flags combined for plan",
			stackPath:      "/path/to/stack",
			command:        "plan",
			terraformExtra: []string{"-input=false"},
			terraformCommand: map[string][]string{
				"plan": {"-detailed-exitcode"},
			},
			expected: []string{"run", "--all", "--working-dir", "/path/to/stack", "--log-format", "pretty", "--", "plan", "-input=false", "-detailed-exitcode", "-out=terrax-tfplan-0.binary"},
		},
		{
			name:      "no terraform flags produces no extra args",
			stackPath: "/path/to/stack",
			command:   "apply",
			expected:  []string{"run", "--all", "--working-dir", "/path/to/stack", "--log-format", "pretty", "--", "apply"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			viper.Reset()
			viper.Set("log_format", "pretty")

			if len(tt.terraformExtra) > 0 {
				viper.Set("terraform.extra_flags", tt.terraformExtra)
			}
			for cmd, flags := range tt.terraformCommand {
				viper.Set(fmt.Sprintf("terraform.command_flags.%s", cmd), flags)
			}

			args := buildTerragruntArgs(tt.stackPath, tt.command)

			assert.Equal(t, tt.expected, args, "Arguments should match expected output.")
		})
	}
}
```

- [ ] **Step 2: Run the test to confirm it fails**

```bash
go test ./internal/executor/... -run TestBuildTerragruntArgs_TerraformFlags -v
```

Expected: `FAIL` — `appendTerraformExtraFlags` and `appendCommandTerraformFlags` do not exist yet.

- [ ] **Step 3: Add both helpers and wire into buildTerragruntArgs**

Replace `buildTerragruntArgs` with its final form, then add the two new helpers after `appendCommandTerragruntFlags`:

```go
// buildTerragruntArgs constructs the full Terragrunt command arguments.
func buildTerragruntArgs(absoluteStackPath, command string) []string {
	args := []string{"run", "--all", "--working-dir", absoluteStackPath}

	args = appendLoggingFlags(args)
	args = appendTerragruntFlags(args)
	args = appendFeatureFlags(args)
	args = appendExtraTerragruntFlags(args)
	args = appendCommandTerragruntFlags(args, command)

	args = append(args, "--", command)

	args = appendTerraformExtraFlags(args)
	args = appendCommandTerraformFlags(args, command)

	// If command is "plan", output to a binary file for later analysis.
	if command == "plan" {
		timestamp := viper.GetInt64("terrax.session_timestamp")
		planFile := fmt.Sprintf("terrax-tfplan-%d.binary", timestamp)
		args = append(args, fmt.Sprintf("-out=%s", planFile))
	}

	return args
}
```

Add the two new helpers directly after `appendCommandTerragruntFlags`:

```go
// appendTerraformExtraFlags appends global extra Terraform flags from terraform.extra_flags.
// These flags are passed to Terraform directly, after the -- separator.
func appendTerraformExtraFlags(args []string) []string {
	return append(args, viper.GetStringSlice("terraform.extra_flags")...)
}

// appendCommandTerraformFlags appends per-command Terraform flags from
// terraform.command_flags.<command>. Only the flags for the active command are added.
func appendCommandTerraformFlags(args []string, command string) []string {
	return append(args, viper.GetStringSlice(fmt.Sprintf("terraform.command_flags.%s", command))...)
}
```

- [ ] **Step 4: Run all executor tests**

```bash
go test ./internal/executor/... -v
```

Expected: all tests pass — including all pre-existing tests (`TestBuildTerragruntArgs`, `TestBuildTerragruntArgs_DynamicFlags`, `TestDisplayExecutionSummary`, `TestLogExecutionToHistory`) and the three new test functions.

- [ ] **Step 5: Run the full test suite**

```bash
go test ./...
```

Expected: `ok` for every package.

- [ ] **Step 6: Commit**

```bash
git add internal/executor/executor.go internal/executor/executor_test.go
git commit -m "feat(executor): add Terraform extra flags and per-command Terraform flags"
```

---

### Task 5: Document new config options in .terrax.yaml example

**Files:**
- Modify: `examples/terragrunt/.terrax.yaml`

**Interfaces:**
- Consumes: nothing (documentation only).
- Produces: nothing consumed by code.

- [ ] **Step 1: Add three new sections to the example config**

The current file ends with:
```yaml
terragrunt:
  queue_include_external: true
```

Append the following to the end of `examples/terragrunt/.terrax.yaml`:

```yaml

# TerraX feature shortcuts
# These enable high-level Terragrunt behaviors with simple boolean toggles.
features:
  # Forward Terraform stdout directly without Terragrunt prefixes.
  # Equivalent to --tf-forward-stdout.
  # Default: false
  tf_forward_stdout: true

  # Print a summary per stack unit after execution.
  # Equivalent to --summary-per-unit.
  # Default: false
  summary_per_unit: true

  # Generate a structured JSON report file after execution.
  # Equivalent to --report-file <file> --report-format <format>.
  report:
    enabled: true
    # Default: "./tmp/report.json"
    file: "./tmp/report.json"
    # Default: "json"
    format: "json"

# Per-command extra Terragrunt flags (added before the -- separator).
# Use this for flags only relevant to specific commands.
# terragrunt:
#   command_flags:
#     plan:
#       - "--out-dir=./tmp/plans"
#       - "--json-out-dir=./tmp/json-plans"

# Extra Terraform flags (added after the -- separator, passed directly to Terraform).
# terraform:
#   extra_flags:
#     - "-input=false"
#   command_flags:
#     plan:
#       - "-detailed-exitcode"
```

- [ ] **Step 2: Build to confirm YAML is valid (go build parses it)**

```bash
go build ./...
```

Expected: no output (success).

- [ ] **Step 3: Commit**

```bash
git add examples/terragrunt/.terrax.yaml
git commit -m "docs(config): document features, terragrunt.command_flags, and terraform flags"
```

---

## Self-Review

**Spec coverage check:**

| Spec requirement | Task |
|-----------------|------|
| `features.tf_forward_stdout` toggle | Task 2 |
| `features.summary_per_unit` toggle | Task 2 |
| `features.report.enabled/file/format` | Task 2 (uses `DefaultReportFile`, `DefaultReportFormat` from Task 1) |
| `terragrunt.extra_flags` refactored to function | Task 2 |
| `terragrunt.command_flags.<cmd>` | Task 3 |
| `terraform.extra_flags` | Task 4 |
| `terraform.command_flags.<cmd>` | Task 4 |
| Config example updated | Task 5 |
| All comments end with periods | ✓ in all code blocks |
| `viper.Reset()` in every test case | ✓ in all test blocks |
| `assert.Equal` assertion pattern | ✓ in all test blocks |

**No placeholders found.**

**Type consistency:** `appendCommandTerragruntFlags` and `appendCommandTerraformFlags` both take `(args []string, command string) []string` — consistent across Tasks 3 and 4. `buildTerragruntArgs` signature unchanged across all tasks.
