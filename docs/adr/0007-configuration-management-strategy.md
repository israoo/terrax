# ADR-0007: Configuration Management Strategy

**Status**: Accepted

**Date**: 2025-12-28

**Deciders**: TerraX Core Team

**Related**: [ADR-0004: Separation of Concerns](0004-separation-of-concerns.md)

## Context

TerraX requires configuration for various behaviors (logging levels, specific Terragrunt flags, UI preferences). Users need sensible out-of-the-box defaults but must be able to override them at both a global level (user preferences) and a project level (repo-specific settings).

## Decision

We will adopt a **tiered configuration strategy** using the **Viper** library, prioritizing sources in the following order (highest to lowest):

1.  **Command-Line Flags** (Runtime overrides)
2.  **Project Configuration** (`.terrax.yaml` in current directory)
3.  **User Configuration** (`~/.terrax.yaml` in home directory)
4.  **Application Defaults** (Hardcoded in Go)

We will use **YAML** as the configuration format due to its ubiquity in the infrastructure ecosystem.

## Consequences

### Positive
*   **Flexibility**: Users can set global defaults (e.g., log level) while enforcing project-specific rules (e.g., specific Terragrunt flags).
*   **Zero Config**: The application runs without any config file by falling back to sensible hardcoded defaults.
*   **Standardization**: Viper is a standard, battle-tested library in the Go ecosystem, reducing maintenance burden.

### Negative
*   **External Dependency**: Adds a dependency on `spf13/viper`, which is not lightweight, but the trade-off is worth the functionality provided.
*   **Stringly Typed Keys**: Viper keys are strings, which can lead to typos. We mitigate this by defining constant keys in a centralized `defaults` package where possible.

## Alternatives Considered

### Option 1: Environment Variables Only

**Description**: Rely exclusively on environment variables (e.g., `TERRAX_LOG_LEVEL`) for all configuration.

**Pros**:

- Standard 12-factor app approach.
- No configuration files to manage or git-ignore.

**Cons**:

- Becomes extremely verbose for complex configurations (e.g., lists of flags).
- Difficult to share consistent project-level settings across a team.

**Why rejected**: Config files provide a better user experience for sharing and documenting complex settings.

### Option 2: TOML Format

**Description**: Use TOML (Tom's Obvious, Minimal Language) instead of YAML for configuration files.

**Pros**:

- Unambiguous syntax (avoids YAML's whitespace pitfalls).
- Strongly typed.

**Cons**:

- Less familiar to the target audience of infrastructure engineers (who live in YAML/HCL).
- Ecosystem fragmentation (Terraform uses HCL, Kubernetes uses YAML).

**Why rejected**: YAML is the de facto standard for the surrounding ecosystem (Kubernetes, Ansible, GitHub Actions).

### Option 3: HCL (HashiCorp Configuration Language)

**Description**: Use HCL to match the syntax of Terraform itself.

**Pros**:

- Consistent syntax with the files being managed (`.tf`, `.hcl`).
- Powerful features like expression evaluation.

**Cons**:

- Parsing HCL is "heavier" and more complex than parsing YAML.
- Overkill for simple key-value configuration.

**Why rejected**: YAML is sufficient, ubiquitous, and simpler to parse for this use case.

## Future Enhancements

**Potential Improvements**:
1.  **Config Generation**: A `terrax config init` command to generate a documented default config file.
2.  **Schema Validation**: Use JSON Schema or a Go validation library to ensure config values are within expected ranges (e.g., `parallelism > 0`).
3.  **Hot Reload**: Watch config files for changes and re-apply settings without restart.

## References

- **Viper Documentation**: https://github.com/spf13/viper
- **YAML Specification**: https://yaml.org/spec/
- **12-Factor App Config**: https://12factor.net/config
