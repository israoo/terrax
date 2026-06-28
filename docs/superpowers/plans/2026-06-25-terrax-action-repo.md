# TerraX GitHub Actions Repo Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Create the `israoo/terrax-action` public repo with three composable GitHub Actions composite actions: `setup-terrax`, `find-stacks`, and `summary`.

**Architecture:** Each action lives in its own subdirectory (`setup-terrax/action.yml`, etc.) as a YAML composite action using only `bash` steps. An integration test workflow inside the repo calls each action via local path references (`uses: ./setup-terrax`) and verifies behavior. No JavaScript, no Docker.

**Tech Stack:** GitHub Actions composite actions · bash · jq · curl · GitHub Releases API

## Global Constraints

- All actions: composite (`runs.using: composite`), bash-only steps.
- Archive naming (verified from live release): `terrax_{VERSION}_{OS}_{ARCH}.{EXT}` where VERSION has no `v` prefix (e.g. `0.4.0`), OS is title-cased (`Linux`, `Darwin`, `Windows`), ARCH is `x86_64` or `arm64`, EXT is `tar.gz` (Linux/macOS) or `zip` (Windows).
- `RUNNER_OS` values: `Linux`, `macOS`, `Windows`. `RUNNER_ARCH` values: `X64`, `ARM64`.
- `terrax find` output: one absolute path per line, empty output when no stacks found.
- `terrax summary` reads `.terrax/plans/` relative to repo root (detected via `root.hcl`).
- Versioning: repo-level tags `v1` and `v1.x.x` (standard GitHub Actions convention).
- jq must be available on the runner (pre-installed on all GitHub-hosted runners).

---

## File Map

```
(new repo: israoo/terrax-action)
├── setup-terrax/
│   └── action.yml               # Task 1 — installs terrax binary
├── find-stacks/
│   └── action.yml               # Task 2 — wraps `terrax find`
├── summary/
│   └── action.yml               # Task 3 — wraps `terrax summary`
├── tests/
│   └── fixtures/
│       ├── root.hcl             # Task 2 — required by terrax to detect repo root
│       ├── prod/
│       │   ├── vpc/
│       │   │   └── terragrunt.hcl   # Task 2
│       │   └── eks/
│       │       └── terragrunt.hcl   # Task 2
│       └── staging/
│           └── vpc/
│               └── terragrunt.hcl   # Task 2
├── .github/
│   └── workflows/
│       └── test.yml             # Tasks 1–3 — integration test workflow
└── README.md                    # Task 4
```

---

## Task 1: Repo scaffold + `setup-terrax` action

**Files:**
- Create: `setup-terrax/action.yml`
- Create: `.github/workflows/test.yml` (scaffold, extended in Tasks 2–3)

**Interfaces:**
- Produces: `setup-terrax/action.yml` — inputs: `version` (string, default `"latest"`); outputs: `terrax-version` (string, e.g. `"v0.4.0"`)

- [ ] **Step 1: Init the repo**

```bash
mkdir terrax-action && cd terrax-action
git init
git commit --allow-empty -m "chore: initial commit"
```

- [ ] **Step 2: Create `setup-terrax/action.yml`**

```yaml
# setup-terrax/action.yml
name: 'Setup TerraX'
description: 'Install the TerraX binary from GitHub Releases and add it to PATH.'
inputs:
  version:
    description: 'TerraX version to install (e.g. v0.5.1 or latest).'
    required: false
    default: 'latest'
outputs:
  terrax-version:
    description: 'Resolved version that was installed (e.g. v0.4.0).'
    value: ${{ steps.install.outputs.terrax-version }}
runs:
  using: 'composite'
  steps:
    - name: Install TerraX
      id: install
      shell: bash
      env:
        GH_VERSION: ${{ inputs.version }}
      run: |
        set -euo pipefail

        VERSION="$GH_VERSION"
        if [ "$VERSION" = "latest" ]; then
          VERSION=$(curl -fsSL \
            -H "Accept: application/vnd.github+json" \
            "https://api.github.com/repos/israoo/terrax/releases/latest" \
            | grep '"tag_name"' \
            | sed 's/.*"tag_name": "\(.*\)".*/\1/')
        fi

        # GoReleaser strips the leading 'v' from .Version in archive names.
        VERSION_CLEAN="${VERSION#v}"

        case "$RUNNER_OS" in
          Linux)   OS="Linux"  ;;
          macOS)   OS="Darwin" ;;
          Windows) OS="Windows" ;;
          *) echo "Unsupported OS: $RUNNER_OS" && exit 1 ;;
        esac

        case "$RUNNER_ARCH" in
          X64)   ARCH="x86_64" ;;
          ARM64) ARCH="arm64"  ;;
          *) echo "Unsupported arch: $RUNNER_ARCH" && exit 1 ;;
        esac

        if [ "$RUNNER_OS" = "Windows" ]; then
          EXT="zip"
          BIN="terrax.exe"
        else
          EXT="tar.gz"
          BIN="terrax"
        fi

        ARCHIVE="terrax_${VERSION_CLEAN}_${OS}_${ARCH}.${EXT}"
        URL="https://github.com/israoo/terrax/releases/download/${VERSION}/${ARCHIVE}"
        TMPDIR="$(mktemp -d)"

        echo "Downloading ${URL}"
        curl -fsSL "$URL" -o "${TMPDIR}/${ARCHIVE}"

        if [ "$EXT" = "tar.gz" ]; then
          tar -xzf "${TMPDIR}/${ARCHIVE}" -C "$TMPDIR"
        else
          7z x "${TMPDIR}/${ARCHIVE}" -o"${TMPDIR}" -y > /dev/null
        fi

        chmod +x "${TMPDIR}/${BIN}"
        echo "${TMPDIR}" >> "$GITHUB_PATH"
        echo "terrax-version=${VERSION}" >> "$GITHUB_OUTPUT"
```

- [ ] **Step 3: Create the test workflow scaffold with the `setup-terrax` job**

```yaml
# .github/workflows/test.yml
name: Test Actions

on:
  push:
    branches: [main]
  pull_request:
    branches: [main]

jobs:
  test-setup-terrax:
    name: Test setup-terrax
    runs-on: ${{ matrix.os }}
    strategy:
      matrix:
        os: [ubuntu-latest, macos-latest, windows-latest]
    steps:
      - uses: actions/checkout@v4

      - name: Install TerraX (latest)
        id: setup
        uses: ./setup-terrax

      - name: Verify binary is in PATH
        shell: bash
        run: terrax --version

      - name: Verify version output
        shell: bash
        run: |
          echo "Installed: ${{ steps.setup.outputs.terrax-version }}"
          [[ "${{ steps.setup.outputs.terrax-version }}" =~ ^v[0-9]+\.[0-9]+\.[0-9]+$ ]] \
            || (echo "terrax-version output has unexpected format" && exit 1)

      - name: Install TerraX (pinned)
        uses: ./setup-terrax
        with:
          version: v0.4.0

      - name: Verify pinned version
        shell: bash
        run: |
          VERSION=$(terrax --version 2>&1 | grep -oE 'v[0-9]+\.[0-9]+\.[0-9]+' | head -1)
          [ "$VERSION" = "v0.4.0" ] || (echo "Expected v0.4.0 got $VERSION" && exit 1)
```

- [ ] **Step 4: Commit**

```bash
git add setup-terrax/ .github/
git commit -m "feat: add setup-terrax composite action"
```

---

## Task 2: `find-stacks` action

**Files:**
- Create: `find-stacks/action.yml`
- Create: `tests/fixtures/root.hcl`
- Create: `tests/fixtures/prod/vpc/terragrunt.hcl`
- Create: `tests/fixtures/prod/eks/terragrunt.hcl`
- Create: `tests/fixtures/staging/vpc/terragrunt.hcl`
- Modify: `.github/workflows/test.yml` — add `test-find-stacks` job

**Interfaces:**
- Consumes: `setup-terrax/action.yml` from Task 1
- Produces: `find-stacks/action.yml` — inputs: `dir` (string, default `"."`), `base` (string, default `""`); outputs: `stacks` (JSON array string), `count` (string containing integer)

- [ ] **Step 1: Create test fixtures**

`tests/fixtures/root.hcl` — required by terrax to detect the repo root:
```hcl
# Root HCL marker for terrax repo root detection.
```

`tests/fixtures/prod/vpc/terragrunt.hcl`:
```hcl
# Fixture: prod/vpc stack.
```

`tests/fixtures/prod/eks/terragrunt.hcl`:
```hcl
# Fixture: prod/eks stack.
```

`tests/fixtures/staging/vpc/terragrunt.hcl`:
```hcl
# Fixture: staging/vpc stack.
```

- [ ] **Step 2: Create `find-stacks/action.yml`**

```yaml
# find-stacks/action.yml
name: 'Find TerraX Stacks'
description: 'List Terragrunt stacks under a directory. With --base, returns only stacks affected by changes between base and HEAD.'
inputs:
  dir:
    description: 'Root directory to scan for stacks.'
    required: false
    default: '.'
  base:
    description: 'Git base ref for change detection (e.g. origin/main, a commit SHA). When empty, all stacks under dir are returned. Requires fetch-depth: 0 on actions/checkout.'
    required: false
    default: ''
outputs:
  stacks:
    description: 'JSON array of stack paths (e.g. ["/home/runner/work/repo/infra/prod/vpc"]).'
    value: ${{ steps.find.outputs.stacks }}
  count:
    description: 'Number of stacks found.'
    value: ${{ steps.find.outputs.count }}
runs:
  using: 'composite'
  steps:
    - name: Find stacks
      id: find
      shell: bash
      env:
        INPUT_DIR: ${{ inputs.dir }}
        INPUT_BASE: ${{ inputs.base }}
      run: |
        set -euo pipefail

        ARGS=()
        [ -n "$INPUT_DIR" ] && [ "$INPUT_DIR" != "." ] && ARGS+=(--dir "$INPUT_DIR")
        [ -n "$INPUT_BASE" ] && ARGS+=(--base "$INPUT_BASE")

        PATHS=$(terrax find "${ARGS[@]}" 2>/dev/null || true)

        if [ -z "$PATHS" ]; then
          JSON="[]"
        else
          JSON=$(printf '%s' "$PATHS" | jq -R -s 'split("\n") | map(select(length > 0))')
        fi

        COUNT=$(printf '%s' "$JSON" | jq 'length')

        echo "stacks=${JSON}" >> "$GITHUB_OUTPUT"
        echo "count=${COUNT}" >> "$GITHUB_OUTPUT"
```

- [ ] **Step 3: Add `test-find-stacks` job to `.github/workflows/test.yml`**

Append this job to the existing `test.yml` (after `test-setup-terrax`):

```yaml
  test-find-stacks:
    name: Test find-stacks
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4

      - uses: ./setup-terrax

      - name: Find all stacks under fixtures
        id: all
        uses: ./find-stacks
        with:
          dir: tests/fixtures

      - name: Verify count is 3
        run: |
          COUNT="${{ steps.all.outputs.count }}"
          [ "$COUNT" = "3" ] || (echo "Expected 3 stacks, got $COUNT. stacks=${{ steps.all.outputs.stacks }}" && exit 1)

      - name: Verify stacks is a JSON array
        run: |
          echo '${{ steps.all.outputs.stacks }}' | jq 'if type == "array" then . else error("not an array") end'

      - name: find-stacks with no stacks dir returns empty array
        id: empty
        uses: ./find-stacks
        with:
          dir: /tmp

      - name: Verify empty result
        run: |
          [ "${{ steps.empty.outputs.count }}" = "0" ] \
            && echo '${{ steps.empty.outputs.stacks }}' | jq '. == []' \
            || (echo "Expected empty result" && exit 1)
```

- [ ] **Step 4: Commit**

```bash
git add find-stacks/ tests/ .github/
git commit -m "feat: add find-stacks composite action"
```

---

## Task 3: `summary` action

**Files:**
- Create: `summary/action.yml`
- Modify: `.github/workflows/test.yml` — add `test-summary` job

**Interfaces:**
- Consumes: `setup-terrax/action.yml` from Task 1
- Produces: `summary/action.yml` — input: `dir` (string, default `"."`); no outputs (stdout only)

- [ ] **Step 1: Create `summary/action.yml`**

```yaml
# summary/action.yml
name: 'TerraX Summary'
description: 'Print a grouped terminal summary of pending vs. no-change stacks from .terrax/plans/ plan files.'
inputs:
  dir:
    description: 'Working directory (same as terrax summary --dir). Defaults to repo root.'
    required: false
    default: '.'
runs:
  using: 'composite'
  steps:
    - name: Print plan summary
      shell: bash
      env:
        INPUT_DIR: ${{ inputs.dir }}
      run: |
        set -euo pipefail

        ARGS=()
        [ -n "$INPUT_DIR" ] && [ "$INPUT_DIR" != "." ] && ARGS+=(--dir "$INPUT_DIR")

        terrax summary "${ARGS[@]}"
```

- [ ] **Step 2: Add `test-summary` job to `.github/workflows/test.yml`**

Append this job to the existing `test.yml`:

```yaml
  test-summary:
    name: Test summary
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4

      - uses: ./setup-terrax

      - name: Create minimal repo root marker
        run: |
          echo "# root" > root.hcl
          mkdir -p .terrax/plans

      - name: Run summary with empty plans dir
        uses: ./summary
        # terrax summary exits 0 and prints "No plan files found." when .terrax/plans/ is empty.
```

- [ ] **Step 3: Commit**

```bash
git add summary/ .github/
git commit -m "feat: add summary composite action"
```

---

## Task 4: README + publish

**Files:**
- Create: `README.md`

- [ ] **Step 1: Create `README.md`**

```markdown
# terrax-action

GitHub Actions for [TerraX](https://github.com/israoo/terrax) — the interactive TUI executor for Terragrunt stacks.

## Actions

| Action | Description |
|--------|-------------|
| [`setup-terrax`](#setup-terrax) | Install the TerraX binary |
| [`find-stacks`](#find-stacks) | List stacks, optionally filtered by git diff |
| [`summary`](#summary) | Print a plan summary to stdout |

---

## `setup-terrax`

Install TerraX from GitHub Releases and add it to `PATH`.

### Inputs

| Name | Required | Default | Description |
|------|----------|---------|-------------|
| `version` | no | `latest` | Version to install (e.g. `v0.5.1` or `latest`). |

### Outputs

| Name | Description |
|------|-------------|
| `terrax-version` | Resolved version installed (e.g. `v0.4.0`). |

### Example

```yaml
- uses: israoo/terrax-action/setup-terrax@v1
  with:
    version: latest
```

---

## `find-stacks`

List Terragrunt stacks under a directory. With `base`, returns only stacks affected by changes between `base` and `HEAD` — including transitive dependents and stacks that consume changed YAML files.

> **Note:** When using `base`, set `fetch-depth: 0` on `actions/checkout`.

### Inputs

| Name | Required | Default | Description |
|------|----------|---------|-------------|
| `dir` | no | `.` | Root directory to scan. |
| `base` | no | `""` | Git ref for change detection (e.g. `origin/main`, a commit SHA). |

### Outputs

| Name | Description |
|------|-------------|
| `stacks` | JSON array of stack paths. |
| `count` | Number of stacks found. |

### Example

```yaml
- uses: israoo/terrax-action/find-stacks@v1
  id: find
  with:
    dir: infra/
    base: origin/main

- name: Show affected stacks
  run: echo '${{ steps.find.outputs.stacks }}'
```

---

## `summary`

Print a grouped terminal summary of pending vs. no-change stacks from `.terrax/plans/` plan files written by a previous `terrax run plan` step.

### Inputs

| Name | Required | Default | Description |
|------|----------|---------|-------------|
| `dir` | no | `.` | Working directory (same as `terrax summary --dir`). |

### Example

```yaml
- uses: israoo/terrax-action/summary@v1
  with:
    dir: infra/
```

---

## Full workflow example

```yaml
jobs:
  plan:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
        with:
          fetch-depth: 0

      - uses: israoo/terrax-action/setup-terrax@v1

      - uses: israoo/terrax-action/find-stacks@v1
        id: find
        with:
          dir: infra/
          base: origin/main

      - name: Run plan on affected stacks
        run: |
          echo '${{ steps.find.outputs.stacks }}' | jq -r '.[]' | while read -r stack; do
            terrax run plan --dir "$stack"
          done

      - uses: israoo/terrax-action/summary@v1
```

## License

Apache 2.0
```

- [ ] **Step 2: Commit README**

```bash
git add README.md
git commit -m "docs: add README with usage examples"
```

- [ ] **Step 3: Create the repo on GitHub and push**

```bash
gh repo create israoo/terrax-action --public --description "GitHub Actions for TerraX"
git remote add origin https://github.com/israoo/terrax-action.git
git push -u origin main
```

- [ ] **Step 4: Verify the test workflow passes on GitHub**

Open `https://github.com/israoo/terrax-action/actions` and confirm all three jobs in the `Test Actions` workflow pass.

- [ ] **Step 5: Tag v1**

```bash
git tag v1
git tag v1.0.0
git push origin v1 v1.0.0
```
