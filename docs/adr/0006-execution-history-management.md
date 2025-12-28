# ADR-0005: Execution History Management with Project-Aware Filtering

**Status**: Accepted

**Date**: 2025-12-28

**Deciders**: TerraX Core Team

**Related**: [ADR-0004: Separation of Concerns](0004-separation-of-concerns.md)

## Context

TerraX users need to track which Terragrunt commands they've executed across different stacks and projects. Key requirements:

1. **Audit trail**: Record what was executed, where, when, and with what result.
2. **Quick re-execution**: Re-run the last command without navigating through the TUI.
3. **Project isolation**: Filter history by current project to avoid cross-project confusion.
4. **Persistence**: History survives application restarts.
5. **Cross-platform**: Works on Linux, macOS, and Windows.
6. **Performance**: History operations shouldn't slow down command execution.

### Problem

Without execution history:
- Users must remember previous commands and manually re-type them.
- No audit trail of what was executed when and where.
- No quick way to repeat the last operation.
- Difficult to debug issues without knowing execution history.

### Requirements

- Persist execution history across sessions.
- Support filtering by project (based on `root.hcl` boundaries).
- Enable quick re-execution of last command (`--last` flag).
- Provide interactive history browser (`--history` flag).
- Store both absolute and relative paths for portability.
- Handle symlinked project directories correctly.
- Automatic pruning to prevent unbounded growth.
- Non-blocking: history failures shouldn't prevent command execution.

## Decision

Implement comprehensive execution history management with:

1. **Storage Format**: JSONL (JSON Lines) - one log entry per line.
2. **Storage Location**: XDG-compliant config directory (`~/.config/terrax/history.log`).
3. **Architecture**: Repository Pattern with Service Layer.
4. **Project Detection**: Use `root.hcl` to determine project boundaries.
5. **Dual Path Storage**: Store both relative (for display) and absolute (for execution) paths.
6. **Access Patterns**: Interactive viewer and quick re-execution flag.

### Architecture

```
┌─────────────────────────────────────────────────────────────┐
│                    Executor Package                          │
│  (Executes commands, logs to history after execution)       │
└───────────────────────┬─────────────────────────────────────┘
                        │
                        │ HistoryLogger interface
                        ↓
┌─────────────────────────────────────────────────────────────┐
│                   History Package                            │
│                                                              │
│  ┌──────────────┐         ┌──────────────┐                  │
│  │   Service    │────────→│  Repository  │                  │
│  │  (Business   │         │  (Storage)   │                  │
│  │   Logic)     │         │              │                  │
│  └──────────────┘         └──────────────┘                  │
│         │                         │                          │
│         │ Filter by project       │ JSONL file I/O          │
│         │ Trim old entries        │ XDG directory           │
│         │ Find last entry         │ Atomic writes           │
│         ↓                         ↓                          │
│  ExecutionLogEntry         history.log                      │
└─────────────────────────────────────────────────────────────┘
                        ↑
                        │ Read history
                        │
        ┌───────────────┴────────────────┐
        │                                │
    ┌───────┐                     ┌──────────┐
    │ --last│                     │ --history│
    │  Flag │                     │   Flag   │
    └───────┘                     └──────────┘
    Quick re-exec                 Interactive viewer
```

### Data Model

```go
type ExecutionLogEntry struct {
    Timestamp     time.Time // When executed
    Command       string    // Terragrunt command (plan, apply, etc.)
    StackPath     string    // Relative path from project root (for display)
    AbsolutePath  string    // Absolute path (for execution)
    ProjectRoot   string    // Absolute path to root.hcl directory
    ExitCode      int       // Command exit code
    Duration      float64   // Execution time in seconds
    User          string    // OS username
}
```

### Storage Format: JSONL

Each line is a complete JSON object:

```json
{"timestamp":"2025-12-28T10:15:30Z","command":"plan","stackPath":"env/dev/vpc","absolutePath":"/home/user/infra/env/dev/vpc","projectRoot":"/home/user/infra","exitCode":0,"duration":12.34,"user":"alice"}
{"timestamp":"2025-12-28T10:20:45Z","command":"apply","stackPath":"env/dev/vpc","absolutePath":"/home/user/infra/env/dev/vpc","projectRoot":"/home/user/infra","exitCode":0,"duration":45.67,"user":"alice"}
```

**Benefits of JSONL**:
- **Append-only**: Fast writes, no need to read entire file to add entry.
- **Streamable**: Can process line-by-line without loading entire file.
- **Human-readable**: Easy to inspect with `tail`, `grep`, etc.
- **Robust**: Corrupted line doesn't break entire file.

### Repository Pattern

```go
// Repository interface for abstraction
type Repository interface {
    Append(entry ExecutionLogEntry) error
    ReadAll() ([]ExecutionLogEntry, error)
    Trim(maxEntries int) error
}

// FileRepository implements Repository
type FileRepository struct {
    filePath string
}
```

**Benefits**:
- **Abstraction**: Can swap storage backends (file → database) without changing business logic.
- **Testability**: Easy to mock for unit tests.
- **Single Responsibility**: Repository only handles I/O, Service handles business logic.

### Service Layer

```go
type Service struct {
    repo Repository
}

// Business logic methods
func (s *Service) FilterByProject(projectRoot string) ([]ExecutionLogEntry, error)
func (s *Service) GetLastForProject(projectRoot string) (*ExecutionLogEntry, error)
func (s *Service) LogExecution(entry ExecutionLogEntry, maxEntries int) error
```

**Responsibilities**:
- Filter entries by project.
- Find last execution for current project.
- Coordinate logging with automatic trimming.
- Handle backward compatibility (entries without StackPath).

### Project Detection

Uses `FindProjectRoot()` to locate `root.hcl`:

```go
func FindProjectRoot(startPath string) (string, error) {
    // Walk up directory tree looking for root.hcl
    // Returns absolute path to directory containing root.hcl
}
```

**Project Boundary Logic**:
1. Start from current working directory (or target stack path).
2. Walk up directory tree looking for `root.hcl`.
3. If found: that directory is the project root.
4. If not found: treat entire filesystem as single "project" (no filtering).

**Benefits**:
- **Automatic detection**: No manual configuration required.
- **Multi-project support**: Users can work on multiple terragrunt projects.
- **Correct filtering**: History only shows entries for current project.

### Dual Path Storage

Store **both** relative and absolute paths:

```go
StackPath    string  // "env/dev/vpc" (relative from project root)
AbsolutePath string  // "/home/user/infra/env/dev/vpc" (absolute)
ProjectRoot  string  // "/home/user/infra" (absolute)
```

**Why Both**:
- **StackPath**: Human-readable display in history viewer.
- **AbsolutePath**: Reliable execution (works even if cwd changes).
- **ProjectRoot**: Enables filtering and relative path calculation.

**Relative Path Calculation**:

```go
relPath, err := filepath.Rel(projectRoot, absolutePath)
if err != nil {
    // Fallback: use absolute path if relative calculation fails
    relPath = absolutePath
}
```

### Symlink Resolution

Resolve symlinks before path comparisons:

```go
resolvedPath, err := filepath.EvalSymlinks(path)
if err != nil {
    // Fallback: use original path
    resolvedPath = path
}
```

**Why Important**:
- Users might access same project via symlink.
- Without resolution, symlinked paths treated as different projects.
- Ensures consistent project detection and filtering.

### Automatic Trimming

Limit history size with automatic pruning:

```go
const DefaultMaxHistoryEntries = 500

func (r *FileRepository) Trim(maxEntries int) error {
    // Read all entries
    // Keep last N entries
    // Atomic write via temp file + rename
}
```

**Strategy**:
1. Read all entries from history file.
2. Keep only last `maxEntries` entries (FIFO).
3. Write to temporary file.
4. Atomically rename temp file to history file.

**Atomic Write**:

```go
tmpFile := historyPath + ".tmp"
// Write to tmpFile
os.Rename(tmpFile, historyPath)  // Atomic on Unix
```

**Windows Handling**:

```go
// Windows: rename fails if target exists
os.Remove(historyPath)       // Delete old file
os.Rename(tmpFile, historyPath)  // Rename temp file
```

### Access Patterns

**Pattern 1: Quick Re-execution (`--last`)**

```bash
$ terrax --last
# Retrieves last execution for current project
# Immediately executes that command
# No TUI, direct execution
```

**Use Case**: "Run the same command I just ran" workflow.

**Implementation**:

```go
if lastFlag {
    projectRoot, _ := history.FindProjectRoot(workingDir)
    lastEntry, err := historyService.GetLastForProject(projectRoot)
    if err != nil {
        return err
    }
    // Execute lastEntry.Command at lastEntry.AbsolutePath
}
```

**Pattern 2: Interactive History Viewer (`--history`)**

```bash
$ terrax --history
# Launches TUI showing history table
# User selects entry, presses Enter
# Executes selected command
```

**Use Case**: "Browse and re-run a previous command" workflow.

**Implementation**:

```go
if historyFlag {
    projectRoot, _ := history.FindProjectRoot(workingDir)
    entries, _ := historyService.FilterByProject(projectRoot)

    // Launch history viewer TUI
    historyModel := tui.NewHistoryModel(entries)
    program := tea.NewProgram(historyModel)
    finalModel, _ := program.Run()

    if finalModel.ShouldReExecuteFromHistory() {
        selected := finalModel.GetSelectedEntry()
        // Execute selected.Command at selected.AbsolutePath
    }
}
```

### XDG Base Directory Compliance

Use XDG specification for config storage:

```go
import "github.com/adrg/xdg"

configDir := filepath.Join(xdg.ConfigHome, "terrax")
historyPath := filepath.Join(configDir, "history.log")
```

**Platform Mappings**:
- **Linux**: `~/.config/terrax/history.log`
- **macOS**: `~/Library/Application Support/terrax/history.log`
- **Windows**: `%APPDATA%/terrax/history.log`

**Benefits**:
- Follows platform conventions.
- No hardcoded paths.
- Integrates with platform backup/sync tools.

### Non-Blocking Execution

History failures never block command execution:

```go
func (e *Executor) Execute(ctx context.Context, cmd Command) error {
    // Execute command
    err := executeCommand(cmd)

    // Log to history (failures logged but don't return error)
    if logErr := e.logExecutionToHistory(cmd, exitCode); logErr != nil {
        fmt.Fprintf(os.Stderr, "Warning: failed to log to history: %v\n", logErr)
    }

    return err  // Return original execution error, not history error
}
```

**Graceful Degradation**:
- History append fails → warning printed, execution succeeds.
- History read fails → `--last` fails gracefully, normal execution works.
- Project detection fails → falls back to no filtering (shows all entries).

## Consequences

### Positive

- **Audit trail**: Complete record of all executions with timestamps, exit codes, durations.
- **Quick re-execution**: `--last` enables rapid iteration workflow.
- **Project awareness**: Filtering prevents confusion across projects.
- **Cross-platform**: Works consistently on Linux, macOS, Windows.
- **Human-readable**: JSONL format inspectable with standard tools.
- **Testable**: Repository abstraction enables easy unit testing.
- **Non-blocking**: History failures don't break execution.
- **Automatic cleanup**: Trimming prevents unbounded growth.
- **Symlink-safe**: Resolves symlinks for consistent project detection.

### Negative

- **Single file lock contention**: Concurrent TerraX instances might conflict (rare in practice).
- **No encryption**: History file stores paths and commands in plaintext (acceptable for local config).
- **Manual cleanup**: Users must manually delete history if needed (mitigated by automatic trimming).
- **Relative path dependency**: Assumes project structure stable (renaming project root breaks relative paths).

### Neutral

- **Separate history file**: Not integrated with shell history (intentional trade-off).
- **XDG dependency**: Adds `github.com/adrg/xdg` package (small, well-maintained).
- **JSONL format**: Queryable with jq but not as rich as SQL (acceptable trade-off).


## Alternatives Considered

### Alternative 1: Sqlite Database

**Pros**:
- Rich querying capabilities (SQL).
- ACID guarantees.
- Efficient filtering and indexing.

**Cons**:
- Additional dependency (database driver).
- Overkill for simple append/read operations.
- Harder to inspect manually (requires SQL client).
- Cross-platform complications (CGo for sqlite).

**Decision**: JSONL is simpler and sufficient for current needs.

### Alternative 2: Shell History Integration

Store history in shell's history file (`.bash_history`, `.zsh_history`).

**Pros**:
- No separate history file.
- Integrates with shell's history search.

**Cons**:
- Shell-specific format (different for bash/zsh/fish).
- Can't store structured data (exit code, duration, project).
- Pollutes shell history with terrax-specific entries.
- No control over retention/filtering.

**Decision**: Dedicated history file with structured data is superior.

### Alternative 3: In-Memory Only (No Persistence)

Keep history only for current session.

**Pros**:
- No file I/O overhead.
- Simpler implementation.

**Cons**:
- History lost on exit.
- No `--last` across sessions.
- No audit trail.

**Decision**: Persistence is core requirement.

### Alternative 4: Per-Project History Files

Store `history.log` in each project directory (`.terrax/history.log`).

**Pros**:
- Project-isolated by default.
- Easy to delete project-specific history.

**Cons**:
- Pollutes project directories.
- Risk of accidental commit to git.
- No cross-project history view.
- Requires project write permissions.

**Decision**: Single user-level history file with project filtering is cleaner.

### Alternative 5: Relative Paths Only

Store only relative paths (no absolute paths).

**Pros**:
- Shorter entries.
- Portable across different checkout locations.

**Cons**:
- Fails if cwd changes.
- Can't execute from different directory.
- Requires storing cwd separately.

**Decision**: Dual paths (relative for display, absolute for execution) is most robust.

## References

- **XDG Base Directory Specification**: https://specifications.freedesktop.org/basedir-spec/basedir-spec-latest.html
- **JSONL Format**: https://jsonlines.org/
- **Repository Pattern**: Martin Fowler's "Patterns of Enterprise Application Architecture"
- **Related ADRs**: [ADR-0004: Separation of Concerns](0004-separation-of-concerns.md)
