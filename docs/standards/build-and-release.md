# Build and Release Standards

**Status**: Active

**Last Updated**: 2025-12-27

## Overview

This document defines build processes, release procedures, and versioning standards for the TerraX project.

## Core Principles

1. **Reproducible builds**: Same input always produces same output.
2. **Automated releases**: Minimize manual steps, reduce errors.
3. **Semantic versioning**: Version numbers convey meaning.
4. **Testing before release**: All releases thoroughly tested.
5. **Changelog maintenance**: Track all notable changes.

## Build System

### Makefile

TerraX uses a Makefile for build automation.

**Location**: `/Makefile`

**Essential targets**:

```makefile
# Build binary to ./build/terrax
build:
	@mkdir -p build
	go build -o build/terrax .

# Run directly without building
run:
	go run .

# Run all tests
test:
	go test ./...

# Run tests with coverage
test-coverage:
	go test -cover ./... -coverprofile=coverage.out
	go tool cover -html=coverage.out -o coverage.html

# Format code
fmt:
	go fmt ./...

# Run linters
lint:
	golangci-lint run

# Clean build artifacts
clean:
	rm -rf build/
	rm -f coverage.out coverage.html

# Install to $GOPATH/bin
install:
	go install .

# Run all checks (format, lint, test)
check: fmt lint test

# Help
help:
	@echo "TerraX Build Commands:"
	@echo "  make build         - Build binary to ./build/terrax"
	@echo "  make run           - Run directly"
	@echo "  make test          - Run tests"
	@echo "  make test-coverage - Run tests with coverage report"
	@echo "  make fmt           - Format code"
	@echo "  make lint          - Run linters"
	@echo "  make clean         - Remove build artifacts"
	@echo "  make install       - Install to GOPATH/bin"
	@echo "  make check         - Run all checks"

.PHONY: build run test test-coverage fmt lint clean install check help
```

### Build Commands

**Development build**:
```bash
make build
# or
go build -o build/terrax .
```

**Run without building**:
```bash
make run
# or
go run . /path/to/terragrunt
```

**Install to system**:
```bash
make install
# or
go install .
```

### Build Flags

**Production build** (with version info):

```bash
# Set version at build time
VERSION=$(git describe --tags --always --dirty)
COMMIT=$(git rev-parse --short HEAD)
BUILD_TIME=$(date -u '+%Y-%m-%d_%H:%M:%S')

go build -ldflags="-X main.Version=${VERSION} -X main.Commit=${COMMIT} -X main.BuildTime=${BUILD_TIME}" -o build/terrax .
```

**Optimized release build**:

```bash
# Strip debug info, reduce binary size
go build -ldflags="-s -w" -o build/terrax .
```

**Cross-compilation**:

```bash
# Linux
GOOS=linux GOARCH=amd64 go build -o build/terrax-linux-amd64 .

# macOS
GOOS=darwin GOARCH=amd64 go build -o build/terrax-darwin-amd64 .
GOOS=darwin GOARCH=arm64 go build -o build/terrax-darwin-arm64 .

# Windows
GOOS=windows GOARCH=amd64 go build -o build/terrax-windows-amd64.exe .
```

### Build Verification

Before releasing, verify build:

```bash
# Build succeeds
make build

# Tests pass
make test

# Code formatted
make fmt
git diff --exit-code  # Should have no changes

# Linting passes
make lint

# Binary works
./build/terrax --version
./build/terrax /path/to/test/stack
```

## Versioning

### Semantic Versioning

TerraX follows [Semantic Versioning 2.0.0](https://semver.org/):

**Format**: `MAJOR.MINOR.PATCH`

**Rules**:
- **MAJOR** (1.x.x): Incompatible API changes, breaking changes
- **MINOR** (x.1.x): New features, backward-compatible
- **PATCH** (x.x.1): Bug fixes, backward-compatible

**Examples**:
- `0.1.0` - Initial development release
- `0.2.0` - Added sliding window navigation
- `0.2.1` - Fixed breadcrumb bug
- `1.0.0` - First stable release
- `1.1.0` - Added command execution feature
- `2.0.0` - Breaking: Changed CLI interface

### Pre-release Versions

**Format**: `MAJOR.MINOR.PATCH-PRERELEASE`

**Pre-release identifiers**:
- `alpha` - Early testing, unstable
- `beta` - Feature complete, testing
- `rc` - Release candidate

**Examples**:
- `1.0.0-alpha.1`
- `1.0.0-beta.1`
- `1.0.0-rc.1`

### Version Numbering

**Starting version**: `0.1.0`

**During development** (pre-1.0.0):
- Breaking changes increment MINOR: `0.1.0` → `0.2.0`
- Features increment MINOR: `0.1.0` → `0.2.0`
- Fixes increment PATCH: `0.1.0` → `0.1.1`

**After 1.0.0**:
- Breaking changes increment MAJOR: `1.0.0` → `2.0.0`
- Features increment MINOR: `1.0.0` → `1.1.0`
- Fixes increment PATCH: `1.0.0` → `1.0.1`

### Version in Code

**main.go**:

```go
package main

var (
    // Version is set during build via -ldflags
    Version = "dev"

    // Commit is the git commit hash
    Commit = "none"

    // BuildTime is when the binary was built
    BuildTime = "unknown"
)

func printVersion() {
    fmt.Printf("terrax version %s (commit: %s, built: %s)\n",
        Version, Commit, BuildTime)
}
```

**Set during build**:

```bash
go build -ldflags="-X main.Version=1.0.0" .
```

## Release Process

### Preparing a Release

**1. Verify state**:

```bash
# Ensure main is up-to-date
git checkout main
git pull origin main

# Ensure all tests pass
make test

# Ensure code is formatted
make fmt

# Ensure linting passes
make lint

# Ensure no uncommitted changes
git status
```

**2. Update CHANGELOG.md**:

```markdown
# Changelog

## [1.0.0] - 2025-01-15

### Added
- Command execution feature
- Breadcrumb navigation improvements

### Changed
- Updated TUI rendering for better performance

### Fixed
- Fixed tree scanning on Windows
- Fixed selection out of bounds error

### Breaking Changes
- Removed deprecated CLI flags

## [0.2.0] - 2025-01-01
...
```

**3. Bump version**:

Update version in:
- `main.go` (if hardcoded)
- `README.md` (installation instructions)
- Any other version references

**4. Commit version bump**:

```bash
git add CHANGELOG.md main.go README.md
git commit -m "chore: bump version to 1.0.0"
git push origin main
```

### Creating a Release

TerraX uses **GoReleaser** to automate the release process. Simply push a version tag and the CD pipeline handles everything.

**1. Create and push tag**:

```bash
# Create annotated tag
git tag -a v1.0.0 -m "Release v1.0.0

Summary of changes in this release.

- Feature 1
- Feature 2
- Bug fix 3
"

# Push tag - this triggers the CD workflow
git push origin v1.0.0
```

**2. Automated release process**:

When you push a version tag, the `.github/workflows/cd.yml` workflow automatically:

1. **Builds binaries** for all platforms (via GoReleaser):
   - Linux (amd64, arm64, 386)
   - macOS (amd64, arm64)
   - Windows (amd64, arm64, 386)

2. **Creates archives**:
   - `.tar.gz` for Linux and macOS
   - `.zip` for Windows

3. **Generates checksums**: SHA256 checksums for all artifacts

4. **Creates GitHub Release**: With auto-generated release notes

5. **Uploads artifacts**: All binaries, archives, and checksums

**GoReleaser Configuration**: `.goreleaser.yaml`

The release configuration is defined in `.goreleaser.yaml` at the repository root. This file specifies:

- Build targets (platforms and architectures)
- Archive formats
- Binary names
- Changelog generation
- Release note templates

**3. Verify release**:

After the CD workflow completes (~5-10 minutes), verify the release:

1. Visit GitHub Releases: `https://github.com/israoo/terrax/releases/tag/v1.0.0`
2. Check all artifacts are present
3. Download and test a binary:

```bash
# Example: Test macOS binary
curl -LO https://github.com/israoo/terrax/releases/download/v1.0.0/terrax_1.0.0_darwin_amd64.tar.gz
tar -xzf terrax_1.0.0_darwin_amd64.tar.gz
./terrax --version
./terrax /path/to/test/stack
```

**Manual release (fallback)**:

If you need to create a release manually or locally:

```bash
# Install GoReleaser
go install github.com/goreleaser/goreleaser@latest

# Create release snapshot (local testing)
goreleaser release --snapshot --clean

# Create actual release (use CI instead)
# goreleaser release --clean
```

**Note**: Prefer automated releases via CD workflow. Manual releases should only be used for testing or emergency situations.

### Post-Release Tasks

**1. Update documentation**:
- Update README.md with latest version
- Update installation instructions
- Update screenshots if UI changed

**2. Announce release**:
- GitHub Discussions
- Project website
- Social media (if applicable)

**3. Monitor for issues**:
- Watch for bug reports
- Check CI/CD status
- Monitor download metrics

## Hotfix Release Process

For critical production issues requiring immediate release:

**1. Create hotfix branch**:

```bash
git checkout main
git pull origin main
git checkout -b hotfix/1.0.1-critical-fix
```

**2. Make minimal fix**:

```bash
# Fix issue
git add <files>
git commit -m "fix: critical production issue

Detailed description.

Fixes #urgent-issue"
```

**3. Fast-track PR**:

```bash
git push -u origin hotfix/1.0.1-critical-fix
# Create PR with "urgent" label
# Request immediate review
```

**4. After merge, immediate release**:

```bash
git checkout main
git pull origin main

# Update CHANGELOG
# Bump PATCH version
git add CHANGELOG.md
git commit -m "chore: bump version to 1.0.1"
git push origin main

# Tag and release
git tag -a v1.0.1 -m "Hotfix v1.0.1: critical production issue"
git push origin v1.0.1

# Build and publish release
./scripts/build-release.sh 1.0.1
gh release create v1.0.1 --title "TerraX v1.0.1 (Hotfix)" ...
```

## Continuous Integration

### GitHub Actions

**Location**: `.github/workflows/`

**TerraX uses two main workflows**:

1. **`.github/workflows/ci.yml`** - Continuous Integration (runs on push/PR)
2. **`.github/workflows/cd.yml`** - Continuous Deployment (runs on tag push)

#### CI Workflow (ci.yml)

The CI workflow runs comprehensive checks on every push and pull request:

**Jobs** (in order):

1. **`lint`** - Code linting with golangci-lint
2. **`codeql`** - CodeQL security analysis
3. **`gitguardian`** - Secret scanning
4. **`trivy`** - Dependency vulnerability scanning
5. **`sonarcloud`** - Code quality analysis + test coverage
6. **`codecov`** - Upload coverage reports
7. **`build_test`** - Multi-platform build verification (Linux, macOS, Windows on amd64 + arm64)

**Key features**:

- Runs only when Go code changes (path filters)
- Security scanning (CodeQL, GitGuardian, Trivy)
- Code quality analysis (SonarCloud)
- Test coverage reporting (Codecov)
- Multi-platform build matrix
- Artifact retention for reports

**Actual workflow**: `.github/workflows/ci.yml`

#### CD Workflow (cd.yml)

The CD workflow automates releases using GoReleaser when version tags are pushed:

**Trigger**: Push of version tag (e.g., `v1.0.0`)

**Process**:

1. Checkout code with full history
2. Setup Go environment
3. Run GoReleaser to build multi-platform binaries and create GitHub release

**Actual workflow**: `.github/workflows/cd.yml`

**GoReleaser configuration**: `.goreleaser.yaml` (handles multi-platform builds, archives, checksums, and GitHub release creation)

## Dependency Management

### Go Modules

**go.mod**: Manages dependencies.

**Essential commands**:

```bash
# Add dependency
go get github.com/some/package@latest

# Update dependency
go get -u github.com/some/package

# Update all dependencies
go get -u ./...

# Tidy modules (remove unused)
go mod tidy

# Verify dependencies
go mod verify

# Vendor dependencies (optional)
go mod vendor
```

### Dependency Updates

**Regular updates**:
- Review dependencies monthly
- Update to patch versions weekly
- Update to minor versions after testing
- Update to major versions cautiously

**Security updates**:
- Apply immediately
- Test thoroughly
- Release hotfix if needed

**Checking for updates**:

```bash
# List outdated dependencies
go list -u -m all

# Or use tool
go install github.com/psampaz/go-mod-outdated@latest
go list -u -m -json all | go-mod-outdated -update -direct
```

## Build Environment

### Local Development

**Requirements**:
- Go 1.25+
- Make (optional but recommended)
- Git

**Setup**:

```bash
# Clone repository
git clone https://github.com/israoo/terrax.git
cd terrax

# Install dependencies
go mod download

# Build
make build

# Run tests
make test
```

### CI Environment

**Requirements**:
- Go 1.25+
- golangci-lint
- GitHub CLI (for releases)

**Configuration**: See `.github/workflows/`

## Release Checklist

### Pre-Release

- [ ] All tests pass on all platforms
- [ ] Code formatted and linted
- [ ] Documentation updated
- [ ] CHANGELOG.md updated
- [ ] Version bumped in all necessary files
- [ ] No security vulnerabilities
- [ ] Dependencies up-to-date
- [ ] Breaking changes documented
- [ ] Migration guide written (if breaking changes)

### Release

- [ ] Version commit pushed to main
- [ ] Tag created and pushed
- [ ] Binaries built for all platforms
- [ ] GitHub release created
- [ ] Release notes published
- [ ] Binaries uploaded to release
- [ ] Installation instructions verified

### Post-Release

- [ ] Release announcement published
- [ ] Documentation site updated
- [ ] Installation tested on all platforms
- [ ] Monitor for issues
- [ ] Close related issues/milestones

## Rollback Procedure

If critical issue discovered after release:

**1. Assess severity**:
- Critical: Immediate hotfix
- High: Hotfix within 24-48 hours
- Medium: Include in next regular release

**2. For critical issues**:

```bash
# Option 1: Hotfix (preferred)
# Follow hotfix release process above

# Option 2: Delete release (last resort)
gh release delete v1.0.0
git tag -d v1.0.0
git push origin :refs/tags/v1.0.0

# Communicate to users via GitHub Discussions/Issues
```

## Related Documentation

- [Git Workflow](git-workflow.md)
- [Testing Strategy](testing-strategy.md)
- [Documentation Requirements](documentation-requirements.md)

## References

- [Semantic Versioning](https://semver.org/)
- [Keep a Changelog](https://keepachangelog.com/)
- [Go Release Process](https://go.dev/doc/devel/release)
- [GitHub Releases](https://docs.github.com/en/repositories/releasing-projects-on-github)
