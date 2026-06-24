# Comment Guidelines

**Status**: Active

**Last Updated**: 2025-12-27

## Overview

This document defines standards for writing, maintaining, and managing comments in the TerraX codebase. Comments are documentation that lives with the code, providing context that isn't obvious from the code itself.

## Core Principles

1. **Comments explain WHY, not WHAT**: Code shows what happens; comments explain why.
2. **Comments are mandatory, not optional**: Well-commented code is a requirement.
3. **Comments should end with periods**: Comments that are complete sentences must end with periods. Brief phrases do not require periods.
4. **Preserve existing comments**: Never delete comments without strong reason.
5. **Update, don't remove**: Outdated comments should be fixed, not deleted.

## Comment Style (MANDATORY)

### Punctuation Rule

```go
// CORRECT: Complete sentence
// This function builds a hierarchical tree structure.

// CORRECT: Brief phrase
// Resolve absolute path
```

**Rationale**: Strikes a balance between professionalism and developer ergonomics. Complete thoughts should look like sentences; quick notes can be more casual.

### Multi-line Comments

```go
// CORRECT: Multiple single-line comments
// This function performs complex tree traversal by first scanning
// the filesystem for terragrunt.hcl files, then building a hierarchical
// structure based on directory relationships.

// ALSO CORRECT: Block comment style
/*
This function performs complex tree traversal by first scanning
the filesystem for terragrunt.hcl files, then building a hierarchical
structure based on directory relationships.
*/
```

**Preference**: Use single-line (`//`) style for most comments. Use block (`/* */`) style for:
- Package documentation.
- Long explanatory blocks (5+ lines).
- Temporarily disabling code during debugging (remove before committing).

## When to Comment

### DO Comment

#### 1. Package Documentation (MANDATORY)

Every package must have package-level documentation:

```go
// Package stack provides tree building and navigation for Terragrunt stacks.
//
// The package implements core business logic for TerraX, including filesystem
// scanning, tree construction, and hierarchical navigation operations.
// It is designed to be UI-agnostic and testable without any framework dependencies.
package stack
```

**Format**:
- First sentence: Brief description starting with "Package <name>".
- Following paragraphs: Detailed explanation of package purpose and design.
- Blank line before `package` declaration.

#### 2. Exported Declarations (MANDATORY)

All exported types, functions, methods, and constants must be documented:

```go
// Navigator provides tree navigation operations for Terragrunt stacks.
// It encapsulates business logic for traversal, selection propagation,
// and breadcrumb generation without UI dependencies.
type Navigator struct {
    root     *Node
    maxDepth int
}

// NewNavigator creates a Navigator for the given tree root and maximum depth.
// The maxDepth parameter determines how many levels deep the navigation can go.
func NewNavigator(root *Node, maxDepth int) *Navigator {
    return &Navigator{
        root:     root,
        maxDepth: maxDepth,
    }
}

// PropagateSelection updates selection indices when parent selection changes.
// This is necessary because children lists change when parent selection moves,
// so child indices must be reset to 0 to avoid out-of-bounds access.
func (n *Navigator) PropagateSelection(state *NavigationState) {
    // Implementation
}
```

**Format**:
- Start with declaration name (type, function, method name).
- First sentence: Brief description of what it is or does.
- Following sentences: Details on behavior, parameters, return values, edge cases.

#### 3. Complex Logic

Explain non-obvious algorithms or complex operations:

```go
func (m Model) handleRightNavigation() Model {
    // Window slides right when focus exceeds visible columns.
    // This maintains the invariant that focusedColumn < maxVisibleNavColumns,
    // ensuring the sliding window always shows the current focus position.
    if m.focusedColumn >= maxVisibleNavColumns {
        m.navigationOffset++
        m.focusedColumn = maxVisibleNavColumns - 1
    }
    return m
}
```

#### 4. Edge Cases and Gotchas

Document subtle behavior or corner cases:

```go
// GetChildrenAtDepth returns children at the specified depth level.
//
// Returns nil if:
// - depth exceeds tree depth
// - no node is selected at parent levels
// - selected index is out of bounds
//
// Note: Depth is 0-indexed, so depth=0 returns root's children.
func (n *Navigator) GetChildrenAtDepth(state *NavigationState, depth int) []*Node {
    // Validate depth is within bounds.
    if depth < 0 || depth >= n.maxDepth {
        return nil
    }

    // Implementation
}
```

#### 5. Non-Obvious Decisions

Explain why something is done a particular way:

```go
// We use filepath.Join instead of string concatenation to ensure
// cross-platform compatibility. Hardcoded "/" fails on Windows.
path := filepath.Join(dir, file)

// Pre-allocate slice capacity to avoid repeated allocations during tree building.
// Benchmarks show 30% performance improvement for large directory trees.
children := make([]*Node, 0, estimatedChildCount)
```

#### 6. TODOs and FIXMEs

Document technical debt and future work:

```go
// TODO(username): Add caching for repeated breadcrumb generation.
// Currently regenerates on every render, which is inefficient for deep trees.
// See issue #123 for discussion.

// FIXME: This doesn't handle symlinks correctly. Need to use filepath.EvalSymlinks.
// Tracked in issue #456.
```

**Format**:
- Use `TODO`, `FIXME`, `HACK`, `NOTE`, or `DEPRECATED` prefixes.
- Include author or issue reference.
- Explain what needs to be done and why.

#### 7. Magic Numbers

Explain the meaning of numeric constants:

```go
// Maximum of 3 navigation columns visible at once to fit standard 80-column terminals.
// Each column is ~30 chars wide: 3 columns + commands column = ~120 chars total.
const maxVisibleNavColumns = 3

// Timeout for filesystem operations to prevent hanging on network drives.
const fsTimeout = 10 * time.Second
```

### DON'T Comment

#### 1. Obvious Code

**Don't state what code clearly expresses. This is the most common violation.**

Comments should explain WHY, not WHAT. If the code is self-explanatory, the comment is redundant noise.

**Common patterns of obvious comments to avoid:**

**Function call comments** - Don't describe obvious function calls:
```go
// WRONG: Comment just repeats function name
// Get working directory
workDir, err := getWorkingDirectory()

// Initialize history service
historyService, err := getHistoryService()

// Build stack tree
stackRoot, maxDepth, err := buildStackTree(workDir)

// Display results
displayResults(model)

// CORRECT: No comment needed - function names are clear
workDir, err := getWorkingDirectory()
historyService, err := getHistoryService()
stackRoot, maxDepth, err := buildStackTree(workDir)
displayResults(model)
```

**Assignment comments** - Don't describe simple assignments:
```go
// WRONG: Comment restates the assignment
// Set default values
viper.SetDefault("commands", config.DefaultCommands)

// Get root config file from configuration
rootConfigFile := viper.GetString("root_config_file")

// Resolve absolute path
absPath, err := filepath.Abs(rootDir)

// CORRECT: No comment needed - code is self-documenting
viper.SetDefault("commands", config.DefaultCommands)
rootConfigFile := viper.GetString("root_config_file")
absPath, err := filepath.Abs(rootDir)
```

**Control flow comments** - Don't describe obvious conditionals or loops:
```go
// WRONG: Comment just describes the if statement
// Check if --history flag is set
historyFlag, _ := cmd.Flags().GetBool("history")
if historyFlag {
    return runHistoryViewer(ctx, historyService)
}

// Skip non-directories and hidden directories.
if !entry.IsDir() || strings.HasPrefix(entry.Name(), ".") {
    continue
}

// CORRECT: No comment needed - condition is clear
historyFlag, _ := cmd.Flags().GetBool("history")
if historyFlag {
    return runHistoryViewer(ctx, historyService)
}

if !entry.IsDir() || strings.HasPrefix(entry.Name(), ".") {
    continue
}
```

**Generic section headers** - Don't use vague comments that add no information:
```go
// WRONG: Too generic to be useful
// Configure professional CLI behavior
rootCmd.SilenceUsage = true

// Set default values
viper.SetDefault("commands", config.DefaultCommands)
viper.SetDefault("max_navigation_columns", config.DefaultMaxNavigationColumns)

// CORRECT: Either no comment, or specific WHY comment if there's a reason
// Suppress usage text on command errors to avoid noisy error output.
rootCmd.SilenceUsage = true

viper.SetDefault("commands", config.DefaultCommands)
viper.SetDefault("max_navigation_columns", config.DefaultMaxNavigationColumns)
```

**When obvious comments ARE acceptable:**

Comments are acceptable even if somewhat obvious when they:
1. Explain a non-obvious WHY (e.g., "// Backward compatibility with v1.x")
2. Document edge cases or gotchas (e.g., "// Returns nil if depth exceeds tree depth")
3. Provide important context (e.g., "// This must run before initialization")
4. Explain complex algorithms or business logic
5. Serve as organizational section headers in long files (sparingly)

**General rule:** If deleting the comment wouldn't make the code harder to understand, delete it.

#### 2. Bad Names Instead of Good Naming

Fix the code instead of explaining bad names:

```go
// WRONG: Comment explaining unclear variable
// d is the directory path.
d := "/some/path"

// CORRECT: Self-documenting variable name
directoryPath := "/some/path"
```

#### 3. Commented-Out Code

Don't commit commented-out code. Use version control instead:

```go
// WRONG: Committed commented-out code
func BuildTree() {
    // oldImplementation()
    newImplementation()
}

// CORRECT: Remove old code, rely on git history
func BuildTree() {
    newImplementation()
}
```

**Exception**: Temporarily during active development in a feature branch (remove before merging).

## Comment Structure

### Function/Method Documentation

**Template**:

```go
// <Name> <brief description of what it does>.
//
// <Detailed description of behavior, parameters, return values.>
// <Edge cases, special conditions, or important notes.>
//
// Example (optional):
//   nav := NewNavigator(root, 5)
//   children := nav.GetChildrenAtDepth(state, 0)
//
// Returns <description of return values and error conditions>.
func FunctionName(params) (returns) {
    // Implementation
}
```

**Example**:

```go
// BuildTree scans the filesystem starting at rootPath and constructs a tree
// of Nodes representing the Terragrunt stack hierarchy.
//
// The function recursively traverses directories, identifying terragrunt.hcl
// files and organizing them by directory structure. It respects .gitignore
// patterns and skips hidden directories by default.
//
// Returns an error if rootPath does not exist, is not accessible, or if
// filesystem scanning encounters permission errors.
func BuildTree(rootPath string) (*Node, error) {
    // Implementation
}
```

### Type Documentation

**Template**:

```go
// <TypeName> <brief description of what it represents>.
//
// <Detailed description of purpose, invariants, usage patterns.>
//
// Fields (if complex):
//   <field>: <description>
//   <field>: <description>
type TypeName struct {
    // Exported field with inline comment explaining purpose.
    Field string

    // unexported field with inline comment if behavior is non-obvious.
    internal int
}
```

**Example**:

```go
// NavigationState tracks the current position within a tree hierarchy.
//
// It maintains selection indices for each depth level and cached breadcrumbs
// for display. The state is passed between Navigator methods to maintain
// context without modifying the Navigator itself.
//
// Invariants:
// - SelectedIndices length <= tree depth
// - Each index is valid for its corresponding level's children
// - BreadcrumbPath always matches current SelectedIndices
type NavigationState struct {
    // SelectedIndices maps depth level to selected child index.
    // Index 0 is root's selected child, index 1 is that child's selected child, etc.
    SelectedIndices []int

    // BreadcrumbPath caches the path from root to current selection.
    // Regenerated when selection changes to avoid repeated traversal.
    BreadcrumbPath []string
}
```

### Inline Comments

Use sparingly for complex code blocks:

```go
func (n *Navigator) PropagateSelection(state *NavigationState, depth int) {
    // Truncate selections beyond changed depth.
    // When selection changes at level N, all selections at levels N+1 and deeper
    // become invalid because the children lists have changed.
    state.SelectedIndices = state.SelectedIndices[:depth+1]

    // Reset deeper selections to first child.
    // This prevents out-of-bounds access when navigating after selection change.
    for i := depth + 1; i < len(state.SelectedIndices); i++ {
        state.SelectedIndices[i] = 0
    }
}
```

## Comment Preservation (MANDATORY)

**Never delete existing comments without very strong reason.**

### Update, Don't Delete

When code changes, update comments to match:

```go
// WRONG: Deleting outdated comment
// Old comment about old behavior (DELETED)
newImplementation()

// CORRECT: Updating comment to match new code
// Updated comment explaining new behavior.
newImplementation()
```

### When Deletion is Acceptable

Only delete comments if:

1. **Comment is actually incorrect and misleading**:
   ```go
   // WRONG COMMENT: This always returns nil (actually returns error)
   // DELETE this comment, then write correct one
   ```

2. **Code being commented is deleted entirely**:
   ```go
   // If you delete a function, delete its documentation too
   ```

3. **Comment is outdated TODO that's been completed**:
   ```go
   // TODO: Add error handling (COMPLETED in issue #123, delete TODO)
   ```

4. **Comment duplicates what code now clearly expresses**:
   ```go
   // BEFORE:
   // x is the result
   x := calculate()  // Unclear, comment needed

   // AFTER:
   calculationResult := calculate()  // Clear, comment redundant
   ```

### Preservation Process

Before deleting a comment:

1. **Read and understand**: What is the comment explaining?
2. **Check history**: Use `git blame` to see when/why it was added.
3. **Assess value**: Does it provide context not obvious from code?
4. **Update if needed**: Can the comment be improved instead of deleted?
5. **Only then delete**: If truly redundant or wrong after above steps.

## Special Comment Types

### TODO Comments

Track technical debt and future work:

```go
// TODO(username): Short description of what needs to be done.
// Longer explanation of why it's needed and how it should be done.
// Reference: Issue #123, PR #456.
```

**Guidelines**:
- Include username or issue reference for accountability.
- Explain what needs doing and why.
- Link to issues or PRs with more context.
- Review TODOs quarterly and address or update them.

### FIXME Comments

Mark known bugs or problematic code:

```go
// FIXME: This panics when input contains null bytes.
// Temporary workaround: sanitize input before calling.
// Permanent fix tracked in issue #789.
```

**Guidelines**:
- Describe the problem clearly.
- Explain current workaround if any.
- Reference tracking issue.
- Fix FIXMEs before they proliferate.

### HACK Comments

Document non-ideal solutions:

```go
// HACK: We're using string replacement instead of proper parsing
// because the upstream library doesn't expose the AST. This is fragile
// and will break if the format changes. See issue #101 for proper solution.
```

**Guidelines**:
- Explain why the hack is necessary.
- Document what proper solution would be.
- Link to issue for proper implementation.

### NOTE Comments

Highlight important information:

```go
// NOTE: This function is called from multiple goroutines.
// All operations must be thread-safe.
```

**Guidelines**:
- Use for important context that affects how code should be modified.
- Highlight concurrency, performance, or security implications.

### DEPRECATED Comments

Mark deprecated code:

```go
// DEPRECATED: Use NewNavigator instead.
// This function will be removed in v2.0.0.
// Migration guide: See docs/migration/v2.md
func OldNavigator() *Navigator {
    // Old implementation
}
```

**Guidelines**:
- State what to use instead.
- Mention when it will be removed.
- Provide migration guidance.

## Documentation Comments (godoc)

### Package Documentation

Place before `package` declaration in main package file:

```go
// Package stack provides tree building and navigation for Terragrunt stacks.
//
// # Overview
//
// This package implements the core business logic for TerraX, including:
//   - Filesystem scanning for terragrunt.hcl files
//   - Tree construction from directory hierarchies
//   - Navigation operations (traversal, selection, breadcrumbs)
//
// # Architecture
//
// The package is designed to be UI-agnostic. All operations work with
// pure data structures (Node, NavigationState) and have no dependencies
// on UI frameworks.
//
// # Usage
//
//   root, err := stack.BuildTree("/path/to/terragrunt")
//   if err != nil {
//       log.Fatal(err)
//   }
//
//   nav := stack.NewNavigator(root, 5)
//   children := nav.GetChildrenAtDepth(&state, 0)
package stack
```

### Exported Identifiers

All exported types, functions, methods, constants, and variables must be documented:

```go
// MaxDepth is the maximum allowed tree depth for navigation.
// Depths beyond this limit are ignored to prevent performance issues
// with extremely deep directory structures.
const MaxDepth = 20

// Node represents a single node in the Terragrunt stack tree.
// Each node corresponds to a directory that may contain a terragrunt.hcl file.
type Node struct {
    // Name is the directory name (not full path).
    Name string

    // Path is the full filesystem path to this directory.
    Path string

    // Children are the immediate subdirectories of this node.
    Children []*Node
}

// IsLeaf returns true if this node has no children.
// Leaf nodes represent the deepest level of the stack hierarchy.
func (n *Node) IsLeaf() bool {
    return len(n.Children) == 0
}
```

### Examples in Documentation

Include examples for complex functions:

```go
// GenerateBreadcrumbs returns the path from root to current selection as a slice.
// Each element is the name of the selected node at that depth level.
//
// Example:
//   state := &NavigationState{
//       SelectedIndices: []int{0, 2, 1},
//   }
//   breadcrumbs := nav.GenerateBreadcrumbs(state)
//   // breadcrumbs = ["env", "dev", "us-west-2"]
//
// Returns an empty slice if no selections exist.
func (n *Navigator) GenerateBreadcrumbs(state *NavigationState) []string {
    // Implementation
}
```

## Comment Quality

### Good Comments

**Explain intent**:
```go
// Use binary search instead of linear scan to improve performance
// for large sorted lists (benchmarked 10x faster for n > 1000).
index := sort.Search(len(items), func(i int) bool {
    return items[i] >= target
})
```

**Document edge cases**:
```go
// Handle empty tree case explicitly to avoid nil pointer dereference.
// This can occur when scanning an empty directory.
if root == nil || len(root.Children) == 0 {
    return nil
}
```

**Provide context**:
```go
// Navigator is immutable after creation to allow safe concurrent reads
// from multiple goroutines (e.g., TUI and background command execution).
type Navigator struct {
    // Immutable fields
}
```

### Bad Comments

**Stating the obvious**:
```go
// WRONG
// Set x to 5.
x := 5

// Add 1 to counter.
counter++
```

**Outdated information**:
```go
// WRONG
// This uses the old v1 API. (Actually using v2 now)
client := api.NewV2Client()
```

**Replacing good naming**:
```go
// WRONG
// t is the tree
t := buildTree()

// CORRECT
tree := buildTree()
```

## Code Review Checklist

When reviewing comments:

- [ ] All exported declarations have documentation comments.
- [ ] Package has package-level documentation.
- [ ] Complete sentences end with periods (brief phrases exempt).
- [ ] Comments explain WHY, not just WHAT.
- [ ] Complex logic has explanatory comments.
- [ ] Edge cases are documented.
- [ ] No obvious or redundant comments.
- [ ] No commented-out code (except temporarily in active development).
- [ ] TODOs have owner and issue references.
- [ ] Comments are accurate and match current code.

## Tools and Automation

### godoc

View generated documentation:

```bash
# Install godoc
go install golang.org/x/tools/cmd/godoc@latest

# Run local documentation server
go doc -http

# View at http://localhost:6060/pkg/github.com/israoo/terrax/
```

## Related Documentation

- [Go Coding Standards](go-coding-standards.md)
- [Documentation Requirements](documentation-requirements.md)
- [Pitfall: Deleting Helpful Comments](../pitfalls/code-quality/deleting-comments.md)
- [ADR-0004: Separation of Concerns](../adr/0004-separation-of-concerns.md)

## References

- [Effective Go: Commentary](https://golang.org/doc/effective_go#commentary)
- [Go Doc Comments](https://go.dev/doc/comment)
- [Go Code Review Comments](https://github.com/golang/go/wiki/CodeReviewComments#comment-sentences)
