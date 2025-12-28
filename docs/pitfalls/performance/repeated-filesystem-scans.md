# Pitfall: Repeated Filesystem Scans

**Category**: Performance

**Severity**: High

**Date Identified**: 2025-12-27

## Description

Repeatedly scanning the filesystem to rebuild the tree structure on every navigation operation or UI update, instead of caching the tree and reusing it. This causes severe performance degradation, especially with large directory structures, network filesystems, or slow storage devices.

## Impact

Repeated filesystem scans create severe performance problems:

- **Slow response times**: Every navigation operation becomes sluggish.
- **High I/O load**: Excessive disk/network reads strain the system.
- **Poor user experience**: UI feels unresponsive and laggy.
- **Battery drain**: Mobile/laptop users suffer from excessive I/O.
- **Network overhead**: Network filesystems (NFS, SMB) become unusable.
- **Resource contention**: Competing with other processes for I/O.
- **Scalability issues**: Performance degrades exponentially with tree depth.

## Root Cause

Common reasons for repeated filesystem scans:

1. **Lack of caching**: Not storing the built tree structure.
2. **Premature optimization**: "We'll cache it when it's slow" (too late).
3. **Unclear state management**: Not understanding when to rebuild vs reuse.
4. **Convenience over performance**: Easier to rebuild than manage state.
5. **Missing profiling**: Didn't measure to identify the bottleneck.
6. **Over-eager refresh**: Rebuilding on every UI update or key press.
7. **Poor architecture**: Business logic mixed with UI, triggering rebuilds.

## How to Avoid

### Do

- **Cache tree on startup**: Build once, reuse throughout session.
- **Pass tree as state**: Don't rebuild, pass existing tree structure.
- **Profile before optimizing**: Measure to identify real bottlenecks.
- **Explicit refresh**: Only rebuild when user explicitly requests it.
- **Separate concerns**: Tree building separate from navigation/rendering.
- **Use benchmarks**: Measure performance with realistic data.
- **Document caching strategy**: Make it clear when rebuilds happen.

### Don't

- **Don't rebuild on every navigation**: Navigation uses existing tree.
- **Don't rebuild on UI updates**: Rendering doesn't trigger filesystem I/O.
- **Don't scan in tight loops**: Never call BuildTree() repeatedly.
- **Don't ignore performance**: Profile early, not when users complain.
- **Don't rebuild unless necessary**: Only on explicit refresh or startup.
- **Don't mix I/O with UI**: Filesystem operations separate from rendering.

## Detection

Warning signs of repeated filesystem scans:

- **Sluggish navigation**: Each arrow key press takes noticeable time.
- **High I/O wait**: `top` or `htop` shows high %wa (I/O wait).
- **Disk activity**: Constant disk LED activity during navigation.
- **Network traffic**: NFS/SMB traffic on every navigation operation.
- **CPU profiling**: BuildTree() appears frequently in profiles.
- **Slow on deep trees**: Performance degrades with directory depth.
- **Battery drain**: Laptop battery depletes quickly during use.

### Code Smells

```go
// ❌ WRONG: Rebuilding tree on every Update
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
    switch msg := msg.(type) {
    case tea.KeyMsg:
        // ❌ Filesystem scan on every key press!
        m.root, _ = stack.BuildTree(m.rootPath)
        m.navigator = stack.NewNavigator(m.root, m.maxDepth)
        return m.handleKeyPress(msg)
    }
    return m, nil
}

// ❌ WRONG: Rebuilding in View (called on every render)
func (m Model) View() string {
    // ❌ This is called on EVERY frame!
    root, _ := stack.BuildTree(m.rootPath)
    // ... render tree
}

// ❌ WRONG: Rebuilding in Navigator methods
func (n *Navigator) GetChildrenAtDepth(state *NavigationState, depth int) []*Node {
    // ❌ Rebuilding tree on every navigation call
    root, _ := stack.BuildTree(n.rootPath)
    // ... traverse tree
}
```

## Remediation

If you have repeated filesystem scans, here's how to fix it:

### 1. Profile to Confirm

```bash
# Profile CPU usage
go test -cpuprofile=cpu.prof -bench=. ./internal/stack

# Analyze profile
go tool pprof cpu.prof
# In pprof:
(pprof) top10
(pprof) list BuildTree

# Profile during runtime
go build -o terrax .
# Run with profiling
CPUPROFILE=terrax.prof ./terrax /path/to/stack
go tool pprof terrax.prof
```

Look for `BuildTree` or filesystem operations (`os.ReadDir`, `filepath.Walk`) appearing repeatedly.

### 2. Build Once on Startup

```go
// cmd/root.go - Build tree once at initialization
func Execute() error {
    rootCmd.RunE = func(cmd *cobra.Command, args []string) error {
        rootPath := args[0]

        // Build tree ONCE on startup
        root, err := stack.BuildTree(rootPath)
        if err != nil {
            return fmt.Errorf("failed to build tree: %w", err)
        }

        // Pass cached tree to TUI
        tuiModel := tui.NewModel(root, maxDepth)

        p := tea.NewProgram(tuiModel, tea.WithAltScreen())
        if _, err := p.Run(); err != nil {
            return fmt.Errorf("TUI error: %w", err)
        }

        return nil
    }

    return rootCmd.Execute()
}
```

### 3. Store Tree in Model State

```go
// internal/tui/model.go
type Model struct {
    // Cache tree structure - built once, reused throughout
    navigator *stack.Navigator

    // UI state
    state           *stack.NavigationState
    focusedColumn   int
    selectedCommand int
    // ... other state
}

// NewModel accepts pre-built tree
func NewModel(root *stack.Node, maxDepth int) Model {
    return Model{
        // Store cached tree in Navigator
        navigator: stack.NewNavigator(root, maxDepth),
        state: &stack.NavigationState{
            SelectedIndices: []int{0},
        },
        // ... initialize other state
    }
}
```

### 4. Navigate with Cached Tree

```go
// internal/tui/model.go - Navigation uses cached tree
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
    switch msg := msg.(type) {
    case tea.KeyMsg:
        switch msg.String() {
        case "down", "up":
            // ✅ Navigate using cached tree (no I/O)
            m.state = m.handleVerticalNavigation(msg.String())
            return m, nil

        case "left", "right":
            // ✅ Navigate using cached tree (no I/O)
            m.state = m.handleHorizontalNavigation(msg.String())
            return m, nil

        case "ctrl+r":
            // ✅ Explicit refresh - only rebuild when user requests
            return m.refreshTree()
        }
    }
    return m, nil
}

// Explicit refresh function
func (m Model) refreshTree() (Model, tea.Cmd) {
    // Only rebuild when explicitly requested
    root, err := stack.BuildTree(m.rootPath)
    if err != nil {
        // Handle error appropriately
        return m, nil
    }

    // Update cached tree
    m.navigator = stack.NewNavigator(root, m.maxDepth)

    return m, nil
}
```

### 5. Render with Cached Tree

```go
// internal/tui/view.go - Rendering uses cached tree (no I/O)
func (m Model) View() string {
    if !m.ready {
        return "Initializing..."
    }

    calc := LayoutCalculator{}
    renderer := Renderer{}

    // ✅ Use cached tree from Navigator (no filesystem access)
    startDepth, endDepth := calc.CalculateVisibleColumns(m.maxDepth, m.navigationOffset)

    var columns []string
    for depth := startDepth; depth < endDepth; depth++ {
        // ✅ Navigator uses cached tree, no I/O
        children := m.navigator.GetChildrenAtDepth(m.state, depth)
        if len(children) == 0 {
            break
        }

        column := renderer.RenderColumn(children, m.getSelectedIndex(depth), depth == m.focusedColumn)
        columns = append(columns, column)
    }

    return lipgloss.JoinHorizontal(lipgloss.Top, columns...)
}
```

### 6. Navigator Uses Cached Tree

```go
// internal/stack/navigator.go
type Navigator struct {
    // Cached tree - built once, reused for all operations
    root     *Node
    maxDepth int
}

func NewNavigator(root *Node, maxDepth int) *Navigator {
    return &Navigator{
        root:     root,  // Store cached tree
        maxDepth: maxDepth,
    }
}

// ✅ All navigation methods use cached tree
func (n *Navigator) GetChildrenAtDepth(state *NavigationState, depth int) []*Node {
    if n.root == nil || depth < 0 || depth >= n.maxDepth {
        return nil
    }

    // ✅ Traverse cached tree (no I/O)
    current := n.root
    for i := 0; i < depth && i < len(state.SelectedIndices); i++ {
        selectedIdx := state.SelectedIndices[i]
        if selectedIdx >= len(current.Children) {
            return nil
        }
        current = current.Children[selectedIdx]
    }

    return current.Children
}

// ✅ Other methods also use cached tree
func (n *Navigator) GenerateBreadcrumbs(state *NavigationState) []string {
    // Traverse cached tree, no filesystem access
    // ...
}

func (n *Navigator) GetSelectedPath(state *NavigationState) string {
    // Traverse cached tree, no filesystem access
    // ...
}
```

## Performance Comparison

### Before (Repeated Scans)

```go
// ❌ BAD: Rebuilding on every navigation
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
    switch msg := msg.(type) {
    case tea.KeyMsg:
        // Rebuild tree on EVERY key press
        m.root, _ = stack.BuildTree(m.rootPath)
        // ...
    }
    return m, nil
}
```

**Performance characteristics**:
- **Navigation latency**: 50-500ms per key press (depends on tree size)
- **I/O operations**: Hundreds to thousands per navigation
- **CPU usage**: High (filesystem traversal)
- **Scalability**: O(n) where n = number of files (on every operation)

### After (Cached Tree)

```go
// ✅ GOOD: Build once, reuse cached tree
func Execute() error {
    // Build ONCE
    root, err := stack.BuildTree(rootPath)
    if err != nil {
        return err
    }

    // Pass cached tree
    tuiModel := tui.NewModel(root, maxDepth)
    // ... run TUI
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
    switch msg := msg.(type) {
    case tea.KeyMsg:
        // Navigate using cached tree (in-memory)
        m.state = m.handleNavigation(msg)
        // ...
    }
    return m, nil
}
```

**Performance characteristics**:
- **Navigation latency**: < 1ms per key press (in-memory traversal)
- **I/O operations**: Zero (after initial build)
- **CPU usage**: Minimal (pointer traversal)
- **Scalability**: O(1) for navigation (O(depth) for traversal)

**Improvement**: **50-500x faster** for navigation operations.

## Benchmarking

### Create Benchmark Tests

```go
// internal/stack/navigator_bench_test.go
package stack

import (
    "testing"
)

// Benchmark tree building (should be called ONCE)
func BenchmarkBuildTree(b *testing.B) {
    rootPath := "/path/to/test/directory"

    b.ResetTimer()
    for i := 0; i < b.N; i++ {
        _, err := BuildTree(rootPath)
        if err != nil {
            b.Fatal(err)
        }
    }
}

// Benchmark navigation with cached tree (called FREQUENTLY)
func BenchmarkNavigator_GetChildrenAtDepth(b *testing.B) {
    // Build tree ONCE (outside benchmark loop)
    root := createLargeTestTree(1000) // 1000 nodes
    nav := NewNavigator(root, 10)
    state := &NavigationState{SelectedIndices: []int{0, 0, 0}}

    b.ResetTimer()
    for i := 0; i < b.N; i++ {
        // This should be FAST (in-memory)
        _ = nav.GetChildrenAtDepth(state, 2)
    }
}

// Compare: Rebuilding tree every time (BAD)
func BenchmarkRepeatedBuildTree(b *testing.B) {
    rootPath := "/path/to/test/directory"

    b.ResetTimer()
    for i := 0; i < b.N; i++ {
        // ❌ Simulating repeated scans on every operation
        root, err := BuildTree(rootPath)
        if err != nil {
            b.Fatal(err)
        }
        nav := NewNavigator(root, 10)
        state := &NavigationState{SelectedIndices: []int{0, 0, 0}}
        _ = nav.GetChildrenAtDepth(state, 2)
    }
}
```

### Run Benchmarks

```bash
# Run benchmarks
go test -bench=. ./internal/stack

# Expected results (example):
# BenchmarkBuildTree-8                     100      12000000 ns/op
# BenchmarkNavigator_GetChildrenAtDepth-8  1000000    1200 ns/op
# BenchmarkRepeatedBuildTree-8             100      12000000 ns/op

# 10,000x difference between cached (1,200 ns) vs repeated (12,000,000 ns)
```

## Related

- [ADR-0002: Navigator Pattern](../../adr/0002-navigator-pattern.md)
- [ADR-0004: Separation of Concerns](../../adr/0004-separation-of-concerns.md)
- [Standard: Go Coding Standards](../../standards/go-coding-standards.md)
- [Pitfall: Mixing Business Logic with UI](../architecture/mixing-business-logic-ui.md)

## Examples

### Bad: Rebuilding on Every Update

```go
// internal/tui/model.go
package tui

import (
    tea "github.com/charmbracelet/bubbletea"
    "github.com/israoo/terrax/internal/stack"
)

type Model struct {
    rootPath  string
    maxDepth  int
    // ❌ NOT storing tree, rebuilding every time
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
    switch msg := msg.(type) {
    case tea.KeyMsg:
        switch msg.String() {
        case "down":
            // ❌ Filesystem scan on every down arrow press!
            root, _ := stack.BuildTree(m.rootPath)
            nav := stack.NewNavigator(root, m.maxDepth)
            // ... navigate down

        case "up":
            // ❌ Another scan on every up arrow press!
            root, _ := stack.BuildTree(m.rootPath)
            nav := stack.NewNavigator(root, m.maxDepth)
            // ... navigate up

        case "right":
            // ❌ Yet another scan!
            root, _ := stack.BuildTree(m.rootPath)
            nav := stack.NewNavigator(root, m.maxDepth)
            // ... navigate right
        }
    }
    return m, nil
}

func (m Model) View() string {
    // ❌ CATASTROPHIC: Rebuilding tree on EVERY render!
    root, _ := stack.BuildTree(m.rootPath)
    nav := stack.NewNavigator(root, m.maxDepth)

    // ... render tree
    return "view"
}
```

**Problems**:
- Filesystem scan on every key press
- Filesystem scan on every render (potentially 60 times per second!)
- Exponentially slow with large directory structures
- Unusable on network filesystems
- High battery drain
- Poor user experience

### Good: Build Once, Cache Forever

```go
// cmd/root.go
package cmd

import (
    "fmt"
    "os"

    tea "github.com/charmbracelet/bubbletea"
    "github.com/spf13/cobra"

    "github.com/israoo/terrax/internal/stack"
    "github.com/israoo/terrax/internal/tui"
)

func Execute() error {
    rootCmd.RunE = func(cmd *cobra.Command, args []string) error {
        if len(args) == 0 {
            return fmt.Errorf("root path required")
        }

        rootPath := args[0]

        // ✅ Build tree ONCE at startup
        root, err := stack.BuildTree(rootPath)
        if err != nil {
            return fmt.Errorf("failed to build tree: %w", err)
        }

        // ✅ Pass cached tree to TUI
        tuiModel := tui.NewModel(root, maxDepth)

        p := tea.NewProgram(tuiModel, tea.WithAltScreen())
        if _, err := p.Run(); err != nil {
            return fmt.Errorf("TUI error: %w", err)
        }

        return nil
    }

    return rootCmd.Execute()
}
```

```go
// internal/tui/model.go
package tui

import (
    tea "github.com/charmbracelet/bubbletea"
    "github.com/israoo/terrax/internal/stack"
)

type Model struct {
    // ✅ Store cached tree in Navigator
    navigator *stack.Navigator

    // ✅ Store navigation state separately
    state           *stack.NavigationState
    focusedColumn   int
    selectedCommand int
    // ... other UI state
}

// ✅ Constructor accepts pre-built tree
func NewModel(root *stack.Node, maxDepth int) Model {
    return Model{
        navigator: stack.NewNavigator(root, maxDepth),
        state: &stack.NavigationState{
            SelectedIndices: []int{0},
        },
        focusedColumn:   1,
        selectedCommand: 0,
    }
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
    switch msg := msg.(type) {
    case tea.KeyMsg:
        switch msg.String() {
        case "down":
            // ✅ Navigate using cached tree (no I/O)
            m.handleDownNavigation()

        case "up":
            // ✅ Navigate using cached tree (no I/O)
            m.handleUpNavigation()

        case "right":
            // ✅ Navigate using cached tree (no I/O)
            m.handleRightNavigation()

        case "ctrl+r":
            // ✅ Explicit refresh - only when user requests
            return m.refreshTree()
        }
    }
    return m, nil
}

func (m Model) View() string {
    // ✅ Render using cached tree (no I/O)
    calc := LayoutCalculator{}
    renderer := Renderer{}

    startDepth, endDepth := calc.CalculateVisibleColumns(m.navigator.MaxDepth(), m.navigationOffset)

    var columns []string
    for depth := startDepth; depth < endDepth; depth++ {
        // ✅ GetChildrenAtDepth uses cached tree
        children := m.navigator.GetChildrenAtDepth(m.state, depth)
        if len(children) == 0 {
            break
        }

        column := renderer.RenderColumn(children, m.getSelectedIndex(depth), depth == m.focusedColumn)
        columns = append(columns, column)
    }

    return lipgloss.JoinHorizontal(lipgloss.Top, columns...)
}

// ✅ Explicit refresh function
func (m Model) refreshTree() (Model, tea.Cmd) {
    // Only rebuild when user explicitly requests (Ctrl+R)
    root, err := stack.BuildTree(m.rootPath)
    if err != nil {
        // Handle error (could show error message in UI)
        return m, nil
    }

    // Update cached tree
    m.navigator = stack.NewNavigator(root, m.navigator.MaxDepth())

    // Reset state to root
    m.state = &stack.NavigationState{
        SelectedIndices: []int{0},
    }

    return m, nil
}
```

**Benefits**:
- Instant navigation (< 1ms)
- Zero I/O during navigation
- Scalable to large directory structures
- Works well on network filesystems
- Low battery usage
- Excellent user experience

### Bad: Rebuilding in Navigator

```go
// internal/stack/navigator.go
package stack

type Navigator struct {
    rootPath string  // ❌ Storing path instead of tree
    maxDepth int
}

func (n *Navigator) GetChildrenAtDepth(state *NavigationState, depth int) []*Node {
    // ❌ Rebuilding tree on every method call!
    root, err := BuildTree(n.rootPath)
    if err != nil {
        return nil
    }

    // Traverse freshly built tree
    current := root
    for i := 0; i < depth; i++ {
        selectedIdx := state.SelectedIndices[i]
        current = current.Children[selectedIdx]
    }

    return current.Children
}

func (n *Navigator) GenerateBreadcrumbs(state *NavigationState) []string {
    // ❌ Another rebuild!
    root, _ := BuildTree(n.rootPath)
    // ... generate breadcrumbs
}
```

### Good: Navigator Uses Cached Tree

```go
// internal/stack/navigator.go
package stack

type Navigator struct {
    // ✅ Store cached tree, not path
    root     *Node
    maxDepth int
}

func NewNavigator(root *Node, maxDepth int) *Navigator {
    return &Navigator{
        root:     root,  // ✅ Cached tree
        maxDepth: maxDepth,
    }
}

func (n *Navigator) GetChildrenAtDepth(state *NavigationState, depth int) []*Node {
    if n.root == nil || depth < 0 || depth >= n.maxDepth {
        return nil
    }

    // ✅ Traverse cached tree (no I/O)
    current := n.root
    for i := 0; i < depth && i < len(state.SelectedIndices); i++ {
        selectedIdx := state.SelectedIndices[i]
        if selectedIdx >= len(current.Children) {
            return nil
        }
        current = current.Children[selectedIdx]
    }

    return current.Children
}

func (n *Navigator) GenerateBreadcrumbs(state *NavigationState) []string {
    if n.root == nil {
        return []string{}
    }

    breadcrumbs := []string{n.root.Name}

    // ✅ Traverse cached tree (no I/O)
    current := n.root
    for i := 0; i < len(state.SelectedIndices) && current != nil; i++ {
        selectedIdx := state.SelectedIndices[i]
        if selectedIdx >= len(current.Children) {
            break
        }
        current = current.Children[selectedIdx]
        breadcrumbs = append(breadcrumbs, current.Name)
    }

    return breadcrumbs
}

func (n *Navigator) GetSelectedPath(state *NavigationState) string {
    // ✅ Traverse cached tree (no I/O)
    current := n.root
    for i := 0; i < len(state.SelectedIndices) && current != nil; i++ {
        selectedIdx := state.SelectedIndices[i]
        if selectedIdx >= len(current.Children) {
            break
        }
        current = current.Children[selectedIdx]
    }

    if current != nil {
        return current.Path
    }

    return ""
}
```

## When to Rebuild

**Build tree on**:
- ✅ Application startup
- ✅ Explicit user refresh (Ctrl+R)
- ✅ File watcher events (if implemented)
- ✅ Navigating to new root path

**DON'T rebuild on**:
- ❌ Every navigation operation
- ❌ Every UI render
- ❌ Every key press
- ❌ Every state update
- ❌ Timer/polling intervals

## TerraX-Specific Implementation

Per [ADR-0002: Navigator Pattern](../../adr/0002-navigator-pattern.md):

> **Tree Caching**
>
> The tree is built once at startup via `stack.BuildTree()` and cached in the Navigator. All navigation operations use this cached tree, ensuring zero filesystem I/O during navigation.
>
> **Benefits**:
> - Instant navigation response (< 1ms)
> - Scalable to large directory structures
> - Works well with network filesystems
> - Low resource usage

**TerraX current implementation** (correct):

```go
// cmd/root.go
root, err := stack.BuildTree(rootPath)  // Build once
tuiModel := tui.NewModel(root, maxDepth)  // Pass cached tree

// internal/tui/model.go
navigator: stack.NewNavigator(root, maxDepth)  // Store cached tree

// All navigation uses cached tree - no rebuilds
```

This is **correct**. Do not change this pattern.

## Enforcement

### Code Review Checklist

When reviewing code:

- [ ] Tree built once at startup
- [ ] Tree cached in Navigator or Model
- [ ] Navigation methods don't call BuildTree()
- [ ] Update() doesn't call BuildTree()
- [ ] View() doesn't call BuildTree()
- [ ] Refresh is explicit (user-initiated)
- [ ] No filesystem I/O in hot paths

### Performance Testing

```bash
# Run benchmarks regularly
go test -bench=. ./internal/stack

# Profile during development
go test -cpuprofile=cpu.prof -bench=BenchmarkNavigator ./internal/stack
go tool pprof cpu.prof

# Test with large directory structures
./terrax /usr  # Large system directory
# Navigation should be instant
```

### Static Analysis

```bash
# Search for potential repeated scans
grep -r "BuildTree" internal/tui/
# Should NOT appear in model.go, view.go, or update.go

grep -r "BuildTree" internal/stack/
# Should only appear in tree.go (implementation) and tests

# Check for filesystem operations in hot paths
grep -r "os.ReadDir" internal/tui/
grep -r "filepath.Walk" internal/tui/
# Should return ZERO results
```

## Quick Reference

| ❌ DON'T | ✅ DO |
|----------|-------|
| Rebuild tree on every navigation | Build once, cache in Navigator |
| Call BuildTree() in Update() | Pass cached tree to Model |
| Call BuildTree() in View() | Use cached tree for rendering |
| Scan filesystem in Navigator methods | Traverse cached in-memory tree |
| Rebuild on every render | Render from cached tree |
| Poll filesystem for changes | Explicit user refresh (Ctrl+R) |
| Store path in Navigator | Store cached tree in Navigator |

## Summary

**The Golden Rule**: Build the tree **once** at startup, cache it in Navigator, and reuse it for all navigation operations. Only rebuild when the user explicitly requests a refresh.

**Performance Impact**: Caching eliminates 99.9% of filesystem I/O, resulting in 50-500x faster navigation and a vastly better user experience.
