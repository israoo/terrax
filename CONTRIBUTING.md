# Contributing to TerraX

Thank you for your interest in contributing to TerraX! We welcome contributions from the community and are grateful for your support.

## Table of contents

- [Contributing to TerraX](#contributing-to-terrax)
  - [Table of contents](#table-of-contents)
  - [Code of conduct](#code-of-conduct)
  - [How to report bugs](#how-to-report-bugs)
  - [How to suggest features](#how-to-suggest-features)
  - [Development process](#development-process)
    - [1. Fork and clone](#1-fork-and-clone)
    - [2. Create a feature branch](#2-create-a-feature-branch)
    - [3. Set up development environment](#3-set-up-development-environment)
    - [4. Make your changes](#4-make-your-changes)
  - [Pull request guidelines](#pull-request-guidelines)
    - [Before submitting](#before-submitting)
    - [Submitting your PR](#submitting-your-pr)
    - [PR requirements](#pr-requirements)
  - [Code standards](#code-standards)
    - [General guidelines](#general-guidelines)
    - [Project-specific patterns](#project-specific-patterns)
    - [Linting](#linting)
    - [Formatting](#formatting)
  - [Testing requirements](#testing-requirements)
    - [Unit tests](#unit-tests)
    - [Test execution](#test-execution)
    - [Test organization](#test-organization)
  - [Commit message guidelines](#commit-message-guidelines)
    - [Types](#types)
    - [Examples](#examples)
    - [Commit best practices](#commit-best-practices)
  - [License](#license)
  - [Questions?](#questions)

## Code of conduct

This project adheres to a standard of professionalism and respect. Please be considerate and constructive in all interactions.

## How to report bugs

We use [GitHub Issues](https://github.com/israoo/terrax/issues) to track bugs. Before creating a new issue, please check if it already exists.

When reporting a bug, please include:

- **Environment details:**
  - Operating System (Linux, macOS, Windows)
  - Go Version (`go version`)
  - TerraX Version (`terrax --version`)

- **Steps to reproduce:**
  1. Describe the exact steps to reproduce the issue
  2. Include any relevant configuration files
  3. Provide sample stack structures if applicable

- **Expected behavior:** What you expected to happen

- **Actual behavior:** What actually happened

- **Error messages:** Include complete error messages and stack traces

- **Additional context:** Screenshots, logs, or any other relevant information

## How to suggest features

We welcome feature suggestions! Please use [GitHub Issues](https://github.com/israoo/terrax/issues) and include:

- **Use case:** Describe the problem you're trying to solve
- **Proposed solution:** How you envision the feature working
- **Alternatives considered:** Other approaches you've thought about
- **Additional context:** Examples, mockups, or references to similar features in other tools

## Development process

### 1. Fork and clone

```bash
# Fork the repository on GitHub
# Clone your fork
git clone https://github.com/YOUR_USERNAME/terrax.git
cd terrax

# Add upstream remote
git remote add upstream https://github.com/israoo/terrax.git
```

### 2. Create a feature branch

Use descriptive branch names with prefixes:

```bash
# For new features
git checkout -b feature/your-feature-name

# For bug fixes
git checkout -b bugfix/issue-description

# For documentation
git checkout -b docs/what-you-are-documenting
```

### 3. Set up development environment

```bash
# Install Go 1.25 or later
# Verify installation
go version

# Install dependencies
go mod download

# Install development tools
go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest

# Install pre-commit hooks (optional but recommended)
pip install pre-commit
pre-commit install
```

### 4. Make your changes

- Write clean, idiomatic Go code
- Follow the existing code structure and patterns
- Add or update tests as needed
- Update documentation if you're changing functionality

## Pull request guidelines

### Before submitting

1. **Sync with upstream:**

   ```bash
   git fetch upstream
   git rebase upstream/main
   ```

2. **Run tests:**

   ```bash
   make test
   # Or manually:
   go test ./... -race -coverprofile=coverage.out
   ```

3. **Run linters:**

   ```bash
   golangci-lint run --timeout=5m
   # Or use pre-commit:
   pre-commit run --all-files
   ```

4. **Build verification:**

   ```bash
   make build
   # Or manually:
   go build -v .
   ```

### Submitting your PR

1. Push your branch to your fork
2. Create a Pull Request against the `main` branch
3. Fill out the PR template completely
4. Link any related issues

### PR requirements

- ✅ All tests pass
- ✅ Code coverage maintained or improved
- ✅ No linting errors
- ✅ Clear commit messages
- ✅ Documentation updated if needed
- ✅ No merge conflicts

## Code standards

### General guidelines

- Follow [Effective Go](https://golang.org/doc/effective_go) principles
- Use meaningful variable and function names
- Keep functions focused and concise
- Add comments for exported functions and complex logic
- Avoid global variables

### Project-specific patterns

See [CLAUDE.md](CLAUDE.md) for detailed architectural patterns, including:

- **3-Layer Separation:** `cmd/`, `internal/stack/`, `internal/tui/`
- **Navigator Pattern:** Business logic delegation
- **Bubble Tea Model-Update-View:** TUI architecture
- **Cross-Platform Paths:** Always use `filepath.Join()`

### Linting

We use `golangci-lint` with configuration in `.golangci.yml`. The pre-commit hooks will automatically run linting.

```bash
# Run manually
golangci-lint run --timeout=5m
```

### Formatting

Code must be formatted with `gofmt`:

```bash
# Format all files
gofmt -w .

# Or use pre-commit
pre-commit run go-fmt --all-files
```

## Testing requirements

### Unit tests

- All new features must include unit tests
- Aim for >85% code coverage
- Use table-driven tests where appropriate
- Mock external dependencies using interfaces

```go
func TestNavigatorSelection(t *testing.T) {
    tests := []struct {
        name     string
        input    NavigationState
        expected string
    }{
        {"basic selection", state1, "/expected/path"},
        {"nested selection", state2, "/expected/nested/path"},
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            result := navigator.GetSelectedPath(tt.input)
            assert.Equal(t, tt.expected, result)
        })
    }
}
```

### Test execution

```bash
# Run all tests
make test

# Run with coverage
make test-coverage
```

### Test organization

- Place tests in `*_test.go` files
- Use `afero.MemMapFs` for filesystem isolation (see `internal/stack/tree_test.go`)
- Use `teatest` for Bubble Tea TUI testing (see `internal/tui/model_test_helpers.go`)
- Keep Navigator tests pure - no Bubble Tea dependencies

## Commit message guidelines

We follow [Conventional Commits](https://www.conventionalcommits.org/) format:

```
<type>(<scope>): <subject>

<body>

<footer>
```

### Types

- `feat`: New feature
- `fix`: Bug fix
- `docs`: Documentation changes
- `style`: Code style changes (formatting, no logic change)
- `refactor`: Code refactoring
- `perf`: Performance improvements
- `test`: Adding or updating tests
- `chore`: Maintenance tasks, dependency updates
- `ci`: CI/CD changes

### Examples

```
feat(navigator): add support for multi-level filtering

Implement filtering across multiple navigation levels simultaneously.
Filters persist as user navigates through the tree structure.

Closes #42
```

```
fix(tui): resolve race condition in selection state

The selection index was being updated from multiple goroutines.
Added mutex protection for the NavigationState access.

Fixes #67
```

```
docs(readme): update installation instructions

Added Homebrew and Scoop installation methods.
Updated prerequisites section.
```

### Commit best practices

- Keep commits atomic (one logical change per commit)
- Write clear, descriptive messages
- Use present tense ("add feature" not "added feature")
- Reference issues and PRs when applicable
- Keep subject line under 72 characters

## License

By contributing to TerraX, you agree that your contributions will be licensed under the [Apache License 2.0](LICENSE).

All contributions must:

- Be your original work or properly attributed
- Not violate any third-party licenses
- Be compatible with Apache 2.0 license terms

When submitting a PR, you certify that:

1. You have the right to submit the contribution
2. You understand it will be released under Apache 2.0
3. You grant the project maintainers the right to use your contribution

---

## Questions?

If you have questions about contributing, feel free to:

- Open a [Discussion](https://github.com/israoo/terrax/discussions)
- Ask in your PR or Issue
- Review [CLAUDE.md](CLAUDE.md) for architectural guidance

**Thank you for contributing to TerraX!** 🎉
