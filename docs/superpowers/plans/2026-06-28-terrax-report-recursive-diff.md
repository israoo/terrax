# terrax report — Recursive Attribute Diff (B+C) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make `terrax report` show per-field diffs for nested objects/arrays and decode JSON-encoded string attributes (like Terraform's `jsonencode()` outputs) instead of dumping opaque JSON blobs.

**Architecture:** Two-task refactor of `internal/plan/reporter.go`. Task 1 changes `attrDiff` to carry raw `interface{}` values and adds a recursive diff engine (`diffValue` / `recursiveDiff` / `diffMaps` / `diffArrays`). Task 2 updates the text and markdown renderers to handle nested diffs using depth-based indentation (text) and bullet lists (markdown). No new files — only `reporter.go` and `reporter_test.go` change.

**Tech Stack:** Go 1.25.5 · `encoding/json` (stdlib) · Lipgloss 1.1.0 · testify

## Global Constraints

- All comments must end with a period.
- Imports: three groups (stdlib → third-party → `github.com/israoo/terrax/...`), alphabetically sorted within each group.
- Never use `lipgloss.Style.Copy()` — each style uses `lipgloss.NewStyle()`.
- Errors always wrapped: `fmt.Errorf("...: %w", err)`.
- Run `task check` before every commit — fmt + vet + lint + test all must pass.
- Test assertions for text output use `strings.Contains` (ANSI codes); markdown assertions can use exact/partial matching.
- `internal/plan/` must NOT import `internal/tui/` or any other internal TerraX package.
- `attrDiff` is unexported — breaking its fields is fine; only `reporter_test.go` (same package) uses it.

---

## File Map

| File | Action | What changes |
|------|--------|-------------|
| `internal/plan/reporter.go` | Modify | Refactor `attrDiff`; add `diffValue`, `recursiveDiff`, `diffMaps`, `diffArrays`; update renderers for depth/nesting |
| `internal/plan/reporter_test.go` | Modify | Add recursive-diff tests; update render tests to match new nested output |

---

## Current state (read before starting)

Read `internal/plan/reporter.go` in full before touching anything. Key existing internals:

```
attrDiff { key string; before string; after string; computed bool }
diffAttributes(before, after, unknown interface{}) []attrDiff   ← top-level only today
renderAttrText(ew *errWriter, d attrDiff)                       ← no depth param today
renderStackMarkdown uses a flat table for all diffs
```

Both tasks modify the same two files; Task 2 depends on Task 1's types.

---

## Task 1: Recursive diff engine

**Files:**
- Modify: `internal/plan/reporter.go` (lines ~28–133 — `attrDiff` struct and `diffAttributes` section)
- Modify: `internal/plan/reporter_test.go` (add new tests; existing `TestDiffAttributes_*` must still pass)

**Interfaces:**
- Consumes: `ResourceChange.Before`, `ResourceChange.After`, `ResourceChange.Unknown` — all `interface{}` from `models.go`
- Produces (for Task 2):
  - `type attrDiff struct { key string; before string; after string; computed bool; children []attrDiff; unchangedCnt int }`
  - `func diffAttributes(before, after, unknown interface{}) []attrDiff` — unchanged signature, richer output
  - `func attrSymbol(d attrDiff) string` — returns `"+"`, `"-"`, `"~"`, `"±"` for any node
  - Rule: leaf diff has `len(children)==0`; nested diff has `len(children)>0` and empty `before`/`after`

---

- [ ] **Step 1: Write the failing tests for the new diff engine**

Add these tests to `internal/plan/reporter_test.go` (keep all existing tests untouched):

```go
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
	assert.Equal(t, "+", attrSymbol(attrDiff{after: "x"}))
	assert.Equal(t, "-", attrSymbol(attrDiff{before: "x"}))
	assert.Equal(t, "~", attrSymbol(attrDiff{before: "x", after: "y"}))
	assert.Equal(t, "~", attrSymbol(attrDiff{children: []attrDiff{{key: "k"}}}))
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
```

- [ ] **Step 2: Run tests to confirm they fail**

```bash
go test ./internal/plan/ -run "TestDiffAttributes_Nested|TestDiffAttributes_JSON|TestDiffAttributes_Array|TestDiffAttributes_String|TestAttrSymbol" -v
```

Expected: `FAIL` — `attrDiff` has no `children`/`unchangedCnt` fields; `attrSymbol` undefined.

- [ ] **Step 3: Refactor `attrDiff` and add the recursive engine**

Replace the `attrDiff` struct and `diffAttributes` function, and add the new helpers. The complete replacement for the section spanning lines 28–133 in `reporter.go`:

```go
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

	// Attempt recursive diff when both sides are present.
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
```

- [ ] **Step 4: Run the new tests to confirm they pass**

```bash
go test ./internal/plan/ -run "TestDiffAttributes_Nested|TestDiffAttributes_JSON|TestDiffAttributes_Array|TestDiffAttributes_String|TestAttrSymbol" -v
```

Expected: all PASS.

- [ ] **Step 5: Confirm existing tests still pass**

```bash
go test ./internal/plan/ -v
```

Expected: all tests pass including the original `TestDiffAttributes_*`, `TestReport_*` tests.

Note: some `TestReport_*` render tests may now fail if the new nested behavior changes output format — that is expected and will be fixed in Task 2. For Task 1, only the diff-engine tests must pass; if render tests break, that is acceptable.

- [ ] **Step 6: Run full check**

```bash
task check
```

Expected: all pass, 0 lint issues.

- [ ] **Step 7: Commit**

```bash
git add internal/plan/reporter.go internal/plan/reporter_test.go
git commit -m "refactor(plan): add recursive attribute diff engine (B+C)"
```

---

## Task 2: Update renderers for nested diffs

**Files:**
- Modify: `internal/plan/reporter.go` (renderer section — `renderAttrText`, `renderStackMarkdown`, add `renderAttrMarkdown`)
- Modify: `internal/plan/reporter_test.go` (add nested render tests; fix any render tests broken by Task 1)

**Interfaces:**
- Consumes (from Task 1):
  - `type attrDiff struct { key string; before string; after string; computed bool; children []attrDiff; unchangedCnt int }`
  - `func attrSymbol(d attrDiff) string`
- Produces: no new public API — only rendering behavior changes

---

**Target output — text format:**

For the `emails` case (nested array of objects):
```
  ~ aws_identitystore_user.this["npb-hector-trivino"] (aws_identitystore_user)
      ~ emails
          ~ [0]
              value                          "hector@npblabs.com" → "hector2@npblabs.com"
              # (2 unchanged hidden)
```

For the `inline_policy` / `jsonencode` case:
```
  ~ aws_ssoadmin_permission_set_inline_policy.this["ReadOnlyProduction"] (...)
      ~ inline_policy
          ~ Statement
              # (3 unchanged hidden)
              - [4]
                  Action   "lambda:InvokeFunction"
                  Effect   "Allow"
                  Resource [...]
                  Sid      "InvokeKernLambdas"
```

**Target output — markdown format:**

For resources with any nested diff, use bullet lists (not tables):
```markdown
### ~ `aws_identitystore_user.this["npb-hector-trivino"]`

- `~` **emails**
  - `~` **[0]**
    - `~` **value**: `"hector@npblabs.com"` → `"hector2@npblabs.com"`
    - *(2 unchanged hidden)*

### ~ `aws_ssoadmin_permission_set_inline_policy.this["ReadOnlyProduction"]`

- `~` **inline_policy**
  - `~` **Statement**
    - *(3 unchanged hidden)*
    - `-` **[4]**
      - `Action`: `"lambda:InvokeFunction"`
      - `Effect`: `"Allow"`
      - `Sid`: `"InvokeKernLambdas"`
```

For resources with only leaf diffs: keep current table format unchanged.

---

- [ ] **Step 1: Write the failing render tests**

Add to `internal/plan/reporter_test.go`:

```go
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
```

- [ ] **Step 2: Run new tests to confirm they fail**

```bash
go test ./internal/plan/ -run "TestReport_Text_Nested|TestReport_Text_JSON|TestReport_Markdown_Nested|TestReport_Markdown_Flat" -v
```

Expected: FAIL — renderers don't handle `children` yet.

- [ ] **Step 3: Update `renderAttrText` to support depth and children**

Replace the existing `renderAttrText` function in `reporter.go`:

```go
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
```

Also update the call site in `renderStackText` to pass the depth argument:

```go
// In renderStackText, change:
//   renderAttrText(ew, d)
// to:
//   renderAttrText(ew, d, 0)
for _, d := range diffs {
    renderAttrText(ew, d, 0)
}
```

- [ ] **Step 4: Add `renderAttrMarkdown` and update `renderStackMarkdown`**

Add the new helper and replace the `renderStackMarkdown` function:

```go
// hasNestedDiff reports whether any diff in the slice has children.
func hasNestedDiff(diffs []attrDiff) bool {
	for _, d := range diffs {
		if len(d.children) > 0 {
			return true
		}
	}
	return false
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
```

- [ ] **Step 5: Run new tests to confirm they pass**

```bash
go test ./internal/plan/ -run "TestReport_Text_Nested|TestReport_Text_JSON|TestReport_Markdown_Nested|TestReport_Markdown_Flat" -v
```

Expected: all PASS.

- [ ] **Step 6: Run full test suite to confirm no regressions**

```bash
go test ./internal/plan/ -v
```

Expected: all tests pass. If any existing `TestReport_*` tests fail because the output format changed (e.g. a flat test case now produces nested output for some reason), read the failure carefully — it should only happen if a test fixture contains nested data that now recursively diffs. Fix the test assertion to match the new output, not the implementation.

- [ ] **Step 7: Run full check**

```bash
task check
```

Expected: all pass, 0 lint issues.

- [ ] **Step 8: Smoke test with real output**

```bash
task build
mkdir -p .terrax/plans/test/stack
cat > .terrax/plans/test/stack/plan.json <<'EOF'
{"resource_changes":[
  {"address":"aws_identitystore_user.this[\"user\"]","type":"aws_identitystore_user","name":"this[\"user\"]","change":{
    "actions":["update"],
    "before":{"emails":[{"primary":false,"type":"","value":"old@example.com"}],"id":"d-123/abc"},
    "after":{"emails":[{"primary":false,"type":"","value":"new@example.com"}],"id":"d-123/abc"},
    "after_unknown":{}
  }}
]}
EOF
./build/terrax report
./build/terrax report --format markdown
```

Expected text output contains `emails`, `[0]`, `value`, `old@example.com → new@example.com`, `2 unchanged hidden`.
Expected markdown output contains bullet list with `~` symbols, NOT a JSON blob.

- [ ] **Step 9: Commit**

```bash
git add internal/plan/reporter.go internal/plan/reporter_test.go
git commit -m "feat(plan): render nested attribute diffs recursively with JSON-string decoding"
```

---

## Self-Review

**Spec coverage:**
- ✅ B (JSON-string detection): `recursiveDiff` checks `bIsStr && aIsStr`, tries `json.Unmarshal`, recurses if both parse — Task 1.
- ✅ C (nested map/array diff): `diffMaps` and `diffArrays` — Task 1.
- ✅ Unchanged count suppression: `unchangedCnt` field, rendered as `# (N unchanged hidden)` in text and `*(N unchanged hidden)*` in markdown — both tasks.
- ✅ Text format depth-indented rendering — Task 2.
- ✅ Markdown bullet-list for nested, table preserved for flat — Task 2.
- ✅ Existing flat-diff behavior (table format, create/update/delete symbols) preserved — Task 2.

**Placeholder scan:** None found.

**Type consistency:**
- `attrDiff.children []attrDiff` defined in Task 1, consumed in Task 2 ✅
- `attrDiff.unchangedCnt int` defined in Task 1, consumed in Task 2 ✅
- `attrSymbol(d attrDiff) string` defined in Task 1, consumed in Task 2 ✅
- `renderAttrText(ew, d, depth)` — depth=0 call site in `renderStackText` updated in Task 2 Step 3 ✅
- `hasNestedDiff(diffs []attrDiff) bool` defined and used in Task 2 ✅
- `renderAttrMarkdown(ew, d, depth)` defined and used in Task 2 ✅
