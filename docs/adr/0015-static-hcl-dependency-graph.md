# ADR-0015: Static HCL Dependency Graph

**Status**: Accepted

**Date**: 2026-06-20

**Deciders**: TerraX Core Team

**Related**:
- [ADR-0005: Filesystem Tree Building Strategy](0005-filesystem-tree-building-strategy.md)
- [ADR-0013: VS Code Extension Integration](0013-vscode-extension-integration.md)

## Context

Terragrunt stacks declare inter-module dependencies via `dependency "name" { config_path = "..." }` blocks in their `terragrunt.hcl` files. Understanding the dependency chain is valuable for impact analysis: before running `plan` or `apply` on a module, an engineer needs to know what it depends on and what depends on it.

The standard tool for this is `terragrunt graph-dependencies`, which produces a full dependency graph. The problem is that it invokes Terraform init for each module — on a repository with hundreds of modules this takes minutes. TerraX needs a fast alternative that works without network access and without spawning any subprocesses.

A static analysis of the target repository (`hip-iac-cl-aws-caas`, 695 HCL files) showed:

1. **98.6% of `config_path` values are static relative paths** (`../vpc`, `../../shared/networking`).
2. **One file uses `${get_repo_root()}/...`**, resolvable once the repo root is known.
3. **`find_in_parent_folders(...)` calls appear only in `include "root"` blocks**, which reference the root config file and declare no stack dependencies.
4. **Dependencies are often declared in `_envcommon/` shared files**, pulled in via `include "envcommon"` blocks. The leaf `terragrunt.hcl` typically contains only include blocks; the actual `dependency` declarations live in the envcommon file.
5. **No `dependencies { paths = [...] }` blocks are used** in this repository.

The envcommon pattern introduces a critical semantic constraint: `config_path = "../vpc"` in an envcommon file is always resolved by Terragrunt against the **leaf file that includes it**, not against the envcommon file's own directory. A parser that resolves relative paths against the envcommon file produces paths pointing to non-existent directories.

## Decision

### Go: `internal/deps` package

A new `internal/deps` package provides two functions:

```go
// FindRepoRoot walks up from startDir until it finds rootConfigFile, returns startDir on failure.
func FindRepoRoot(startDir, rootConfigFile string) string

// ParseDependencies returns sorted, deduplicated absolute paths of direct dependencies.
// Returns []string{} on any error. Never returns nil.
func ParseDependencies(hclFilePath, repoRoot string) []string
```

**Parsing strategy (regex only, no new Go dependencies):**

1. `configPathRe = regexp.MustCompile("config_path\\s*=\\s*\"([^\"]+)\"")` — extracts all `config_path` values.
2. `extractIncludePaths(content string)` — finds `path =` attributes inside `include "name" { ... }` blocks using a brace-depth counter, handling nested `{...}` (e.g. `locals` maps) that break naive `[^}]*` regexes.
3. `${get_repo_root()}` is replaced with the detected repo root.
4. `find_in_parent_folders(...)` calls are skipped (they are unquoted function calls, so the quoted-string regex never matches them).

**callerDir propagation — the key semantic fix:**

`parseDepsFromFile` accepts a `callerDir string` parameter representing the directory of the original leaf `terragrunt.hcl`. When following an include chain, `callerDir` is passed unchanged:

- `config_path` values are always resolved against `callerDir` (leaf directory).
- Include `path =` attributes are resolved against `fileDir` (the file that declares the include).

This mirrors Terragrunt's runtime behavior exactly. Without this, `config_path = "../vpc"` in `_envcommon/ca-central-1/core/alb.hcl` would resolve to `_envcommon/ca-central-1/core/vpc` — a path that does not appear in the scanned tree. With `callerDir`, it resolves to `workloads/<env>/ca-central-1/core/vpc`, which is the actual stack directory.

### `stack.Node` enrichment

The `Dependencies []string` field (json tag `"dependencies"`) is added to `stack.Node`. `FindAndBuildTree` now accepts a `rootConfigFile string` parameter (respecting the user's configured value from viper), detects the repo root once, and populates `Dependencies` for every stack node by calling `deps.ParseDependencies`. Non-stack nodes always have `Dependencies: []string{}` (empty array, never null).

### VS Code "Dependencies" panel

A second tree view `terrax.dependencyTree` (name: "Dependencies") is registered in the same Activity Bar container as the Stacks panel. `DependencyTreeProvider` maintains a `nodeMap: Map<string, StackNode>` built from the full tree JSON on each refresh. When a node is selected in the Stacks panel, `treeView.onDidChangeSelection` calls `depProvider.setFocus(node)`, which fires the tree data change event and causes `getChildren` to return that node's direct dependencies.

Each dependency is itself expandable (showing its own `dependencies` field from the JSON), enabling full transitive graph traversal up to `MAX_DEPTH = 10` to prevent infinite loops in circular graphs. Dependency paths not found in `nodeMap` are shown as placeholder nodes with the basename as label and a warning icon.

## Consequences

### Positive

- Tree refresh takes milliseconds instead of minutes — no Terragrunt subprocess, no network, no Terraform init.
- Dependency data is embedded in the existing `terrax tree --json` output, so the VS Code extension makes no additional process calls to display the Dependencies panel.
- `callerDir` propagation means parsed dependency paths match the scanned tree paths exactly, enabling O(1) lookups in the `nodeMap`.
- `FindAndBuildTree` now accepts `rootConfigFile`, so `${get_repo_root()}` resolution respects the user's configured value rather than always defaulting to `root.hcl`.
- The `internal/deps` package has no non-stdlib imports, keeping the dependency surface minimal.

### Negative

- Static parsing cannot resolve dynamically computed `config_path` expressions (e.g. `"${local.env}/vpc"` or paths built from `read_terragrunt_config()` locals). These are silently dropped from the dependency list with no warning.
- The `callerDir` fix assumes all includes in a leaf file resolve their `config_path` values relative to the same leaf. In practice this is always true for the `_envcommon` pattern, but a leaf that includes two different envcommon files where each envcommon's relative paths are meant to resolve against different bases would produce incorrect results.
- `FindRepoRoot` and `history.FindProjectRoot` duplicate the same directory-walk logic with different fallback semantics (`startDir` vs `""`). Kept separate to avoid a circular import dependency between `internal/deps` and `internal/history`.
- `ParseDependencies` is best-effort and silent on errors — a file with wrong permissions or unreadable content silently produces `[]string{}` with no diagnostic.

## Alternatives Considered

### Option 1: `terragrunt graph-dependencies` subprocess

**Description**: Call `terragrunt graph-dependencies` via `exec.CommandContext` and parse the DOT-format output. Terragrunt resolves all includes, locals, and expressions before producing the graph.

**Pros**:

- 100% accurate — handles all dynamic expressions.
- No parser maintenance as the HCL schema evolves.

**Cons**:

- Requires Terragrunt to be installed and accessible.
- Takes 30 seconds to several minutes on large repositories.
- Requires network access for provider downloads on first run.
- Blocks the VS Code extension host thread during the call.

**Why rejected**: The user's repository has 695 HCL files. A multi-minute blocking call per sidebar refresh is unusable. The static analysis revealed that 100% of dependencies in this repository are statically resolvable, making the accuracy tradeoff acceptable.

### Option 2: `hashicorp/hcl/v2` library

**Description**: Parse HCL files using the official HCL library from HashiCorp. This handles all syntactically valid HCL, supports partial expression evaluation, and correctly handles all formatting variations.

**Pros**:

- Robust against any valid HCL formatting.
- Could partially evaluate static expressions beyond what regex captures.
- Would not require custom brace-depth tracking for include blocks.

**Cons**:

- Adds a heavyweight Go dependency (~vendor bloat).
- Still cannot evaluate dynamic expressions without executing Terraform/Terragrunt functions.
- `config_path` extraction would still require understanding Terragrunt's specific schema — the HCL library parses structure but not semantics.

**Why rejected**: The `hashicorp/hcl/v2` library adds significant vendor weight for a package that processes only two specific HCL constructs (`dependency` blocks and `include` blocks). The static analysis showed that the regex approach covers 100% of the patterns in the target repository. The one non-trivial case (nested braces in include blocks) was handled with a brace-depth counter that adds ~25 lines rather than an entire new dependency.

### Option 3: TypeScript filesystem scanner in the VS Code extension

**Description**: Implement a TypeScript scanner that reads `terragrunt.hcl` files directly in the extension, without calling the `terrax` binary for dependency data.

**Pros**:

- Extension works even if the `terrax` binary is outdated.
- No changes to the Go binary needed.

**Cons**:

- Duplicates the scanning and parsing logic in a second language.
- Cannot reuse `callerDir` propagation logic without re-implementing it in TypeScript.
- The extension would need to scan the entire repository on every refresh, bypassing the cached tree the binary already built.
- Any change to Terragrunt's HCL schema must be reflected in both Go and TypeScript parsers.

**Why rejected**: This was also rejected in ADR-0013 for the stack tree. The same argument applies here — the Go binary is the authoritative source for all Terragrunt filesystem operations. Duplicating the dependency parser in TypeScript would create two codebases with the same `callerDir` subtlety to maintain.

## Future Enhancements

**Potential Improvements**:

1. Surface unresolved dynamic `config_path` expressions as a warning in the VS Code panel rather than silently omitting them — helps users identify repos where the static parser has gaps.
2. Add a `terrax deps --dir <path>` subcommand that outputs the dependency graph for a single module as JSON, enabling use in CI pipelines and scripts beyond the VS Code extension.
3. Extend the panel to show reverse dependencies (which modules depend on the selected one), enabling impact analysis before destructive operations.

## References

- [`internal/deps/parser.go`](../../internal/deps/parser.go)
- [`internal/stack/builder.go`](../../internal/stack/builder.go)
- [`extensions/vscode/src/dependencyProvider.ts`](../../extensions/vscode/src/dependencyProvider.ts)
- [ADR-0005: Filesystem Tree Building Strategy](0005-filesystem-tree-building-strategy.md)
- [ADR-0013: VS Code Extension Integration](0013-vscode-extension-integration.md)
