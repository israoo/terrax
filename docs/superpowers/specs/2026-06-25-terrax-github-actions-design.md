# TerraX GitHub Actions — Design Spec

**Date:** 2026-06-25
**Status:** Approved

## Overview

A dedicated public repo (`israoo/terrax-action`) exposing three composable GitHub Actions that other repositories can use to install and run TerraX in their CI workflows.

## Goals

- Let external repos install the TerraX binary in one step.
- Let external repos discover affected stacks (all or filtered by git diff) in one step.
- Let external repos print a plan summary to stdout in one step.
- Keep each action focused on a single responsibility — users compose them.

## Non-goals

- Posting PR comments (out of scope; users can pipe stdout if desired).
- A higher-level orchestrator action that chains the three.
- Any logic that duplicates what the TerraX CLI already does.
- macOS or Windows runner support — Linux (`ubuntu-*`) only.

## Repo structure

```
israoo/terrax-action/
├── setup-terrax/
│   └── action.yml
├── find-stacks/
│   └── action.yml
├── summary/
│   └── action.yml
└── README.md
```

All actions are **composite** (YAML + bash). No JavaScript, no Docker.

Versioning: repo-level tags `v1`, `v1.x.x` (standard GitHub Actions convention).

## Actions

### `setup-terrax`

Installs the TerraX binary from GitHub Releases and adds it to `PATH`.

**Inputs**

| Name | Required | Default | Description |
|------|----------|---------|-------------|
| `version` | no | `latest` | TerraX version to install (e.g. `v0.5.1` or `latest`). |

**Outputs**

| Name | Description |
|------|-------------|
| `terrax-version` | Resolved version that was installed (e.g. `v0.5.1`). |

**Logic**

1. If `version == "latest"`, resolve the tag via the GitHub API (`/repos/israoo/terrax/releases/latest`).
2. Detect OS (`linux`/`darwin`/`windows`) and arch (`amd64`/`arm64`) from runner environment.
3. Download the matching archive from GitHub Releases (`terrax_{version}_{os}_{arch}.tar.gz` or `.zip` on Windows).
4. Extract the binary and add the directory to `$GITHUB_PATH`.

---

### `find-stacks`

Discovers Terragrunt stacks by calling `terrax find`. Without `--base`, returns all stacks under `dir`. With `--base`, returns only stacks affected by changes between `base` and `HEAD` — including transitive dependents and stacks that consume changed YAML files via `mark_as_read()`.

**Inputs**

| Name | Required | Default | Description |
|------|----------|---------|-------------|
| `dir` | no | `.` | Root directory to scan for stacks. |
| `base` | no | `""` | Git ref for change detection (e.g. `origin/main`, a commit SHA). When empty, all stacks under `dir` are returned. |

**Outputs**

| Name | Description |
|------|-------------|
| `stacks` | JSON array of stack paths (e.g. `["infra/prod/vpc","infra/prod/eks"]`). |
| `count` | Number of stacks in the array. |

**Logic**

```bash
PATHS=$(terrax find ${dir:+--dir "$dir"} ${base:+--base "$base"})
JSON=$(echo "$PATHS" | jq -R -s 'split("\n") | map(select(length > 0))')
echo "stacks=$JSON" >> "$GITHUB_OUTPUT"
echo "count=$(echo "$JSON" | jq 'length')" >> "$GITHUB_OUTPUT"
```

The heavy lifting (change detection, transitive deps, YAML graph) is done by TerraX — the action is a thin wrapper.

---

### `summary`

Prints a grouped terminal summary of pending vs. no-change stacks from existing plan files in `.terrax/plans/`.

**Inputs**

| Name | Required | Default | Description |
|------|----------|---------|-------------|
| `dir` | no | `.` | Working directory (same as `terrax summary --dir`). |

**Outputs**

None. Output goes to stdout of the step.

**Logic**

```bash
terrax summary ${dir:+--dir "$dir"}
```

---

## Example workflow (external repo)

```yaml
jobs:
  plan:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
        with:
          fetch-depth: 0  # required for --base git diff

      - uses: israoo/terrax-action/setup-terrax@v1
        with:
          version: latest

      - uses: israoo/terrax-action/find-stacks@v1
        id: find
        with:
          dir: infra/
          base: origin/main

      - name: Run plan on affected stacks
        run: |
          echo '${{ steps.find.outputs.stacks }}' | jq -r '.[]' | while read stack; do
            terrax run plan --dir "$stack"
          done

      - uses: israoo/terrax-action/summary@v1
        with:
          dir: infra/
```

## Action implementation details

### OS/arch detection in `setup-terrax`

```bash
# Linux only. $RUNNER_ARCH is set by GitHub Actions: "X64", "ARM64"
case "$RUNNER_ARCH" in
  X64)   ARCH="x86_64" ;;
  ARM64) ARCH="arm64"  ;;
esac
```

Archive name follows GoReleaser template: `terrax_{version}_Linux_{arch}.tar.gz` (e.g. `terrax_0.4.0_Linux_x86_64.tar.gz`).

### `fetch-depth: 0` requirement

`find-stacks` with `--base` requires full git history to compute the diff. Callers must set `fetch-depth: 0` on their `actions/checkout` step. This is documented in the action's `description` field and README.

## Testing

Each action is validated with an integration workflow inside `israoo/terrax-action` that runs against a fixture repo (or a minimal inline terragrunt structure) on push and PR. The `setup-terrax` action is tested across `ubuntu-latest`, `macos-latest`, and `windows-latest`.
