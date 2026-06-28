# terrax report Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add a `terrax report` subcommand that reads Terragrunt JSON plan files and renders a full per-resource attribute diff in `text` or `markdown` format.

**Architecture:** A new `internal/plan/reporter.go` contains all rendering logic (`diffAttributes`, `renderText`, `renderMarkdown`, `Report`). A new `cmd/report.go` wires the Cobra subcommand following the exact same pattern as `cmd/summary.go`. No existing files are modified.

**Tech Stack:** Go 1.25.5 · Cobra 1.10.2 · Viper 1.21.0 · Lipgloss 1.1.0 · testify (assert + require)

## Global Constraints

- All comments must end with a period.
- Imports: three groups (stdlib, third-party, `github.com/israoo/terrax/...`), alphabetically sorted within each group.
- Never use `lipgloss.Style.Copy()` — define each style with `lipgloss.NewStyle()`.
- Always use `filepath.Join()`, never hardcoded `/` or `\`.
- Errors always wrapped: `fmt.Errorf("...: %w", err)`.
- Run `task check` before every commit (fmt + vet + lint + test).
- Tests are table-driven, use `t.TempDir()` for any filesystem work, inline JSON for plan fixtures.
- Text renderer uses Lipgloss for color; markdown renderer produces plain text (no ANSI codes).
- Test assertions for text output use `strings.Contains` (ANSI codes make exact matching fragile); markdown assertions use exact/partial string matching.

---

## File Map

| File | Action | Responsibility |
|------|--------|----------------|
| `internal/plan/reporter.go` | Create | `ReportOptions`, `Format`, `attrDiff`, `diffAttributes`, `Report`, `renderText`, `renderMarkdown` |
| `internal/plan/reporter_test.go` | Create | Table-driven tests for all renderers and diffAttributes |
| `cmd/report.go` | Create | Cobra subcommand — flag parsing, file open/close, calls `CollectFromJSONDir` + `Report` |

---

## Task 1: Core renderer — `internal/plan/reporter.go`

**Files:**
- Create: `internal/plan/reporter.go`
- Test: `internal/plan/reporter_test.go`

**Interfaces:**
- Consumes: `PlanReport`, `StackResult`, `ResourceChange`, `ChangeType*` constants, `StackStats` — all from `internal/plan/models.go` (same package, no import needed).
- Produces:
  - `type Format string` with constants `FormatText Format = "text"` and `FormatMarkdown Format = "markdown"`
  - `type ReportOptions struct { Format Format; ShowAll bool; Writer io.Writer }`
  - `func Report(report *PlanReport, opts ReportOptions) error`

---

- [ ] **Step 1: Write the failing tests**

Create `internal/plan/reporter_test.go`:

```go
package plan

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// makeReport builds a minimal PlanReport for testing.
func makeReport(stacks ...StackResult) *PlanReport {
	r := &PlanReport{Stacks: stacks}
	r.calculateSummary()
	return r
}

// ---- diffAttributes tests ----

func TestDiffAttributes_Create(t *testing.T) {
	after := map[string]interface{}{
		"ami":           "ami-123",
		"instance_type": "t3.micro",
	}
	diffs := diffAttributes(nil, after, nil)
	require.Len(t, diffs, 2)
	keys := make([]string, len(diffs))
	for i, d := range diffs {
		keys[i] = d.key
	}
	assert.Contains(t, keys, "ami")
	assert.Contains(t, keys, "instance_type")
	for _, d := range diffs {
		assert.NotEmpty(t, d.after)
		assert.Empty(t, d.before)
		assert.False(t, d.computed)
	}
}

func TestDiffAttributes_Delete(t *testing.T) {
	before := map[string]interface{}{"bucket": "my-bucket"}
	diffs := diffAttributes(before, nil, nil)
	require.Len(t, diffs, 1)
	assert.Equal(t, "bucket", diffs[0].key)
	assert.NotEmpty(t, diffs[0].before)
	assert.Empty(t, diffs[0].after)
}

func TestDiffAttributes_Update_OnlyChangedKeys(t *testing.T) {
	before := map[string]interface{}{"name": "old", "type": "CNAME", "ttl": float64(300)}
	after := map[string]interface{}{"name": "new", "type": "CNAME", "ttl": float64(300)}
	diffs := diffAttributes(before, after, nil)
	require.Len(t, diffs, 1)
	assert.Equal(t, "name", diffs[0].key)
	assert.Contains(t, diffs[0].before, "old")
	assert.Contains(t, diffs[0].after, "new")
}

func TestDiffAttributes_Computed(t *testing.T) {
	after := map[string]interface{}{"id": nil}
	unknown := map[string]interface{}{"id": true}
	diffs := diffAttributes(nil, after, unknown)
	require.Len(t, diffs, 1)
	assert.True(t, diffs[0].computed)
}

func TestDiffAttributes_NilBothSides(t *testing.T) {
	diffs := diffAttributes(nil, nil, nil)
	assert.Empty(t, diffs)
}

// ---- Report/renderMarkdown tests (no ANSI codes in markdown) ----

func TestReport_Markdown_Create(t *testing.T) {
	stack := StackResult{
		StackPath:  "workloads/dev/acm",
		HasChanges: true,
		Stats:      StackStats{Add: 1},
		ResourceChanges: []ResourceChange{
			{
				Address:    "aws_acm_certificate.main",
				Type:       "aws_acm_certificate",
				Name:       "main",
				ChangeType: ChangeTypeCreate,
				Before:     nil,
				After:      map[string]interface{}{"domain_name": "new.example.com"},
				Unknown:    nil,
			},
		},
	}
	var sb strings.Builder
	err := Report(makeReport(stack), ReportOptions{Format: FormatMarkdown, Writer: &sb})
	require.NoError(t, err)
	out := sb.String()
	assert.Contains(t, out, "workloads/dev/acm")
	assert.Contains(t, out, "aws_acm_certificate.main")
	assert.Contains(t, out, "domain_name")
	assert.Contains(t, out, "new.example.com")
	// Summary line present
	assert.Contains(t, out, "+1")
}

func TestReport_Markdown_Update(t *testing.T) {
	stack := StackResult{
		StackPath:  "workloads/dev/dns",
		HasChanges: true,
		Stats:      StackStats{Change: 1},
		ResourceChanges: []ResourceChange{
			{
				Address:    "aws_route53_record.val",
				Type:       "aws_route53_record",
				Name:       "val",
				ChangeType: ChangeTypeUpdate,
				Before:     map[string]interface{}{"name": "old.example.com"},
				After:      map[string]interface{}{"name": "new.example.com"},
				Unknown:    nil,
			},
		},
	}
	var sb strings.Builder
	err := Report(makeReport(stack), ReportOptions{Format: FormatMarkdown, Writer: &sb})
	require.NoError(t, err)
	out := sb.String()
	assert.Contains(t, out, "old.example.com")
	assert.Contains(t, out, "new.example.com")
}

func TestReport_Markdown_Delete(t *testing.T) {
	stack := StackResult{
		StackPath:  "workloads/dev/s3",
		HasChanges: true,
		Stats:      StackStats{Destroy: 1},
		ResourceChanges: []ResourceChange{
			{
				Address:    "aws_s3_bucket.old",
				Type:       "aws_s3_bucket",
				Name:       "old",
				ChangeType: ChangeTypeDelete,
				Before:     map[string]interface{}{"bucket": "my-bucket"},
				After:      nil,
				Unknown:    nil,
			},
		},
	}
	var sb strings.Builder
	err := Report(makeReport(stack), ReportOptions{Format: FormatMarkdown, Writer: &sb})
	require.NoError(t, err)
	out := sb.String()
	assert.Contains(t, out, "aws_s3_bucket.old")
	assert.Contains(t, out, "my-bucket")
}

func TestReport_Markdown_Replace(t *testing.T) {
	stack := StackResult{
		StackPath:  "workloads/dev/iam",
		HasChanges: true,
		Stats:      StackStats{Add: 1, Destroy: 1},
		ResourceChanges: []ResourceChange{
			{
				Address:    "aws_iam_role.worker",
				Type:       "aws_iam_role",
				Name:       "worker",
				ChangeType: ChangeTypeReplace,
				Before:     map[string]interface{}{"name": "old-role"},
				After:      map[string]interface{}{"name": "new-role"},
				Unknown:    nil,
			},
		},
	}
	var sb strings.Builder
	err := Report(makeReport(stack), ReportOptions{Format: FormatMarkdown, Writer: &sb})
	require.NoError(t, err)
	out := sb.String()
	assert.Contains(t, out, "aws_iam_role.worker")
}

func TestReport_Markdown_Computed(t *testing.T) {
	stack := StackResult{
		StackPath:  "workloads/dev/ec2",
		HasChanges: true,
		Stats:      StackStats{Add: 1},
		ResourceChanges: []ResourceChange{
			{
				Address:    "aws_instance.web",
				Type:       "aws_instance",
				Name:       "web",
				ChangeType: ChangeTypeCreate,
				Before:     nil,
				After:      map[string]interface{}{"id": nil},
				Unknown:    map[string]interface{}{"id": true},
			},
		},
	}
	var sb strings.Builder
	err := Report(makeReport(stack), ReportOptions{Format: FormatMarkdown, Writer: &sb})
	require.NoError(t, err)
	assert.Contains(t, sb.String(), "computed")
}

func TestReport_ShowAll_False_SkipsNoChanges(t *testing.T) {
	noChange := StackResult{StackPath: "workloads/dev/noop", HasChanges: false}
	withChange := StackResult{
		StackPath:  "workloads/dev/acm",
		HasChanges: true,
		Stats:      StackStats{Add: 1},
		ResourceChanges: []ResourceChange{
			{Address: "r.r", Type: "r", Name: "r", ChangeType: ChangeTypeCreate, After: map[string]interface{}{"k": "v"}},
		},
	}
	var sb strings.Builder
	err := Report(makeReport(noChange, withChange), ReportOptions{Format: FormatMarkdown, Writer: &sb})
	require.NoError(t, err)
	out := sb.String()
	assert.NotContains(t, out, "workloads/dev/noop")
	assert.Contains(t, out, "workloads/dev/acm")
}

func TestReport_ShowAll_True_IncludesNoChanges(t *testing.T) {
	noChange := StackResult{StackPath: "workloads/dev/noop", HasChanges: false}
	var sb strings.Builder
	err := Report(makeReport(noChange), ReportOptions{Format: FormatMarkdown, ShowAll: true, Writer: &sb})
	require.NoError(t, err)
	assert.Contains(t, sb.String(), "workloads/dev/noop")
}

func TestReport_EmptyReport_NoError(t *testing.T) {
	var sb strings.Builder
	err := Report(makeReport(), ReportOptions{Format: FormatMarkdown, Writer: &sb})
	require.NoError(t, err)
}

// ---- Report/renderText tests (check key strings, not exact output) ----

func TestReport_Text_ContainsResourceAddress(t *testing.T) {
	stack := StackResult{
		StackPath:  "workloads/dev/acm",
		HasChanges: true,
		Stats:      StackStats{Add: 1},
		ResourceChanges: []ResourceChange{
			{Address: "aws_acm_certificate.main", Type: "aws_acm_certificate", Name: "main",
				ChangeType: ChangeTypeCreate, After: map[string]interface{}{"domain_name": "x.com"}},
		},
	}
	var sb strings.Builder
	err := Report(makeReport(stack), ReportOptions{Format: FormatText, Writer: &sb})
	require.NoError(t, err)
	// Strip ANSI for reliable assertions.
	plain := stripANSI(sb.String())
	assert.Contains(t, plain, "workloads/dev/acm")
	assert.Contains(t, plain, "aws_acm_certificate.main")
	assert.Contains(t, plain, "domain_name")
}

func TestReport_InvalidFormat(t *testing.T) {
	var sb strings.Builder
	err := Report(makeReport(), ReportOptions{Format: "xml", Writer: &sb})
	require.Error(t, err)
}

// stripANSI removes ANSI escape sequences for plain-text assertions.
func stripANSI(s string) string {
	var b strings.Builder
	inEsc := false
	for _, r := range s {
		if r == '\x1b' {
			inEsc = true
			continue
		}
		if inEsc {
			if r == 'm' {
				inEsc = false
			}
			continue
		}
		b.WriteRune(r)
	}
	return b.String()
}
```

- [ ] **Step 2: Run the tests to confirm they fail**

```bash
cd /path/to/terrax
go test ./internal/plan/ -run TestDiffAttributes -v
go test ./internal/plan/ -run TestReport_ -v
```

Expected: `FAIL` — `diffAttributes`, `Report`, `FormatText`, `FormatMarkdown`, `ReportOptions` undefined.

- [ ] **Step 3: Implement `internal/plan/reporter.go`**

Create `internal/plan/reporter.go`:

```go
package plan

import (
	"encoding/json"
	"fmt"
	"io"
	"sort"

	"github.com/charmbracelet/lipgloss"
)

// Format is the output format for Report.
type Format string

const (
	FormatText     Format = "text"
	FormatMarkdown Format = "markdown"
)

// ReportOptions configures the Report renderer.
type ReportOptions struct {
	Format  Format
	ShowAll bool // If false, stacks with no changes are skipped.
	Writer  io.Writer
}

// attrDiff represents a single attribute comparison between before and after.
type attrDiff struct {
	key      string
	before   string // JSON-formatted; empty for creates.
	after    string // JSON-formatted; empty for deletes.
	computed bool   // True when field is marked unknown in the plan.
}

// Report writes a full per-resource attribute diff to opts.Writer.
// Returns an error for unknown formats; stack parse warnings are printed inline.
func Report(report *PlanReport, opts ReportOptions) error {
	switch opts.Format {
	case FormatText:
		renderText(opts.Writer, report, opts)
	case FormatMarkdown:
		renderMarkdown(opts.Writer, report, opts)
	default:
		return fmt.Errorf("unknown report format %q: use text or markdown", opts.Format)
	}
	return nil
}

// diffAttributes computes the attribute-level diff between before and after.
// before and after are expected to be map[string]interface{} or nil.
// unknown is a map[string]interface{} where true marks computed fields.
func diffAttributes(before, after, unknown interface{}) []attrDiff {
	beforeMap, _ := before.(map[string]interface{})
	afterMap, _ := after.(map[string]interface{})
	unknownMap, _ := unknown.(map[string]interface{})

	// Collect all keys from both sides.
	keySet := map[string]struct{}{}
	for k := range beforeMap {
		keySet[k] = struct{}{}
	}
	for k := range afterMap {
		keySet[k] = struct{}{}
	}

	var diffs []attrDiff
	for k := range keySet {
		bVal, bExists := beforeMap[k]
		aVal, aExists := afterMap[k]

		isComputed := unknownMap[k] == true

		// For updates: skip attributes that haven't changed.
		if bExists && aExists && !isComputed && jsonEqual(bVal, aVal) {
			continue
		}

		d := attrDiff{
			key:      k,
			computed: isComputed,
		}
		if bExists && bVal != nil {
			d.before = formatValue(bVal)
		}
		if isComputed {
			d.after = "(computed)"
		} else if aExists && aVal != nil {
			d.after = formatValue(aVal)
		}
		diffs = append(diffs, d)
	}

	sort.Slice(diffs, func(i, j int) bool { return diffs[i].key < diffs[j].key })
	return diffs
}

// formatValue returns a compact JSON representation of v.
func formatValue(v interface{}) string {
	b, err := json.Marshal(v)
	if err != nil {
		return fmt.Sprintf("%v", v)
	}
	return string(b)
}

// jsonEqual returns true when a and b marshal to identical JSON.
func jsonEqual(a, b interface{}) bool {
	aJSON, _ := json.Marshal(a)
	bJSON, _ := json.Marshal(b)
	return string(aJSON) == string(bJSON)
}

// symbolFor returns the single-character symbol for a change type.
func symbolFor(ct ChangeType) string {
	switch ct {
	case ChangeTypeCreate:
		return "+"
	case ChangeTypeDelete:
		return "-"
	case ChangeTypeUpdate:
		return "~"
	case ChangeTypeReplace:
		return "±"
	default:
		return " "
	}
}

// ---- Text renderer ----

var (
	textCreate  = lipgloss.NewStyle().Foreground(lipgloss.Color("#22c55e")).Bold(true)
	textDelete  = lipgloss.NewStyle().Foreground(lipgloss.Color("#ef4444")).Bold(true)
	textUpdate  = lipgloss.NewStyle().Foreground(lipgloss.Color("#eab308")).Bold(true)
	textReplace = lipgloss.NewStyle().Foreground(lipgloss.Color("#f97316")).Bold(true)
	textDim     = lipgloss.NewStyle().Foreground(lipgloss.Color("#888888"))
	textBold    = lipgloss.NewStyle().Bold(true)
	textSep     = lipgloss.NewStyle().Foreground(lipgloss.Color("#444444"))
)

func renderText(w io.Writer, report *PlanReport, opts ReportOptions) {
	sep := textSep.Render("────────────────────────────────────────────────────────────")
	printed := false
	for _, stack := range report.Stacks {
		if !opts.ShowAll && !stack.HasChanges {
			continue
		}
		if printed {
			fmt.Fprintln(w)
		}
		printed = true
		renderStackText(w, stack, sep)
	}
	fmt.Fprintln(w)
	fmt.Fprintln(w, sep)
	s := report.Summary
	fmt.Fprintf(w, "%s %d stacks · %d with changes · +%d ~%d -%d\n",
		textBold.Render("Summary:"),
		s.TotalStacks, s.StacksWithChanges,
		s.TotalAdd, s.TotalChange, s.TotalDestroy,
	)
}

func renderStackText(w io.Writer, stack StackResult, sep string) {
	stats := fmt.Sprintf("+%d ~%d -%d", stack.Stats.Add, stack.Stats.Change, stack.Stats.Destroy)
	header := fmt.Sprintf("%-52s %s", stack.StackPath, stats)
	fmt.Fprintln(w, textBold.Render(header))
	fmt.Fprintln(w, sep)

	if !stack.HasChanges {
		fmt.Fprintln(w, textDim.Render("  No changes."))
		return
	}

	for _, rc := range stack.ResourceChanges {
		sym := symbolFor(rc.ChangeType)
		var symStyle lipgloss.Style
		switch rc.ChangeType {
		case ChangeTypeCreate:
			symStyle = textCreate
		case ChangeTypeDelete:
			symStyle = textDelete
		case ChangeTypeUpdate:
			symStyle = textUpdate
		case ChangeTypeReplace:
			symStyle = textReplace
		default:
			symStyle = textDim
		}
		line := fmt.Sprintf("  %s %s (%s)", sym, rc.Address, rc.Type)
		fmt.Fprintln(w, symStyle.Render(line))

		diffs := diffAttributes(rc.Before, rc.After, rc.Unknown)
		for _, d := range diffs {
			renderAttrText(w, d)
		}
		fmt.Fprintln(w)
	}
}

func renderAttrText(w io.Writer, d attrDiff) {
	if d.computed {
		fmt.Fprintf(w, "%s\n",
			textDim.Render(fmt.Sprintf("      %-30s (computed)", d.key)),
		)
		return
	}
	if d.before == "" {
		fmt.Fprintf(w, "%s\n",
			textDim.Render(fmt.Sprintf("      %-30s %s", d.key, d.after)),
		)
		return
	}
	if d.after == "" {
		fmt.Fprintf(w, "%s\n",
			textDim.Render(fmt.Sprintf("      %-30s %s", d.key, d.before)),
		)
		return
	}
	fmt.Fprintf(w, "%s\n",
		textDim.Render(fmt.Sprintf("      %-30s %s → %s", d.key, d.before, d.after)),
	)
}

// ---- Markdown renderer ----

func renderMarkdown(w io.Writer, report *PlanReport, opts ReportOptions) {
	for _, stack := range report.Stacks {
		if !opts.ShowAll && !stack.HasChanges {
			continue
		}
		renderStackMarkdown(w, stack)
	}
	s := report.Summary
	fmt.Fprintf(w, "\n---\n**Summary:** %d stacks · %d with changes · +%d ~%d -%d\n",
		s.TotalStacks, s.StacksWithChanges,
		s.TotalAdd, s.TotalChange, s.TotalDestroy,
	)
}

func renderStackMarkdown(w io.Writer, stack StackResult) {
	stats := fmt.Sprintf("+%d ~%d -%d", stack.Stats.Add, stack.Stats.Change, stack.Stats.Destroy)
	fmt.Fprintf(w, "\n## `%s` — %s\n", stack.StackPath, stats)

	if !stack.HasChanges {
		fmt.Fprintln(w, "\nNo changes.")
		return
	}

	for _, rc := range stack.ResourceChanges {
		sym := symbolFor(rc.ChangeType)
		fmt.Fprintf(w, "\n### %s `%s`\n", sym, rc.Address)

		diffs := diffAttributes(rc.Before, rc.After, rc.Unknown)
		if len(diffs) == 0 {
			fmt.Fprintln(w, "\n*(no attribute changes)*")
			continue
		}

		// Determine whether we need a Before column.
		hasBeforeCol := false
		for _, d := range diffs {
			if d.before != "" {
				hasBeforeCol = true
				break
			}
		}

		if hasBeforeCol {
			fmt.Fprintln(w, "\n| Attribute | Before | After |")
			fmt.Fprintln(w, "|-----------|--------|-------|")
			for _, d := range diffs {
				after := d.after
				if d.computed {
					after = "*(computed)*"
				}
				fmt.Fprintf(w, "| `%s` | `%s` | `%s` |\n", d.key, d.before, after)
			}
		} else {
			fmt.Fprintln(w, "\n| Attribute | Value |")
			fmt.Fprintln(w, "|-----------|-------|")
			for _, d := range diffs {
				val := d.after
				if d.computed {
					val = "*(computed)*"
				}
				fmt.Fprintf(w, "| `%s` | `%s` |\n", d.key, val)
			}
		}
	}
}
```

- [ ] **Step 4: Run the tests and verify they pass**

```bash
go test ./internal/plan/ -run "TestDiffAttributes|TestReport_" -v
```

Expected: all tests PASS. If any fail, check whether `diffAttributes` skips unchanged keys correctly (`jsonEqual`) and whether `calculateSummary` is called on the report in `makeReport`.

- [ ] **Step 5: Run the full check**

```bash
task check
```

Expected: all tasks pass, `0 issues` from golangci-lint.

- [ ] **Step 6: Commit**

```bash
git add internal/plan/reporter.go internal/plan/reporter_test.go
git commit -m "feat(plan): add Report renderer with text and markdown output"
```

---

## Task 2: Cobra subcommand — `cmd/report.go`

**Files:**
- Create: `cmd/report.go`

**Interfaces:**
- Consumes (from Task 1):
  - `plan.FormatText`, `plan.FormatMarkdown` — `plan.Format` constants
  - `plan.ReportOptions{Format, ShowAll, Writer}`
  - `plan.Report(report *plan.PlanReport, opts plan.ReportOptions) error`
- Consumes (existing):
  - `plan.CollectFromJSONDir(ctx context.Context, jsonDir, runDir string) (*plan.PlanReport, error)`
  - `getWorkingDirectory(dirFlag string) (string, error)` — defined in `cmd/root.go`
  - `ensureConfigFromWorkDir(workDir string)` — defined in `cmd/root.go`
  - `resolveWorkDir(workDir string) string` — defined in `cmd/root.go`
  - `deps.FindRepoRoot(workDir, rootConfigFile string) string`
  - `config.DefaultRootConfigFile`, `config.DefaultJSONOutDir`
  - `viper.GetString(key string) string`, `viper.Set(key, value string)`
- Produces: `reportCmd` registered on `rootCmd` via `init()`

---

- [ ] **Step 1: Create `cmd/report.go`**

```go
package cmd

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/israoo/terrax/internal/config"
	"github.com/israoo/terrax/internal/deps"
	"github.com/israoo/terrax/internal/plan"
)

var reportCmd = &cobra.Command{
	Use:   "report",
	Short: "Generate a per-resource diff report from JSON plan files",
	Long: `Read Terraform JSON plan files written by Terragrunt's --json-out-dir and render
a full attribute-level diff per stack — the equivalent of 'terraform show' without
requiring the plan binary or .terragrunt-cache.

By default only stacks with pending changes are shown. Use --all to include stacks
with no changes.`,
	RunE: runReportCmd,
}

func init() {
	reportCmd.Flags().String("dir", "", "Working directory (overrides current directory)")
	reportCmd.Flags().String("plans-dir", "", "Directory for JSON plan output files (overrides plan.json_out_dir in config)")
	reportCmd.Flags().String("format", "text", "Output format: text or markdown")
	reportCmd.Flags().String("output", "", "Output file path (default: stdout)")
	reportCmd.Flags().Bool("all", false, "Include stacks with no changes")
	rootCmd.AddCommand(reportCmd)
}

func runReportCmd(cmd *cobra.Command, _ []string) error {
	ctx := context.Background()

	dirFlag, _ := cmd.Flags().GetString("dir")
	workDir, err := getWorkingDirectory(dirFlag)
	if err != nil {
		return fmt.Errorf("failed to get working directory: %w", err)
	}

	ensureConfigFromWorkDir(workDir)
	workDir = resolveWorkDir(workDir)

	if plansDir, _ := cmd.Flags().GetString("plans-dir"); plansDir != "" {
		viper.Set("plan.json_out_dir", plansDir)
	}

	rootConfigFile := viper.GetString("root_config_file")
	if rootConfigFile == "" {
		rootConfigFile = config.DefaultRootConfigFile
	}

	repoRoot := deps.FindRepoRoot(workDir, rootConfigFile)

	jsonOutDir := viper.GetString("plan.json_out_dir")
	if jsonOutDir == "" {
		jsonOutDir = config.DefaultJSONOutDir
	}
	var jsonDir string
	if filepath.IsAbs(jsonOutDir) {
		jsonDir = jsonOutDir
	} else {
		jsonDir = filepath.Join(repoRoot, jsonOutDir)
	}

	report, err := plan.CollectFromJSONDir(ctx, jsonDir, workDir)
	if err != nil {
		return fmt.Errorf("failed to collect plan files: %w", err)
	}

	if len(report.Stacks) == 0 {
		fmt.Fprintf(cmd.OutOrStdout(), "No plan files found in %s\n", jsonDir)
		return nil
	}

	formatFlag, _ := cmd.Flags().GetString("format")
	showAll, _ := cmd.Flags().GetBool("all")
	outputFlag, _ := cmd.Flags().GetString("output")

	var fmt_ plan.Format
	switch formatFlag {
	case "text":
		fmt_ = plan.FormatText
	case "markdown":
		fmt_ = plan.FormatMarkdown
	default:
		return fmt.Errorf("unknown format %q: use text or markdown", formatFlag)
	}

	w := cmd.OutOrStdout()
	if outputFlag != "" {
		f, err := os.Create(outputFlag)
		if err != nil {
			return fmt.Errorf("failed to create output file: %w", err)
		}
		defer f.Close()
		w = f
	}

	return plan.Report(report, plan.ReportOptions{
		Format:  fmt_,
		ShowAll: showAll,
		Writer:  w,
	})
}
```

- [ ] **Step 2: Run full test suite to verify no regressions**

```bash
task check
```

Expected: all tasks pass. The new `reportCmd` has no unit tests of its own (it is pure delegation), but the existing `cmd` package tests must still pass.

- [ ] **Step 3: Smoke test the command manually**

If you have plan JSON files available:
```bash
task build
./build/terrax report --help
./build/terrax report --format markdown
./build/terrax report --format markdown --output /tmp/report.md && cat /tmp/report.md
./build/terrax report --all
```

If no plan files exist, create a minimal fixture:
```bash
mkdir -p .terrax/plans/test/stack
cat > .terrax/plans/test/stack/plan.json <<'EOF'
{"resource_changes":[{"address":"aws_s3_bucket.example","type":"aws_s3_bucket","name":"example","change":{"actions":["create"],"before":null,"after":{"bucket":"my-test-bucket","region":"us-east-1"},"after_unknown":{"id":true}}}]}
EOF
./build/terrax report
./build/terrax report --format markdown
```

Expected text output contains:
```
+ aws_s3_bucket.example (aws_s3_bucket)
      bucket                         "my-test-bucket"
      id                             (computed)
      region                         "us-east-1"
```

Expected markdown output contains:
```markdown
## `test/stack` — +1 ~0 -0

### + `aws_s3_bucket.example`

| Attribute | Value |
|-----------|-------|
| `bucket` | `"my-test-bucket"` |
| `id` | `*(computed)*` |
| `region` | `"us-east-1"` |
```

- [ ] **Step 4: Commit**

```bash
git add cmd/report.go
git commit -m "feat(cmd): add terrax report subcommand with text and markdown output"
```

---

## Self-Review

**Spec coverage check:**
- ✅ `terrax report` subcommand — Task 2
- ✅ Reads JSON plan files via `CollectFromJSONDir` — Task 2
- ✅ Per-resource attribute diff — Task 1 (`diffAttributes`, renderers)
- ✅ Text format with colors — Task 1 (`renderText`, Lipgloss styles)
- ✅ Markdown format — Task 1 (`renderMarkdown`)
- ✅ `--format` flag — Task 2
- ✅ `--output` flag — Task 2
- ✅ `--all` flag — Task 2, tested in Task 1 (`ShowAll`)
- ✅ `--dir` / `--plans-dir` flags — Task 2 (follows `summary.go` pattern)
- ✅ Stacks with error show warning inline — `CollectFromJSONDir` already handles this (prints to stderr and continues); `Report` renders the stacks it received.
- ✅ No changes → informative message, exit 0 — Task 2 (`len(report.Stacks) == 0` branch)
- ✅ Unknown format → error — Task 1 (`Report` returns error) + Task 2 (flag validation)
- ✅ Table-driven tests — Task 1
- ✅ `null` values omitted — `diffAttributes` skips nil before/after; `formatValue` not called for nil

**Placeholder scan:** None found.

**Type consistency:**
- `FormatText`, `FormatMarkdown` defined in Task 1, used in Task 2 ✅
- `ReportOptions{Format, ShowAll, Writer}` defined in Task 1, constructed in Task 2 ✅
- `Report(report *PlanReport, opts ReportOptions) error` — signature matches both tasks ✅
- `diffAttributes(before, after, unknown interface{}) []attrDiff` — internal; consistent across Task 1 ✅
