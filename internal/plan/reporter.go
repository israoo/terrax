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

// attrDiff represents one attribute comparison at any nesting depth.
// Leaf diffs have len(children)==0 and non-empty before/after strings.
// Nested diffs have len(children)>0 and empty before/after strings.
type attrDiff struct {
	key          string
	before       string     // JSON-formatted leaf value; empty for adds and nested diffs.
	after        string     // JSON-formatted leaf value; empty for removes and nested diffs.
	computed     bool       // True when field is marked unknown in the plan.
	children     []attrDiff // Non-empty for nested (map/array/JSON-string) diffs.
	unchangedCnt int        // Count of equal siblings omitted from children.
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

// attrSymbol returns the diff symbol (+/-/~) for any attrDiff node.
func attrSymbol(d attrDiff) string {
	if len(d.children) > 0 {
		return "~"
	}
	if d.before == "" && !d.computed {
		return "+"
	}
	if d.after == "" {
		return "-"
	}
	return "~"
}

// diffAttributes computes the top-level attribute diff between before and after.
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

		var bv, av interface{}
		if bExists {
			bv = bVal
		}
		if aExists {
			av = aVal
		}
		diffs = append(diffs, diffValue(k, bv, av, isComputed))
	}

	sort.Slice(diffs, func(i, j int) bool { return diffs[i].key < diffs[j].key })
	return diffs
}

// diffValue constructs an attrDiff for a single key, recursing when possible.
func diffValue(key string, bv, av interface{}, computed bool) attrDiff {
	if computed {
		return attrDiff{key: key, computed: true}
	}

	// Attempt recursive diff when at least one side is present.
	if bv != nil || av != nil {
		children, unchanged := recursiveDiff(bv, av)
		if len(children) > 0 || unchanged > 0 {
			return attrDiff{key: key, children: children, unchangedCnt: unchanged}
		}
	}

	// Leaf diff.
	d := attrDiff{key: key}
	if bv != nil {
		d.before = formatValue(bv)
	}
	if av != nil {
		d.after = formatValue(av)
	}
	return d
}

// recursiveDiff attempts to produce a structured diff of before and after.
// Returns (nil, 0) when the values cannot be recursed (e.g. primitive scalars).
func recursiveDiff(before, after interface{}) ([]attrDiff, int) {
	// Case B: both are JSON-encoded strings — decode and recurse.
	bStr, bIsStr := before.(string)
	aStr, aIsStr := after.(string)
	if bIsStr && aIsStr {
		var bParsed, aParsed interface{}
		bErr := json.Unmarshal([]byte(bStr), &bParsed)
		aErr := json.Unmarshal([]byte(aStr), &aParsed)
		if bErr == nil && aErr == nil {
			children, unchanged := recursiveDiff(bParsed, aParsed)
			if children != nil || unchanged > 0 {
				return children, unchanged
			}
		}
		return nil, 0
	}

	// Case C1: both are maps.
	bMap, bIsMap := before.(map[string]interface{})
	aMap, aIsMap := after.(map[string]interface{})
	if bIsMap && aIsMap {
		return diffMaps(bMap, aMap)
	}

	// Case C2: both are arrays.
	bArr, bIsArr := before.([]interface{})
	aArr, aIsArr := after.([]interface{})
	if bIsArr && aIsArr {
		return diffArrays(bArr, aArr)
	}

	return nil, 0
}

// diffMaps computes a recursive diff between two maps.
// Returns changed diffs and the count of unchanged keys.
func diffMaps(before, after map[string]interface{}) ([]attrDiff, int) {
	keySet := map[string]struct{}{}
	for k := range before {
		keySet[k] = struct{}{}
	}
	for k := range after {
		keySet[k] = struct{}{}
	}

	var diffs []attrDiff
	unchanged := 0
	for k := range keySet {
		bv, bExists := before[k]
		av, aExists := after[k]
		if bExists && aExists && jsonEqual(bv, av) {
			unchanged++
			continue
		}
		var bvp, avp interface{}
		if bExists {
			bvp = bv
		}
		if aExists {
			avp = av
		}
		diffs = append(diffs, diffValue(k, bvp, avp, false))
	}
	sort.Slice(diffs, func(i, j int) bool { return diffs[i].key < diffs[j].key })
	return diffs, unchanged
}

// diffArrays computes an index-based diff between two arrays.
// Returns changed diffs and the count of unchanged elements.
func diffArrays(before, after []interface{}) ([]attrDiff, int) {
	maxLen := len(before)
	if len(after) > maxLen {
		maxLen = len(after)
	}

	var diffs []attrDiff
	unchanged := 0
	for i := 0; i < maxLen; i++ {
		key := fmt.Sprintf("[%d]", i)
		var bv, av interface{}
		if i < len(before) {
			bv = before[i]
		}
		if i < len(after) {
			av = after[i]
		}
		if bv != nil && av != nil && jsonEqual(bv, av) {
			unchanged++
			continue
		}
		diffs = append(diffs, diffValue(key, bv, av, false))
	}
	return diffs, unchanged
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
			renderAttrText(ew, d, 0)
		}
		ew.println()
	}
}

// renderAttrText renders one attrDiff at the given nesting depth.
// depth 0 = direct child of a resource (base indent = 6 spaces).
func renderAttrText(ew *errWriter, d attrDiff, depth int) {
	indent := strings.Repeat("  ", depth) + "      " // 6 spaces base + 2 per depth.

	if len(d.children) > 0 {
		sym := attrSymbol(d)
		ew.printf("%s\n", textDim.Render(fmt.Sprintf("%s%s %s", indent, sym, d.key)))
		for _, child := range d.children {
			renderAttrText(ew, child, depth+1)
		}
		if d.unchangedCnt > 0 {
			ew.printf("%s\n", textDim.Render(fmt.Sprintf("%s  # (%d unchanged hidden)", indent, d.unchangedCnt)))
		}
		return
	}

	// Leaf diff.
	if d.computed {
		ew.printf("%s\n", textDim.Render(fmt.Sprintf("%s%-30s (computed)", indent, d.key)))
		return
	}
	if d.before == "" {
		ew.printf("%s\n", textDim.Render(fmt.Sprintf("%s%-30s %s", indent, d.key, d.after)))
		return
	}
	if d.after == "" {
		ew.printf("%s\n", textDim.Render(fmt.Sprintf("%s%-30s %s", indent, d.key, d.before)))
		return
	}
	ew.printf("%s\n", textDim.Render(fmt.Sprintf("%s%-30s %s → %s", indent, d.key, d.before, d.after)))
}

// ---- Markdown renderer ----

// hasNestedDiff reports whether any diff in the slice has children.
func hasNestedDiff(diffs []attrDiff) bool {
	for _, d := range diffs {
		if len(d.children) > 0 {
			return true
		}
	}
	return false
}

// escapeMarkdownCode escapes backticks in a string for use inside a Markdown code span.
func escapeMarkdownCode(s string) string {
	return strings.ReplaceAll(s, "`", "\\`")
}

// renderAttrMarkdown renders one attrDiff as a Markdown bullet at the given depth.
// depth 0 = no extra indent; each depth adds 2 spaces.
func renderAttrMarkdown(ew *errWriter, d attrDiff, depth int) {
	indent := strings.Repeat("  ", depth)
	sym := attrSymbol(d)

	if len(d.children) > 0 {
		ew.printf("%s- `%s` **%s**\n", indent, sym, escapeMarkdownCode(d.key))
		if d.unchangedCnt > 0 {
			ew.printf("%s  - *(%d unchanged hidden)*\n", indent, d.unchangedCnt)
		}
		for _, child := range d.children {
			renderAttrMarkdown(ew, child, depth+1)
		}
		return
	}

	// Leaf diff.
	if d.computed {
		ew.printf("%s- `%s` **%s**: *(computed)*\n", indent, sym, escapeMarkdownCode(d.key))
		return
	}
	if d.before == "" {
		ew.printf("%s- `%s` **%s**: `%s`\n", indent, sym, escapeMarkdownCode(d.key), escapeMarkdownCode(d.after))
		return
	}
	if d.after == "" {
		ew.printf("%s- `%s` **%s**: `%s`\n", indent, sym, escapeMarkdownCode(d.key), escapeMarkdownCode(d.before))
		return
	}
	ew.printf("%s- `%s` **%s**: `%s` → `%s`\n",
		indent, sym,
		escapeMarkdownCode(d.key),
		escapeMarkdownCode(d.before),
		escapeMarkdownCode(d.after),
	)
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

		if hasNestedDiff(diffs) {
			// Bullet-list format for nested diffs.
			ew.println()
			for _, d := range diffs {
				renderAttrMarkdown(ew, d, 0)
			}
		} else {
			// Table format for flat diffs (backward-compatible).
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
}
