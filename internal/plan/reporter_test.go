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
