# Pitfall: Platform-Specific Path Handling

**Category**: Tooling

**Severity**: High

**Date Identified**: 2025-12-27

## Description

Using hardcoded path separators (`/` or `\`) or platform-specific path operations instead of Go's `filepath` package, causing failures on Windows or other operating systems.

## Impact

Platform-specific path handling creates serious issues:

- **Cross-platform failures**: Code works on Linux/macOS but breaks on Windows (or vice versa).
- **Silent bugs**: Paths may "work" in testing but fail in production on different OS.
- **User frustration**: Application advertised as cross-platform doesn't work universally.
- **Security risks**: Incorrect path handling can lead to directory traversal vulnerabilities.
- **Maintenance burden**: Platform-specific workarounds accumulate over time.
- **CI/CD complications**: Tests must run on multiple OSes to catch issues.

## Root Cause

Common reasons this happens:

1. **Developer environment bias**: "I develop on macOS, so I use `/`."
2. **Convenience**: Typing `"/"` is easier than `filepath.Join()`.
3. **Unfamiliarity**: Not knowing Go's `filepath` package exists.
4. **Copy-paste**: Copying code from Linux-specific examples.
5. **Testing gaps**: No CI testing on Windows to catch issues.
6. **String manipulation**: Treating paths as strings instead of structured data.

## How to Avoid

### Do

- **Always use `filepath.Join()`**: Constructs paths correctly for any OS.

  ```go
  path := filepath.Join("internal", "stack", "tree.go")
  ```

- **Use `filepath.Clean()`**: Normalizes paths to OS conventions.

  ```go
  cleanPath := filepath.Clean(userInput)
  ```

- **Use `filepath.Abs()`**: Converts relative to absolute paths safely.

  ```go
  absPath, err := filepath.Abs(relPath)
  ```

- **Use `filepath.Walk()` or `filepath.WalkDir()`**: Traverses directories portably.

- **Use `os.PathSeparator`**: When you must use separator directly.

  ```go
  parts := strings.Split(path, string(os.PathSeparator))
  ```

### Don't

- **Don't hardcode `/`**: Will break on Windows.

  ```go
  // WRONG
  path := "internal/stack/tree.go"  // Fails on Windows
  ```

- **Don't hardcode `\`**: Will break on Linux/macOS.

  ```go
  // WRONG
  path := "internal\\stack\\tree.go"  // Fails on Unix
  ```

- **Don't use string concatenation**: Not cross-platform.

  ```go
  // WRONG
  path := dir + "/" + file  // Use filepath.Join() instead
  ```

- **Don't use `strings.Split("/")` on paths**: Use `filepath.Split()` or `filepath.Dir()`.

- **Don't assume case sensitivity**: Windows is case-insensitive, Unix is case-sensitive.

## Detection

Warning signs of platform-specific path issues:

- **Hardcoded separators**: `/` or `\` in string literals for paths.
- **String concatenation**: Using `+` to build paths.
- **Path splitting**: `strings.Split(path, "/")` or `strings.Split(path, "\\")`.
- **CI failures on Windows**: Tests pass on Linux/macOS but fail on Windows.
- **User reports**: "Doesn't work on my Windows machine."

## Remediation

If you find platform-specific path handling:

1. **Identify all path operations**: Search codebase for `/` in path-related code.

   ```bash
   grep -r '".*/..*"' internal/ cmd/
   ```

2. **Replace with `filepath` functions**:

   ```go
   // Before
   path := "internal/stack/tree.go"

   // After
   path := filepath.Join("internal", "stack", "tree.go")
   ```

3. **Normalize existing paths**:

   ```go
   // Clean up user input or external paths
   path = filepath.Clean(path)
   ```

4. **Test on multiple platforms**:

   ```bash
   # Add Windows CI or test locally with WSL/VM
   GOOS=windows go build .
   ```

5. **Add lint rule**: Consider adding linter to catch hardcoded separators.

## Related

- [Standard: Cross-Platform Requirements](../../standards/cross-platform.md)
- [ADR-0004: Separation of Concerns](../../adr/0004-separation-of-concerns.md)

## Examples

### Bad: Hardcoded Unix Paths

```go
// internal/stack/tree.go
func BuildTree(rootPath string) (*Node, error) {
    // WRONG: Hardcoded "/" separator
    terragruntPath := rootPath + "/terragrunt.hcl"

    // WRONG: Unix-specific path
    configPath := "config/defaults.yaml"

    // WRONG: String split assumes Unix separator
    parts := strings.Split(path, "/")

    return node, nil
}
```

**Problems**:
- Fails on Windows where separator is `\`.
- Not portable to other operating systems.
- May cause subtle bugs with path resolution.

### Good: Cross-Platform Paths

```go
// internal/stack/tree.go
import "path/filepath"

func BuildTree(rootPath string) (*Node, error) {
    // CORRECT: Use filepath.Join()
    terragruntPath := filepath.Join(rootPath, "terragrunt.hcl")

    // CORRECT: Portable path construction
    configPath := filepath.Join("config", "defaults.yaml")

    // CORRECT: Use filepath functions for splitting
    dir := filepath.Dir(path)
    base := filepath.Base(path)

    return node, nil
}
```

**Benefits**:
- Works on Windows, Linux, macOS.
- Uses OS-appropriate separators automatically.
- Handles edge cases correctly.

### Bad: String Concatenation

```go
func getChildPath(parent, child string) string {
    // WRONG: String concatenation
    return parent + "/" + child
}
```

### Good: filepath.Join

```go
func getChildPath(parent, child string) string {
    // CORRECT: Cross-platform path joining
    return filepath.Join(parent, child)
}
```

### Bad: Splitting Paths

```go
func getPathComponents(path string) []string {
    // WRONG: Assumes Unix separator
    return strings.Split(path, "/")
}
```

### Good: filepath Operations

```go
func getPathComponents(path string) []string {
    // CORRECT: Use filepath for path operations
    var components []string
    for {
        dir, file := filepath.Split(path)
        if file == "" {
            break
        }
        components = append([]string{file}, components...)
        path = filepath.Clean(dir)
        if path == "." || path == string(filepath.Separator) {
            break
        }
    }
    return components
}

// Or use filepath.WalkDir for directory traversal
```

## Testing Cross-Platform Code

### Local Testing

```bash
# Test compilation for different platforms
GOOS=windows GOARCH=amd64 go build .
GOOS=linux GOARCH=amd64 go build .
GOOS=darwin GOARCH=amd64 go build .

# Run tests with different path separators
go test ./... -v
```

### CI Configuration

Ensure CI tests on multiple platforms:

```yaml
# .github/workflows/test.yml
jobs:
  test:
    strategy:
      matrix:
        os: [ubuntu-latest, windows-latest, macos-latest]
    runs-on: ${{ matrix.os }}
    steps:
      - uses: actions/checkout@v3
      - uses: actions/setup-go@v4
      - run: go test ./...
```

## Quick Reference

| **DON'T** | **DO** |
|-----------|--------|
| `path := dir + "/" + file` | `path := filepath.Join(dir, file)` |
| `strings.Split(path, "/")` | Use `filepath.Dir()`, `filepath.Base()` |
| `"internal/stack/tree.go"` | `filepath.Join("internal", "stack", "tree.go")` |
| `strings.Contains(path, "/")` | Use `filepath` operations |
| Assume case-sensitive | Use `filepath.EvalSymlinks()` for canonical paths |

## Advanced Windows Gotchas

The basic `filepath.Join` rule avoids most issues, but these subtler problems have been found in TerraX (2026-06-28):

### 1. `filepath.Dir` converts forward slashes to backslashes on Windows

`filepath.Dir` is OS-aware. If your map or slice stores paths with forward slashes (e.g. from `Node.Path` normalized elsewhere, or from user config) and you walk ancestors with `filepath.Dir`, the returned keys will use backslashes on Windows — causing lookups to silently fail.

**Bad:**
```go
// selectedPaths stores "/repo/env" but cur becomes "\repo\env" on Windows
cur := filepath.Dir(path)
for cur != path {
    if selectedPaths[cur] { // MISS on Windows
        return cur
    }
    path = cur
    cur = filepath.Dir(cur)
}
```

**Good:** normalize with `filepath.ToSlash` at every Dir call AND at the point where paths enter the map:
```go
path = filepath.ToSlash(path) // normalize at entry
prev := path
cur := filepath.ToSlash(filepath.Dir(path))
for cur != prev {
    if selectedPaths[cur] { // always forward-slash key
        return cur
    }
    prev = cur
    cur = filepath.ToSlash(filepath.Dir(cur))
}
```

Similarly, use `"/"` as the separator constant instead of `string(filepath.Separator)` when building prefixes for `strings.HasPrefix` checks against forward-slash paths:
```go
// Bad on Windows if childPath uses "/" but sep is "\"
prefix := child.Path + string(filepath.Separator)

// Good: always "/"
prefix := filepath.ToSlash(child.Path) + "/"
```

### 2. `filepath.IsAbs` returns `false` for Unix-rooted paths on Windows

On Windows, a path like `/custom/plans` has no drive letter, so `filepath.IsAbs` considers it *not absolute*. This causes it to be incorrectly joined with a repoRoot.

**Bad:**
```go
if filepath.IsAbs(jsonOutDir) { // false on Windows for "/custom/plans"!
    absDir = jsonOutDir
} else {
    absDir = filepath.Join(repoRoot, jsonOutDir) // wrong: \repo\custom\plans
}
```

**Good:** add a leading-slash check as a fallback:
```go
if filepath.IsAbs(jsonOutDir) || strings.HasPrefix(jsonOutDir, "/") {
    absDir = jsonOutDir
} else {
    absDir = filepath.Join(repoRoot, jsonOutDir)
}
```

### 3. Flags passed to external tools must use forward slashes

Even on Windows, tools like Terragrunt and Terraform expect forward slashes in flag values. Apply `filepath.ToSlash` to the final value before embedding it in a flag string.

**Bad:**
```go
args = append(args, fmt.Sprintf("--json-out-dir=%s", absDir))
// produces --json-out-dir=\repo\.terrax\plans on Windows
```

**Good:**
```go
args = append(args, fmt.Sprintf("--json-out-dir=%s", filepath.ToSlash(absDir)))
// always produces --json-out-dir=/repo/.terrax/plans
```

And in tests, expected values must match:
```go
// Bad: filepath.Join produces backslashes on Windows
want := "--json-out-dir=" + filepath.Join("/repo", ".terrax", "plans")

// Good: normalize expected value too
want := "--json-out-dir=" + filepath.ToSlash(filepath.Join("/repo", ".terrax", "plans"))
```

### 4. `t.TempDir` + `os.Chdir` cleanup ordering on Windows

On Windows, a directory cannot be deleted while it is the current working directory of any thread. `t.Cleanup` functions run in **LIFO** order, so the order of registration matters.

**Bad:** TempDir's cleanup (delete dir) runs before the Chdir cleanup (restore cwd):
```go
t.Cleanup(func() { _ = os.Chdir(originalWd) }) // registered first → runs SECOND
tmpDir := t.TempDir()                            // registered second → runs FIRST (deletes while cwd=tmpDir → FAIL)
require.NoError(t, os.Chdir(tmpDir))
```

**Good:** TempDir registered first → its cleanup runs second (after cwd is restored):
```go
tmpDir := t.TempDir()                            // registered first → runs SECOND
t.Cleanup(func() { _ = os.Chdir(originalWd) }) // registered second → runs FIRST (restores cwd)
require.NoError(t, os.Chdir(tmpDir))
```

## TerraX-Specific Guidelines

Per [CLAUDE.md](../../CLAUDE.md):

> **Cross-Platform (MANDATORY)**
>
> - Use `filepath.Join()` for paths, never hardcoded `/` or `\`
> - Test on Linux, macOS, and Windows
> - Use Go stdlib for filesystem operations
> - Use `filepath.ToSlash` when storing paths in maps/slices and walking ancestors
> - Use `|| strings.HasPrefix(path, "/")` fallback alongside `filepath.IsAbs`
> - Apply `filepath.ToSlash` to flag values passed to external tools
> - In tests: register `t.TempDir()` before `t.Cleanup(os.Chdir)` to get correct LIFO order

This is a **CRITICAL** requirement for TerraX. All path operations must be cross-platform.
