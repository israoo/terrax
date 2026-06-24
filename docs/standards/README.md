# Standards

## Overview

This directory contains coding, design, and operational standards that define conventions, best practices, and quality guidelines for the TerraX project.

## Purpose

Standards ensure:

- **Consistency**: Uniform code style and structure across the codebase.
- **Quality**: Maintainable, testable, and reliable code.
- **Onboarding**: Clear guidelines for new contributors.
- **Scalability**: Patterns that work as the project grows.
- **Maintainability**: Code that's easy to understand and modify.

## Standard Categories

### Coding Standards

- [Go Coding Standards](go-coding-standards.md)
- [Comment Guidelines](comment-guidelines.md)
- [Error Handling](error-handling.md)

### Design Standards

- [File Organization](file-organization.md)
- [Testing Strategy](testing-strategy.md)

### Operational Standards

- [Documentation Requirements](documentation-requirements.md)
- [Git Workflow](git-workflow.md)
- [Build and Release](build-and-release.md)

## Enforcement

Standards are enforced through:

1. **Code Review**: All pull requests must adhere to standards.
2. **Automated Tools**: `go fmt`, `go vet`, `golangci-lint`.
3. **CI/CD**: Automated checks in continuous integration pipeline.
4. **Documentation**: Clear examples and rationale for each standard.

## Updating Standards

Standards should evolve with the project. To propose a change:

1. Create an issue describing the problem with the current standard.
2. Propose the new standard with examples and rationale.
3. Discuss with the team.
4. Update relevant documentation and code.
5. Create a migration guide if needed.

## Relationship to ADRs

- **ADRs** document **why** we made architectural decisions.
- **Standards** document **how** we implement those decisions consistently.

Example: ADR-0002 establishes the Navigator Pattern (why), while standards/file-organization.md specifies where Navigator code should live (how).

## Quick Reference

### Pre-Commit Checklist

Before committing code:

- [ ] Code compiles: `go build .`
- [ ] All tests pass: `go test ./...`
- [ ] Code formatted: `go fmt ./...`
- [ ] Imports organized (stdlib, third-party, internal)
- [ ] All comments end with periods
- [ ] Errors wrapped with context using `%w`
- [ ] Cross-platform paths using `filepath.Join()`
- [ ] No commented-out code
- [ ] Documentation updated if needed

### Code Review Checklist

**Architecture**:

- [ ] Business logic in `internal/stack/`, not `internal/tui/`
- [ ] Zero Bubble Tea imports in `internal/stack/`
- [ ] Model delegates to Navigator for tree operations
- [ ] Rendering logic in `view.go`, not `model.go`

**Code Quality**:

- [ ] Imports organized (stdlib, third-party, internal)
- [ ] Comments end with periods
- [ ] Errors handled and wrapped with context
- [ ] Cross-platform path handling (`filepath.Join()`)
- [ ] Tests co-located with implementation
- [ ] Edge cases tested
- [ ] No obvious or redundant comments
- [ ] Helpful comments preserved, not deleted

**Testing**:

- [ ] Business logic has unit tests
- [ ] Tests use table-driven format where appropriate
- [ ] Error cases tested
- [ ] Test names are descriptive
