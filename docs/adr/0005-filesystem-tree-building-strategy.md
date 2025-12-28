# ADR-0005: Filesystem Tree Building Strategy

**Status**: Accepted

**Date**: 2025-12-28

**Deciders**: TerraX Core Team

**Related**:
- [ADR-0002: Navigator Pattern](0002-navigator-pattern.md)
- [ADR-0003: Sliding Window Navigation](0003-sliding-window-navigation.md)

## Context

TerraX needs to represent the hierarchical structure of Terragrunt stacks to allow users to navigate them efficiently. A typical infrastructure repository may contain hundreds or thousands of directories, but only a subset are actual Terragrunt stacks (indicated by `terragrunt.hcl`).

Scanning the filesystem specifically during navigation (lazy loading) can introduce latency, especially on slower I/O systems or network drives. Conversely, watching the entire filesystem for changes adds significant complexity.

## Decision

We will implement a **single-pass, recursive filesystem scan** at application startup to build an **in-memory tree representation** of the available stacks.

The strategy involves:
1.  **Startup Scan**: The application performs one complete traversal of the target directory upon launch.
2.  **Stack Detection**: A directory is considered a "stack" if it contains a `terragrunt.hcl` file.
3.  **Pruning**: Directories that do not contain stacks and do not have children containing stacks are excluded from the tree.
4.  **In-Memory caching**: The resulting tree structure is stored entirely in memory.

## Consequences

### Positive
*   **Navigation Performance**: Once the tree is built, navigation operations (moving up/down/into directories) are instantaneous (O(1)) as they occur in memory without disk I/O.
*   **Clean UI**: Pruning ensures the user only sees relevant directories, reducing visual noise.
*   **Simplicity**: Avoids the complexity of managing asynchronous file watchers or handling race conditions during lazy loading.

### Negative
*   **Startup Cost**: Initial startup time is proportional to the size of the filesystem (O(N)). For extremely large repositories, this may introduce a noticeable delay.
*   **Staleness**: The tree structure is static after startup. Filesystem changes made external to TerraX (e.g., creating a new stack in a separate terminal) will not appear until the application is restarted.

### Mitigation
*   Performance testing suggests that for typical repo sizes (thousands of files), the scan takes <500ms, which is acceptable.
*   A manual "refresh" command can be added in the future if staleness becomes a major friction point.

## Alternatives Considered

### Option 1: Scan on Every Navigation

**Description**: Scan directory contents only when the user enters them (lazy loading).

**Pros**:

- Zero startup delay.
- Always perfectly consistent with the current filesystem state.

**Cons**:

- Slower navigation (I/O latency on every step).
- "Sluggish" user experience, especially on network drives or slow disks.

**Why rejected**: The primary goal is a snappy, 60fps TUI experience; blocking I/O on navigation breaks this flow.

### Option 2: Watch Filesystem for Changes

**Description**: Build the tree once at startup and use `fsnotify` to listen for file events and update the live tree model.

**Pros**:

- Tree is always up-to-date.
- Fast navigation (since tree is cached in memory).

**Cons**:

- Significantly higher complexity to handle cross-platform file watching quirks (Windows vs Linux vs macOS).
- Potential for race conditions between UI updates and file events.

**Why rejected**: Stacks rarely change *during* a session, so the added complexity is considered overkill for v1.

### Option 3: Lazy Loading

**Description**: Load only the root level initially, then fetch children nodes only when the user attempts to expand or enter them.

**Pros**:

- Fast startup.
- Lower memory usage (only loaded paths are stored).

**Cons**:

- Inconsistent latency (first visit to a node is slow, subsequent visits fast).
- Complicates the "sliding window" logic, which relies on knowing the full depth structure upfront to render columns correctly.

**Why rejected**: The sliding window navigation paradigm requires pre-calculated depth information.

## Future Enhancements

**Potential Improvements**:
1.  **Refresh Command**: Add a keybinding (e.g., `r`) to trigger a re-scan of the filesystem without restarting.
2.  **Configurable Skip List**: Allow users to specify additional directory names to ignore (e.g., `node_modules`, `.venv`) in their config file.
3.  **Incremental Updates**: If startup time becomes an issue for massive monorepos, implement a background file watcher.

## References

- **os.ReadDir Documentation**: https://pkg.go.dev/os#ReadDir
- **filepath Package**: https://pkg.go.dev/path/filepath
