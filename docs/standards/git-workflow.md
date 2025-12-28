# Git Workflow Standards

**Status**: Active

**Last Updated**: 2025-12-27

## Overview

This document defines Git workflow standards for the TerraX project, including branching strategy, commit conventions, and collaboration processes.

## Core Principles

1. **Main is stable**: The `main` branch always contains production-ready code.
2. **Feature branches**: All development happens in feature branches.
3. **Atomic commits**: Each commit represents a single logical change.
4. **Descriptive messages**: Commit messages explain what and why.
5. **Clean history**: Keep history readable and meaningful.

## Branch Strategy

### Branch Types

#### Main Branch

**Name**: `main`

**Purpose**: Production-ready code, stable releases.

**Rules**:
- Never commit directly to `main`.
- All changes via pull requests.
- Must pass all CI checks before merge.
- Protected branch (requires reviews).

#### Feature Branches

**Name**: `feature/<description>` or `feature/<issue-number>-<description>`

**Examples**:
- `feature/add-command-execution`
- `feature/123-sliding-window-navigation`
- `feature/improve-error-messages`

**Purpose**: Develop new features.

**Lifecycle**:
1. Create from `main`: `git checkout -b feature/my-feature main`
2. Develop and commit changes
3. Push to remote: `git push -u origin feature/my-feature`
4. Create pull request
5. Merge to `main` after review
6. Delete branch after merge

**Rules**:
- Branch from latest `main`
- Keep focused on single feature
- Regularly sync with `main` if long-lived
- Delete after merge

#### Bugfix Branches

**Name**: `bugfix/<description>` or `fix/<issue-number>-<description>`

**Examples**:
- `bugfix/tree-scan-crash`
- `fix/456-breadcrumb-overflow`
- `fix/selection-out-of-bounds`

**Purpose**: Fix bugs in existing features.

**Lifecycle**: Same as feature branches.

**Rules**: Same as feature branches.

#### Hotfix Branches

**Name**: `hotfix/<version>-<description>`

**Examples**:
- `hotfix/0.2.1-critical-crash`
- `hotfix/0.3.1-security-fix`

**Purpose**: Emergency fixes for production issues.

**Lifecycle**:
1. Create from `main`
2. Fix issue with minimal changes
3. Create PR with urgent label
4. Fast-track review
5. Merge to `main`
6. Tag new version immediately
7. Delete branch

**Rules**:
- Use sparingly, only for critical production issues
- Minimal changes only (no feature additions)
- Fast-track review process
- Immediate version bump and release

#### Refactor Branches

**Name**: `refactor/<description>`

**Examples**:
- `refactor/extract-renderer`
- `refactor/simplify-navigation`

**Purpose**: Code improvements without changing behavior.

**Rules**:
- No behavior changes
- Must maintain all existing tests
- Add tests if coverage improves

#### Documentation Branches

**Name**: `docs/<description>`

**Examples**:
- `docs/update-readme`
- `docs/add-adr-navigation`

**Purpose**: Documentation-only changes.

**Rules**:
- No code changes
- Can be fast-tracked if trivial

### Branch Naming Conventions

**Format**: `<type>/<description>`

**Type**:
- `feature/` - New features
- `bugfix/` or `fix/` - Bug fixes
- `hotfix/` - Critical production fixes
- `refactor/` - Code refactoring
- `docs/` - Documentation only
- `test/` - Test improvements only
- `chore/` - Build, CI, or tooling changes

**Description**:
- Lowercase with hyphens
- Descriptive and concise
- Optional issue number prefix

**Examples**:
```bash
# Good
feature/command-execution
fix/234-tree-scan-crash
refactor/extract-layout-calculator
docs/add-testing-guide

# Bad
Feature/Command-Execution      # Wrong case
fix_tree_crash                 # Underscores instead of hyphens
my-branch                      # Not descriptive
```

## Commit Conventions

### Commit Message Format

TerraX uses [Conventional Commits](https://www.conventionalcommits.org/) format:

```
<type>[optional scope]: <description>

[optional body]

[optional footer(s)]
```

### Commit Types

- **feat**: New feature
- **fix**: Bug fix
- **refactor**: Code change that neither fixes a bug nor adds a feature
- **docs**: Documentation only changes
- **test**: Adding or updating tests
- **chore**: Changes to build process, CI, or tooling
- **perf**: Performance improvement
- **style**: Code style changes (formatting, missing semicolons, etc.)

### Commit Scope (Optional)

Scope indicates the area of change:

- `stack` - Business logic (internal/stack)
- `tui` - UI layer (internal/tui)
- `cmd` - CLI layer (cmd/)
- `docs` - Documentation
- `ci` - CI/CD configuration
- `build` - Build system

### Commit Message Examples

**Feature**:
```
feat(tui): add sliding window navigation for deep hierarchies

Implement sliding window pattern that shows maximum 3 navigation
columns, with window sliding right as users navigate deeper. This
allows navigation of arbitrarily deep trees without horizontal
overflow.

Closes #123
```

**Bug Fix**:
```
fix(stack): handle empty tree in breadcrumb generation

PropagateSelection was panicking when called on empty tree.
Added nil check and early return.

Fixes #456
```

**Refactor**:
```
refactor(tui): extract rendering logic to Renderer pattern

Separate rendering concerns from Model by introducing Renderer
type. LayoutCalculator handles layout math, Renderer handles
Lipgloss styling.

No behavior changes, existing tests still pass.
```

**Documentation**:
```
docs: add ADR for Navigator Pattern

Document architectural decision to separate business logic from
UI framework using Navigator pattern.
```

**Chore**:
```
chore(ci): add test coverage reporting

Configure CI to generate and upload coverage reports to Codecov.
```

### Commit Message Guidelines

**Subject line** (first line):
- Start with type and optional scope
- Use imperative mood ("add", not "added" or "adds")
- No period at end
- Max 72 characters
- Capitalize first word after colon

**Body** (optional):
- Separate from subject with blank line
- Wrap at 72 characters
- Explain what and why, not how
- Use imperative mood
- Can include multiple paragraphs

**Footer** (optional):
- Reference issues: `Closes #123`, `Fixes #456`, `Refs #789`
- Breaking changes: `BREAKING CHANGE: description`
- Co-authors: `Co-Authored-By: Name <email>`

### What Makes a Good Commit

**Atomic commits**:
- One logical change per commit
- Tests pass after each commit
- Can be reverted independently

**Good examples**:
```
feat(stack): add tree depth validation

fix(tui): prevent panic on window resize

refactor(tui): extract LayoutCalculator

test(stack): add tests for edge cases in navigation
```

**Bad examples**:
```
# Too vague
fix: bug fix

# Multiple changes
feat: add feature and fix bugs and update docs

# Past tense
fixed: the crash

# No context
update code
```

## Working with Git

### Starting New Work

```bash
# Update main
git checkout main
git pull origin main

# Create feature branch
git checkout -b feature/my-feature

# Make changes and commit
git add .
git commit -m "feat(scope): description"

# Push to remote
git push -u origin feature/my-feature
```

### Making Commits

```bash
# Stage specific files (preferred)
git add path/to/file1.go path/to/file2.go

# Or stage all changes (use carefully)
git add .

# Check what's staged
git status
git diff --staged

# Commit with message
git commit -m "feat(scope): description"

# Or commit with editor for longer message
git commit
```

### Updating Your Branch

```bash
# Fetch latest changes
git fetch origin

# Rebase on main (preferred for clean history)
git rebase origin/main

# Or merge (if conflicts are complex)
git merge origin/main

# Resolve conflicts if any
# Edit conflicted files
git add <resolved-files>
git rebase --continue  # or git merge --continue
```

### Syncing with Main

For long-lived branches, regularly sync with main:

```bash
# Update main
git checkout main
git pull origin main

# Return to feature branch
git checkout feature/my-feature

# Rebase onto main
git rebase main

# Force push (only if branch not shared)
git push --force-with-lease origin feature/my-feature
```

### Amending Commits

```bash
# Amend last commit (message or content)
git add <files>
git commit --amend

# Amend without changing message
git add <files>
git commit --amend --no-edit

# Force push if already pushed (only if branch not shared)
git push --force-with-lease origin feature/my-feature
```

**Rules for amending**:
- Only amend commits not yet pushed
- If pushed, only amend if you're sole contributor to branch
- Use `--force-with-lease` not `--force`
- Never amend commits on `main`

### Interactive Rebase

Clean up commits before creating PR:

```bash
# Rebase last 3 commits
git rebase -i HEAD~3

# Or rebase from main
git rebase -i main

# In editor, choose action for each commit:
# pick   - keep commit as-is
# reword - change commit message
# edit   - amend commit content
# squash - merge with previous commit
# fixup  - like squash but discard message
# drop   - remove commit
```

**When to use**:
- Squash "fix typo" commits
- Reorder commits logically
- Split large commits
- Clean up work-in-progress commits

**When NOT to use**:
- Commits already pushed to shared branches
- Commits on `main`
- Public history

### Resolving Conflicts

```bash
# During rebase or merge
git status  # See conflicted files

# Edit conflicted files, look for:
# <<<<<<< HEAD
# Your changes
# =======
# Their changes
# >>>>>>> branch-name

# After resolving
git add <resolved-files>

# Continue rebase
git rebase --continue

# Or abort if needed
git rebase --abort
```

## Pull Requests

### Creating a Pull Request

**Before creating PR**:
1. Ensure all commits follow conventions
2. All tests pass locally: `go test ./...`
3. Code is formatted: `go fmt ./...`
4. Branch is up-to-date with `main`
5. No merge conflicts

**PR Title**:
- Follow commit convention format
- Descriptive and concise

**PR Description**:

```markdown
## Summary

Brief description of changes and motivation.

## Changes

- Change 1
- Change 2
- Change 3

## Testing

How were these changes tested?

- [ ] Unit tests added/updated
- [ ] Manual testing completed
- [ ] All existing tests pass

## Screenshots (if UI changes)

[Add screenshots or GIFs if applicable]

## Related Issues

Closes #123
Refs #456

## Checklist

- [ ] Code follows project standards
- [ ] Comments added/updated
- [ ] Documentation updated
- [ ] Tests added/updated
- [ ] No breaking changes (or documented)
```

### PR Labels

Use labels to categorize PRs:

- `feature` - New feature
- `bugfix` - Bug fix
- `enhancement` - Improvement to existing feature
- `refactor` - Code refactoring
- `docs` - Documentation
- `tests` - Test improvements
- `breaking` - Breaking change
- `urgent` - Needs fast-track review
- `work-in-progress` - Not ready for review

### PR Size Guidelines

**Ideal PR**:
- ~200-400 lines of changes
- Single focused change
- Reviewable in 15-30 minutes

**Large PR** (>500 lines):
- Consider splitting into multiple PRs
- Provide detailed description
- Highlight areas needing review

**When large PRs are acceptable**:
- Initial feature implementation
- Large refactoring (pre-discussed)
- Generated code
- Documentation

### Review Process

**Author responsibilities**:
1. Respond to review comments promptly
2. Address all feedback or discuss alternatives
3. Mark conversations as resolved after addressing
4. Request re-review after changes

**Reviewer responsibilities**:
1. Review within 24-48 hours
2. Provide constructive feedback
3. Approve when satisfied
4. Be respectful and helpful

**Review checklist**: See [Code Review Checklist](README.md#code-review-checklist)

### Merging Pull Requests

**Merge strategies**:

1. **Squash and merge** (default for TerraX):
   - Combines all commits into one
   - Clean, linear history
   - Use for feature branches

2. **Rebase and merge**:
   - Preserves individual commits
   - Use when commits are well-structured
   - Use for multiple logical changes

3. **Merge commit**:
   - Creates merge commit
   - Use sparingly
   - Use for long-lived branches

**After merge**:
- Delete feature branch
- Close related issues
- Update project board if applicable

## Git Configuration

### Recommended Git Config

```bash
# Set user info
git config --global user.name "Your Name"
git config --global user.email "your.email@example.com"

# Use main as default branch
git config --global init.defaultBranch main

# Better diff output
git config --global diff.algorithm histogram

# Enable rerere (reuse recorded resolution)
git config --global rerere.enabled true

# Auto-prune on fetch
git config --global fetch.prune true

# Push current branch only
git config --global push.default current

# Colorful output
git config --global color.ui auto

# Better merge conflict markers
git config --global merge.conflictstyle diff3
```

### Git Aliases

Add to `~/.gitconfig`:

```ini
[alias]
    # Short status
    st = status -sb

    # Pretty log
    lg = log --graph --pretty=format:'%Cred%h%Creset -%C(yellow)%d%Creset %s %Cgreen(%cr) %C(bold blue)<%an>%Creset' --abbrev-commit

    # Amend last commit
    amend = commit --amend --no-edit

    # List branches
    br = branch -v

    # Checkout
    co = checkout

    # Create and checkout branch
    cob = checkout -b

    # Diff staged changes
    ds = diff --staged

    # Push with lease
    pushf = push --force-with-lease

    # Undo last commit (keep changes)
    undo = reset HEAD~1 --soft

    # Clean up merged branches
    cleanup = !git branch --merged | grep -v \"\\*\" | grep -v main | xargs -n 1 git branch -d
```

## Common Workflows

### Adding a New Feature

```bash
# 1. Create feature branch
git checkout main
git pull origin main
git checkout -b feature/my-feature

# 2. Make changes and commit
# ... edit files ...
git add <files>
git commit -m "feat(scope): add feature"

# 3. Keep branch updated
git fetch origin
git rebase origin/main

# 4. Push to remote
git push -u origin feature/my-feature

# 5. Create pull request on GitHub

# 6. Address review feedback
# ... make changes ...
git add <files>
git commit -m "fix: address review comments"
git push

# 7. After merge, clean up
git checkout main
git pull origin main
git branch -d feature/my-feature
```

### Fixing a Bug

```bash
# 1. Create bugfix branch
git checkout -b fix/123-bug-description

# 2. Fix bug and add test
# ... edit files ...
git add <files>
git commit -m "fix(scope): fix bug description

Detailed explanation of bug and fix.

Fixes #123"

# 3. Push and create PR
git push -u origin fix/123-bug-description

# 4. Follow review process
```

### Emergency Hotfix

```bash
# 1. Create hotfix branch
git checkout main
git pull origin main
git checkout -b hotfix/0.2.1-critical-fix

# 2. Make minimal fix
# ... edit files ...
git add <files>
git commit -m "fix: critical production issue

Description of issue and fix.

Fixes #urgent-issue"

# 3. Push and create urgent PR
git push -u origin hotfix/0.2.1-critical-fix

# 4. After merge, tag immediately
git checkout main
git pull origin main
git tag -a v0.2.1 -m "Hotfix: critical production issue"
git push origin v0.2.1
```

### Updating Documentation

```bash
# 1. Create docs branch
git checkout -b docs/update-readme

# 2. Update documentation
# ... edit docs ...
git add docs/
git commit -m "docs: update README with new features"

# 3. Push and create PR
git push -u origin docs/update-readme
```

## What NOT to Commit

### Files to Ignore

See `.gitignore`:

```gitignore
# Build outputs
build/
terrax
*.exe
*.dll
*.so
*.dylib

# Test artifacts
*.test
*.out
coverage.html

# IDE files
.vscode/
.idea/
*.swp
*.swo
*~

# OS files
.DS_Store
.DS_Store?
._*
.Spotlight-V100
.Trashes
ehthumbs.db
Thumbs.db

# Environment
.env
.env.local

# Dependencies
vendor/

# Temporary files
tmp/
temp/
*.tmp
```

### Never Commit

- Secrets or credentials
- API keys or tokens
- Passwords
- Private keys
- Build artifacts
- Binary files (unless necessary)
- Large files (>1MB without good reason)
- IDE-specific settings (personal preferences)
- Temporary or cache files
- Log files

### Removing Committed Secrets

If secrets are accidentally committed:

```bash
# 1. Remove from current commit (if not pushed)
git reset HEAD~1
# Remove secrets, add to .gitignore
git add .
git commit -m "fix: remove secrets"

# 2. If already pushed, rotate secrets immediately
# Then use BFG Repo-Cleaner or git-filter-repo
# See https://github.com/newren/git-filter-repo

# 3. Force push (only after secret rotation)
git push --force-with-lease
```

**Important**: Rotated compromised secrets immediately. Removing from Git history is not sufficient.

## Git Best Practices

### Do

- Commit early and often
- Write descriptive commit messages
- Keep commits atomic and focused
- Test before committing
- Pull before starting new work
- Use branches for all changes
- Rebase to keep history clean
- Review your changes before committing
- Use `.gitignore` properly

### Don't

- Commit directly to `main`
- Commit broken code
- Commit secrets or credentials
- Use `git push --force` (use `--force-with-lease`)
- Create huge commits with unrelated changes
- Commit commented-out code
- Commit IDE-specific files
- Amend public history
- Mix refactoring with feature work

## Troubleshooting

### Undo Last Commit (Not Pushed)

```bash
# Keep changes staged
git reset --soft HEAD~1

# Keep changes unstaged
git reset HEAD~1

# Discard changes (careful!)
git reset --hard HEAD~1
```

### Undo Last Commit (Already Pushed)

```bash
# Create new commit that reverses changes
git revert HEAD
git push
```

### Discard Local Changes

```bash
# Discard changes to specific file
git checkout -- <file>

# Discard all uncommitted changes (careful!)
git reset --hard HEAD
```

### Recover Deleted Branch

```bash
# Find commit hash
git reflog

# Recreate branch
git checkout -b <branch-name> <commit-hash>
```

### Fix Wrong Branch

```bash
# If you committed to main instead of feature branch
git checkout -b feature/my-feature  # Create feature branch at current commit
git checkout main                    # Switch back to main
git reset --hard origin/main         # Reset main to remote state
```

### Clean Up Local Branches

```bash
# Delete merged branches
git branch --merged main | grep -v "main" | xargs git branch -d

# Delete specific branch
git branch -d feature/my-feature

# Force delete unmerged branch
git branch -D feature/my-feature
```

## Git Commit Hooks

TerraX can use Git hooks for automation (optional):

### Pre-commit Hook

Create `.git/hooks/pre-commit`:

```bash
#!/bin/bash

# Format code
go fmt ./...

# Run tests
if ! go test ./...; then
    echo "Tests failed. Commit aborted."
    exit 1
fi

# Run linter
if ! golangci-lint run; then
    echo "Linting failed. Commit aborted."
    exit 1
fi

exit 0
```

### Commit-msg Hook

Create `.git/hooks/commit-msg`:

```bash
#!/bin/bash

commit_msg=$(cat "$1")

# Check conventional commit format
if ! echo "$commit_msg" | grep -qE '^(feat|fix|docs|style|refactor|test|chore)(\(.+\))?: .{1,72}'; then
    echo "Error: Commit message must follow conventional commits format"
    echo "Format: <type>[optional scope]: <description>"
    echo "Example: feat(stack): add tree validation"
    exit 1
fi

exit 0
```

Make executable:

```bash
chmod +x .git/hooks/pre-commit
chmod +x .git/hooks/commit-msg
```

## Related Documentation

- [Build and Release](build-and-release.md)
- [Testing Strategy](testing-strategy.md)
- [Documentation Requirements](documentation-requirements.md)
- [Go Coding Standards](go-coding-standards.md)

## References

- [Conventional Commits](https://www.conventionalcommits.org/)
- [Git Best Practices](https://git-scm.com/book/en/v2)
- [GitHub Flow](https://guides.github.com/introduction/flow/)
- [Semantic Versioning](https://semver.org/)
