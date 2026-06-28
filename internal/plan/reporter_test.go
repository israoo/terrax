package plan

import (
	"encoding/json"
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
	// Summary line present.
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

func TestReport_Text_AllNoChanges_NoSeparator(t *testing.T) {
	stack := StackResult{StackPath: "workloads/dev/noop", HasChanges: false}
	var sb strings.Builder
	err := Report(makeReport(stack), ReportOptions{Format: FormatText, Writer: &sb})
	require.NoError(t, err)
	plain := stripANSI(sb.String())
	assert.Contains(t, plain, "No stacks with pending changes")
	assert.NotContains(t, plain, "Summary:")
}

func TestReport_Markdown_BacktickInValue(t *testing.T) {
	stack := StackResult{
		StackPath:  "workloads/dev/test",
		HasChanges: true,
		Stats:      StackStats{Add: 1},
		ResourceChanges: []ResourceChange{
			{
				Address:    "null_resource.test",
				Type:       "null_resource",
				Name:       "test",
				ChangeType: ChangeTypeCreate,
				After:      map[string]interface{}{"cmd": "echo `hello`"},
			},
		},
	}
	var sb strings.Builder
	err := Report(makeReport(stack), ReportOptions{Format: FormatMarkdown, Writer: &sb})
	require.NoError(t, err)
	out := sb.String()
	// Backtick must be escaped so the table cell is valid Markdown.
	assert.Contains(t, out, "\\`hello\\`")
	// The cell must not contain an unescaped backtick inside the value column.
	assert.NotContains(t, out, "| `echo `hello`")
}

// ---- Recursive diff tests ----

func TestDiffAttributes_NestedMap(t *testing.T) {
	// "emails" is an array of objects; the nested "value" field changes.
	before := map[string]interface{}{
		"emails": []interface{}{
			map[string]interface{}{"primary": false, "type": "", "value": "a@example.com"},
		},
	}
	after := map[string]interface{}{
		"emails": []interface{}{
			map[string]interface{}{"primary": false, "type": "", "value": "b@example.com"},
		},
	}
	diffs := diffAttributes(before, after, nil)
	require.Len(t, diffs, 1)
	assert.Equal(t, "emails", diffs[0].key)
	// Must have children — not a flat blob.
	require.NotEmpty(t, diffs[0].children, "emails diff must be nested, not a flat string")
	// The [0] child must itself have children (it's an object).
	elem := diffs[0].children[0]
	assert.Equal(t, "[0]", elem.key)
	require.NotEmpty(t, elem.children)
	// Only the "value" key changed; "primary" and "type" are suppressed.
	valueChild := findChild(elem.children, "value")
	require.NotNil(t, valueChild)
	assert.Contains(t, valueChild.before, "a@example.com")
	assert.Contains(t, valueChild.after, "b@example.com")
	// 2 unchanged attributes hidden on [0].
	assert.Equal(t, 2, elem.unchangedCnt)
}

func TestDiffAttributes_JSONString(t *testing.T) {
	// "inline_policy" is a JSON-encoded string (Terraform jsonencode pattern).
	// The before has 2 statements; the after removes one.
	stmt1 := map[string]interface{}{"Sid": "Keep", "Effect": "Allow", "Action": "s3:GetObject", "Resource": "*"}
	stmt2 := map[string]interface{}{"Sid": "Remove", "Effect": "Allow", "Action": "lambda:InvokeFunction", "Resource": "*"}
	stmt1After := map[string]interface{}{"Sid": "Keep", "Effect": "Allow", "Action": "s3:GetObject", "Resource": "*"}

	mustJSON := func(v interface{}) string {
		b, _ := json.Marshal(map[string]interface{}{"Statement": v})
		return string(b)
	}
	before := map[string]interface{}{"inline_policy": mustJSON([]interface{}{stmt1, stmt2})}
	after := map[string]interface{}{"inline_policy": mustJSON([]interface{}{stmt1After})}

	diffs := diffAttributes(before, after, nil)
	require.Len(t, diffs, 1)
	assert.Equal(t, "inline_policy", diffs[0].key)
	// Must recurse into the decoded JSON — not a flat blob.
	require.NotEmpty(t, diffs[0].children, "inline_policy diff must be nested after JSON decoding")
}

func TestDiffAttributes_NestedMap_UnchangedCount(t *testing.T) {
	// Object with 4 keys; only one changes. unchangedCnt must be 3.
	before := map[string]interface{}{"obj": map[string]interface{}{"a": 1.0, "b": 2.0, "c": 3.0, "d": 4.0}}
	after := map[string]interface{}{"obj": map[string]interface{}{"a": 1.0, "b": 99.0, "c": 3.0, "d": 4.0}}
	diffs := diffAttributes(before, after, nil)
	require.Len(t, diffs, 1)
	assert.Equal(t, "obj", diffs[0].key)
	require.Len(t, diffs[0].children, 1) // only "b" changed
	assert.Equal(t, 3, diffs[0].unchangedCnt)
}

func TestDiffAttributes_ArrayAdd(t *testing.T) {
	// Array grows from 1 to 2 elements.
	before := map[string]interface{}{"tags": []interface{}{"prod"}}
	after := map[string]interface{}{"tags": []interface{}{"prod", "new-tag"}}
	diffs := diffAttributes(before, after, nil)
	require.Len(t, diffs, 1)
	require.NotEmpty(t, diffs[0].children)
	// Element [0] unchanged (1 hidden), element [1] is an add.
	assert.Equal(t, 1, diffs[0].unchangedCnt)
	require.Len(t, diffs[0].children, 1)
	assert.Equal(t, "[1]", diffs[0].children[0].key)
	assert.Empty(t, diffs[0].children[0].before)
	assert.NotEmpty(t, diffs[0].children[0].after)
}

func TestDiffAttributes_StringValueUnchanged_NoChildren(t *testing.T) {
	// Plain string that parses as JSON but is equal on both sides → no diff entry at all.
	val := `{"key":"value"}`
	before := map[string]interface{}{"policy": val}
	after := map[string]interface{}{"policy": val}
	diffs := diffAttributes(before, after, nil)
	assert.Empty(t, diffs, "unchanged JSON string must produce no diff")
}

func TestAttrSymbol(t *testing.T) {
	// Leaf cases.
	assert.Equal(t, "+", attrSymbol(attrDiff{after: "x"}))
	assert.Equal(t, "-", attrSymbol(attrDiff{before: "x"}))
	assert.Equal(t, "~", attrSymbol(attrDiff{before: "x", after: "y"}))
	assert.Equal(t, "~", attrSymbol(attrDiff{computed: true}))

	// Nested: all children removed → "-".
	assert.Equal(t, "-", attrSymbol(attrDiff{children: []attrDiff{{key: "k", before: "x"}}}))
	// Nested: all children added → "+".
	assert.Equal(t, "+", attrSymbol(attrDiff{children: []attrDiff{{key: "k", after: "x"}}}))
	// Nested: mixed children → "~".
	assert.Equal(t, "~", attrSymbol(attrDiff{children: []attrDiff{{key: "a", before: "x"}, {key: "b", after: "y"}}}))
	// Nested: all removed but some unchanged siblings → "~" (partial remove = update).
	assert.Equal(t, "~", attrSymbol(attrDiff{children: []attrDiff{{key: "k", before: "x"}}, unchangedCnt: 1}))
}

func TestReport_Text_NestedArray(t *testing.T) {
	stack := StackResult{
		StackPath:  "workloads/dev/identity",
		HasChanges: true,
		Stats:      StackStats{Change: 1},
		ResourceChanges: []ResourceChange{
			{
				Address:    `aws_identitystore_user.this["npb-hector"]`,
				Type:       "aws_identitystore_user",
				Name:       `this["npb-hector"]`,
				ChangeType: ChangeTypeUpdate,
				Before: map[string]interface{}{
					"emails": []interface{}{
						map[string]interface{}{"primary": false, "type": "", "value": "old@example.com"},
					},
				},
				After: map[string]interface{}{
					"emails": []interface{}{
						map[string]interface{}{"primary": false, "type": "", "value": "new@example.com"},
					},
				},
			},
		},
	}
	var sb strings.Builder
	err := Report(makeReport(stack), ReportOptions{Format: FormatText, Writer: &sb})
	require.NoError(t, err)
	plain := stripANSI(sb.String())
	assert.Contains(t, plain, "emails")
	assert.Contains(t, plain, "[0]")
	assert.Contains(t, plain, "value")
	assert.Contains(t, plain, "old@example.com")
	assert.Contains(t, plain, "new@example.com")
	assert.Contains(t, plain, "→")
	assert.Contains(t, plain, "unchanged hidden")
}

func TestReport_Text_JSONString(t *testing.T) {
	mustJSON := func(v interface{}) string { b, _ := json.Marshal(v); return string(b) }
	stmt1 := map[string]interface{}{"Sid": "Keep", "Effect": "Allow", "Action": "s3:Get"}
	stmt2 := map[string]interface{}{"Sid": "Remove", "Effect": "Allow", "Action": "lambda:Invoke"}
	before := map[string]interface{}{"inline_policy": mustJSON(map[string]interface{}{"Statement": []interface{}{stmt1, stmt2}})}
	after := map[string]interface{}{"inline_policy": mustJSON(map[string]interface{}{"Statement": []interface{}{stmt1}})}
	stack := StackResult{
		StackPath:  "workloads/dev/sso",
		HasChanges: true,
		Stats:      StackStats{Change: 1},
		ResourceChanges: []ResourceChange{
			{Address: "res.r", Type: "r", Name: "r", ChangeType: ChangeTypeUpdate,
				Before: before, After: after},
		},
	}
	var sb strings.Builder
	err := Report(makeReport(stack), ReportOptions{Format: FormatText, Writer: &sb})
	require.NoError(t, err)
	plain := stripANSI(sb.String())
	assert.Contains(t, plain, "inline_policy")
	assert.Contains(t, plain, "Statement")
	// The removed statement's Sid should appear.
	assert.Contains(t, plain, "Remove")
}

func TestReport_Markdown_NestedArray(t *testing.T) {
	stack := StackResult{
		StackPath:  "workloads/dev/identity",
		HasChanges: true,
		Stats:      StackStats{Change: 1},
		ResourceChanges: []ResourceChange{
			{
				Address:    `aws_identitystore_user.this["npb-hector"]`,
				Type:       "aws_identitystore_user",
				Name:       `this["npb-hector"]`,
				ChangeType: ChangeTypeUpdate,
				Before: map[string]interface{}{
					"emails": []interface{}{
						map[string]interface{}{"primary": false, "type": "", "value": "old@example.com"},
					},
				},
				After: map[string]interface{}{
					"emails": []interface{}{
						map[string]interface{}{"primary": false, "type": "", "value": "new@example.com"},
					},
				},
			},
		},
	}
	var sb strings.Builder
	err := Report(makeReport(stack), ReportOptions{Format: FormatMarkdown, Writer: &sb})
	require.NoError(t, err)
	out := sb.String()
	assert.Contains(t, out, "emails")
	assert.Contains(t, out, "[0]")
	assert.Contains(t, out, "value")
	assert.Contains(t, out, "old@example.com")
	assert.Contains(t, out, "new@example.com")
	assert.Contains(t, out, "unchanged hidden")
	// Must NOT be a flat table row with the full JSON blob.
	assert.NotContains(t, out, `"primary":false`)
}

func TestReport_Markdown_FlatDiff_StillUsesTable(t *testing.T) {
	// When all diffs are leaves (no nested), keep the table format.
	stack := StackResult{
		StackPath:  "workloads/dev/dns",
		HasChanges: true,
		Stats:      StackStats{Change: 1},
		ResourceChanges: []ResourceChange{
			{
				Address: "aws_route53_record.val", Type: "r", Name: "r",
				ChangeType: ChangeTypeUpdate,
				Before:     map[string]interface{}{"name": "old.example.com"},
				After:      map[string]interface{}{"name": "new.example.com"},
			},
		},
	}
	var sb strings.Builder
	err := Report(makeReport(stack), ReportOptions{Format: FormatMarkdown, Writer: &sb})
	require.NoError(t, err)
	out := sb.String()
	// Table format preserved.
	assert.Contains(t, out, "| Attribute |")
	assert.Contains(t, out, "old.example.com")
	assert.Contains(t, out, "new.example.com")
}

// findChild returns the first attrDiff with the given key, or nil.
func findChild(diffs []attrDiff, key string) *attrDiff {
	for i := range diffs {
		if diffs[i].key == key {
			return &diffs[i]
		}
	}
	return nil
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
