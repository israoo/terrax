# ADR-0021: Local Config Override via .terrax.local.yaml

**Status**: Accepted

**Date**: 2026-06-21

**Deciders**: TerraX Core Team

**Related**:
- [ADR-0007: Configuration Management Strategy](0007-configuration-management-strategy.md)
- [ADR-0020: Stack Groups and Classified Execution](0020-stack-groups-and-classified-execution.md)

## Context

TerraX configuration lives in `.terrax.yaml`, which is committed to the repository and shared across all team members. ADR-0020 introduced `stack_groups` with per-group env vars, but the env vars needed for a local developer (e.g. `TF_VAR_host=localhost TF_VAR_port=15432` for tunneled database connections) differ fundamentally from those needed in CI (where the runner has direct VPC access).

Committing local-only values into `.terrax.yaml` would pollute the shared config with machine-specific settings, require developers to remember not to commit their changes, and create merge conflicts as different developers use different tunnel configurations.

The pattern of a `.local` override file — gitignored, taking priority over the shared file — is established in many ecosystems (`.env.local` in Next.js, `docker-compose.override.yml`, `.npmrc.local`). It provides a clean separation between shared configuration and developer-specific overrides without requiring environment variables or wrapper scripts.

## Decision

TerraX supports a second configuration file, `.terrax.local.yaml`, which is:

1. **Gitignored by default** — `.terrax.local.yaml` is added to the repository's `.gitignore`.
2. **Deep-merged with priority** — values in `.terrax.local.yaml` override the corresponding values in `.terrax.yaml`. Nested keys are merged recursively so a developer can override a single nested field without repeating the entire parent structure.
3. **Searched in the same locations** — current directory, repository root, and home directory.
4. **Silently skipped if absent** — no error or warning when the file doesn't exist.

**Implementation:** `cmd/root.go:mergeLocalConfig` creates a secondary Viper instance, loads `.terrax.local.yaml`, and calls `viper.MergeConfigMap(local.AllSettings())`. Viper's `MergeConfigMap` performs a deep map merge, so the local file only needs to specify the keys it overrides.

**Example:** A developer who needs tunnel env vars creates `.terrax.local.yaml`:

```yaml
# .terrax.local.yaml — gitignored, machine-specific
stack_groups:
  private_connection:
    detect: "require_private_connection = true"
    depends_on: [default]
    env:
      TF_VAR_host: "localhost"
      TF_VAR_port: "15432"
```

The shared `.terrax.yaml` does not contain `stack_groups.private_connection.env`, or contains production values. The local file overrides only the env block without touching `detect` or `depends_on`.

**Load order:** `.terrax.yaml` is loaded first via `viper.ReadInConfig`. Then `mergeLocalConfig` is called, which loads `.terrax.local.yaml` into a fresh Viper instance and merges it into the global config. This applies in both `initConfig` (process startup) and `ensureConfigFromWorkDir` (per-command, when `--dir` differs from CWD).

## Consequences

### Positive

- **Zero coupling** — developers configure their local overrides independently without affecting shared config or creating merge conflicts.
- **Gitignored by default** — the file never accidentally gets committed since the repository's `.gitignore` includes it from the start.
- **Deep merge** — a local file can override a single nested key (e.g. `stack_groups.private_connection.env.TF_VAR_host`) without repeating the entire `stack_groups` block.
- **Predictable priority** — `.terrax.local.yaml` always wins over `.terrax.yaml`, matching the established convention of `.local` files.
- **No process changes** — no new CLI flags or environment variables needed. The local file is loaded automatically.

### Negative

- **Two-file mental model** — developers must know about both files. A misconfigured `.terrax.local.yaml` can produce unexpected behavior that is invisible in code review.
- **Deep merge edge cases** — merging maps with Viper's `MergeConfigMap` replaces leaf values but not entire subtrees. A developer who wants to completely replace a `stack_groups` entry (e.g. remove all `depends_on`) must explicitly set the key to empty in the local file rather than omitting it.
- **Onboarding friction** — new team members must create their own `.terrax.local.yaml` to match their local environment. This is not self-documenting unless the project's README or onboarding guide mentions it.

## Alternatives Considered

### Option 1: Environment variables for local overrides

**Description**: Support `TERRAX_STACK_GROUPS_PRIVATE_CONNECTION_ENV_TF_VAR_HOST=localhost` style env vars that override config values, following the Viper convention of `APPNAME_KEY=value`.

**Pros**:

- No file to manage.
- Env vars are already gitignored by nature.

**Cons**:

- Nested config keys produce extremely verbose env var names.
- Cannot express list or map values cleanly (e.g. `stack_groups.private_connection.env` is a `map[string]string`).
- No standard mechanism for persisting env vars across shell sessions without a `.envrc` or similar file — which is itself a file to gitignore.

**Why rejected**: The env var naming convention becomes unmanageable for nested keys like `stack_groups`. A file-based approach is cleaner for structured configuration.

### Option 2: `--local-config` CLI flag

**Description**: Add a `--local-config <path>` flag that points to an override file, defaulting to `.terrax.local.yaml`.

**Pros**:

- Explicit — users know when a local config is active.

**Cons**:

- Must be passed on every invocation or set in a shell alias.
- VS Code extension invocations (via `spawnSync`) would need to pass the flag, requiring extension changes.
- No standard convention for per-developer CLI flags in shared projects.

**Why rejected**: An automatic discovery mechanism (searching for `.terrax.local.yaml` alongside `.terrax.yaml`) is more ergonomic and consistent with the existing config search behavior.

### Option 3: Per-user config in home directory only

**Description**: Reserve the home directory `~/.terrax.yaml` for personal overrides and document that it takes priority over project-level config.

**Pros**:

- Already partially supported — Viper searches `~/.terrax.yaml`.
- No new file name to explain.

**Cons**:

- A single `~/.terrax.yaml` cannot contain per-project overrides without complex conditionals.
- All TerraX projects would share the same home-directory config, making it impossible to have project-specific local settings.

**Why rejected**: Developers work with multiple projects, each with different stack group configurations. A per-project `.terrax.local.yaml` (at the project root, gitignored) solves this more precisely than a single home-directory file.

## References

- `cmd/root.go` — `mergeLocalConfig`, `initConfig`, `ensureConfigFromWorkDir`
- `.gitignore` — `.terrax.local.yaml` entry
- `.terrax.yaml` — documentation comment describing the local override mechanism
- [ADR-0007: Configuration Management Strategy](0007-configuration-management-strategy.md)
