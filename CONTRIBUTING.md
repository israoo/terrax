# Contributing to TerraX

Thank you for your interest in contributing to TerraX! We welcome contributions from the community.

## Quick Links

- **[Code Standards](docs/standards/go-coding-standards.md)**: Coding conventions and best practices.
- **[Testing Strategy](docs/standards/testing-strategy.md)**: How to write and run tests.
- **[Git Workflow](docs/standards/git-workflow.md)**: Branching, commits, and PRs.
- **[Architecture Guide](CLAUDE.md)**: High-level architectural overview.

## Table of Contents

- [Contributing to TerraX](#contributing-to-terrax)
  - [Quick Links](#quick-links)
  - [Table of Contents](#table-of-contents)
  - [Code of Conduct](#code-of-conduct)
  - [How to Report Bugs](#how-to-report-bugs)
  - [Development Process](#development-process)
    - [1. Setup Environment](#1-setup-environment)
    - [2. Create a Branch](#2-create-a-branch)
    - [3. Make Changes](#3-make-changes)
  - [Pull Request Process](#pull-request-process)
  - [Getting Help](#getting-help)

## Code of Conduct

This project adheres to a standard of professionalism and respect. Please be considerate and constructive in all interactions.

## How to Report Bugs

We use [GitHub Issues](https://github.com/israoo/terrax/issues). Please include:

- Operating System & Go Version
- Steps to reproduce
- Expected vs Actual behavior
- Error messages/logs

## Development Process

### 1. Setup Environment

```bash
# Fork & Clone
git clone https://github.com/YOUR_USERNAME/terrax.git
cd terrax

# Install Dependencies
go mod download

# Verify Setup
make test
```

### 2. Create a Branch

Follow our [Git Workflow](docs/standards/git-workflow.md#branch-strategy):

```bash
git checkout -b feature/short-description
```

### 3. Make Changes

- Follow [Go Coding Standards](docs/standards/go-coding-standards.md).
- Write tests as per [Testing Strategy](docs/standards/testing-strategy.md).
- Run linters: `make lint` (or `golangci-lint run`).
- Format code: `make fmt`.

## Pull Request Process

1. **Test**: Ensure `make test` passes.
2. **Lint**: Ensure no lint errors.
3. **Commit**: Use [Conventional Commits](docs/standards/git-workflow.md#commit-conventions).
4. **Push & Open PR**: clearly describe your changes.

## Getting Help

- Open a [Discussion](https://github.com/israoo/terrax/discussions).
- Review [Internal Documentation](docs/README.md).
