# terrax find — Affected Stacks Detection Design

**Date:** 2026-06-22
**Status:** Approved

---

## Goal

Add a `terrax find` subcommand that lists stacks affected by a git commit range, enabling CI pipelines to replace `terragrunt find --filter "...[BASE...HEAD]"` with a call that requires no Terragrunt installation at detection time and resolves dependencies in milliseconds via static HCL analysis.

---

## Background

The CI `detect-changes.yml` workflow currently uses:

```bash
terragrunt find --filter "...[${BASE}...HEAD]"
```

This requires Terragrunt to be installed in the detect job and runs Terraform init for each module to evaluate dynamic expressions. On a repo with hundreds of modules this can take minutes.

TerraX already has a static HCL dependency parser (`internal/deps`) that resolves `config_path` references without spawning any subprocesses. This feature extends that parser to also understand the two additional dependency mechanisms Terragrunt uses for change detection:

1. **`mark_as_read()`** — explicit non-HCL file dependency declaration. When a YAML config file is read with `mark_as_read("${get_repo_root()}/configuration/db.yaml")`, Terragrunt records that the consuming stack depends on that file for change-detection purposes.
2. **`include` chains** — a stack's `terragrunt.hcl` typically only contains `include` blocks; the actual `mark_as_read` calls live in shared `_envcommon/*.hcl` files that are included at runtime.

---

## Three-Layer Dependency Model

```
Layer 1 (existing — ADR-0015):
  stack A  →  config_path = "../B"  →  stack B
  Already handled by ParseDependencies + AnalyzeGraph.

Layer 2 (new):
  configuration/globals.yaml  →  mark_as_read()  →  globals/globals.hcl
  The YAML is an explicit non-HCL input to the HCL file.

Layer 3 (new):
  globals/globals.hcl  →  include  →  workloads/prod/*/terragrunt.hcl
  The HCL is not a stack itself; stacks include it.
```

When `configuration/globals.yaml` changes in a git diff, the full chain is:
```
changed YAML
  → mark_as_read reverse map → globals/globals.hcl
  → include reverse map      → all stacks that include it
  → Dependents (existing)    → all stacks that depend on those
```

---

## mark_as_read — Static vs Dynamic Paths

All `mark_as_read` calls use one of two patterns:

**Static** (majority): path is fully resolvable after substituting `${get_repo_root()}`:
```hcl
_aurora_yaml = mark_as_read("${get_repo_root()}/configuration/database/aurora.yaml")
```
→ Resolved to an exact absolute path. Exact match against changed files.

**Dynamic** (minority): path contains an unresolvable local variable:
```hcl
_sg_yaml = mark_as_read("${get_repo_root()}/configuration/network/security-groups/${local.sg_name}.yaml")
```
→ Extract the static prefix before the first `${` after `${get_repo_root()}` substitution:
  `<repoRoot>/configuration/network/security-groups`
→ Any changed file whose directory matches this prefix triggers all HCL files that declared the dynamic reference. Conservative (may over-include) but never under-includes.

This is the same tradeoff documented in ADR-0015 for dynamic `config_path` expressions.

---

## FileGraph — Three Reverse Maps

`BuildFileGraph` scans all `.hcl` files in the repository (including `_envcommon/`, `globals/`, `account.hcl`, `region.hcl`, etc. — files invisible to `FindAndBuildTree`) and produces:

```go
type FileGraph struct {
    // yaml_abs_path → []hcl files that declared mark_as_read with that exact path.
    MarkAsReadExact  map[string][]string
    // dir_prefix → []hcl files with a dynamic mark_as_read path under that directory.
    MarkAsReadPrefix map[string][]string
    // hcl_abs_path → []hcl files that include it (direct, not transitive).
    IncludeReverse   map[string][]string
}
```

Construction is two passes over `ScanAllHCLFiles(repoRoot)`:

- **Pass 1:** For each `.hcl` file: call `ParseMarkAsRead` → populate `MarkAsReadExact` and `MarkAsReadPrefix`; call `ParseIncludes` → build a forward include map.
- **Pass 2:** Invert the forward include map → `IncludeReverse`.

---

## AffectedStacks Algorithm

```
git diff BASE...HEAD --name-only
         │
         ▼
for each changed file:
  ├─ terragrunt.hcl          → its directory is a direct stack
  ├─ other .hcl              → BFS via IncludeReverse until terragrunt.hcl leaves
  ├─ .yaml or other file     → hclsForChangedFile (MarkAsReadExact + MarkAsReadPrefix)
  │                            → each matched HCL → BFS via IncludeReverse
  └─ no mark_as_read match   → owningStack (walk up dirs until a known stack dir)
         │
         ▼
directly affected stacks
         │
         ▼  expand via node.Dependents (from AnalyzeGraph — already computed)
         ▼
all affected stacks (sorted, deduplicated)
```

---

## CLI Interface

```bash
# List all stacks (no git filter):
terrax find [--dir <path>]

# List stacks affected by changes since BASE:
terrax find --base <sha> [--dir <path>]
```

**`--base` semantics:** `git diff BASE...HEAD` — BASE is the last known-good state (previous tag, branch merge base). The commit that introduced the change must come **after** BASE; passing the SHA of the change itself as BASE produces an empty result because that change is already included in BASE.

**`--dir` semantics:** Rarely needed. Without `--dir`, TerraX uses the current working directory and `FindRepoRoot` walks up to the repo root automatically. Explicit `--dir` is only needed when running from outside the repo.

---

## New Packages and Functions

### `internal/deps/parser.go` — extensions

```go
// ParseIncludes returns the absolute paths of all statically resolvable include blocks.
// find_in_parent_folders() and unresolvable ${...} expressions are silently skipped.
func ParseIncludes(hclFilePath, repoRoot string) []string

// ParseMarkAsRead extracts mark_as_read() file references from an HCL file.
// staticPaths: exact absolute paths (after ${get_repo_root()} substitution).
// dynamicPrefixes: absolute directory prefixes for paths with unresolvable interpolations.
// Both slices are sorted and deduplicated. Returns empty slices on error or no matches.
func ParseMarkAsRead(hclFilePath, repoRoot string) (staticPaths []string, dynamicPrefixes []string)

// ScanAllHCLFiles walks repoRoot and returns the absolute paths of every .hcl file,
// skipping .git, .terraform, .terragrunt-cache, vendor, and hidden directories.
func ScanAllHCLFiles(repoRoot string) []string
```

Internal helper (unexported):
```go
// extractStaticPrefix returns the absolute directory prefix of a mark_as_read path that
// contains unresolvable interpolations.
func extractStaticPrefix(raw, repoRoot string) string
```

### `internal/changes/changes.go` — new package

```go
type FileGraph struct {
    MarkAsReadExact  map[string][]string
    MarkAsReadPrefix map[string][]string
    IncludeReverse   map[string][]string
}

func BuildFileGraph(repoRoot, rootConfigFile string) (*FileGraph, error)
func AffectedStacks(repoRoot, baseCommit string, graph *FileGraph, stackTree *stack.Node) ([]string, error)
```

### `cmd/find.go` — new subcommand

Registers `terrax find` with `--base` and `--dir` flags. Delegates to `runFindAll` (no base) or `runFindAffected` (with base).

---

## Layer Rules Compliance

- `internal/deps/` — stdlib only; no viper, cobra, or UI imports. ✓
- `internal/changes/` — imports only `internal/deps` and `internal/stack`; no UI imports. ✓
- `cmd/find.go` — CLI glue only; delegates all logic to `internal/changes`. ✓

---

## Limitations

1. **Dynamic `mark_as_read` paths** — conservative prefix match may include stacks that wouldn't actually be affected at runtime. Acceptable tradeoff (same as ADR-0015 for dynamic `config_path`).
2. **`file()` without `mark_as_read`** — not detectable statically. By contract, Terragrunt itself cannot detect these either; `mark_as_read` is the explicit declaration required for change tracking.
3. **`--dir` with subdirectory** — `BuildFileGraph` resolves to the actual repo root via `FindRepoRoot`, but `AffectedStacks` uses the passed `workDir` as the git root. Passing a subdirectory produces incorrect path joins. Workaround: always pass the repo root or omit `--dir` entirely.
