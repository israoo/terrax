# TerraX Knowledge Base

**Last Updated**: 2025-12-27

## Overview

This documentation provides a comprehensive knowledge base for TerraX development, including architectural decisions, coding standards, and lessons learned. It serves as a semantic and architectural reference framework to ensure the repository remains modular, scalable, and maintainable.

## Documentation Structure

### 📋 [Architecture Decision Records (ADR)](adr/)

Documents significant architectural and technical decisions, including context, alternatives considered, and rationale.

**Current ADRs**:
- [ADR-0001: Bubble Tea Architecture](adr/0001-bubble-tea-architecture.md)
- [ADR-0002: Navigator Pattern for Business Logic](adr/0002-navigator-pattern.md)
- [ADR-0003: Sliding Window for Deep Hierarchies](adr/0003-sliding-window-navigation.md)
- [ADR-0004: Separation of Concerns](adr/0004-separation-of-concerns.md)

### 📐 [Standards](standards/)

Coding, design, and operational standards that define conventions and best practices.

**Current Standards**:
- [Go Coding Standards](standards/go-coding-standards.md)
- [Comment Guidelines](standards/comment-guidelines.md)
- [Error Handling](standards/error-handling.md)
- [File Organization](standards/file-organization.md)
- [Testing Strategy](standards/testing-strategy.md)
- [Documentation Requirements](standards/documentation-requirements.md)
- [Git Workflow](standards/git-workflow.md)
- [Build and Release](standards/build-and-release.md)

### ⚠️ [Pitfalls](pitfalls/)

Lessons learned, known pitfalls, and common errors to avoid.

**Current Pitfalls**:

**Architecture**:
- [Mixing Business Logic with UI](pitfalls/architecture/mixing-business-logic-ui.md)
- [Tight Coupling Between Layers](pitfalls/architecture/tight-coupling.md)

**Code Quality**:

- [Deleting Helpful Comments](pitfalls/code-quality/deleting-comments.md)
- [Ignoring Errors Silently](pitfalls/code-quality/ignoring-errors.md)

**Testing**:

- [Skipping Unit Tests for Business Logic](pitfalls/testing/skipping-unit-tests.md)

**Performance**:

- [Repeated Filesystem Scans](pitfalls/performance/repeated-filesystem-scans.md)

**Tooling**:

- [Platform-Specific Path Handling](pitfalls/tooling/platform-paths.md)

## Quick Reference

### For New Contributors

1. **Start here**: [CLAUDE.md](../CLAUDE.md) - Project overview and essential commands.
2. **Understand architecture**: [ADR-0001](adr/0001-bubble-tea-architecture.md), [ADR-0004](adr/0004-separation-of-concerns.md).
3. **Follow standards**: [Go Coding Standards](standards/go-coding-standards.md), [File Organization](standards/file-organization.md).
4. **Avoid mistakes**: Review [Pitfalls](pitfalls/) before coding.

### For Implementing Features

1. **Read relevant ADRs**: Understand architectural patterns before implementing.
2. **Follow Navigator Pattern**: Business logic goes in `internal/stack/`, not `internal/tui/`.
3. **Maintain separation**: UI state in `Model`, business logic in `Navigator`, rendering in `View`.
4. **Write tests**: Unit test business logic without UI dependencies.
5. **Check pitfalls**: Ensure you're not repeating known mistakes.

### For Code Review

**Architecture Checklist**:
- [ ] Business logic in `internal/stack/`, not in `internal/tui/`.
- [ ] Zero Bubble Tea imports in `internal/stack/`.
- [ ] Model delegates to Navigator for tree operations.
- [ ] Rendering logic in `view.go`, not `model.go`.

**Code Quality Checklist**:
- [ ] Imports organized per standard (stdlib, third-party, internal).
- [ ] Comments end with periods.
- [ ] Errors handled and wrapped with context.
- [ ] Cross-platform path handling (`filepath.Join()`, never hardcoded `/`).
- [ ] Tests co-located with implementation.

**See also**: Full checklists in [Standards](standards/) directory.

## Document Relationships

```text
CLAUDE.md (project guide)
    ↓
docs/README.md (this file)
    ↓
    ├── ADRs (why we made decisions)
    │   └── inform → Standards (how we implement decisions)
    │
    ├── Standards (how to write code)
    │   └── prevent → Pitfalls (what to avoid)
    │
    └── Pitfalls (what went wrong)
        └── update → ADRs & Standards (improve process)
```

**Flow**:
1. **ADRs** document architectural decisions and trade-offs.
2. **Standards** translate decisions into concrete coding practices.
3. **Pitfalls** capture mistakes and inform future standards/ADRs.

## Contributing to Documentation

### When to Write an ADR

Create an ADR when making decisions that:
- Affect overall architecture or structure.
- Involve significant trade-offs.
- Are difficult to change later.
- Need to be understood by future contributors.

**See**: [ADR README](adr/README.md) for format and guidelines.

### When to Write a Standard

Create a standard when you need to:
- Enforce consistency across the codebase.
- Document best practices for recurring tasks.
- Translate architectural decisions into concrete guidelines.

**See**: [Standards README](standards/README.md) for format and guidelines.

### When to Document a Pitfall

Document a pitfall when you:
- Encounter a bug or issue caused by architectural violation.
- Identify a common mistake that could be prevented.
- Fix a problem that might recur without documentation.
- Learn something that would benefit future contributors.

**See**: [Pitfalls README](pitfalls/README.md) for format and guidelines.

## Maintenance

### Review Cycle

- **Monthly**: Review recent pitfalls for patterns, update standards if needed.
- **Quarterly**: Review ADRs for status updates, deprecate or supersede if needed.
- **Annually**: Comprehensive review of all documentation for relevance.

### Updating Documents

**ADRs**:
- Never delete, only deprecate or supersede.
- Update status: Proposed → Accepted → Deprecated/Superseded.
- Link to replacement ADR when superseding.

**Standards**:
- Keep current with codebase evolution.
- Propose changes via issue/PR discussion.
- Include migration guide for breaking changes.

**Pitfalls**:
- Update as issues are resolved.
- Archive if no longer relevant.
- Link to related ADRs/standards when root cause is addressed.

## Project Context

### TerraX Architecture Overview

```text
terrax/
├── cmd/                    # CLI layer (coordination)
├── internal/
│   ├── stack/              # Business logic (zero UI dependencies)
│   └── tui/                # Presentation layer (Bubble Tea)
└── docs/                   # This knowledge base
```

**Core Principles**:
1. **Separation of Concerns**: Business logic, UI state, and rendering are isolated.
2. **Bubble Tea Architecture**: Strict Model-Update-View pattern.
3. **Navigator Pattern**: Business logic encapsulated in Navigator.
4. **Cross-Platform**: Works on Linux, macOS, Windows.

**See**: [CLAUDE.md](../CLAUDE.md) for comprehensive project overview.

## Getting Help

### Questions About...

**Architecture**: Read relevant ADRs, check CLAUDE.md.

**Coding Practices**: Check standards directory.

**Specific Mistakes**: Search pitfalls directory.

**General Questions**: Open an issue or discussion in the repository.

## External References

- [Go Documentation](https://golang.org/doc/)
- [Effective Go](https://golang.org/doc/effective_go)
- [Bubble Tea Documentation](https://github.com/charmbracelet/bubbletea)
- [Lipgloss Documentation](https://github.com/charmbracelet/lipgloss)
- [Architecture Decision Records](https://adr.github.io/)

## Feedback

This knowledge base is a living document. If you find:
- Missing information
- Unclear explanations
- Outdated content
- Mistakes or errors

Please open an issue or submit a pull request to improve the documentation.

---

**Remember**: The goal is not perfect documentation, but **useful** documentation that helps developers build and maintain TerraX effectively. Write for your future self and your teammates.
