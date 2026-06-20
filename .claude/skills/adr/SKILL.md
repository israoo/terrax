---
name: terrax-adr
description: Use when creating a new Architectural Decision Record for TerraX. Use when a significant technical decision has been made — new package, new pattern, new integration strategy, or any choice with meaningful trade-offs that future contributors need to understand.
---

# TerraX ADR

## Step 1 — Determine the next number

```bash
ls docs/adr/*.md | grep -v README | sort | tail -1
```

Increment the 4-digit number (e.g. `0010-...` → next is `0011`).

## Step 2 — Create the file

Filename: `docs/adr/NNNN-kebab-case-title.md`

## Step 3 — Fill the template

```markdown
# ADR-NNNN: Title

**Status**: Accepted

**Date**: YYYY-MM-DD

**Deciders**: TerraX Core Team

**Related**:
- [ADR-XXXX: Title](XXXX-file.md)

## Context

[Problem statement. What forced this decision? What constraints exist?
What would happen if no decision were made? Numbered list of requirements
if multiple constraints apply.]

## Decision

[What was decided and how it works. Include a code snippet or directory
structure if it clarifies the decision. Be specific about what the
implementation looks like, not just the principle.]

## Consequences

### Positive

- [Concrete benefit with brief explanation if non-obvious.]
- [Another benefit.]

### Negative

- [Real trade-off. Be honest — a decision with no negatives wasn't a decision.]
- [Another trade-off.]

## Alternatives Considered

### Option 1: [Name]

**Description**: [One paragraph describing the approach.]

**Pros**:

- [Benefit.]

**Cons**:

- [Drawback.]

**Why rejected**: [One paragraph. Tie the rejection back to the context constraints, not just "our approach is better".]

### Option 2: [Name]

[Same structure.]

## Future Enhancements

**Potential Improvements**:

1. [Possible future improvement that doesn't belong in the current decision.]
2. [Another improvement.]

## References

- [Link to relevant code, ADR, or external doc]
```

## Section rules

| Section | Required | Notes |
|---|---|---|
| Status / Date / Deciders | Yes | Always "TerraX Core Team" for Deciders |
| Related | No | Omit if no related ADRs; list all if multiple |
| Context | Yes | The problem, not the solution |
| Decision | Yes | Concrete — include structure/code if it helps |
| Consequences | Yes | Must have both Positive and Negative subsections |
| Alternatives Considered | Yes | Minimum 2 options; "Why rejected" is prose, not a list |
| Future Enhancements | No | Add when there are known follow-on possibilities |
| References | Yes | Always present, even if just one link |

**Writing style:**
- "Why rejected" explains why the context made this option unsuitable — not just that the chosen option is better.
- Consequences are honest: a decision with no Negative consequences wasn't a real trade-off.
- Decision section is concrete: describe what the implementation looks like, not just the principle.

## Step 4 — Update the README index

File: `docs/adr/README.md`

Add one line to the appropriate category in the "Current ADRs" section:

```markdown
- [ADR-NNNN: Title](NNNN-file.md) - One-line description of the decision
```

**Categories:**
- **Core Architecture** — patterns, layers, framework choices, separation of concerns
- **Feature Implementation** — specific feature designs (history, executor, plan analysis, config)

## Step 5 — Commit

```bash
git add docs/adr/NNNN-file.md docs/adr/README.md
git commit -m "docs(adr): add ADR-NNNN for [topic]"
```
