# Architecture Decision Records (ADR)

## Overview

This directory contains Architecture Decision Records (ADRs) that document significant architectural and technical decisions made in the TerraX project.

## Purpose

ADRs serve to:

- **Capture context**: Record the circumstances and constraints that led to a decision.
- **Document alternatives**: List options considered and why they were rejected.
- **Explain rationale**: Provide clear reasoning for the chosen approach.
- **Guide future work**: Help contributors understand architectural foundations.
- **Prevent regression**: Avoid revisiting settled decisions without new context.

## Format

Each ADR follows this structure:

```markdown
# ADR-NNNN: Title

**Status**: [Proposed | Accepted | Deprecated | Superseded]

**Date**: YYYY-MM-DD

**Deciders**: [Names/Roles]

## Context

What is the issue we're facing? What are the constraints and requirements?

## Decision

What decision did we make?

## Consequences

What are the positive and negative outcomes of this decision?

### Positive

- Benefit 1
- Benefit 2

### Negative

- Trade-off 1
- Trade-off 2

## Alternatives Considered

### Option 1: [Name]

Description and why it was rejected.

### Option 2: [Name]

Description and why it was rejected.

## References

- Links to relevant discussions, documentation, or external resources
```

## Naming Convention

ADRs are numbered sequentially and use descriptive titles:

- `0001-bubble-tea-architecture.md`
- `0002-navigator-pattern.md`
- `0003-sliding-window-navigation.md`

## Status Lifecycle

- **Proposed**: Under discussion, not yet accepted.
- **Accepted**: Decision made and being implemented/maintained.
- **Deprecated**: No longer recommended but still in use.
- **Superseded**: Replaced by a newer ADR (link to replacement).

## When to Write an ADR

Create an ADR for decisions that:

- Affect the overall architecture or structure.
- Involve significant trade-offs.
- Are difficult to change later.
- Need to be understood by future contributors.
- Establish patterns or conventions for the project.

## Current ADRs

- [ADR-0001: Bubble Tea Architecture](0001-bubble-tea-architecture.md)
- [ADR-0002: Navigator Pattern for Business Logic](0002-navigator-pattern.md)
- [ADR-0003: Sliding Window for Deep Hierarchies](0003-sliding-window-navigation.md)
- [ADR-0004: Separation of Concerns](0004-separation-of-concerns.md)
