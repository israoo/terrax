# Pitfalls and Lessons Learned

## Overview

This directory documents lessons learned, known pitfalls, common errors, and anti-patterns encountered during TerraX development.

## Purpose

This knowledge base serves to:

- **Prevent repetition**: Avoid making the same mistakes twice.
- **Share knowledge**: Help team members learn from past experiences.
- **Identify patterns**: Recognize recurring issues and address root causes.
- **Improve onboarding**: Warn new contributors about common traps.
- **Drive improvement**: Inform future architectural and process decisions.

## Format

Each pitfall document follows this structure:

```markdown
# Pitfall: Title

**Category**: [Architecture | Code Quality | Testing | Performance | Tooling]

**Severity**: [Critical | High | Medium | Low]

**Date Identified**: YYYY-MM-DD

## Description

Clear description of the pitfall or mistake.

## Impact

What problems did this cause? What were the consequences?

- Impact 1
- Impact 2

## Root Cause

What led to this mistake? Why did it happen?

## How to Avoid

Specific, actionable guidance to prevent this in the future.

### Do

- Positive action 1
- Positive action 2

### Don't

- Anti-pattern 1
- Anti-pattern 2

## Detection

How can you recognize this pitfall early?

- Warning sign 1
- Warning sign 2

## Remediation

If you've already made this mistake, how do you fix it?

1. Step 1
2. Step 2

## Related

- Related ADRs
- Related standards
- Related pitfalls
- External references

## Examples

### Bad

```go
// Example of the pitfall
```

### Good

```go
// Correct approach
```
```

## Categories

### Architecture

Structural and design mistakes that affect overall system organization.

- [Mixing Business Logic with UI](architecture/mixing-business-logic-ui.md)
- [Tight Coupling Between Layers](architecture/tight-coupling.md)

### Code Quality

Code-level issues that reduce maintainability or reliability.

- [Ignoring Errors Silently](code-quality/ignoring-errors.md)
- [Deleting Helpful Comments](code-quality/deleting-comments.md)

### Testing

Testing-related mistakes and gaps.

- [Skipping Unit Tests for Business Logic](testing/skipping-unit-tests.md)
- [Testing UI Instead of Business Logic](testing/testing-wrong-layer.md)

### Performance

Performance issues and inefficiencies.

- [Repeated Filesystem Scans](performance/repeated-fs-scans.md)
- [Excessive String Allocations in Rendering](performance/string-allocations.md)

### Tooling

Build, development environment, and tooling issues.

- [Platform-Specific Path Handling](tooling/platform-paths.md)
- [Committing Build Artifacts](tooling/committing-binaries.md)

## Current Pitfalls

- [Mixing Business Logic with UI](architecture/mixing-business-logic-ui.md)
- [Ignoring Errors Silently](code-quality/ignoring-errors.md)
- [Deleting Helpful Comments](code-quality/deleting-comments.md)
- [Platform-Specific Path Handling](tooling/platform-paths.md)

## Contributing

When you encounter a new pitfall:

1. Document it immediately while context is fresh.
2. Include concrete examples (anonymized if needed).
3. Link to related ADRs or standards.
4. Update checklists or standards to prevent recurrence.
5. Share with the team for discussion.

## Review Cycle

Pitfalls should be reviewed periodically:

- **Monthly**: Review recent pitfalls for patterns.
- **Quarterly**: Update standards or ADRs based on learnings.
- **Annually**: Archive resolved pitfalls or elevate critical ones.
