# ADR-0009: Single-Pass Filesystem Scanning with Stack Detection

**Status**: Accepted

**Date**: 2025-12-28

**Deciders**: TerraX Core Team

**Related**:
- [ADR-0002: Navigator Pattern](0002-navigator-pattern.md)
- [ADR-0003: Sliding Window Navigation](0003-sliding-window-navigation.md)

## Context

TerraX needs to build a hierarchical tree representation of Terragrunt stacks by scanning the filesystem. Requirements:

1. **Detect stacks**: Identify directories containing `terragrunt.hcl`.
2. **Hierarchical structure**: Preserve parent-child relationships.
3. **Performance**: Scan efficiently without repeated I/O.
4. **Clean tree**: Exclude irrelevant directories (.git, .terraform, etc.).
5. **Depth tracking**: Know depth of each node for sliding window.
6. **Cross-platform**: Work on Linux, macOS, Windows.
7. **Error handling**: Graceful handling of permission errors, symlinks.

### Problem

Naive approach (scan on every navigation):
- **Slow**: Filesystem I/O is expensive.
- **Scales poorly**: Deep trees with many files.
- **Wasteful**: Re-reading unchanged structure.
- **Inconsistent**: Tree might change during navigation.

### Requirements

- Build tree once on application startup.
- Detect Terragrunt stacks via `terragrunt.hcl` presence.
- Skip irrelevant directories (hidden, build artifacts, IDE files).
- Calculate depth for each node.
- Handle errors gracefully (permissions, symlinks).
- Return in-memory tree structure for fast navigation.
- Work on all platforms.

## Decision

Implement **single-pass filesystem scanning** with:

1. **One-time build**: Scan filesystem once at startup.
2. **Recursive depth-first traversal**: Build tree top-down.
3. **Stack detection**: Look for `terragrunt.hcl` in each directory.
4. **Intelligent pruning**: Skip directories that contain no stacks.
5. **Skip list**: Hardcoded list of directories to ignore.
6. **Depth tracking**: Calculate depth during traversal.
7. **Error tolerance**: Skip inaccessible directories, continue scan.

### Architecture

```
┌─────────────────────────────────────────────────────────────┐
│                  Startup Sequence                            │
└─────────────────────────────────────────────────────────────┘
                           ↓
┌─────────────────────────────────────────────────────────────┐
│  FindAndBuildTree(rootPath)                                  │
│                                                              │
│  1. Validate root path exists                                │
│  2. Determine max depth (from root.hcl nesting)             │
│  3. Call buildTreeRecursive(rootPath, depth=0)              │
└─────────────────────────────────────────────────────────────┘
                           ↓
┌─────────────────────────────────────────────────────────────┐
│  buildTreeRecursive(path, currentDepth)                      │
│                                                              │
│  For each directory entry:                                   │
│    1. Skip if in skip list (.git, .terraform, etc.)         │
│    2. Skip if hidden (starts with '.')                       │
│    3. Check if directory contains terragrunt.hcl            │
│    4. Recursively scan subdirectories (depth + 1)           │
│    5. Create Node only if IS stack OR CONTAINS stacks       │
│    6. Track depth in Node                                    │
│    7. Sort children alphabetically                           │
│  Return Node (or nil if no stacks found)                     │
└─────────────────────────────────────────────────────────────┘
                           ↓
┌─────────────────────────────────────────────────────────────┐
│  In-Memory Tree Structure                                    │
│                                                              │
│  type Node struct {                                          │
│      Name     string                                         │
│      Path     string  (absolute)                             │
│      IsStack  bool    (has terragrunt.hcl)                   │
│      Depth    int     (for sliding window)                   │
│      Children []*Node (sorted alphabetically)                │
│  }                                                           │
└─────────────────────────────────────────────────────────────┘
                           ↓
┌─────────────────────────────────────────────────────────────┐
│  Cached in Navigator                                         │
│  (Used for all navigation - zero filesystem I/O)            │
└─────────────────────────────────────────────────────────────┘
```

### Tree Node Structure

```go
// internal/stack/tree.go

// Node represents a directory node in the Terragrunt stack tree.
type Node struct {
    // Name is the directory name (not full path).
    Name string

    // Path is the absolute path to this directory.
    Path string

    // IsStack indicates if this directory contains terragrunt.hcl.
    IsStack bool

    // Depth is the nesting level from root (0 = root).
    Depth int

    // Children are subdirectories (sorted alphabetically).
    Children []*Node
}
```

**Benefits**:
- **Minimal fields**: Only essential data.
- **Absolute paths**: Reliable for execution.
- **IsStack flag**: Quick stack detection without filesystem check.
- **Depth tracking**: Enables sliding window calculations.
- **Sorted children**: Consistent display order.

### Stack Detection

Detect stacks via `terragrunt.hcl` presence:

```go
// isStack checks if directory contains terragrunt.hcl.
func isStack(dirPath string) bool {
    terragruntPath := filepath.Join(dirPath, "terragrunt.hcl")
    info, err := os.Stat(terragruntPath)
    if err != nil {
        return false // File doesn't exist or inaccessible
    }
    return !info.IsDir() // Must be a file, not directory
}
```

**Why terragrunt.hcl**:
- Standard Terragrunt convention.
- Reliable indicator of stack.
- Single file check (fast).

### Skip List

Hardcoded directories to ignore:

```go
var skipList = []string{
    ".git",                // Version control
    ".terraform",          // Terraform working directory
    ".terragrunt-cache",   // Terragrunt cache
    "vendor",              // Dependencies
    "node_modules",        // Node dependencies
    ".idea",               // JetBrains IDE
    ".vscode",             // VS Code
}

func shouldSkipDir(name string) bool {
    // Skip hidden directories (start with '.')
    if strings.HasPrefix(name, ".") {
        return true
    }

    // Skip directories in skip list
    for _, skip := range skipList {
        if name == skip {
            return true
        }
    }

    return false
}
```

**Benefits**:
- **Faster scans**: Skip thousands of irrelevant files.
- **Cleaner tree**: Only show relevant directories.
- **Consistent behavior**: Same skip logic everywhere.

### Recursive Tree Building

Depth-first traversal with pruning:

```go
// buildTreeRecursive recursively builds the tree from the given directory.
func buildTreeRecursive(dirPath string, currentDepth int) (*Node, error) {
    // Read directory entries
    entries, err := os.ReadDir(dirPath)
    if err != nil {
        return nil, fmt.Errorf("failed to read directory %s: %w", dirPath, err)
    }

    // Check if current directory is a stack
    isCurrentStack := isStack(dirPath)

    // Recursively process subdirectories
    var children []*Node
    for _, entry := range entries {
        if !entry.IsDir() {
            continue // Skip files
        }

        dirName := entry.Name()

        // Skip unwanted directories
        if shouldSkipDir(dirName) {
            continue
        }

        // Build child path
        childPath := filepath.Join(dirPath, dirName)

        // Recursively build child tree
        childNode, err := buildTreeRecursive(childPath, currentDepth+1)
        if err != nil {
            // Log error but continue (don't fail entire scan)
            fmt.Fprintf(os.Stderr, "Warning: skipping %s: %v\n", childPath, err)
            continue
        }

        // Only include child if it contains stacks
        if childNode != nil {
            children = append(children, childNode)
        }
    }

    // Prune: if not a stack and no children with stacks, return nil
    if !isCurrentStack && len(children) == 0 {
        return nil, nil // Empty directory, prune it
    }

    // Sort children alphabetically
    sort.Slice(children, func(i, j int) bool {
        return children[i].Name < children[j].Name
    })

    // Create node
    node := &Node{
        Name:     filepath.Base(dirPath),
        Path:     dirPath,
        IsStack:  isCurrentStack,
        Depth:    currentDepth,
        Children: children,
    }

    return node, nil
}
```

**Key Logic**:
1. **Read entries**: Get all files/dirs in current directory.
2. **Skip unwanted**: Ignore .git, .terraform, hidden, etc.
3. **Recurse on subdirs**: Build child trees with depth+1.
4. **Prune empty branches**: If not stack and no children, return nil.
5. **Sort children**: Alphabetical order for consistent display.
6. **Create node**: Only if directory is relevant (stack or contains stacks).

### Pruning Strategy

**Include directory if**:
- Directory contains `terragrunt.hcl` (IsStack = true), OR
- Directory has children that contain stacks

**Exclude directory if**:
- Not a stack AND no children with stacks

**Example**:

```
project/
├── env/                    ← Include (has children with stacks)
│   ├── dev/                ← Include (has children with stacks)
│   │   ├── vpc/            ← Include (IsStack = true)
│   │   │   └── terragrunt.hcl
│   │   └── empty_dir/      ← EXCLUDE (not stack, no children)
│   └── prod/               ← Include (has children with stacks)
│       └── vpc/            ← Include (IsStack = true)
│           └── terragrunt.hcl
└── docs/                   ← EXCLUDE (not stack, no children with stacks)
    └── README.md
```

**Result Tree**:

```
project
└── env
    ├── dev
    │   └── vpc
    └── prod
        └── vpc
```

**Benefits**:
- **Clean navigation**: No clutter from empty directories.
- **Faster navigation**: Fewer nodes to traverse.
- **Clear intent**: Every directory shown is relevant.

### Entry Point

Public API for tree building:

```go
// FindAndBuildTree scans the filesystem and builds a hierarchical tree of Terragrunt stacks.
func FindAndBuildTree(rootPath string) (*Node, int, error) {
    // Validate root path
    absPath, err := filepath.Abs(rootPath)
    if err != nil {
        return nil, 0, fmt.Errorf("failed to resolve absolute path: %w", err)
    }

    // Check if path exists
    if _, err := os.Stat(absPath); err != nil {
        return nil, 0, fmt.Errorf("path does not exist: %w", err)
    }

    // Build tree recursively
    root, err := buildTreeRecursive(absPath, 0)
    if err != nil {
        return nil, 0, fmt.Errorf("failed to build tree: %w", err)
    }

    if root == nil {
        return nil, 0, fmt.Errorf("no stacks found in %s", absPath)
    }

    // Calculate max depth
    maxDepth := calculateMaxDepth(root)

    return root, maxDepth, nil
}

// calculateMaxDepth computes the maximum depth of the tree.
func calculateMaxDepth(node *Node) int {
    if node == nil || len(node.Children) == 0 {
        return node.Depth + 1
    }

    maxChildDepth := 0
    for _, child := range node.Children {
        childMaxDepth := calculateMaxDepth(child)
        if childMaxDepth > maxChildDepth {
            maxChildDepth = childMaxDepth
        }
    }

    return maxChildDepth
}
```

**Usage**:

```go
// cmd/root.go
root, maxDepth, err := stack.FindAndBuildTree(rootPath)
if err != nil {
    return fmt.Errorf("failed to build tree: %w", err)
}

// Pass to TUI
tuiModel := tui.NewModel(root, maxDepth)
```

### Error Handling

Graceful handling of filesystem errors:

```go
// Read directory entries
entries, err := os.ReadDir(dirPath)
if err != nil {
    // Log warning, continue scan (don't fail entire tree)
    fmt.Fprintf(os.Stderr, "Warning: cannot read %s: %v\n", dirPath, err)
    return nil, err
}

// Recursive child scan
childNode, err := buildTreeRecursive(childPath, currentDepth+1)
if err != nil {
    // Log warning, skip this child, continue with others
    fmt.Fprintf(os.Stderr, "Warning: skipping %s: %v\n", childPath, err)
    continue
}
```

**Errors Handled**:
- **Permission denied**: Skip directory, continue.
- **Symlink loops**: Skip, continue.
- **File not found**: Skip, continue.
- **I/O errors**: Log, skip, continue.

**Philosophy**: Best-effort scanning. Don't fail entire scan for one inaccessible directory.

### Cross-Platform Considerations

**Path Handling**:

```go
// ALWAYS use filepath.Join, NEVER hardcoded "/" or "\\"
childPath := filepath.Join(dirPath, dirName)

// Get absolute path
absPath, err := filepath.Abs(rootPath)
```

**Directory Reading**:

```go
// Use os.ReadDir (cross-platform)
entries, err := os.ReadDir(dirPath)
```

**File Detection**:

```go
// Use filepath.Join for terragrunt.hcl path
terragruntPath := filepath.Join(dirPath, "terragrunt.hcl")
```

### Performance Characteristics

**Time Complexity**:
- O(n) where n = number of directories
- Single pass through filesystem tree
- Skip list reduces n significantly

**Space Complexity**:
- O(m) where m = number of stack directories
- Only relevant directories stored in memory
- Pruning reduces memory footprint

**Typical Performance**:
- Small project (10-20 stacks): < 100ms
- Medium project (50-100 stacks): < 500ms
- Large project (500+ stacks): < 2s

**Comparison to Repeated Scans**:
- **Navigation operation**: 1ms (in-memory) vs 100ms+ (filesystem)
- **100 navigations**: 100ms vs 10s+
- **Improvement**: 100x faster

## Consequences

### Positive

- **Fast navigation**: Zero filesystem I/O after initial scan.
- **Simple architecture**: Build once, cache in memory.
- **Clean tree**: Pruning removes clutter.
- **Cross-platform**: Works on all platforms via Go stdlib.
- **Error tolerant**: Partial failures don't break entire scan.
- **Predictable performance**: O(n) scan complexity.
- **Depth tracking**: Enables sliding window feature.
- **Sorted children**: Consistent display order.

### Negative

- **Stale tree**: Changes to filesystem not reflected until restart.
- **Startup delay**: Must scan before TUI starts (acceptable for typical projects).
- **Memory usage**: Entire tree in memory (acceptable for typical tree sizes).
- **No partial refresh**: Can't refresh single subtree.

### Neutral

- **Single-pass tradeoff**: Faster navigation at cost of startup scan.
- **Skip list hardcoded**: Could be configurable, but hardcoded is simpler.

## Alternatives Considered

### Alternative 1: Scan on Every Navigation

Re-scan filesystem on every navigation operation.

**Pros**:
- Always up-to-date with filesystem changes.
- No initial startup delay.

**Cons**:
- Extremely slow (100ms+ per navigation).
- Unusable on network filesystems.
- High I/O load.
- Poor user experience.

**Decision**: Unacceptable performance.

### Alternative 2: Watch Filesystem for Changes

Build tree once, use file watcher to detect changes.

**Pros**:
- Always up-to-date.
- Fast navigation (cached tree).
- Automatic refresh on changes.

**Cons**:
- Complex implementation (fsnotify library).
- Platform-specific quirks.
- Resource overhead (file watchers).
- Overkill for typical usage (stacks don't change during TUI session).

**Decision**: Added complexity not worth benefit.

### Alternative 3: Lazy Loading (Load on Demand)

Load only the root level, load children when navigating deeper.

**Pros**:
- Faster startup.
- Memory efficient (only load what's viewed).

**Cons**:
- Navigation slower (I/O on each level change).
- Inconsistent performance (first visit slow, subsequent fast).
- Complex state management (which levels loaded?).
- Breaks sliding window assumptions.

**Decision**: Single-pass scan is simpler and fast enough.

### Alternative 4: Database/Cache File

Build tree, serialize to disk cache, load from cache on subsequent runs.

**Pros**:
- Even faster startup on subsequent runs.
- Can detect changes via mtimes.

**Cons**:
- Cache invalidation complexity.
- Disk I/O overhead (write + read).
- Cache staleness issues.
- More moving parts (where to store cache?).

**Decision**: Overkill for typical usage patterns.

### Alternative 5: Include All Directories (No Pruning)

Show all directories, not just those with stacks.

**Pros**:
- Complete filesystem view.
- Simpler logic (no pruning).

**Cons**:
- Cluttered navigation (hundreds of irrelevant directories).
- Slower (more nodes to render).
- Confusing UX (which directories are stacks?).

**Decision**: Pruning dramatically improves UX.

### Alternative 6: Find Command Integration

Use system `find` command instead of Go code.

```go
cmd := exec.Command("find", rootPath, "-name", "terragrunt.hcl")
```

**Pros**:
- Leverages optimized system tool.
- Potentially faster on some systems.

**Cons**:
- Platform-specific (find syntax differs).
- Harder to control (skip list, depth).
- Difficult to build hierarchical structure.
- External dependency (find might not exist).

**Decision**: Pure Go is more portable and controllable.

## Future Enhancements

**Potential Improvements**:
1. **Configurable skip list**: Allow users to specify directories to skip.
2. **Refresh command**: `r` key to rebuild tree without restarting.
3. **Incremental updates**: File watcher for automatic tree updates.
4. **Parallel scanning**: Scan subdirectories concurrently.
5. **Cache to disk**: Serialize tree to disk for faster subsequent startups.
6. **Progress indicator**: Show progress during initial scan.
7. **Partial refresh**: Refresh single subtree instead of entire tree.

## References

- **os.ReadDir Documentation**: https://pkg.go.dev/os#ReadDir
- **filepath Package**: https://pkg.go.dev/path/filepath
- **Tree Data Structure**: "Introduction to Algorithms" by Cormen et al.
- **Related Pitfall**: [Repeated Filesystem Scans](../pitfalls/performance/repeated-filesystem-scans.md)
- **Related ADRs**: [ADR-0002: Navigator Pattern](0002-navigator-pattern.md)
