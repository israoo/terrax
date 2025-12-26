# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added

### Changed

### Deprecated

### Removed

### Fixed

### Security

## [0.3.0] - 2025-12-25

### Added

- Page-based navigation for command and navigation selections to enhance scrolling efficiency.
- Pagination support with improved item rendering constants for better layout management.

### Changed

- CI workflow updated to include Go file changes and improved coverage reporting with SonarCloud integration.

### Deprecated

### Removed

- Unnecessary push event triggers for Go files in CI workflow removed to streamline the process.

### Fixed

### Security

## [0.2.0] - 2025-12-17

### Added

- Execution history tracking with persistent storage.
- Interactive execution history viewer with `--history` flag.
- Re-execution capability for selected history entries in TUI.
- `--last` flag to execute the most recent command from history.
- Execution summary display after running Terragrunt commands.
- Root configuration file support (`.terrax.yaml`).
- Dynamic stackPath width in history table columns.
- Cyclic navigation for MoveUp and MoveDown methods.
- Test coverage for history model and view components.

### Changed

- History entries now display most recent first (reversed order).
- Stack paths in history table are truncated to show most relevant end.
- Exit code formatting improved to avoid lipgloss styles and ensure proper row background.
- Dependency updates: SonarSource/sonarqube-scan-action to v7.0.0.
- Dependency updates: actions/upload-artifact from v5 to v6.

### Fixed

- File handling in TrimHistory function improved to ensure proper closure and safe renaming on Windows.
- Simplified exit logic from history viewer.

## [0.1.1] - 2025-12-07

### Fixed

- Homebrew SHA mismatch issue resolved.

## [0.1.0] - 2025-12-07

### Added

- Initial release of TerraX interactive TUI executor for Terragrunt stacks
- Dynamic hierarchical navigation with automatic tree structure detection
- Sliding window navigation for deep hierarchies (max 3 columns visible by default)
- Smart column display that shows/hides based on navigation context
- Dual execution modes (commands column and navigation column)
- Interactive filtering with `/` key for real-time item filtering
- Customizable configuration via `.terrax.yaml`
- Keyboard-driven interface with intuitive controls
- Cross-platform support (Linux, macOS, Windows)

### Changed

- N/A (initial release)

### Fixed

- N/A (initial release)

[unreleased]: https://github.com/israoo/terrax/compare/v0.1.0...HEAD
[0.3.0]: https://github.com/israoo/terrax/releases/tag/v0.3.0
[0.2.0]: https://github.com/israoo/terrax/releases/tag/v0.2.0
[0.1.1]: https://github.com/israoo/terrax/releases/tag/v0.1.1
[0.1.0]: https://github.com/israoo/terrax/releases/tag/v0.1.0
