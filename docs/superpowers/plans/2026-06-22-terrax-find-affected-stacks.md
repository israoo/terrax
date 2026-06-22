# terrax find — Affected Stacks Detection Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add `terrax find --base <sha>` to detect which Terragrunt stacks are affected by a git commit range, including stacks that consume changed YAML files via `mark_as_read()` and stacks that transitively depend on any directly affected stack.

**Architecture:** Three-layer detection — (1) direct `terragrunt.hcl` changes, (2) non-HCL file changes propagated via `mark_as_read()` reverse map, (3) non-stack HCL changes propagated via `include` reverse map — all fed through the existing `AnalyzeGraph` dependents expansion. Two new passes are required before touching `cmd/`: first extend `internal/deps` with three new public functions, then build `internal/changes` which owns the graph and git diff logic.

**Tech Stack:** Go 1.25.5 · `github.com/spf13/cobra` · `github.com/spf13/viper` · `github.com/stretchr/testify`

## Global Constraints

- All comments must end with periods.
- Imports: 3 groups (stdlib, third-party, `github.com/israoo/terrax/...`), alphabetically sorted within each group.
- `internal/deps/` must remain stdlib-only — no viper, cobra, or UI imports.
- `internal/changes/` must not import viper, cobra, or any UI package; only `internal/deps` and `internal/stack`.
- Errors wrapped: `fmt.Errorf("context: %w", err)`.
- Tests: table-driven where multiple cases exist; use `t.TempDir()` for filesystem fixtures; use real git repos (not mocks) for git diff tests.
- Run `task check` before committing — it runs fmt, vet, lint, and tests with `-race`.

---

## File Map

| File | Change |
|------|--------|
| `internal/deps/parser.go` | Add `ParseIncludes`, `ParseMarkAsRead`, `ScanAllHCLFiles`, `extractStaticPrefix`, `shouldSkipHCLScanDir`; add `markAsReadRe` regex; add `io/fs` import |
| `internal/deps/parser_test.go` | Add tests for the three new public functions |
| `internal/changes/changes.go` | NEW — `FileGraph`, `BuildFileGraph`, `AffectedStacks`, and private helpers |
| `internal/changes/changes_test.go` | NEW — tests using real git repos in `t.TempDir()` |
| `cmd/find.go` | NEW — `terrax find` subcommand with `--base` and `--dir` flags |
| `cmd/find_test.go` | NEW — integration tests that build the binary and run it against a git fixture |

---

### Task 1: Extend `internal/deps` with `ParseIncludes`, `ParseMarkAsRead`, `ScanAllHCLFiles`

**Files:**
- Modify: `internal/deps/parser.go`
- Modify: `internal/deps/parser_test.go`

**Interfaces:**
- Produces:
  - `ParseIncludes(hclFilePath, repoRoot string) []string`
  - `ParseMarkAsRead(hclFilePath, repoRoot string) (staticPaths []string, dynamicPrefixes []string)`
  - `ScanAllHCLFiles(repoRoot string) []string`

- [ ] **Step 1: Write the failing tests for `ParseIncludes`**

Append to `internal/deps/parser_test.go`:

```go
func TestParseIncludes_ResolvesStaticGetRepoRoot(t *testing.T) {
    dir := t.TempDir()
    hclPath := filepath.Join(dir, "workloads", "prod", "app", "terragrunt.hcl")
    require.NoError(t, os.MkdirAll(filepath.Dir(hclPath), 0755))
    require.NoError(t, os.WriteFile(hclPath, []byte(`
include "root" {
  path = find_in_parent_folders("root.hcl")
}
include "envcommon" {
  path = "${get_repo_root()}/_envcommon/app.hcl"
}
`), 0644))

    got := ParseIncludes(hclPath, dir)
    assert.Equal(t, []string{filepath.Join(dir, "_envcommon", "app.hcl")}, got)
}

func TestParseIncludes_ResolvesRelativePath(t *testing.T) {
    dir := t.TempDir()
    hclPath := filepath.Join(dir, "workloads", "prod", "app", "terragrunt.hcl")
    require.NoError(t, os.MkdirAll(filepath.Dir(hclPath), 0755))
    require.NoError(t, os.WriteFile(hclPath, []byte(`
include "sibling" {
  path = "../shared.hcl"
}
`), 0644))

    got := ParseIncludes(hclPath, dir)
    assert.Equal(t, []string{filepath.Join(dir, "workloads", "prod", "shared.hcl")}, got)
}

func TestParseIncludes_SkipsDynamicPaths(t *testing.T) {
    dir := t.TempDir()
    hclPath := filepath.Join(dir, "app", "terragrunt.hcl")
    require.NoError(t, os.MkdirAll(filepath.Dir(hclPath), 0755))
    require.NoError(t, os.WriteFile(hclPath, []byte(`
include "root" {
  path = find_in_parent_folders("root.hcl")
}
include "dynamic" {
  path = "${local.some_var}/envcommon.hcl"
}
`), 0644))

    got := ParseIncludes(hclPath, dir)
    assert.Empty(t, got)
}

func TestParseIncludes_MissingFile(t *testing.T) {
    got := ParseIncludes("/nonexistent/terragrunt.hcl", "/repo")
    assert.Empty(t, got)
}
```

- [ ] **Step 2: Write the failing tests for `ParseMarkAsRead`**

Append to `internal/deps/parser_test.go`:

```go
func TestParseMarkAsRead_StaticGetRepoRootPath(t *testing.T) {
    dir := t.TempDir()
    hclPath := filepath.Join(dir, "_envcommon", "aurora.hcl")
    require.NoError(t, os.MkdirAll(filepath.Dir(hclPath), 0755))
    require.NoError(t, os.WriteFile(hclPath, []byte(`
locals {
  _aurora_yaml = mark_as_read("${get_repo_root()}/configuration/database/aurora.yaml")
  config       = yamldecode(file(local._aurora_yaml))
}
`), 0644))

    staticPaths, dynamicPrefixes := ParseMarkAsRead(hclPath, dir)
    assert.Equal(t, []string{filepath.Join(dir, "configuration", "database", "aurora.yaml")}, staticPaths)
    assert.Empty(t, dynamicPrefixes)
}

func TestParseMarkAsRead_DynamicPathExtractsDirectoryPrefix(t *testing.T) {
    dir := t.TempDir()
    hclPath := filepath.Join(dir, "_envcommon", "security-group.hcl")
    require.NoError(t, os.MkdirAll(filepath.Dir(hclPath), 0755))
    require.NoError(t, os.WriteFile(hclPath, []byte(`
locals {
  _sg_yaml = mark_as_read("${get_repo_root()}/configuration/network/security-groups/${local.sg_name}.yaml")
}
`), 0644))

    staticPaths, dynamicPrefixes := ParseMarkAsRead(hclPath, dir)
    assert.Empty(t, staticPaths)
    assert.Equal(t, []string{filepath.Join(dir, "configuration", "network", "security-groups")}, dynamicPrefixes)
}

func TestParseMarkAsRead_MultipleEntriesMixed(t *testing.T) {
    dir := t.TempDir()
    hclPath := filepath.Join(dir, "_envcommon", "ecs.hcl")
    require.NoError(t, os.MkdirAll(filepath.Dir(hclPath), 0755))
    require.NoError(t, os.WriteFile(hclPath, []byte(`
locals {
  _ecs_yaml     = mark_as_read("${get_repo_root()}/configuration/compute/ecs-services.yaml")
  _cluster_yaml = mark_as_read("${get_repo_root()}/configuration/compute/ecs-cluster.yaml")
  _sg_yaml      = mark_as_read("${get_repo_root()}/configuration/network/security-groups/${local.sg_name}.yaml")
}
`), 0644))

    staticPaths, dynamicPrefixes := ParseMarkAsRead(hclPath, dir)
    assert.Equal(t, []string{
        filepath.Join(dir, "configuration", "compute", "ecs-cluster.yaml"),
        filepath.Join(dir, "configuration", "compute", "ecs-services.yaml"),
    }, staticPaths)
    assert.Equal(t, []string{filepath.Join(dir, "configuration", "network", "security-groups")}, dynamicPrefixes)
}

func TestParseMarkAsRead_NoMarkAsRead(t *testing.T) {
    dir := t.TempDir()
    hclPath := filepath.Join(dir, "app", "terragrunt.hcl")
    require.NoError(t, os.MkdirAll(filepath.Dir(hclPath), 0755))
    require.NoError(t, os.WriteFile(hclPath, []byte(`
dependency "vpc" {
  config_path = "../vpc"
}
`), 0644))

    staticPaths, dynamicPrefixes := ParseMarkAsRead(hclPath, dir)
    assert.Empty(t, staticPaths)
    assert.Empty(t, dynamicPrefixes)
}

func TestParseMarkAsRead_MissingFile(t *testing.T) {
    staticPaths, dynamicPrefixes := ParseMarkAsRead("/nonexistent/terragrunt.hcl", "/repo")
    assert.Empty(t, staticPaths)
    assert.Empty(t, dynamicPrefixes)
}
```

- [ ] **Step 3: Write the failing test for `ScanAllHCLFiles`**

Append to `internal/deps/parser_test.go`:

```go
func TestScanAllHCLFiles_FindsAllHCLFilesExcludingSystemDirs(t *testing.T) {
    dir := t.TempDir()

    wantFiles := []string{
        "root.hcl",
        "globals/globals.hcl",
        "_envcommon/core.hcl",
        "workloads/prod/app/terragrunt.hcl",
    }
    skipFiles := []string{
        ".git/config.hcl",
        ".terraform/backend.hcl",
        ".terragrunt-cache/cached.hcl",
        "vendor/module/main.hcl",
    }
    otherFiles := []string{
        "configuration/globals.yaml",
        "README.md",
    }

    for _, f := range append(append(wantFiles, skipFiles...), otherFiles...) {
        p := filepath.Join(dir, f)
        require.NoError(t, os.MkdirAll(filepath.Dir(p), 0755))
        require.NoError(t, os.WriteFile(p, []byte(""), 0644))
    }

    got := ScanAllHCLFiles(dir)

    var expected []string
    for _, f := range wantFiles {
        expected = append(expected, filepath.Join(dir, f))
    }
    assert.ElementsMatch(t, expected, got)
}
```

- [ ] **Step 4: Run to confirm all new tests fail**

```bash
go test ./internal/deps/...
```

Expected: build failure — `undefined: ParseIncludes`, `undefined: ParseMarkAsRead`, `undefined: ScanAllHCLFiles`.

- [ ] **Step 5: Add the `markAsReadRe` regex and `io/fs` import to `parser.go`**

In `internal/deps/parser.go`, update the `var` block and imports:

```go
import (
    "io/fs"
    "os"
    "path/filepath"
    "regexp"
    "sort"
    "strings"
)

var (
    configPathRe = regexp.MustCompile(`config_path\s*=\s*"([^"]+)"`)
    markAsReadRe = regexp.MustCompile(`mark_as_read\s*\(\s*"([^"]+)"\s*\)`)
)
```

- [ ] **Step 6: Implement `ParseIncludes`**

Add to `internal/deps/parser.go` (before `resolvePath`):

```go
// ParseIncludes reads an HCL file and returns the absolute paths of all statically resolvable
// include blocks. Paths containing unresolvable expressions (e.g. find_in_parent_folders,
// unresolved ${...} interpolations) are silently skipped. Returns an empty slice on error.
func ParseIncludes(hclFilePath, repoRoot string) []string {
    content, err := os.ReadFile(hclFilePath)
    if err != nil {
        return []string{}
    }
    fileDir := filepath.Dir(hclFilePath)
    var result []string
    for _, raw := range extractIncludePaths(string(content)) {
        if resolved := resolvePath(raw, fileDir, repoRoot); resolved != "" {
            result = append(result, resolved)
        }
    }
    return result
}
```

- [ ] **Step 7: Implement `ParseMarkAsRead` and `extractStaticPrefix`**

Add to `internal/deps/parser.go`:

```go
// ParseMarkAsRead reads an HCL file and extracts mark_as_read() file references.
// Returns two slices: staticPaths contains absolute paths for fully resolvable expressions;
// dynamicPrefixes contains absolute directory prefixes for expressions with unresolvable
// interpolations (conservative: any file under that prefix is treated as a potential match).
// Both slices are sorted and deduplicated. Returns empty slices on error or no matches.
func ParseMarkAsRead(hclFilePath, repoRoot string) (staticPaths []string, dynamicPrefixes []string) {
    content, err := os.ReadFile(hclFilePath)
    if err != nil {
        return []string{}, []string{}
    }
    fileDir := filepath.Dir(hclFilePath)
    seenStatic := make(map[string]bool)
    seenPrefix := make(map[string]bool)
    for _, match := range markAsReadRe.FindAllStringSubmatch(string(content), -1) {
        raw := match[1]
        if resolved := resolvePath(raw, fileDir, repoRoot); resolved != "" {
            if !seenStatic[resolved] {
                seenStatic[resolved] = true
                staticPaths = append(staticPaths, resolved)
            }
            continue
        }
        if prefix := extractStaticPrefix(raw, repoRoot); prefix != "" {
            if !seenPrefix[prefix] {
                seenPrefix[prefix] = true
                dynamicPrefixes = append(dynamicPrefixes, prefix)
            }
        }
    }
    sort.Strings(staticPaths)
    sort.Strings(dynamicPrefixes)
    if staticPaths == nil {
        staticPaths = []string{}
    }
    if dynamicPrefixes == nil {
        dynamicPrefixes = []string{}
    }
    return staticPaths, dynamicPrefixes
}

// extractStaticPrefix returns the absolute directory prefix of a mark_as_read path that
// contains unresolvable interpolations. It substitutes ${get_repo_root()} and takes
// everything up to the first remaining ${, then returns filepath.Dir of that prefix.
// Returns an empty string if no static prefix can be determined.
func extractStaticPrefix(raw, repoRoot string) string {
    substituted := strings.ReplaceAll(raw, "${get_repo_root()}", repoRoot)
    idx := strings.Index(substituted, "${")
    if idx < 0 {
        return ""
    }
    prefix := strings.TrimRight(substituted[:idx], "/\\")
    if !filepath.IsAbs(prefix) {
        return ""
    }
    return filepath.Clean(prefix)
}
```

- [ ] **Step 8: Implement `ScanAllHCLFiles` and `shouldSkipHCLScanDir`**

Add to `internal/deps/parser.go`:

```go
// ScanAllHCLFiles walks the repo and returns the absolute paths of every .hcl file found,
// skipping hidden directories and known non-Terragrunt directories (.git, .terraform,
// .terragrunt-cache, vendor). Unlike FindAndBuildTree this includes non-stack HCL files
// such as _envcommon, globals, and account/region configs.
func ScanAllHCLFiles(repoRoot string) []string {
    var files []string
    _ = filepath.WalkDir(repoRoot, func(path string, d fs.DirEntry, err error) error {
        if err != nil {
            return nil
        }
        if d.IsDir() {
            name := d.Name()
            if strings.HasPrefix(name, ".") || shouldSkipHCLScanDir(name) {
                return filepath.SkipDir
            }
            return nil
        }
        if filepath.Ext(path) == ".hcl" {
            files = append(files, path)
        }
        return nil
    })
    return files
}

// shouldSkipHCLScanDir returns true for directories that should be skipped when scanning
// for HCL files. Mirrors the skip list in internal/stack/builder.go.
func shouldSkipHCLScanDir(name string) bool {
    switch name {
    case ".git", ".terraform", ".terragrunt-cache", "vendor", ".idea", ".vscode":
        return true
    }
    return false
}
```

- [ ] **Step 9: Run tests to confirm GREEN**

```bash
go test ./internal/deps/... -v
```

Expected: all existing tests pass, all 9 new tests pass. Output ends with `PASS`.

- [ ] **Step 10: Commit**

```bash
git add internal/deps/parser.go internal/deps/parser_test.go
git commit -m "feat(deps): add ParseIncludes, ParseMarkAsRead, ScanAllHCLFiles"
```

---

### Task 2: New `internal/changes` package — `BuildFileGraph` and `AffectedStacks`

**Files:**
- Create: `internal/changes/changes.go`
- Create: `internal/changes/changes_test.go`

**Interfaces:**
- Consumes:
  - `deps.ParseMarkAsRead(hclFilePath, repoRoot string) ([]string, []string)`
  - `deps.ParseIncludes(hclFilePath, repoRoot string) []string`
  - `deps.ScanAllHCLFiles(repoRoot string) []string`
  - `deps.FindRepoRoot(startDir, rootConfigFile string) string`
  - `stack.FindAndBuildTree(rootDir, rootConfigFile string) (*stack.Node, int, error)`
  - `stack.Node.Path string`, `stack.Node.IsStack bool`, `stack.Node.Dependents []string`, `stack.Node.Children []*stack.Node`
- Produces:
  - `type FileGraph struct { MarkAsReadExact map[string][]string; MarkAsReadPrefix map[string][]string; IncludeReverse map[string][]string }`
  - `BuildFileGraph(repoRoot, rootConfigFile string) (*FileGraph, error)`
  - `AffectedStacks(repoRoot, baseCommit string, graph *FileGraph, stackTree *stack.Node) ([]string, error)`

- [ ] **Step 1: Write the failing tests for `BuildFileGraph`**

Create `internal/changes/changes_test.go`:

```go
package changes_test

import (
    "os"
    "os/exec"
    "path/filepath"
    "testing"

    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"

    "github.com/israoo/terrax/internal/changes"
    "github.com/israoo/terrax/internal/stack"
)

// repoFixture creates:
//   <root>/root.hcl
//   <root>/configuration/database/aurora.yaml
//   <root>/configuration/compute/ecs-services.yaml
//   <root>/configuration/network/security-groups/alb.yaml
//   <root>/_envcommon/aurora.hcl      <- mark_as_read aurora.yaml (static)
//   <root>/_envcommon/ecs.hcl         <- mark_as_read ecs-services.yaml (static)
//   <root>/_envcommon/sg.hcl          <- mark_as_read security-groups/${local.sg_name}.yaml (dynamic)
//   <root>/workloads/prod/aurora/terragrunt.hcl  <- include _envcommon/aurora.hcl
//   <root>/workloads/prod/ecs/terragrunt.hcl     <- include _envcommon/ecs.hcl
//   <root>/workloads/prod/sg-alb/terragrunt.hcl  <- include _envcommon/sg.hcl
//   <root>/workloads/prod/alb/terragrunt.hcl     <- standalone
func repoFixture(t *testing.T) string {
    t.Helper()
    dir := t.TempDir()

    files := map[string]string{
        "root.hcl": "",
        "configuration/database/aurora.yaml":             "aurora: {}",
        "configuration/compute/ecs-services.yaml":        "ecs_services: {}",
        "configuration/network/security-groups/alb.yaml": "rules: []",
        "_envcommon/aurora.hcl": `locals {
  _aurora_yaml = mark_as_read("${get_repo_root()}/configuration/database/aurora.yaml")
}`,
        "_envcommon/ecs.hcl": `locals {
  _ecs_yaml = mark_as_read("${get_repo_root()}/configuration/compute/ecs-services.yaml")
}`,
        "_envcommon/sg.hcl": `locals {
  _sg_yaml = mark_as_read("${get_repo_root()}/configuration/network/security-groups/${local.sg_name}.yaml")
}`,
        "workloads/prod/aurora/terragrunt.hcl": `include "envcommon" {
  path = "${get_repo_root()}/_envcommon/aurora.hcl"
}`,
        "workloads/prod/ecs/terragrunt.hcl": `include "envcommon" {
  path = "${get_repo_root()}/_envcommon/ecs.hcl"
}`,
        "workloads/prod/sg-alb/terragrunt.hcl": `include "envcommon" {
  path = "${get_repo_root()}/_envcommon/sg.hcl"
}`,
        "workloads/prod/alb/terragrunt.hcl": `# standalone stack`,
    }

    for rel, content := range files {
        p := filepath.Join(dir, rel)
        require.NoError(t, os.MkdirAll(filepath.Dir(p), 0755))
        require.NoError(t, os.WriteFile(p, []byte(content), 0644))
    }
    return dir
}

func gitRepo(t *testing.T) (root, baseSHA string) {
    t.Helper()
    root = repoFixture(t)
    gitRun(t, root, "init")
    gitRun(t, root, "config", "user.email", "test@test.com")
    gitRun(t, root, "config", "user.name", "Test")
    gitRun(t, root, "add", ".")
    gitRun(t, root, "commit", "-m", "initial")
    out, err := exec.Command("git", "-C", root, "rev-parse", "HEAD").Output()
    require.NoError(t, err)
    baseSHA = string(out[:len(out)-1])
    return root, baseSHA
}

func gitRun(t *testing.T, dir string, args ...string) {
    t.Helper()
    cmd := exec.Command("git", args...)
    cmd.Dir = dir
    out, err := cmd.CombinedOutput()
    require.NoError(t, err, "git %v: %s", args, out)
}

func gitCommitAll(t *testing.T, dir string) {
    t.Helper()
    gitRun(t, dir, "add", ".")
    gitRun(t, dir, "commit", "-m", "change")
}

func TestBuildFileGraph_MarkAsReadExactMapsYAMLToEnvcommonHCL(t *testing.T) {
    dir := repoFixture(t)
    graph, err := changes.BuildFileGraph(dir, "root.hcl")
    require.NoError(t, err)
    auroraYAML := filepath.Join(dir, "configuration", "database", "aurora.yaml")
    auroraHCL := filepath.Join(dir, "_envcommon", "aurora.hcl")
    assert.Contains(t, graph.MarkAsReadExact[auroraYAML], auroraHCL)
}

func TestBuildFileGraph_MarkAsReadPrefixMapsDirectoryToDynamicHCL(t *testing.T) {
    dir := repoFixture(t)
    graph, err := changes.BuildFileGraph(dir, "root.hcl")
    require.NoError(t, err)
    sgDir := filepath.Join(dir, "configuration", "network", "security-groups")
    sgHCL := filepath.Join(dir, "_envcommon", "sg.hcl")
    assert.Contains(t, graph.MarkAsReadPrefix[sgDir], sgHCL)
}

func TestBuildFileGraph_IncludeReverseMapsEnvcommonToLeafStack(t *testing.T) {
    dir := repoFixture(t)
    graph, err := changes.BuildFileGraph(dir, "root.hcl")
    require.NoError(t, err)
    auroraHCL := filepath.Join(dir, "_envcommon", "aurora.hcl")
    auroraStack := filepath.Join(dir, "workloads", "prod", "aurora", "terragrunt.hcl")
    assert.Contains(t, graph.IncludeReverse[auroraHCL], auroraStack)
}
```

- [ ] **Step 2: Write the failing tests for `AffectedStacks`**

Append to `internal/changes/changes_test.go`:

```go
func TestAffectedStacks_DirectlyChangedStackIsReturned(t *testing.T) {
    root, baseSHA := gitRepo(t)
    require.NoError(t, os.WriteFile(
        filepath.Join(root, "workloads", "prod", "alb", "terragrunt.hcl"),
        []byte("# changed"), 0644))
    gitCommitAll(t, root)

    graph, err := changes.BuildFileGraph(root, "root.hcl")
    require.NoError(t, err)
    tree, _, err := stack.FindAndBuildTree(root, "root.hcl")
    require.NoError(t, err)

    affected, err := changes.AffectedStacks(root, baseSHA, graph, tree)
    require.NoError(t, err)
    assert.Contains(t, affected, filepath.Join(root, "workloads", "prod", "alb"))
}

func TestAffectedStacks_ChangedStaticYAMLAffectsConsumingStack(t *testing.T) {
    root, baseSHA := gitRepo(t)
    require.NoError(t, os.WriteFile(
        filepath.Join(root, "configuration", "database", "aurora.yaml"),
        []byte("aurora: {changed: true}"), 0644))
    gitCommitAll(t, root)

    graph, err := changes.BuildFileGraph(root, "root.hcl")
    require.NoError(t, err)
    tree, _, err := stack.FindAndBuildTree(root, "root.hcl")
    require.NoError(t, err)

    affected, err := changes.AffectedStacks(root, baseSHA, graph, tree)
    require.NoError(t, err)
    assert.Contains(t, affected, filepath.Join(root, "workloads", "prod", "aurora"))
    assert.NotContains(t, affected, filepath.Join(root, "workloads", "prod", "ecs"))
}

func TestAffectedStacks_ChangedDynamicYAMLPrefixAffectsConservativeMatch(t *testing.T) {
    root, baseSHA := gitRepo(t)
    require.NoError(t, os.WriteFile(
        filepath.Join(root, "configuration", "network", "security-groups", "alb.yaml"),
        []byte("rules: [changed]"), 0644))
    gitCommitAll(t, root)

    graph, err := changes.BuildFileGraph(root, "root.hcl")
    require.NoError(t, err)
    tree, _, err := stack.FindAndBuildTree(root, "root.hcl")
    require.NoError(t, err)

    affected, err := changes.AffectedStacks(root, baseSHA, graph, tree)
    require.NoError(t, err)
    assert.Contains(t, affected, filepath.Join(root, "workloads", "prod", "sg-alb"))
}

func TestAffectedStacks_ChangedEnvcommonHCLAffectsIncludingStack(t *testing.T) {
    root, baseSHA := gitRepo(t)
    require.NoError(t, os.WriteFile(
        filepath.Join(root, "_envcommon", "ecs.hcl"),
        []byte(`locals { changed = true }`), 0644))
    gitCommitAll(t, root)

    graph, err := changes.BuildFileGraph(root, "root.hcl")
    require.NoError(t, err)
    tree, _, err := stack.FindAndBuildTree(root, "root.hcl")
    require.NoError(t, err)

    affected, err := changes.AffectedStacks(root, baseSHA, graph, tree)
    require.NoError(t, err)
    assert.Contains(t, affected, filepath.Join(root, "workloads", "prod", "ecs"))
    assert.NotContains(t, affected, filepath.Join(root, "workloads", "prod", "aurora"))
}

func TestAffectedStacks_NoChangesReturnsEmpty(t *testing.T) {
    root, baseSHA := gitRepo(t)
    graph, err := changes.BuildFileGraph(root, "root.hcl")
    require.NoError(t, err)
    tree, _, err := stack.FindAndBuildTree(root, "root.hcl")
    require.NoError(t, err)

    affected, err := changes.AffectedStacks(root, baseSHA, graph, tree)
    require.NoError(t, err)
    assert.Empty(t, affected)
}
```

- [ ] **Step 3: Run to confirm build fails**

```bash
go test ./internal/changes/...
```

Expected: `no non-test Go files in .../internal/changes`.

- [ ] **Step 4: Implement `changes.go`**

Create `internal/changes/changes.go`:

```go
// Package changes detects Terragrunt stacks affected by a git commit range.
package changes

import (
    "fmt"
    "os/exec"
    "path/filepath"
    "sort"
    "strings"

    "github.com/israoo/terrax/internal/deps"
    "github.com/israoo/terrax/internal/stack"
)

// FileGraph holds the reverse-edge maps needed to propagate file changes to stacks.
type FileGraph struct {
    // MarkAsReadExact maps an absolute YAML path to the HCL files that declared
    // mark_as_read() with that exact path.
    MarkAsReadExact map[string][]string
    // MarkAsReadPrefix maps an absolute directory prefix to the HCL files that
    // declared mark_as_read() with a dynamic path under that prefix.
    MarkAsReadPrefix map[string][]string
    // IncludeReverse maps an absolute HCL file path to the HCL files that include it.
    IncludeReverse map[string][]string
}

// BuildFileGraph scans every .hcl file under repoRoot and builds the three reverse-edge
// maps in two passes: pass 1 extracts mark_as_read references and include paths; pass 2
// inverts the forward include map to produce IncludeReverse.
func BuildFileGraph(repoRoot, rootConfigFile string) (*FileGraph, error) {
    if rootConfigFile == "" {
        rootConfigFile = "root.hcl"
    }
    resolvedRoot := deps.FindRepoRoot(repoRoot, rootConfigFile)

    g := &FileGraph{
        MarkAsReadExact:  make(map[string][]string),
        MarkAsReadPrefix: make(map[string][]string),
        IncludeReverse:   make(map[string][]string),
    }

    hclFiles := deps.ScanAllHCLFiles(resolvedRoot)
    includeForward := make(map[string][]string)

    for _, hclFile := range hclFiles {
        staticPaths, dynamicPrefixes := deps.ParseMarkAsRead(hclFile, resolvedRoot)
        for _, yamlPath := range staticPaths {
            g.MarkAsReadExact[yamlPath] = appendUnique(g.MarkAsReadExact[yamlPath], hclFile)
        }
        for _, prefix := range dynamicPrefixes {
            g.MarkAsReadPrefix[prefix] = appendUnique(g.MarkAsReadPrefix[prefix], hclFile)
        }
        if includes := deps.ParseIncludes(hclFile, resolvedRoot); len(includes) > 0 {
            includeForward[hclFile] = append(includeForward[hclFile], includes...)
        }
    }

    for includer, included := range includeForward {
        for _, inc := range included {
            g.IncludeReverse[inc] = appendUnique(g.IncludeReverse[inc], includer)
        }
    }

    return g, nil
}

// AffectedStacks returns the sorted, deduplicated absolute stack directory paths affected
// by changes between baseCommit and HEAD. See the spec for the full algorithm description.
func AffectedStacks(repoRoot, baseCommit string, graph *FileGraph, stackTree *stack.Node) ([]string, error) {
    changedFiles, err := gitDiff(repoRoot, baseCommit)
    if err != nil {
        return nil, fmt.Errorf("failed to get git diff: %w", err)
    }

    nodeMap := flattenStackNodes(stackTree)
    directStacks := make(map[string]bool)

    for _, rel := range changedFiles {
        absFile := filepath.Join(repoRoot, rel)
        switch {
        case filepath.Base(absFile) == "terragrunt.hcl":
            if stackDir := filepath.Dir(absFile); nodeMap[stackDir] != nil {
                directStacks[stackDir] = true
            }
        case filepath.Ext(absFile) == ".hcl":
            for _, s := range hclToStacks(absFile, graph.IncludeReverse, nodeMap) {
                directStacks[s] = true
            }
        default:
            if hcls := hclsForChangedFile(absFile, graph); len(hcls) > 0 {
                for _, hcl := range hcls {
                    for _, s := range hclToStacks(hcl, graph.IncludeReverse, nodeMap) {
                        directStacks[s] = true
                    }
                }
            } else if s := owningStack(absFile, repoRoot, nodeMap); s != "" {
                directStacks[s] = true
            }
        }
    }

    allAffected := make(map[string]bool)
    for stackDir := range directStacks {
        allAffected[stackDir] = true
        if node := nodeMap[stackDir]; node != nil {
            for _, dep := range node.Dependents {
                allAffected[dep] = true
            }
        }
    }

    result := make([]string, 0, len(allAffected))
    for s := range allAffected {
        result = append(result, s)
    }
    sort.Strings(result)
    return result, nil
}

func hclsForChangedFile(absFile string, graph *FileGraph) []string {
    seen := make(map[string]bool)
    var result []string
    for _, hcl := range graph.MarkAsReadExact[absFile] {
        if !seen[hcl] {
            seen[hcl] = true
            result = append(result, hcl)
        }
    }
    fileDir := filepath.Dir(absFile)
    for prefix, hcls := range graph.MarkAsReadPrefix {
        if fileDir == prefix || strings.HasPrefix(fileDir+string(filepath.Separator), prefix+string(filepath.Separator)) {
            for _, hcl := range hcls {
                if !seen[hcl] {
                    seen[hcl] = true
                    result = append(result, hcl)
                }
            }
        }
    }
    return result
}

func hclToStacks(hclFile string, includeReverse map[string][]string, nodeMap map[string]*stack.Node) []string {
    var stacks []string
    visited := make(map[string]bool)
    queue := []string{hclFile}
    for len(queue) > 0 {
        cur := queue[0]
        queue = queue[1:]
        if visited[cur] {
            continue
        }
        visited[cur] = true
        if filepath.Base(cur) == "terragrunt.hcl" {
            if dir := filepath.Dir(cur); nodeMap[dir] != nil {
                stacks = append(stacks, dir)
            }
            continue
        }
        queue = append(queue, includeReverse[cur]...)
    }
    return stacks
}

func owningStack(absFile, repoRoot string, nodeMap map[string]*stack.Node) string {
    dir := filepath.Dir(absFile)
    for {
        if nodeMap[dir] != nil {
            return dir
        }
        parent := filepath.Dir(dir)
        if parent == dir || !strings.HasPrefix(dir, repoRoot) {
            return ""
        }
        dir = parent
    }
}

func flattenStackNodes(root *stack.Node) map[string]*stack.Node {
    m := make(map[string]*stack.Node)
    if root == nil {
        return m
    }
    var walk func(*stack.Node)
    walk = func(n *stack.Node) {
        if n.IsStack {
            m[n.Path] = n
        }
        for _, c := range n.Children {
            walk(c)
        }
    }
    walk(root)
    return m
}

func gitDiff(repoRoot, baseCommit string) ([]string, error) {
    cmd := exec.Command("git", "diff", "--name-only", baseCommit+"...HEAD")
    cmd.Dir = repoRoot
    out, err := cmd.Output()
    if err != nil {
        return nil, fmt.Errorf("git diff failed: %w", err)
    }
    var files []string
    for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
        if line != "" {
            files = append(files, line)
        }
    }
    return files, nil
}

func appendUnique(slice []string, value string) []string {
    for _, v := range slice {
        if v == value {
            return slice
        }
    }
    return append(slice, value)
}
```

- [ ] **Step 5: Run tests to confirm GREEN**

```bash
go test ./internal/changes/... -v
```

Expected: all 8 tests pass. Output ends with `PASS ok github.com/israoo/terrax/internal/changes`.

- [ ] **Step 6: Commit**

```bash
git add internal/changes/
git commit -m "feat(changes): add BuildFileGraph and AffectedStacks for git-based detection"
```

---

### Task 3: `cmd/find.go` — new `terrax find` subcommand

**Files:**
- Create: `cmd/find.go`
- Create: `cmd/find_test.go`

**Interfaces:**
- Consumes:
  - `changes.BuildFileGraph(repoRoot, rootConfigFile string) (*changes.FileGraph, error)`
  - `changes.AffectedStacks(repoRoot, baseCommit string, graph *changes.FileGraph, stackTree *stack.Node) ([]string, error)`
  - `stack.FindAndBuildTree(rootDir, rootConfigFile string) (*stack.Node, int, error)`
  - `stack.CollectStackPaths(rootDir string) ([]string, error)`
  - `getWorkingDirectory(dirFlag string) (string, error)` — existing in `cmd/root.go`
  - `viper.GetString("root_config_file")` — configured via `.terrax.yaml`

- [ ] **Step 1: Write failing integration tests**

Create `cmd/find_test.go`:

```go
package cmd_test

import (
    "os"
    "os/exec"
    "path/filepath"
    "strings"
    "testing"

    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"
)

func buildTerrax(t *testing.T) string {
    t.Helper()
    bin := filepath.Join(t.TempDir(), "terrax")
    out, err := exec.Command("go", "build", "-o", bin, "../main.go").CombinedOutput()
    require.NoError(t, err, "go build: %s", out)
    return bin
}

func findTestRepo(t *testing.T) (root, baseSHA string) {
    t.Helper()
    dir := t.TempDir()
    files := map[string]string{
        "root.hcl":                 "",
        "configuration/db/db.yaml": "db: {}",
        "_envcommon/db.hcl": `locals {
  _db_yaml = mark_as_read("${get_repo_root()}/configuration/db/db.yaml")
}`,
        "workloads/prod/db/terragrunt.hcl": `include "envcommon" {
  path = "${get_repo_root()}/_envcommon/db.hcl"
}`,
        "workloads/prod/api/terragrunt.hcl": `# standalone`,
    }
    for rel, content := range files {
        p := filepath.Join(dir, rel)
        require.NoError(t, os.MkdirAll(filepath.Dir(p), 0755))
        require.NoError(t, os.WriteFile(p, []byte(content), 0644))
    }
    run := func(args ...string) {
        cmd := exec.Command("git", args...)
        cmd.Dir = dir
        out, err := cmd.CombinedOutput()
        require.NoError(t, err, "git %v: %s", args, out)
    }
    run("init")
    run("config", "user.email", "test@test.com")
    run("config", "user.name", "Test")
    run("add", ".")
    run("commit", "-m", "initial")
    out, err := exec.Command("git", "-C", dir, "rev-parse", "HEAD").Output()
    require.NoError(t, err)
    baseSHA = string(out[:len(out)-1])
    return dir, baseSHA
}

func TestFindCmd_WithBase_DirectStack(t *testing.T) {
    bin := buildTerrax(t)
    root, baseSHA := findTestRepo(t)

    require.NoError(t, os.WriteFile(
        filepath.Join(root, "workloads", "prod", "api", "terragrunt.hcl"),
        []byte("# changed"), 0644))
    exec.Command("git", "-C", root, "add", ".").Run()           //nolint
    exec.Command("git", "-C", root, "commit", "-m", "c").Run() //nolint

    out, err := exec.Command(bin, "find", "--base", baseSHA, "--dir", root).Output()
    require.NoError(t, err)

    lines := strings.Split(strings.TrimSpace(string(out)), "\n")
    assert.Contains(t, lines, filepath.Join(root, "workloads", "prod", "api"))
    assert.NotContains(t, lines, filepath.Join(root, "workloads", "prod", "db"))
}

func TestFindCmd_WithBase_YAMLChangeViaMarkAsRead(t *testing.T) {
    bin := buildTerrax(t)
    root, baseSHA := findTestRepo(t)

    require.NoError(t, os.WriteFile(
        filepath.Join(root, "configuration", "db", "db.yaml"),
        []byte("db: {changed: true}"), 0644))
    exec.Command("git", "-C", root, "add", ".").Run()           //nolint
    exec.Command("git", "-C", root, "commit", "-m", "c").Run() //nolint

    out, err := exec.Command(bin, "find", "--base", baseSHA, "--dir", root).Output()
    require.NoError(t, err)

    lines := strings.Split(strings.TrimSpace(string(out)), "\n")
    assert.Contains(t, lines, filepath.Join(root, "workloads", "prod", "db"))
    assert.NotContains(t, lines, filepath.Join(root, "workloads", "prod", "api"))
}

func TestFindCmd_WithoutBase_ListsAllStacks(t *testing.T) {
    bin := buildTerrax(t)
    root, _ := findTestRepo(t)

    out, err := exec.Command(bin, "find", "--dir", root).Output()
    require.NoError(t, err)

    lines := strings.Split(strings.TrimSpace(string(out)), "\n")
    assert.Contains(t, lines, filepath.Join(root, "workloads", "prod", "db"))
    assert.Contains(t, lines, filepath.Join(root, "workloads", "prod", "api"))
}

func TestFindCmd_WithBase_NoChanges_EmptyOutput(t *testing.T) {
    bin := buildTerrax(t)
    root, baseSHA := findTestRepo(t)

    out, err := exec.Command(bin, "find", "--base", baseSHA, "--dir", root).Output()
    require.NoError(t, err)
    assert.Empty(t, strings.TrimSpace(string(out)))
}
```

- [ ] **Step 2: Run to confirm tests fail**

```bash
go test ./cmd/... -run TestFind
```

Expected: binary builds but exits with status 1 (unknown command "find").

- [ ] **Step 3: Implement `cmd/find.go`**

Create `cmd/find.go`:

```go
package cmd

import (
    "fmt"
    "os"

    "github.com/spf13/cobra"
    "github.com/spf13/viper"

    "github.com/israoo/terrax/internal/changes"
    "github.com/israoo/terrax/internal/stack"
)

var findCmd = &cobra.Command{
    Use:   "find",
    Short: "List stacks, optionally filtered to those affected by a git commit range",
    Long: `List Terragrunt stacks. Without --base, lists all stacks under the working directory.
With --base, lists only stacks affected by changes between <base> and HEAD, including stacks
that consume changed YAML files via mark_as_read() and stacks that transitively depend on
any directly affected stack.`,
    RunE: runFind,
}

func init() {
    findCmd.Flags().String("dir", "", "Working directory (overrides current directory)")
    findCmd.Flags().String("base", "", "Base commit SHA for change detection; omit to list all stacks")
    rootCmd.AddCommand(findCmd)
}

func runFind(cmd *cobra.Command, _ []string) error {
    dirFlag, _ := cmd.Flags().GetString("dir")
    workDir, err := getWorkingDirectory(dirFlag)
    if err != nil {
        return fmt.Errorf("failed to get working directory: %w", err)
    }

    rootConfigFile := viper.GetString("root_config_file")
    baseCommit, _ := cmd.Flags().GetString("base")

    if baseCommit == "" {
        return runFindAll(workDir, rootConfigFile)
    }
    return runFindAffected(workDir, rootConfigFile, baseCommit)
}

func runFindAll(workDir, rootConfigFile string) error {
    paths, err := stack.CollectStackPaths(workDir)
    if err != nil {
        return fmt.Errorf("failed to collect stack paths: %w", err)
    }
    for _, p := range paths {
        if _, err := fmt.Fprintln(os.Stdout, p); err != nil {
            return fmt.Errorf("failed to write output: %w", err)
        }
    }
    return nil
}

func runFindAffected(workDir, rootConfigFile, baseCommit string) error {
    graph, err := changes.BuildFileGraph(workDir, rootConfigFile)
    if err != nil {
        return fmt.Errorf("failed to build file graph: %w", err)
    }
    tree, _, err := stack.FindAndBuildTree(workDir, rootConfigFile)
    if err != nil {
        return fmt.Errorf("failed to build stack tree: %w", err)
    }
    affected, err := changes.AffectedStacks(workDir, baseCommit, graph, tree)
    if err != nil {
        return fmt.Errorf("failed to detect affected stacks: %w", err)
    }
    for _, p := range affected {
        if _, err := fmt.Fprintln(os.Stdout, p); err != nil {
            return fmt.Errorf("failed to write output: %w", err)
        }
    }
    return nil
}
```

- [ ] **Step 4: Run tests to confirm GREEN**

```bash
go test ./cmd/... -run TestFind -v
```

Expected: all 4 tests pass.

- [ ] **Step 5: Run full suite**

```bash
task check
```

Expected: `0 issues.` from linter; all packages pass; no race conditions.

- [ ] **Step 6: Commit**

```bash
git add cmd/find.go cmd/find_test.go
git commit -m "feat(cmd): add terrax find subcommand with --base change detection"
```
