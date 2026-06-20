# ADR-0006: Execution History Management

**Status**: Accepted

**Date**: 2025-12-28

**Deciders**: TerraX Core Team

**Related**: [ADR-0004: Separation of Concerns](0004-separation-of-concerns.md)

## Context

Users frequently need to re-run recently executed Terragrunt commands or review the outcome of previous actions. Without a persistent history, this context is lost when the application closes.

We need a mechanism to persist execution logs (command, timestamp, status, duration) that is robust, easy to maintain, and supports filtering by project to avoid confusion when switching between different infrastructure repositories.

## Decision

We will implement an **Execution History** system using **JSON Lines (JSONL)** for storage and the **Repository Pattern** for data access.

The solution includes:
1.  **Storage Format**: `JSONL` (one JSON object per line). This is chosen for its append-only nature, resistance to file corruption (a bad line doesn't break the whole file), and human readability.
2.  **Location**: XDG-compliant configuration directory (e.g., `~/.config/terrax/history.log`).
3.  **Project Isolation**: History entries will include the project root path. Queries will filter based on the current active project root to show only relevant history.
4.  **Dual Path Storage**: We store both absolute paths (for reliable re-execution) and relative paths (for readable UI display).

## Consequences

### Positive
*   **Reliability**: JSONL allows for safe, atomic appends without loading the full history into memory.
*   **User Experience**: Users see only history relevant to their current project.
*   **Reusability**: Storing absolute paths allows commands to be re-run reliably even if the current working directory changes.
*   **Maintainability**: The Repository pattern decouples the storage mechanism from the business logic, allowing future backend swaps (e.g., to SQLite) without changing the UI code.

### Negative
*   **Concurrency**: Simple file appending may have race conditions if multiple TerraX instances write exactly simultaneously, though in practice this is rare for a single-user CLI tool.
*   **No detailed query language**: Unlike SQLite, complex analytical queries require full table scans.

## Alternatives Considered

### Option 1: SQLite Database

**Description**: Use an embedded SQL database (SQLite) to store execution logs.

**Pros**:

- Full SQL query support for complex analytics.
- ACID compliance and reliable concurrent access.

**Cons**:

- Adds a heavy dependency (CGo often required for performance).
- Complicates cross-compilation for different platforms.
- Overkill for a feature that primarily needs "append" and "read last".

**Why rejected**: JSONL is simpler, sufficient, and avoids the build complexity of CGo.

### Option 2: Shell History Integration

**Description**: Store history in the user's standard shell history file (e.g., `.bash_history`, `.zsh_history`).

**Pros**:

- No separate history file to manage.
- Integrates natively with shell search (Ctrl+R).

**Cons**:

- Pollutes the user's shell history with internal metadata (exit codes, execution duration).
- Highly dependent on the specific shell (Bash vs Zsh vs Fish).
- Cannot easily store structured data for project-aware filtering.

**Why rejected**: We need structured metadata (duration, project root, exit code) which plain shell history cannot reliably support.

### Option 3: Per-Project History Files

**Description**: Store a `.terrax_history` file inside each project's root directory.

**Pros**:

- Naturally isolated by project.
- Easy to clear history for a specific project (just delete the file).

**Cons**:

- Clutters user repositories with uncommitted files.
- High risk of users accidentally committing history files to version control.
- Prevents a global "what did I do today across all projects" view.

**Why rejected**: Centralized storage complies better with XDG standards and keeps user repositories clean.

## Future Enhancements

**Potential Improvements**:
1.  **Export/Import**: Allow users to export history to other formats (CSV) for analysis.
2.  **Analytics**: Add a command to show stats like "average plan duration" or "success rate".

## References

- **XDG Base Directory Specification**: https://specifications.freedesktop.org/basedir-spec/basedir-spec-latest.html
- **JSON Lines Format**: https://jsonlines.org/
- **Repository Pattern**: Martin Fowler's "Patterns of Enterprise Application Architecture"
