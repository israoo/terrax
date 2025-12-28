# ADR-0006: Configuration Management Strategy with Defaults-First Approach

**Status**: Accepted

**Date**: 2025-12-28

**Deciders**: TerraX Core Team

**Related**: [ADR-0004: Separation of Concerns](0004-separation-of-concerns.md)

## Context

TerraX needs a flexible configuration system that:

1. **Provides sensible defaults**: Works out-of-the-box without configuration.
2. **Allows customization**: Users can override defaults for their workflow.
3. **Supports multiple sources**: Config file, environment variables, command-line flags.
4. **Is discoverable**: Users can find and understand available options.
5. **Is maintainable**: Changing defaults doesn't require code changes across multiple files.
6. **Is cross-platform**: Works consistently on Linux, macOS, Windows.

### Problem

Without structured configuration management:
- Defaults scattered across codebase (magic numbers, hardcoded strings).
- No central place to see all available options.
- Difficult to override behavior without code changes.
- Inconsistent configuration across different parts of the application.
- No clear hierarchy when multiple config sources conflict.

### Requirements

- Single source of truth for default values.
- Support YAML configuration files for user customization.
- Hierarchical config resolution (defaults → file → flags).
- Type-safe configuration access in code.
- Fail gracefully if config file missing (use defaults).
- Validate configuration values where appropriate.
- Document available configuration options.

## Decision

Implement configuration management using:

1. **Defaults-First Approach**: All defaults centralized in `internal/config/defaults.go`.
2. **Viper Library**: For config file parsing and hierarchical resolution.
3. **YAML Format**: Human-readable config files (`.terrax.yaml`).
4. **Cascading File Discovery**: Check current directory, then home directory.
5. **Graceful Degradation**: Missing config file is not an error.

### Architecture

```
┌─────────────────────────────────────────────────────────────┐
│                  Configuration Hierarchy                     │
│                   (lowest to highest priority)               │
└─────────────────────────────────────────────────────────────┘
                          ↓
    ┌─────────────────────────────────────────────┐
    │  1. Hardcoded Defaults (config/defaults.go) │
    │     - Lowest priority                        │
    │     - Compiled into binary                   │
    │     - Always available                       │
    └─────────────────────────────────────────────┘
                          ↓
    ┌─────────────────────────────────────────────┐
    │  2. Home Directory Config (~/.terrax.yaml)  │
    │     - User-level preferences                 │
    │     - Applies to all projects                │
    └─────────────────────────────────────────────┘
                          ↓
    ┌─────────────────────────────────────────────┐
    │  3. Current Directory Config (.terrax.yaml) │
    │     - Project-specific overrides             │
    │     - Highest priority                       │
    └─────────────────────────────────────────────┘
                          ↓
    ┌─────────────────────────────────────────────┐
    │  4. Command-Line Flags (future expansion)   │
    │     - Runtime overrides                      │
    │     - Highest priority (when implemented)    │
    └─────────────────────────────────────────────┘
```

### Centralized Defaults

All default values defined as Go constants:

```go
// internal/config/defaults.go
package config

const (
    // UI Configuration
    DefaultMaxNavigationColumns = 10

    // History Configuration
    DefaultMaxHistoryEntries = 500
    DefaultRootConfigFile    = "root.hcl"

    // Logging Configuration
    DefaultLogLevel        = "info"
    DefaultLogFormat       = "pretty"
    DefaultLogCustomFormat = ""

    // Terragrunt Configuration
    DefaultTerragruntParallelism     = 0   // 0 = unlimited
    DefaultTerragruntNoColor         = false
    DefaultTerragruntNonInteractive  = true
    DefaultTerragruntIgnoreDependencyErrors      = false
    DefaultTerragruntIgnoreExternalDependencies  = false
    DefaultTerragruntIncludeExternalDependencies = false
    DefaultTerragruntExtraFlags                  = ""
)

// Commands that appear in TUI
var DefaultCommands = []string{
    "plan",
    "apply",
    "destroy",
    "output",
    "validate",
    "init",
}
```

**Benefits**:
- **Single source of truth**: All defaults in one file.
- **Type-safe**: Constants prevent typos at compile time.
- **Discoverable**: Developers know where to find defaults.
- **Documentable**: Easy to generate config documentation from constants.
- **Testable**: Can reference constants in tests.

### Viper Integration

Initialize Viper with defaults before reading config files:

```go
// cmd/root.go
import "github.com/spf13/viper"

func init() {
    // Set all defaults FIRST (before ReadInConfig)
    viper.SetDefault("commands", config.DefaultCommands)
    viper.SetDefault("max_navigation_columns", config.DefaultMaxNavigationColumns)
    viper.SetDefault("history.max_entries", config.DefaultMaxHistoryEntries)
    viper.SetDefault("history.root_config_file", config.DefaultRootConfigFile)
    viper.SetDefault("log_level", config.DefaultLogLevel)
    viper.SetDefault("log_format", config.DefaultLogFormat)
    viper.SetDefault("terragrunt.parallelism", config.DefaultTerragruntParallelism)
    viper.SetDefault("terragrunt.no_color", config.DefaultTerragruntNoColor)
    // ... more defaults

    // Config file discovery
    viper.SetConfigName(".terrax")
    viper.SetConfigType("yaml")
    viper.AddConfigPath(".")           // Current directory
    viper.AddConfigPath("$HOME")       // Home directory

    // Read config (gracefully handle missing file)
    if err := viper.ReadInConfig(); err != nil {
        if _, ok := err.(viper.ConfigFileNotFoundError); ok {
            // No config file: use defaults (this is OK)
        } else {
            // Config file found but has errors: warn user
            fmt.Fprintf(os.Stderr, "Warning: config file error: %v\n", err)
        }
    }
}
```

**Benefits**:
- **Defaults always set**: Even if no config file exists.
- **Hierarchical resolution**: Viper merges config file values over defaults.
- **Silent missing files**: No error if user doesn't create `.terrax.yaml`.
- **Multiple search paths**: Checks current dir then home dir.

### YAML Configuration Format

User-facing config file (`.terrax.yaml`):

```yaml
# TerraX Configuration

# Commands shown in TUI (in order)
commands:
  - plan
  - apply
  - destroy
  - output
  - validate
  - init
  - refresh

# Maximum navigation columns (depth levels) to show
max_navigation_columns: 10

# History settings
history:
  max_entries: 500           # Maximum history entries before trimming
  root_config_file: root.hcl # Filename to detect project root

# Logging configuration
log_level: info                      # debug, info, warn, error
log_format: pretty                   # pretty, bare, key-value, json
log_custom_format: ""                # Custom log format template

# Terragrunt-specific settings
terragrunt:
  parallelism: 10                    # Max parallel executions (0 = unlimited)
  no_color: false                    # Disable colored output
  non_interactive: true              # Run in non-interactive mode
  ignore_dependency_errors: false    # Continue on dependency errors
  ignore_external_dependencies: false
  include_external_dependencies: false
  extra_flags: "--terragrunt-log-level debug"  # Additional flags
```

**Benefits**:
- **Human-readable**: YAML is familiar to infrastructure engineers.
- **Self-documenting**: Comments explain each option.
- **Structured**: Nested configuration (e.g., `history.max_entries`).
- **Version-controllable**: Projects can commit `.terrax.yaml` for team consistency.

### Configuration Categories

**UI Configuration**:
- `commands`: List of Terragrunt commands shown in TUI.
- `max_navigation_columns`: Depth of tree navigation.

**History Configuration**:
- `history.max_entries`: How many history entries to keep.
- `history.root_config_file`: Filename for project root detection.

**Logging Configuration**:
- `log_level`: Verbosity (debug, info, warn, error).
- `log_format`: Output format (pretty, bare, key-value, json).
- `log_custom_format`: Custom format template for key-value/json formats.

**Terragrunt Configuration**:
- `terragrunt.parallelism`: Max parallel module executions.
- `terragrunt.no_color`: Disable colored output.
- `terragrunt.non_interactive`: Prevent interactive prompts.
- `terragrunt.ignore_dependency_errors`: Continue despite dependency failures.
- `terragrunt.ignore_external_dependencies`: Ignore external dependencies.
- `terragrunt.include_external_dependencies`: Include external dependencies.
- `terragrunt.extra_flags`: Additional terragrunt flags.

### Access Pattern

Retrieve configuration values with type-safe getters:

```go
// Retrieve string value
logLevel := viper.GetString("log_level")

// Retrieve int value
maxColumns := viper.GetInt("max_navigation_columns")

// Retrieve bool value
noColor := viper.GetBool("terragrunt.no_color")

// Retrieve string slice
commands := viper.GetStringSlice("commands")
```

**Benefits**:
- **Type-safe**: Viper handles type conversion.
- **Default fallback**: Returns default if key not found.
- **Dot notation**: Access nested values with `section.key`.

### Validation

Some values require validation before use:

```go
// cmd/root.go
maxNavigationColumns := viper.GetInt("max_navigation_columns")

// Validate: must be at least 3
if maxNavigationColumns < minMaxNavigationColumns {
    fmt.Fprintf(os.Stderr,
        "Warning: max_navigation_columns (%d) is less than minimum (%d), using minimum\n",
        maxNavigationColumns, minMaxNavigationColumns)
    maxNavigationColumns = minMaxNavigationColumns
}
```

**Validation Strategy**:
- Validate at initialization time (cmd/root.go).
- Provide clear warning messages.
- Fall back to safe values (don't crash).
- Document constraints in config file comments.

### Cascading File Discovery

Viper searches for config files in order:

1. **Current directory**: `./.terrax.yaml`
2. **Home directory**: `~/.terrax.yaml`

**First file found wins**. If no files found, use defaults.

**Use Cases**:
- **Project-specific config**: Place `.terrax.yaml` in project root.
- **User-level config**: Place `.terrax.yaml` in home directory.
- **No config**: Works with defaults out-of-the-box.

**Example Workflow**:

```bash
# User's global config
$ cat ~/.terrax.yaml
log_level: debug
terragrunt:
  parallelism: 5

# Project-specific config
$ cd ~/infra-project
$ cat .terrax.yaml
terragrunt:
  parallelism: 10        # Overrides user's parallelism=5
  no_color: true         # Project-specific setting

# Result: log_level=debug (from ~/.terrax.yaml)
#         terragrunt.parallelism=10 (from ./.terrax.yaml)
#         terragrunt.no_color=true (from ./.terrax.yaml)
```

### Graceful Degradation

Missing config file is **not an error**:

```go
if err := viper.ReadInConfig(); err != nil {
    if _, ok := err.(viper.ConfigFileNotFoundError); ok {
        // Silently use defaults
        return
    } else {
        // Config exists but has syntax errors: warn user
        fmt.Fprintf(os.Stderr, "Warning: config error: %v\n", err)
    }
}
```

**Philosophy**: TerraX should work perfectly fine without any configuration file.

## Consequences

### Positive

- **Works out-of-the-box**: Sensible defaults mean no config required.
- **Customizable**: Users can override any setting via YAML file.
- **Discoverable**: Single file (`config/defaults.go`) lists all options.
- **Maintainable**: Changing defaults doesn't require hunting through code.
- **Hierarchical**: User-level and project-level configs supported.
- **Type-safe**: Constants and Viper getters prevent errors.
- **Cross-platform**: YAML and Viper work everywhere.
- **Graceful**: Missing config file doesn't cause errors.
- **Documented**: Example YAML serves as documentation.

### Negative

- **Two sources of truth**: Constants in `defaults.go` and Viper `SetDefault()` calls must match.
- **No validation framework**: Validation code scattered (could improve with validation library).
- **Limited discoverability**: Users might not know `.terrax.yaml` exists unless documented.
- **YAML pitfalls**: YAML has quirks (indentation, type coercion) that can confuse users.

### Neutral

- **Viper dependency**: Adds external library (but well-maintained and popular).
- **YAML format**: Familiar to target audience, but not universally loved.
- **Manual sync**: Must manually keep defaults.go and Viper SetDefault calls in sync.

## Alternatives Considered

### Alternative 1: Environment Variables Only

Use environment variables for all configuration (e.g., `TERRAX_LOG_LEVEL=debug`).

**Pros**:
- No config file to manage.
- Standard Unix convention.
- Works well in containerized environments.

**Cons**:
- Verbose for complex configuration (many env vars).
- Hard to version control.
- Difficult to document and discover.
- Not human-friendly for nested configuration.

**Decision**: YAML files are more user-friendly for infrastructure tooling.

### Alternative 2: TOML Format

Use TOML instead of YAML for config files.

**Pros**:
- Simpler syntax than YAML (less ambiguity).
- Explicit types.
- Better for configuration (YAML is better for data).

**Cons**:
- Less familiar to infrastructure engineers.
- Terraform/Terragrunt ecosystem uses HCL and YAML primarily.
- Minimal benefit over YAML for simple config.

**Decision**: YAML aligns with ecosystem conventions.

### Alternative 3: HCL Format (like Terraform)

Use HCL (HashiCorp Configuration Language) for config.

**Pros**:
- Matches Terraform/Terragrunt format.
- Powerful (supports expressions, functions).

**Cons**:
- Overkill for simple key-value config.
- Additional parsing complexity.
- Most users don't need programmable config.

**Decision**: YAML is sufficient and simpler.

### Alternative 4: Flags-Only (No Config File)

Only support command-line flags for configuration.

**Pros**:
- Explicit: all config visible in command.
- No hidden state in config files.

**Cons**:
- Extremely verbose commands.
- Can't save preferences.
- Poor UX for many options.

**Decision**: Config files dramatically improve UX.

### Alternative 5: Distributed Defaults

Define defaults inline where used (e.g., function parameters).

```go
func BuildTree(rootPath string, maxDepth int = 10) { ... }
```

**Pros**:
- Defaults co-located with usage.
- Self-documenting.

**Cons**:
- Defaults scattered across codebase.
- No single source of truth.
- Difficult to override.
- Hard to document all available options.

**Decision**: Centralized defaults are more maintainable.

### Alternative 6: Multiple Config Files

Separate config files for each concern (`.terrax-ui.yaml`, `.terrax-history.yaml`, etc.).

**Pros**:
- Separation of concerns.
- Can enable/disable features independently.

**Cons**:
- Confusing for users (which file to edit?).
- More files to manage.
- Overkill for current complexity.

**Decision**: Single config file is simpler.

## Future Enhancements

**Potential Improvements**:
1. **Command-line flags**: Override config file values at runtime.
2. **Config validation library**: Use something like `go-playground/validator`.
3. **Config generation**: `terrax config init` to create template config file.
4. **Config documentation**: `terrax config docs` to show all options.
5. **Environment variable support**: `TERRAX_LOG_LEVEL` overrides config file.
6. **Config profiles**: Different configs for different environments.

## References

- **Viper Documentation**: https://github.com/spf13/viper
- **YAML Specification**: https://yaml.org/spec/
- **12-Factor App Config**: https://12factor.net/config
- **XDG Base Directory**: https://specifications.freedesktop.org/basedir-spec/basedir-spec-latest.html
