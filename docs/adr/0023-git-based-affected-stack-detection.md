# ADR-0023: Git-Based Affected Stack Detection

**Status**: Accepted

**Date**: 2026-06-22

**Deciders**: TerraX Core Team

**Related**:
- [ADR-0015: Static HCL Dependency Graph](0015-static-hcl-dependency-graph.md)
- [ADR-0016: Cycle Detection and Reverse Dependency Graph](0016-cycle-detection-and-reverse-dependency-graph.md)
- [ADR-0017: Filter-Based Execution Strategy](0017-filter-based-execution-strategy.md)

## Context

CI pipelines that deploy Terragrunt repositories need to determine which stacks are affected by a pull request or a set of commits before running `plan` or `apply`. The current reference implementation uses:

```bash
terragrunt find --filter "...[${BASE_SHA}...HEAD]"
```

This command requires Terragrunt to be installed and executed in the detect job. Terragrunt evaluates all HCL expressions dynamically — including initializing Terraform for each module — which adds latency and forces every CI runner in the detect phase to have Terraform and Terragrunt available.

Beyond direct stack changes (a `terragrunt.hcl` that was modified), two additional dependency mechanisms must be considered:

1. **`mark_as_read()`** — a Terragrunt function that explicitly registers a non-HCL file (typically a YAML configuration file) as an input dependency. When `configuration/aurora.yaml` changes, all stacks that declared `mark_as_read("${get_repo_root()}/configuration/aurora.yaml")` must be considered affected. This declaration appears most commonly in shared `_envcommon/*.hcl` files, not in leaf `terragrunt.hcl` files directly.

2. **`include` chains** — leaf `terragrunt.hcl` files typically contain only `include` blocks pointing to `_envcommon/*.hcl` files where the `mark_as_read` declarations actually live. A change propagation that only looks at direct `terragrunt.hcl` contents misses this indirection layer entirely.

Ignoring either mechanism produces false negatives: a changed YAML config that affects dozens of stacks would not trigger their re-evaluation.

## Decision

### New `internal/deps` functions

Three public functions are added to `internal/deps/parser.go`:

```go
// ParseIncludes returns absolute paths of all statically resolvable include blocks.
// find_in_parent_folders() and unresolvable ${...} expressions are silently skipped.
func ParseIncludes(hclFilePath, repoRoot string) []string

// ParseMarkAsRead extracts mark_as_read() references from a single HCL file.
// staticPaths: fully resolved absolute paths (${get_repo_root()} substituted).
// dynamicPrefixes: absolute directory prefix for paths containing unresolvable
// interpolations (e.g. ${local.sg_name}); any changed file under the prefix matches.
func ParseMarkAsRead(hclFilePath, repoRoot string) (staticPaths []string, dynamicPrefixes []string)

// ScanAllHCLFiles walks repoRoot and returns every .hcl file path, including
// _envcommon/, globals/, account.hcl, region.hcl — files invisible to FindAndBuildTree.
func ScanAllHCLFiles(repoRoot string) []string
```

Both `ParseIncludes` and `ParseMarkAsRead` are file-scoped (single file, no include following). The graph construction layer handles multi-file traversal.

### New `internal/changes` package — `FileGraph`

A `FileGraph` captures three reverse-edge maps built from all `.hcl` files in the repository:

```go
type FileGraph struct {
    // yaml_abs_path → []hcl files that declared mark_as_read with that exact path.
    MarkAsReadExact  map[string][]string
    // dir_prefix → []hcl files with a dynamic mark_as_read path under that prefix.
    MarkAsReadPrefix map[string][]string
    // hcl_abs_path → []hcl files that include it (one level, not transitive).
    IncludeReverse   map[string][]string
}
```

`BuildFileGraph` constructs the graph in two passes over `ScanAllHCLFiles`:

- **Pass 1:** For each HCL file, call `ParseMarkAsRead` to populate `MarkAsReadExact` and `MarkAsReadPrefix`, and call `ParseIncludes` to build a forward include map.
- **Pass 2:** Invert the forward include map to produce `IncludeReverse`.

### `AffectedStacks` — propagation algorithm

```
git diff BASE...HEAD --name-only
         │
         ▼
for each changed file:
  ├─ terragrunt.hcl     → containing directory is a directly affected stack
  ├─ other .hcl         → BFS via IncludeReverse until reaching terragrunt.hcl leaves
  ├─ any other file     → hclsForChangedFile: check MarkAsReadExact (exact path match)
  │                       and MarkAsReadPrefix (directory prefix match)
  │                       → each matched HCL → BFS via IncludeReverse
  └─ no mark_as_read    → owningStack: walk up directory tree to find enclosing stack
         │
         ▼
directly affected stacks
         │
         ▼  expand via node.Dependents (AnalyzeGraph, from ADR-0016)
all affected stacks (sorted, deduplicated)
```

### Dynamic `mark_as_read` path handling

When a `mark_as_read` path contains an unresolvable local variable (e.g. `${local.sg_name}`), the static prefix before the first unresolvable `${` is extracted after `${get_repo_root()}` substitution:

```
mark_as_read("${get_repo_root()}/configuration/network/security-groups/${local.sg_name}.yaml")
→ prefix: <repoRoot>/configuration/network/security-groups
```

Any changed file whose directory matches this prefix triggers all HCL files associated with it. This is conservative: it may produce false positives but never false negatives.

### `terrax find` subcommand

```bash
terrax find                # list all stacks (no git filter)
terrax find --base <sha>   # list stacks affected since <sha>
```

`--base` semantics follow `git diff BASE...HEAD`: BASE is the last known-good state (previous release tag, branch merge base). Passing the SHA of the commit that introduced a change as BASE produces no results for that change — the correct base is the commit's parent.

`--dir` is rarely needed; `FindRepoRoot` resolves upward from the current directory automatically.

## Consequences

### Positive

- No Terragrunt subprocess at detect time — change detection runs in milliseconds using static HCL analysis already present in `internal/deps`.
- Handles the complete three-layer dependency model: direct `terragrunt.hcl` changes, YAML changes via `mark_as_read`, and non-stack HCL changes via `include` chains.
- Reuses the reverse dependency graph from `AnalyzeGraph` (ADR-0016) for transitive dependent expansion — no new graph infrastructure.
- `internal/changes` has no UI imports, making it consumable from both `cmd/` and future non-CLI contexts (e.g. a `terrax-api` gRPC server).
- Single binary handles both detection and execution, ensuring consistent dependency resolution between "what changed?" and "what to run?".

### Negative

- Dynamic `mark_as_read` paths with unresolvable local variables trigger conservative prefix matching, potentially over-including stacks whose runtime `${local.sg_name}` value would not have matched the changed file.
- `--dir <subdirectory>` is a known footgun: `BuildFileGraph` resolves to the repo root correctly, but `AffectedStacks` uses the passed `workDir` as the git diff base, causing incorrect path joins. Users must always pass the repo root or omit `--dir`.
- `git diff BASE...HEAD` semantics are non-obvious: the commit containing a change must come _after_ BASE, not be BASE itself. A misunderstanding here produces silently empty output.
- `ScanAllHCLFiles` scans all `.hcl` files in the repository including non-stack files. On very large repositories with thousands of HCL files this adds measurable startup latency compared to the existing `FindAndBuildTree` which only visits stack directories.

## Alternatives Considered

### Option 1: `terragrunt find --filter` subprocess

**Description**: Run `terragrunt find --filter "...[BASE...HEAD]"` as a subprocess, parse its output, and relay the stack list. All dynamic expression resolution is delegated to Terragrunt.

**Pros**:

- 100% accurate — handles all dynamic `mark_as_read` and `config_path` expressions regardless of complexity.
- No parser maintenance as Terragrunt's HCL schema evolves.

**Cons**:

- Requires Terragrunt (and therefore Terraform) to be installed in the detect CI job.
- Adds significant latency: Terragrunt evaluates all modules to build its dependency graph.
- Terragrunt subprocess failures are opaque — a single module with a broken HCL can abort the entire detection run.

**Why rejected**: The CI detect job is intentionally lightweight — it only needs to compute a job matrix. Requiring a full Terragrunt installation there contradicts this principle. Additionally, TerraX's static analysis covers 100% of the `mark_as_read` and `config_path` patterns observed in the target repository (ADR-0015), making the accuracy tradeoff acceptable.

### Option 2: File-pattern heuristics (no HCL parsing)

**Description**: Map file paths to stacks using only directory structure: a changed file under `workloads/prod/aurora/` affects the aurora stack; a changed file under `configuration/` or `_envcommon/` forces all stacks to be included.

**Pros**:

- Zero parsing — no HCL reading, no regex, no filesystem scanning beyond the git diff itself.
- Trivially correct for direct stack changes.

**Cons**:

- `configuration/` heuristic forces every stack to be re-evaluated on any YAML change, even when only one stack consumes the changed file — defeating the purpose of change detection entirely.
- Cannot distinguish between `_envcommon/aurora.hcl` (affects aurora stacks only) and `_envcommon/ecs.hcl` (affects ECS stacks only) without reading the include declarations.
- Any repository layout that does not follow the expected convention breaks silently.

**Why rejected**: The whole value of change detection is avoiding full-repo re-evaluation. A heuristic that falls back to "run everything" whenever a shared file changes provides no improvement over not running detection at all. The `mark_as_read` and `include` maps are precisely the information needed to scope the impact correctly.

### Option 3: Two-pass per-stack scanning (no `ScanAllHCLFiles`)

**Description**: Instead of scanning all `.hcl` files upfront to build the `FileGraph`, for each file in the git diff, walk the already-built `stack.Node` tree and parse each stack's `mark_as_read` declarations on demand, following include chains with the existing `parseDepsFromFile` logic.

**Pros**:

- No new scanning step — reuses the stack tree already built by `FindAndBuildTree`.
- `ParseMarkAsRead` could follow include chains internally (same as `parseDepsFromFile` does for `config_path`), eliminating the need for a separate `BuildFileGraph` call.

**Cons**:

- `mark_as_read` in `_envcommon` files is declared relative to the envcommon file, not the leaf stack — no `callerDir` semantics apply. But to find _which_ stacks include a given envcommon file, the reverse include map is still required.
- Parsing every stack's include chain for every changed file in the diff is O(stacks × changed_files) per run. `BuildFileGraph` amortizes all scanning into a single pass regardless of how many files changed.
- The reverse include map cannot be built without reading all HCL files; the on-demand approach defers this cost to query time without eliminating it.

**Why rejected**: The two-pass design — build the full graph once, then query it for each changed file — is strictly cheaper when multiple files change in a single diff. Building the reverse include map requires a full repository scan regardless of approach; doing it once upfront and caching it in `FileGraph` is the correct decomposition. The `ScanAllHCLFiles` cost is paid once per `terrax find` invocation, not once per changed file.

## Future Enhancements

**Potential Improvements**:

1. Cache the serialized `FileGraph` to disk (e.g. `.terrax/file-graph.json`) and invalidate it on any `.hcl` file change, avoiding repeated full-repository scans in interactive or watch-mode usage.
2. Resolve dynamic `mark_as_read` paths by reading the local variable from the same HCL file's `locals` block using the existing regex infrastructure — covers the common case of `${local.sg_name}` where `sg_name` is defined in the same file.
3. Expose `terrax find --base <sha> --json` for CI pipeline consumption, outputting a JSON array of relative stack paths compatible with the `classify-and-build-matrices` action.
4. Surface the `--dir <subdirectory>` footgun as an explicit validation error: if the resolved `FindRepoRoot` differs from `workDir`, warn the user and use the resolved root as the git diff base.

## References

- [`internal/deps/parser.go`](../../internal/deps/parser.go) — `ParseIncludes`, `ParseMarkAsRead`, `ScanAllHCLFiles`
- [`internal/changes/changes.go`](../../internal/changes/changes.go) — `FileGraph`, `BuildFileGraph`, `AffectedStacks`
- [`cmd/find.go`](../../cmd/find.go) — `terrax find` subcommand
- [ADR-0015: Static HCL Dependency Graph](0015-static-hcl-dependency-graph.md)
- [ADR-0016: Cycle Detection and Reverse Dependency Graph](0016-cycle-detection-and-reverse-dependency-graph.md)
- [`detect-changes.yml`](../../../../efex/efex-tpl-do-pipeline-infrastructure-terragrunt/.github/workflows/detect-changes.yml) — CI workflow this feature replaces
