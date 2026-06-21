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

**Related**: [ADR-NNNN: Title](NNNN-title.md) (if applicable)

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

**Description**: Description of the alternative.

**Pros**:

- Pros of the alternative.

**Cons**:

- Cons of the alternative.

**Why rejected**: Reason why the alternative was rejected.

### Option 2: [Name]

**Description**: Description of the alternative.

**Pros**:

- Pros of the alternative.

**Cons**:

- Cons of the alternative.

**Why rejected**: Reason why the alternative was rejected.

## Future Enhancements (if applicable)

**Potential Improvements**:

1. **Potential Future Improvement 1**: Description of a potential future improvement.
2. **Potential Future Improvement 2**: Description of another potential future improvement.

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

### Core Architecture

- [ADR-0001: Bubble Tea Architecture](0001-bubble-tea-architecture.md) - Elm Architecture pattern with Model-Update-View
- [ADR-0002: Navigator Pattern for Business Logic](0002-navigator-pattern.md) - Separation of business logic from UI framework
- [ADR-0003: Sliding Window for Deep Hierarchies](0003-sliding-window-navigation.md) - Fixed 3-column navigation window
- [ADR-0004: Separation of Concerns](0004-separation-of-concerns.md) - Layer boundaries and responsibilities

### Feature Implementation

- [ADR-0005: Filesystem Tree Building Strategy](0005-filesystem-tree-building-strategy.md) - Single-pass scanning with stack detection
- [ADR-0006: Execution History Management](0006-execution-history-management.md) - Project-aware history with JSONL persistence
- [ADR-0007: Configuration Management Strategy](0007-configuration-management-strategy.md) - Defaults-first approach with Viper
- [ADR-0008: Dual-Mode TUI Architecture](0008-dual-mode-tui-architecture.md) - State machine for navigation and history modes
- [ADR-0009: Executor Isolation Pattern](0009-executor-isolation-pattern.md) - Command execution with dependency injection
- [ADR-0010: Plan Analysis Workflow](0010-plan-analysis-workflow.md) - Binary plan files and structured TUI presentation
- [ADR-0011: Extensible Flags Configuration](0011-extensible-flags-configuration.md) - Three-layer flag model for pipeline-grade Terragrunt execution
- [ADR-0012: Force Unlock via AWS CLI Lock Discovery](0012-force-unlock-via-aws-cli.md) - Automatic lock ID discovery from S3 using AWS CLI subprocess
- [ADR-0013: VS Code Extension Integration](0013-vscode-extension-integration.md) - Monorepo extension with tree JSON API, direct run subcommand, and single terminal reuse
- [ADR-0014: Plan Summary Mode](0014-plan-summary-mode.md) - Native Go JSON parsing for terminal plan summary with .terrax/ output directory
- [ADR-0015: Static HCL Dependency Graph](0015-static-hcl-dependency-graph.md) - Regex-based static parsing of Terragrunt dependency blocks with callerDir propagation for envcommon include chains
- [ADR-0016: Cycle Detection and Reverse Dependency Graph](0016-cycle-detection-and-reverse-dependency-graph.md) - DFS-based cycle detection and reverse graph embedded in FindAndBuildTree, surfaced in VS Code Dependents panel
- [ADR-0017: Filter-Based Execution Strategy](0017-filter-based-execution-strategy.md) - Pre-computed --filter list from static HCL analysis replaces --all --working-dir and --queue-include-external
- [ADR-0018: Plan Analysis via --json-out-dir](0018-plan-analysis-via-json-out-dir.md) - CollectFromJSONDir replaces binary Collector; --json-out-dir shared by review TUI and terminal summary
