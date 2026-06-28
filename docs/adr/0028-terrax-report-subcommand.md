# ADR-0028: terrax report — Per-Resource Attribute Diff Report

**Status**: Accepted

**Date**: 2026-06-28

**Deciders**: TerraX Core Team

**Related**:
- [ADR-0018: Plan Analysis via --json-out-dir](0018-plan-analysis-via-json-out-dir.md)
- [ADR-0025: On-Demand Plan Summary Subcommand](0025-summary-subcommand.md)

## Context

`terrax summary` shows only change counts per stack (`+2 ~1 -0`). The TUI plan review (`terrax review`) provides interactive navigation but requires a terminal session. Neither gives a flat, scriptable, attribute-level diff suitable for:

1. Auditing specific attribute changes before applying (e.g., "which IAM statement is being removed?").
2. Including plan diffs in pull request descriptions as Markdown.
3. CI pipelines that need a human-readable report without launching a TUI.

`terraform show <binary-plan>` fills this role for a single module but requires the `.terraform/` cache and the plan binary file — both absent after `terrax`'s `--json-out-dir` run. A report derived purely from the JSON output files allows reviewing changes anywhere the JSON files are accessible.

The Terraform JSON plan format stores `before` and `after` as raw `interface{}` (nested maps, arrays, and JSON-encoded strings from `jsonencode()`). A naive diff serializes these as opaque blobs, making it impossible to identify which nested field actually changed — the primary UX complaint against early prototypes.

## Decision

Add `terrax report` as a standalone subcommand that reads existing `.terrax/plans/` JSON files and renders a full attribute-level diff per stack.

### Subcommand interface

```
terrax report [--format text|markdown] [--output <file>] [--all] [--dir <path>] [--plans-dir <path>]
```

- Default format: `text` (Lipgloss-colored terminal output).
- `--all` includes stacks with no changes; default shows only stacks with pending changes.
- `--output` redirects to a file; default is stdout.
- Format is validated before the output file is created to avoid orphaned files on invalid input.

### Renderer architecture

All rendering logic lives in `internal/plan/reporter.go` (same package as `collector.go` and `summarizer.go`). `cmd/report.go` is pure orchestration: resolve flags → `CollectFromJSONDir` → `plan.Report`. No business logic in the `cmd` layer.

```
plan.Report(report *PlanReport, opts ReportOptions) error
  ├── renderText  → Lipgloss-colored terminal diff
  └── renderMarkdown → plain Markdown (bullet lists for nested, tables for flat)
```

### Recursive attribute diff engine

`attrDiff` carries raw `interface{}` values through the diff tree:

```go
type attrDiff struct {
    key          string
    before       string     // leaf value; empty for adds and nested diffs
    after        string     // leaf value; empty for removes and nested diffs
    computed     bool
    children     []attrDiff // non-empty for map/array/JSON-string diffs
    unchangedCnt int        // equal siblings omitted from children
}
```

`recursiveDiff` produces nested children instead of opaque blobs when values are:

- **Maps** (C1): both sides are `map[string]interface{}`, or one side is nil (pure block add/delete).
- **Arrays** (C2): both sides are `[]interface{}`, or one side is nil; compared index-by-index.
- **JSON-encoded strings** (B): both sides are strings that `json.Unmarshal` successfully decodes into a map or array; decoded value is then diffed recursively.

The `jsonencode()` pattern (common in Terraform for `inline_policy`, `assume_role_policy`, etc.) is handled transparently by case B.

### Symbol derivation

`attrSymbol` derives `+`/`-`/`~` for any node recursively:

- Leaf: `before==""` → `+`; `after==""` → `-`; both set → `~`; `computed` → `~`.
- Nested with `unchangedCnt > 0`: always `~` (partial change).
- Nested with `unchangedCnt == 0`: recurse children — all `-` → `-`, all `+` → `+`, mixed → `~`.

This ensures a Statement block that is entirely deleted shows `-` at every level, not `~`.

### Color scheme (text format)

Only the diff symbol is colored; keys and values use the terminal default (white on dark backgrounds):

| Element | Color |
|---------|-------|
| `~` | Yellow `#eab308` |
| `-` | Red `#ef4444` |
| `+` | Green `#22c55e` |
| key, value | Terminal default |
| `# (N unchanged hidden)` | Dim `#888888` |

### Markdown format

For resources where all attribute diffs are leaves: the existing table format is preserved (backward-compatible). For resources with any nested diff: all attributes for that resource use bullet-list format:

```markdown
- `~` **inline_policy**
  - `~` **Statement**
    - *(4 unchanged hidden)*
    - `-` **[4]**
      - `-` **Action**: `"lambda:InvokeFunction"`
      - `-` **Sid**: `"InvokeKernLambdas"`
```

## Consequences

### Positive

- Provides a `terraform show`-equivalent derived purely from JSON plan files — no plan binary or `.terraform/` cache required.
- JSON-string decoding (`jsonencode()` pattern) and recursive map/array diffing make nested changes as readable as flat ones.
- Markdown output integrates directly into PR descriptions and CI artifact reports.
- `cmd/report.go` contains zero business logic; the renderer is fully testable without a CLI invocation.
- Symbol propagation correctly identifies pure-delete blocks as `-` rather than misleadingly showing `~`.

### Negative

- Recursive diffing of deeply nested structures (e.g., complex IAM policies with many statement levels) can produce verbose output with many indentation levels.
- Arrays are diffed index-by-index. Reordered arrays with identical elements appear as changes rather than no-ops, which can produce misleading diffs for unordered sets (e.g., security group rules).
- JSON-string decoding only triggers when both sides parse successfully. A field that changes from a JSON string to a non-JSON string falls back to an opaque blob diff with no warning.
- `internal/plan/reporter.go` now imports Lipgloss — the `internal/plan` package is no longer a pure-Go dependency-free package.

## Alternatives Considered

### Option 1: Extend `terrax summary` with a `--verbose` flag

**Description**: Add a `--detail` or `--verbose` flag to `terrax summary` that expands the count line into per-resource attribute diffs. No new subcommand; the existing `plan.Summarize` is extended.

**Pros**:

- Fewer subcommands in the CLI surface.
- Users already familiar with `terrax summary`.

**Cons**:

- `summarizer.go` already formats directly to stdout and has no concept of output format or writer injection.
- Mixing summary (group by change/no-change) and detailed diff (per-resource) in one function conflates two distinct responsibilities.
- A `--format markdown` flag on `summary` would be confusing alongside the existing emoji-based grouped output.

**Why rejected**: `terrax summary` and `terrax report` serve different audiences — summary is a quick pass/fail count for CI gates; report is a human-auditable diff. Merging them into one command with flags would degrade both use cases. Separation keeps each command's output format coherent and each implementation testable in isolation.

### Option 2: Extend `terrax review` TUI with an export command

**Description**: Add a keybinding inside the `StatePlanReview` TUI to export the currently viewed plan as Markdown or text to a file, reusing the TUI's existing tree structure.

**Pros**:

- Reuses the already-built `PlanReport` and `TreeNode` types.
- No new subcommand; export is a discoverable TUI action.

**Cons**:

- Requires a terminal session to produce the report — cannot be used in headless CI.
- The TUI tree is hierarchical (directory nodes aggregate child stats) rather than resource-flat, which is the natural format for a diff report.
- Adding file I/O to the TUI update loop introduces side effects that complicate testing.

**Why rejected**: The CI and PR-description use cases fundamentally require a non-interactive mode. A headless `terrax report` invocation satisfies these without coupling report generation to a running TUI instance.

### Option 3: Top-level key diff only (no recursion)

**Description**: Keep `attrDiff` as a flat struct (`before string`, `after string`). Show nested values (maps, arrays, `jsonencode` strings) as compact JSON blobs on a single before → after line.

**Pros**:

- Simpler implementation; no recursive traversal.
- No risk of deeply indented output for complex nested structures.

**Cons**:

- `inline_policy` and similar JSON-encoded attributes produce unreadable multi-kilobyte blobs on a single line.
- It is impossible to identify which specific field changed within a nested block — the original UX complaint that motivated this feature.
- The `emails[0].value` case (a single field change inside a nested object) appears as a full-object replacement.

**Why rejected**: The core motivation for this feature was making nested changes readable — specifically the `jsonencode()` policy pattern where a single removed Statement is buried inside a large JSON string. An opaque blob diff does not satisfy this requirement.

## Future Enhancements

**Potential Improvements**:

1. **Set-semantic array diffing**: For unordered arrays (e.g., security group rules, IAM statement lists), detect when elements are semantically reordered rather than changed and suppress false diffs.
2. **Depth limit flag**: `--depth N` to cap recursion at N levels, useful for extremely deep policy documents where full expansion is noise.
3. **JSON-string + nil decoding**: When one side of a JSON-string attribute is nil (the whole attribute is being added or removed), decode the present side and render its fields with `+`/`-` symbols rather than showing a blob.

## References

- [`internal/plan/reporter.go`](../../internal/plan/reporter.go) — diff engine and renderers
- [`cmd/report.go`](../../cmd/report.go) — Cobra subcommand
- [ADR-0018: Plan Analysis via --json-out-dir](0018-plan-analysis-via-json-out-dir.md)
- [Terraform plan JSON format](https://developer.hashicorp.com/terraform/internals/json-format)
