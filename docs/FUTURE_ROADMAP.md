# TerraX Future Roadmap & Enhancements

This document aggregates future enhancements and proposed improvements identified across the Architecture Decision Records (ADRs). It serves as a backlog of ideas to guide the evolution of TerraX.

## 🏆 Classification & Prioritization

The following enhancements have been extracted from the ADRs and classified by domain. They are prioritized based on estimated impact (value to user) versus effort.

### 🔴 Phase 1: High Priority (Core Usability & Standards)

These features address fundamental usability gaps or align TerraX with standard CLI expectations.

| Feature | Source ADR | Category | Description |
|:---|:---|:---|:---|
| **Refresh Command (`r`)** | [ADR-0005](adr/0005-filesystem-tree-building-strategy.md) | Core | Allow users to rebuild the stack tree without restarting the application to pick up filesystem changes. |
| **CLI Strings Override** | [ADR-0007](adr/0007-configuration-management-strategy.md) | Config | Ensure command-line flags (e.g., `--log-level`) consistently override configuration file values. |
| **Env Var Support** | [ADR-0007](adr/0007-configuration-management-strategy.md) | Config | Support `TERRAX_*` environment variables (e.g., `TERRAX_LOG_LEVEL`) to override config. |
| **Command Preview** | [ADR-0009](adr/0009-executor-isolation-pattern.md) | UX | Show the full Terragrunt command to the user before execution for safety and verification. |
| **Config Generation** | [ADR-0007](adr/0007-configuration-management-strategy.md) | DX | implement `terrax config init` to generate a default `.terrax.yaml` file for users. |
| **Dynamic Window Size** | [ADR-0003](adr/0003-sliding-window-navigation.md) | UX | Automatically adjust the number of visible columns based on the terminal width. |

### 🟡 Phase 2: Medium Priority (Advanced Features & Performance)

These features significantly enhance the power and performance of the tool but are not strictly required for basic usage.

| Feature | Source ADR | Category | Description |
|:---|:---|:---|:---|
| **Parallel Execution** | [ADR-0009](adr/0009-executor-isolation-pattern.md) | Performance | Support executing multiple stacks concurrently (e.g., `run-all` parallelization handling). |
| **Configurable Skip List** | [ADR-0005](adr/0005-filesystem-tree-building-strategy.md) | Config | Allow users to define custom directories to ignore during scanning via config. |
| **Dry-Run Mode** | [ADR-0009](adr/0009-executor-isolation-pattern.md) | Core | "What-if" mode to show what would run without actually executing commands. |
| **Diff Viewer Mode** | [ADR-0008](adr/0008-dual-mode-tui-architecture.md) | UX | A dedicated TUI mode for viewing Terraform plan diffs with syntax highlighting. |
| **Runtime Mode Switching** | [ADR-0008](adr/0008-dual-mode-tui-architecture.md) | Core | Allow switching between Navigation and History modes freely during a session. |
| **Progress Tracking** | [ADR-0009](adr/0009-executor-isolation-pattern.md) | UX | Visual progress indicators in the TUI during long-running command executions. |

### 🔵 Phase 3: Low Priority (Polish & Niche Use Cases)

Nice-to-have features that add polish or support specific, less common workflows.

| Feature | Source ADR | Category | Description |
|:---|:---|:---|:---|
| **Smooth Transitions** | [ADR-0003](adr/0003-sliding-window-navigation.md) | UX | Animate column sliding for a more premium feel. |
| **Config Profiles** | [ADR-0007](adr/0007-configuration-management-strategy.md) | Config | Support named profiles (e.g., `dev`, `prod`) with different configurations. |
| **Incremental Updates** | [ADR-0005](adr/0005-filesystem-tree-building-strategy.md) | Core | Use filesystem watchers (`fsnotify`) to auto-update the tree on changes. |
| **Tree Caching** | [ADR-0005](adr/0005-filesystem-tree-building-strategy.md) | Performance | Serialize the scanned tree to disk to speed up subsequent startups. |
| **Retry Logic** | [ADR-0009](adr/0009-executor-isolation-pattern.md) | Core | Automatic retry mechanisms for transient failures. |
| **Config Editor Mode** | [ADR-0008](adr/0008-dual-mode-tui-architecture.md) | UX | Interactive TUI for modifying configuration settings. |

---

## 📂 Detailed Enhancements List

### User Experience (UX)
- **Dynamic Window Size**: Auto-adjust columns to fill terminal width ([ADR-0003](adr/0003-sliding-window-navigation.md)).
- **Configurable Window Size**: Allow user to manually set visible column count ([ADR-0003](adr/0003-sliding-window-navigation.md)).
- **Smooth Transitions**: Animation for window movement ([ADR-0003](adr/0003-sliding-window-navigation.md)).
- **Diff Viewer**: Dedicated mode for reading plans ([ADR-0008](adr/0008-dual-mode-tui-architecture.md)).
- **Logs Viewer**: Stream logs in a scrollable TUI pane ([ADR-0008](adr/0008-dual-mode-tui-architecture.md)).
- **Mode Stack**: Support modal overlays (e.g., help, settings) ([ADR-0008](adr/0008-dual-mode-tui-architecture.md)).
- **Command Preview**: Show command string before execution ([ADR-0009](adr/0009-executor-isolation-pattern.md)).
- **Progress Indicator**: Improvements to visual feedback during execution ([ADR-0005](adr/0005-filesystem-tree-building-strategy.md), [ADR-0009](adr/0009-executor-isolation-pattern.md)).

### Configuration System
- **CLI Flags**: Runtime overrides for config values ([ADR-0007](adr/0007-configuration-management-strategy.md)).
- **Validation**: Use a library for robust config validation ([ADR-0007](adr/0007-configuration-management-strategy.md)).
- **Generation**: Init command to create config templates ([ADR-0007](adr/0007-configuration-management-strategy.md)).
- **Documentation**: Command to print config docs ([ADR-0007](adr/0007-configuration-management-strategy.md)).
- **Env Vars**: Support `TERRAX_` variables ([ADR-0007](adr/0007-configuration-management-strategy.md)).
- **Profiles**: Environment-specific configurations ([ADR-0007](adr/0007-configuration-management-strategy.md)).
- **Config Injection**: Refactor to inject config struct instead of global Viper use ([ADR-0009](adr/0009-executor-isolation-pattern.md)).

### Performance & Core
- **Configurable Skip List**: User-defined ignored directories ([ADR-0005](adr/0005-filesystem-tree-building-strategy.md)).
- **Parallel Scanning**: Concurrent directory reading ([ADR-0005](adr/0005-filesystem-tree-building-strategy.md)).
- **Tree Caching**: Disk cache for scanned trees ([ADR-0005](adr/0005-filesystem-tree-building-strategy.md)).
- **Parallel Execution**: Run multiple stacks at once ([ADR-0009](adr/0009-executor-isolation-pattern.md)).
- **Incremental Updates**: File watching for live updates ([ADR-0005](adr/0005-filesystem-tree-building-strategy.md)).
- **Partial Refresh**: Reload only sub-trees ([ADR-0005](adr/0005-filesystem-tree-building-strategy.md)).
- **Output Capture**: Capture command output for parsing ([ADR-0009](adr/0009-executor-isolation-pattern.md)).
