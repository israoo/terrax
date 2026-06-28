package plan

import (
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"strings"

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
		ew := &errWriter{w: opts.Writer}
		renderText(ew, report, opts)
		return ew.err
	case FormatMarkdown:
		ew := &errWriter{w: opts.Writer}
		renderMarkdown(ew, report, opts)
		return ew.err
	default:
		return fmt.Errorf("unknown report format %q: use text or markdown", opts.Format)
	}
}

// errWriter wraps an io.Writer and records the first write error.
// Subsequent writes are skipped once an error has occurred.
type errWriter struct {
	w   io.Writer
	err error
}

func (ew *errWriter) Write(p []byte) (int, error) {
	if ew.err != nil {
		return 0, ew.err
	}
	n, err := ew.w.Write(p)
	if err != nil {
		ew.err = err
	}
	return n, err
}

// printf writes a formatted string to the writer.
func (ew *errWriter) printf(format string, a ...interface{}) {
	if ew.err != nil {
		return
	}
	_, ew.err = fmt.Fprintf(ew, format, a...)
}

// println writes a string followed by a newline to the writer.
func (ew *errWriter) println(a ...interface{}) {
	if ew.err != nil {
		return
	}
	_, ew.err = fmt.Fprintln(ew, a...)
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

func renderText(ew *errWriter, report *PlanReport, opts ReportOptions) {
	sep := textSep.Render("────────────────────────────────────────────────────────────")
	printed := false
	for _, stack := range report.Stacks {
		if !opts.ShowAll && !stack.HasChanges {
			continue
		}
		if printed {
			ew.println()
		}
		printed = true
		renderStackText(ew, stack, sep)
	}
	ew.println()
	if !printed {
		ew.printf("%s\n", textDim.Render("No stacks with pending changes."))
		return
	}
	ew.println(sep)
	s := report.Summary
	ew.printf("%s %d stacks · %d with changes · +%d ~%d -%d\n",
		textBold.Render("Summary:"),
		s.TotalStacks, s.StacksWithChanges,
		s.TotalAdd, s.TotalChange, s.TotalDestroy,
	)
}

func renderStackText(ew *errWriter, stack StackResult, sep string) {
	stats := fmt.Sprintf("+%d ~%d -%d", stack.Stats.Add, stack.Stats.Change, stack.Stats.Destroy)
	header := fmt.Sprintf("%-52s %s", stack.StackPath, stats)
	ew.println(textBold.Render(header))
	ew.println(sep)

	if !stack.HasChanges {
		ew.println(textDim.Render("  No changes."))
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
		ew.println(symStyle.Render(line))

		diffs := diffAttributes(rc.Before, rc.After, rc.Unknown)
		for _, d := range diffs {
			renderAttrText(ew, d)
		}
		ew.println()
	}
}

func renderAttrText(ew *errWriter, d attrDiff) {
	if d.computed {
		ew.printf("%s\n", textDim.Render(fmt.Sprintf("      %-30s (computed)", d.key)))
		return
	}
	if d.before == "" {
		ew.printf("%s\n", textDim.Render(fmt.Sprintf("      %-30s %s", d.key, d.after)))
		return
	}
	if d.after == "" {
		ew.printf("%s\n", textDim.Render(fmt.Sprintf("      %-30s %s", d.key, d.before)))
		return
	}
	ew.printf("%s\n", textDim.Render(fmt.Sprintf("      %-30s %s → %s", d.key, d.before, d.after)))
}

// ---- Markdown renderer ----

// escapeMarkdownCode escapes backticks in a string for use inside a Markdown code span.
func escapeMarkdownCode(s string) string {
	return strings.ReplaceAll(s, "`", "\\`")
}

func renderMarkdown(ew *errWriter, report *PlanReport, opts ReportOptions) {
	for _, stack := range report.Stacks {
		if !opts.ShowAll && !stack.HasChanges {
			continue
		}
		renderStackMarkdown(ew, stack)
	}
	s := report.Summary
	ew.printf("\n---\n**Summary:** %d stacks · %d with changes · +%d ~%d -%d\n",
		s.TotalStacks, s.StacksWithChanges,
		s.TotalAdd, s.TotalChange, s.TotalDestroy,
	)
}

func renderStackMarkdown(ew *errWriter, stack StackResult) {
	stats := fmt.Sprintf("+%d ~%d -%d", stack.Stats.Add, stack.Stats.Change, stack.Stats.Destroy)
	ew.printf("\n## `%s` — %s\n", stack.StackPath, stats)

	if !stack.HasChanges {
		ew.println("\nNo changes.")
		return
	}

	for _, rc := range stack.ResourceChanges {
		sym := symbolFor(rc.ChangeType)
		ew.printf("\n### %s `%s`\n", sym, rc.Address)

		diffs := diffAttributes(rc.Before, rc.After, rc.Unknown)
		if len(diffs) == 0 {
			ew.println("\n*(no attribute changes)*")
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
			ew.println("\n| Attribute | Before | After |")
			ew.println("|-----------|--------|-------|")
			for _, d := range diffs {
				after := d.after
				if d.computed {
					after = "*(computed)*"
				} else {
					after = escapeMarkdownCode(after)
				}
				ew.printf("| `%s` | `%s` | `%s` |\n", d.key, escapeMarkdownCode(d.before), after)
			}
		} else {
			ew.println("\n| Attribute | Value |")
			ew.println("|-----------|-------|")
			for _, d := range diffs {
				val := d.after
				if d.computed {
					val = "*(computed)*"
				} else {
					val = escapeMarkdownCode(val)
				}
				ew.printf("| `%s` | `%s` |\n", d.key, val)
			}
		}
	}
}
