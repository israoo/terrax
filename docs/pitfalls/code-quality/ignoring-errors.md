# Pitfall: Ignoring Errors Silently

**Category**: Code Quality

**Severity**: Critical

**Date Identified**: 2025-12-27

## Description

Using the blank identifier (`_`) to discard errors, or checking errors but taking no action, leading to silent failures, data corruption, and bugs that are extremely difficult to diagnose.

## Impact

Ignoring errors creates catastrophic problems:

- **Silent failures**: Operations fail without any indication.
- **Data corruption**: Failed writes or partial operations go unnoticed.
- **Security vulnerabilities**: Failed security checks silently pass.
- **Debugging nightmares**: Issues surface far from root cause.
- **Production incidents**: Silent failures accumulate into major outages.
- **Lost data**: File operations fail silently, data disappears.
- **Unpredictable behavior**: Program continues in invalid state.

## Root Cause

Common reasons errors are ignored:

1. **Convenience**: "I know it won't fail in practice."
2. **Laziness**: "Handling errors is too much work."
3. **Ignorance**: "I didn't know this could fail."
4. **Copy-paste**: Copying code without understanding error handling.
5. **Time pressure**: "I'll add error handling later" (never happens).
6. **Over-confidence**: "This error isn't important."
7. **Poor examples**: Following tutorials that ignore errors.

## How to Avoid

### Do

- **Always check errors**: Every error return value must be checked.
- **Handle or propagate**: Either handle the error or return it to caller.
- **Add context**: Wrap errors with information about what failed.
- **Log critical errors**: At boundaries, log before returning.
- **Fail fast**: Return immediately on error, don't continue.
- **Use linters**: Enable `errcheck` to catch ignored errors.

### Don't

- **Don't use blank identifier**: `_, err := ...` or `result, _ := ...`
- **Don't ignore in production**: Never ignore errors in production code.
- **Don't check without action**: If you check, you must handle.
- **Don't continue on error**: Return or handle, don't proceed.
- **Don't assume success**: Always verify operations succeeded.

## Detection

Warning signs of ignored errors:

- **Blank identifier**: `_` used for error return values.
- **Unused error variables**: `err` declared but never checked.
- **Empty error handling**: `if err != nil {}` with no action.
- **TODO comments**: `// TODO: handle error` that never get done.
- **Mysterious bugs**: Failures that appear unrelated to root cause.

### Code Smells

```go
// ❌ Blank identifier
result, _ := someFunction()

// ❌ Unused error variable
result, err := someFunction()
// err is never checked

// ❌ Empty error check
result, err := someFunction()
if err != nil {
    // No action taken
}
result, err := someFunction()
if err != nil {
    // TODO: handle this
}
```

## Remediation

If you find ignored errors, here's how to fix them:

### 1. Identify All Ignored Errors

```bash
# Use errcheck linter
go install github.com/kisielk/errcheck@latest
errcheck ./...

# Or use golangci-lint
golangci-lint run --enable=errcheck

# Search for blank identifiers
grep -r ", _" --include="*.go" .
```

### 2. Add Proper Error Handling

```go
// BEFORE (ignored error)
result, _ := os.ReadFile(path)

// AFTER (proper handling)
result, err := os.ReadFile(path)
if err != nil {
    return fmt.Errorf("failed to read file %s: %w", path, err)
}
```

### 3. Wrap Errors with Context

```go
// Add context at each layer
func BuildTree(rootPath string) (*Node, error) {
    entries, err := os.ReadDir(rootPath)
    if err != nil {
        // ✅ Wrap with context
        return nil, fmt.Errorf("failed to read directory %s: %w", rootPath, err)
    }
    // ...
}
```

### 4. Handle at Boundaries

```go
// At application boundaries, handle errors
func main() {
    if err := run(); err != nil {
        log.Fatalf("Application error: %v", err)
        os.Exit(1)
    }
}

func run() error {
    // Business logic that returns errors
    return nil
}
```

## Related

- [Standard: Error Handling](../../standards/error-handling.md)
- [ADR-0004: Separation of Concerns](../../adr/0004-separation-of-concerns.md)
- [Standard: Go Coding Standards](../../standards/go-coding-standards.md)

## Examples

### Bad: Ignoring Errors

```go
// ❌ WRONG: Ignoring error completely
func loadConfig(path string) Config {
    data, _ := os.ReadFile(path)  // ❌ File might not exist

    var cfg Config
    json.Unmarshal(data, &cfg)    // ❌ Unmarshaling might fail

    return cfg                     // ❌ Returning potentially zero value
}

// ❌ WRONG: Checking but not acting
func saveData(path string, data []byte) {
    err := os.WriteFile(path, data, 0644)
    if err != nil {
        // ❌ No action taken - data silently not saved!
    }
}

// ❌ WRONG: TODO comments
func processNode(node *Node) *Node {
    processed, err := transform(node)
    if err != nil {
        // TODO: handle error  // ❌ Never implemented
    }
    return processed
}

// ❌ WRONG: Continuing after error
func buildTree(root string) *Node {
    entries, err := os.ReadDir(root)
    if err != nil {
        log.Printf("Error: %v", err)  // ❌ Logged but continues
    }

    // ❌ Continues with nil/invalid entries
    for _, entry := range entries {
        // Will panic if entries is nil
    }
}
```

**Problems**:
- File read failures go unnoticed
- Data writes fail silently
- TODOs never get addressed
- Program continues in invalid state
- Impossible to debug when issues surface later

### Good: Proper Error Handling

```go
// ✅ CORRECT: Handle errors properly
func loadConfig(path string) (Config, error) {
    data, err := os.ReadFile(path)
    if err != nil {
        return Config{}, fmt.Errorf("failed to read config file %s: %w", path, err)
    }

    var cfg Config
    if err := json.Unmarshal(data, &cfg); err != nil {
        return Config{}, fmt.Errorf("failed to parse config file %s: %w", path, err)
    }

    return cfg, nil
}

// ✅ CORRECT: Return errors
func saveData(path string, data []byte) error {
    if err := os.WriteFile(path, data, 0644); err != nil {
        return fmt.Errorf("failed to save data to %s: %w", path, err)
    }
    return nil
}

// ✅ CORRECT: Handle errors immediately
func processNode(node *Node) (*Node, error) {
    processed, err := transform(node)
    if err != nil {
        return nil, fmt.Errorf("failed to transform node %s: %w", node.Name, err)
    }
    return processed, nil
}

// ✅ CORRECT: Fail fast on error
func buildTree(root string) (*Node, error) {
    entries, err := os.ReadDir(root)
    if err != nil {
        return nil, fmt.Errorf("failed to read directory %s: %w", root, err)
    }

    // Only continues if ReadDir succeeded
    node := &Node{Name: filepath.Base(root)}
    for _, entry := range entries {
        // Safe to iterate
    }

    return node, nil
}
```

**Benefits**:
- All failures are caught and reported
- Errors include context for debugging
- Caller can decide how to handle
- No silent failures
- Clear error propagation chain

### Bad: Specific Anti-Patterns

```go
// ❌ Defer with ignored error
defer file.Close()  // ❌ Close() error ignored

// ❌ Assignment in if without check
if data, _ := readData(); data != nil {  // ❌ Error discarded
    process(data)
}

// ❌ Range over error result
for _, item := range getItems() {  // ❌ If getItems() fails, silently empty
    process(item)
}

// ❌ Panic on error (in library code)
data, err := os.ReadFile(path)
if err != nil {
    panic(err)  // ❌ Don't panic in library code
}
```

### Good: Correct Patterns

```go
// ✅ Check defer errors when they matter
func processFile(path string) error {
    file, err := os.Open(path)
    if err != nil {
        return fmt.Errorf("failed to open %s: %w", path, err)
    }
    defer func() {
        if err := file.Close(); err != nil {
            log.Printf("Warning: failed to close %s: %v", path, err)
        }
    }()

    // Process file
    return nil
}

// ✅ Handle error before using result
func processData() error {
    data, err := readData()
    if err != nil {
        return fmt.Errorf("failed to read data: %w", err)
    }

    if data != nil {
        process(data)
    }
    return nil
}

// ✅ Check function that returns items and error
func processItems() error {
    items, err := getItems()
    if err != nil {
        return fmt.Errorf("failed to get items: %w", err)
    }

    for _, item := range items {
        if err := process(item); err != nil {
            return fmt.Errorf("failed to process item: %w", err)
        }
    }
    return nil
}

// ✅ Return errors, don't panic (library code)
func loadData(path string) ([]byte, error) {
    data, err := os.ReadFile(path)
    if err != nil {
        return nil, fmt.Errorf("failed to read %s: %w", path, err)
    }
    return data, nil
}
```

## Common Scenarios

### File Operations

```go
// ❌ WRONG
func readConfig() Config {
    data, _ := os.ReadFile("config.yaml")
    var cfg Config
    yaml.Unmarshal(data, &cfg)
    return cfg
}

// ✅ CORRECT
func readConfig() (Config, error) {
    data, err := os.ReadFile("config.yaml")
    if err != nil {
        return Config{}, fmt.Errorf("failed to read config: %w", err)
    }

    var cfg Config
    if err := yaml.Unmarshal(data, &cfg); err != nil {
        return Config{}, fmt.Errorf("failed to parse config: %w", err)
    }

    return cfg, nil
}
```

### Network Operations

```go
// ❌ WRONG
func fetchData(url string) []byte {
    resp, _ := http.Get(url)
    body, _ := io.ReadAll(resp.Body)
    defer resp.Body.Close()
    return body
}

// ✅ CORRECT
func fetchData(url string) ([]byte, error) {
    resp, err := http.Get(url)
    if err != nil {
        return nil, fmt.Errorf("failed to fetch %s: %w", url, err)
    }
    defer resp.Body.Close()

    if resp.StatusCode != http.StatusOK {
        return nil, fmt.Errorf("HTTP error: %s", resp.Status)
    }

    body, err := io.ReadAll(resp.Body)
    if err != nil {
        return nil, fmt.Errorf("failed to read response body: %w", err)
    }

    return body, nil
}
```

### Database Operations

```go
// ❌ WRONG
func getUser(id int) User {
    var user User
    db.QueryRow("SELECT * FROM users WHERE id = ?", id).Scan(&user.Name, &user.Email)
    return user
}

// ✅ CORRECT
func getUser(id int) (User, error) {
    var user User
    err := db.QueryRow("SELECT * FROM users WHERE id = ?", id).Scan(&user.Name, &user.Email)
    if err != nil {
        if err == sql.ErrNoRows {
            return User{}, fmt.Errorf("user %d not found", id)
        }
        return User{}, fmt.Errorf("failed to query user %d: %w", id, err)
    }
    return user, nil
}
```

## Enforcement

### Enable Linters

**`.golangci.yml`**:

```yaml
linters:
  enable:
    - errcheck       # Check that errors are checked
    - errorlint      # Find code that will cause problems with error wrapping
    - goerr113       # Enforce error handling best practices
    - wrapcheck      # Check that errors from external packages are wrapped
```

### CI Integration

```yaml
# .github/workflows/ci.yml
- name: Run linters
  run: golangci-lint run --enable=errcheck,errorlint
```

### Pre-commit Hook

```bash
#!/bin/bash
# .git/hooks/pre-commit

# Check for ignored errors
if errcheck ./... 2>&1 | grep -q "Error return value"; then
    echo "Error: Found ignored error return values"
    echo "Run: errcheck ./..."
    exit 1
fi
```

## Testing Error Paths

Always test error cases:

```go
func TestLoadConfig_FileNotFound(t *testing.T) {
    _, err := loadConfig("/nonexistent/path")
    if err == nil {
        t.Error("expected error for nonexistent file")
    }

    if !errors.Is(err, os.ErrNotExist) {
        t.Errorf("expected ErrNotExist, got %v", err)
    }
}

func TestLoadConfig_InvalidYAML(t *testing.T) {
    // Create temp file with invalid YAML
    tmpfile, _ := os.CreateTemp("", "invalid.yaml")
    defer os.Remove(tmpfile.Name())
    tmpfile.WriteString("invalid: yaml: content:")

    _, err := loadConfig(tmpfile.Name())
    if err == nil {
        t.Error("expected error for invalid YAML")
    }
}
```

## Code Review Checklist

When reviewing code, check for:

- [ ] No blank identifier (`_`) for error returns
- [ ] All errors checked immediately after call
- [ ] Errors either handled or returned (with context)
- [ ] No empty `if err != nil {}` blocks
- [ ] No TODO comments for error handling
- [ ] Errors wrapped with `%w` for error chains
- [ ] defer Close() errors handled when important
- [ ] Test cases for error paths

## TerraX-Specific Rules

Per [Error Handling Standards](../../standards/error-handling.md):

> **Never ignore errors**: All errors must be handled explicitly.
>
> **Add context at each layer**: Wrap errors with meaningful information.
>
> **Handle at boundaries**: Process errors at application boundaries.

This is **MANDATORY**. Ignoring errors is treated as a critical bug.

## Quick Reference

| ❌ DON'T | ✅ DO |
|----------|-------|
| `result, _ := func()` | `result, err := func()`<br>`if err != nil { ... }` |
| `if err != nil {}` | `if err != nil { return err }` |
| `// TODO: handle error` | Actually handle the error |
| Continue after error | Return immediately on error |
| `panic(err)` in library | `return err` to caller |
| Ignore defer errors | Check when important |
| Assume success | Verify operations succeeded |
