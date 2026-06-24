<div align="center">

# 🌍 TerraX

**The interactive TUI executor for Terragrunt stacks**

[![Go Version](https://img.shields.io/badge/Go-1.25.5-00ADD8?logo=go)](https://go.dev/) [![Build Status](https://img.shields.io/badge/build-passing-brightgreen)](https://github.com/israoo/terrax) [![codecov](https://codecov.io/github/israoo/terrax/graph/badge.svg?token=X28KESXI19)](https://codecov.io/github/israoo/terrax) [![Quality Gate Status](https://sonarcloud.io/api/project_badges/measure?project=israoo_terrax&metric=alert_status)](https://sonarcloud.io/summary/new_code?id=israoo_terrax) [![License](https://img.shields.io/badge/license-Apache%202.0-blue.svg)](LICENSE) [![PRs Welcome](https://img.shields.io/badge/PRs-welcome-brightgreen.svg)](CONTRIBUTING.md)

Navigate your infrastructure as code hierarchies with an elegant, keyboard-driven terminal interface. Select your stack and command, and **TerraX executes Terragrunt directly in your terminal** with full interactive control.

[Features](#-key-features) •
[Installation](#-installation) •
[Configuration](#-configuration) •
[Quick start](#-quick-start) •
[Documentation](#-documentation) •
[Contributing](#-contributing) •
[License](#-license)

---

<img src="docs/demo.gif" alt="Demo" />*<br/>Example of running TerraX to execute Terragrunt stacks.*

</div>

---

## 🎯 Key features

### ✔︎ Dynamic hierarchical navigation

Automatically detects and builds a complete tree structure from your infrastructure directories with intelligent depth detection.

### ✔︎ Universal stack detection

Identifies Terragrunt (`terragrunt.hcl`) stacks with extensible detection patterns.

### ✔︎ Sliding window navigation

Displays 3 navigation columns simultaneously (plus commands column) with dynamic sliding as you navigate deeper hierarchies.

### ✔︎ Smart column display

Columns appear/disappear dynamically—no empty columns, keeping the UI clean and focused.

### ✔︎ Dual execution modes

- Commands column: Execute at the full propagated depth (deepest selected level)
- Navigation column: Execute at specific levels only (ignore deeper selections)

### ✔︎ Interactive filtering

Filter items in any column with `/` key. Navigate through filtered results in real-time, with multiple filters active simultaneously across columns.

### ✔︎ Customizable configuration

Configure available commands and UI behavior via `.terrax.yaml` in your project or home directory. Customize command list and maximum visible columns.

### ✔︎ Execution history tracking

Complete audit trail of all command executions with persistent history. View, search, and re-execute previous commands with a single keypress.

### ✔︎ Quick command re-execution

Re-run the last executed command instantly with `--last` flag, browse full history interactively with `--history`, or reopen the plan review TUI without re-running with `--review`.

### ✔︎ Plan summary and interactive review

After running `plan`, TerraX can show a grouped terminal summary (no-changes vs pending-changes) and/or launch an interactive review TUI with resource-level diffs. Enable via `plan.summary_enabled` and `plan.review_enabled` in `.terrax.yaml`.

### ✔︎ Smart dependency resolution

TerraX pre-computes the exact set of stacks to run using static HCL analysis — no subprocess required. Transitive dependencies are discovered and passed as explicit `--filter` targets. Control with `include_dependencies: true/false`.

### ✔︎ Force unlock

Select `force-unlock` from the TUI to automatically discover and release a locked Terraform state. TerraX reads the lock ID from S3 via the AWS CLI — no manual copy-paste. Configure `state.bucket` and `state.project` in `.terrax.yaml`.

### ✔︎ Keyboard-first design

Full keyboard navigation with arrow keys (`↑↓←→`), Enter for confirmation, and `q` to quit.

### ✔︎ Direct Terragrunt execution

Once you confirm your selection, TerraX executes `terragrunt` directly in your terminal with full stdout/stderr/stdin passthrough. You maintain complete control and can interact with prompts as if running Terragrunt manually.

---

## 📦 Installation

Choose your preferred installation method:

### Homebrew (macOS/Linux)

```bash
brew tap israoo/tap
brew install --cask terrax
```

### Scoop (Windows)

```powershell
scoop bucket add israoo https://github.com/israoo/scoop-bucket
scoop install israoo/terrax
```

### APT (Debian/Ubuntu)

```bash
# Download the .deb package (replace VERSION with actual version, e.g., 0.1.0)
wget https://github.com/israoo/terrax/releases/download/vVERSION/terrax_VERSION_linux_x86_64.deb

# Install
sudo dpkg -i terrax_VERSION_linux_x86_64.deb
```

### YUM/DNF (Fedora/RHEL/CentOS)

```bash
# Download the .rpm package (replace VERSION with actual version, e.g., 0.1.0)
wget https://github.com/israoo/terrax/releases/download/vVERSION/terrax_VERSION_linux_x86_64.rpm

# Install with YUM
sudo yum install terrax_VERSION_linux_x86_64.rpm

# Or with DNF
sudo dnf install terrax_VERSION_linux_x86_64.rpm
```

### Direct binary download

Download the latest pre-built binary for your platform from the [releases page](https://github.com/israoo/terrax/releases/latest):

```bash
# macOS (Intel)
curl -LO https://github.com/israoo/terrax/releases/latest/download/terrax_Darwin_x86_64.tar.gz
tar -xzf terrax_Darwin_x86_64.tar.gz
chmod +x terrax
sudo mv terrax /usr/local/bin/

# macOS (Apple Silicon)
curl -LO https://github.com/israoo/terrax/releases/latest/download/terrax_Darwin_arm64.tar.gz
tar -xzf terrax_Darwin_arm64.tar.gz
chmod +x terrax
sudo mv terrax /usr/local/bin/

# Linux (AMD64)
curl -LO https://github.com/israoo/terrax/releases/latest/download/terrax_Linux_x86_64.tar.gz
tar -xzf terrax_Linux_x86_64.tar.gz
chmod +x terrax
sudo mv terrax /usr/local/bin/

# Linux (ARM64)
curl -LO https://github.com/israoo/terrax/releases/latest/download/terrax_Linux_arm64.tar.gz
tar -xzf terrax_Linux_arm64.tar.gz
chmod +x terrax
sudo mv terrax /usr/local/bin/

# Windows (download .zip from releases page and add to PATH)
```

### From source

Requires **Go 1.25.5** or higher:

```bash
# Install directly
go install github.com/israoo/terrax@latest

# Or clone and build
git clone https://github.com/israoo/terrax.git
cd terrax
make build
```

### Verify installation

```bash
terrax --version
```

---

## 🔧 Configuration

TerraX allows you to customize available commands via `.terrax.yaml` configuration file.

### Configuration file locations

TerraX searches for configuration in the following order (first found wins):

1. `.terrax.yaml` in current directory
2. `.terrax.yaml` in `$HOME` directory
3. Built-in defaults (if no config file found)

### Example configuration

Create `.terrax.yaml` with your preferred settings:

```yaml
# List of Terragrunt commands to display in the TUI
commands:
  - apply
  - plan
  - destroy
  - init
  - validate
  - output
  - refresh
  - fmt

max_navigation_columns: 3
root_config_file: "root.hcl"

# Whether to include transitive dependencies when computing the execution scope
include_dependencies: true

# History configuration
history:
  max_entries: 1000

# Plan analysis (requires plan in commands list)
plan:
  review_enabled: true    # Launch interactive plan review TUI after plan
  summary_enabled: false  # Print grouped terminal summary after plan

# Feature shortcuts — map to Terragrunt flags
features:
  tf_forward_stdout: false
  summary_per_unit: false
  report:
    enabled: false        # Writes .terrax/report.json

# State backend — required for force-unlock command
# state:
#   bucket: "my-terraform-state"
#   project: "my-project"
#   region: "us-east-1"
```

### Configuration options

| Option | Type | Default | Description |
|--------|------|---------|-------------|
| `max_navigation_columns` | integer | `3` | Maximum navigation columns visible in sliding window |
| `commands` | list | 8 commands | Terragrunt commands shown in TUI (in order) |
| `root_config_file` | string | `root.hcl` | Config file name used to detect project root |
| `include_dependencies` | bool | `true` | Resolve transitive deps via static HCL analysis |
| `history.max_entries` | integer | `500` | Maximum number of history entries to keep |
| `plan.review_enabled` | bool | `true` | Launch plan review TUI after running plan |
| `plan.summary_enabled` | bool | `false` | Print terminal summary after running plan |
| `state.bucket` | string | — | S3 bucket for Terraform state (force-unlock) |
| `state.project` | string | — | S3 key prefix for Terraform state (force-unlock) |

**Notes:**

- Commands appear in the TUI in the order specified
- `max_navigation_columns` must be at least 1 (falls back to 3 if invalid)
- Empty or missing `commands` key falls back to defaults
- Configuration is loaded once at startup
- History location follows XDG Base Directory spec:
  - Linux/BSD: `~/.config/terrax/history.log`
  - macOS: `~/Library/Application Support/terrax/history.log`
  - Windows: `%LOCALAPPDATA%\terrax\history.log`

---

## 🚀 Quick start

### Basic usage

```bash
# Interactive mode: Navigate and select stacks/commands
terrax

# View execution history for current project
terrax --history

# Re-execute the last command from history
terrax --last       # or -l

# Execute a command directly without opening the TUI
terrax run plan --dir ./path/to/stack

# Output stack tree with dependency graph as JSON (used by VS Code extension)
terrax tree --json --dir .

# Output execution history as JSON (used by VS Code extension)
terrax history --dir .

# Display version information
terrax --version
```

### Try with examples

TerraX includes ready-to-use examples that work without AWS credentials:

```bash
# Clone the repository
git clone https://github.com/israoo/terrax.git
cd terrax

# Build TerraX
make build

# Navigate manually
cd examples/terragrunt
../../build/terrax
```

See [`examples/terragrunt/README.md`](examples/terragrunt/README.md) for detailed testing instructions.

### Use with your infrastructure

#### 1. Navigate to your infrastructure directory

```bash
cd /path/to/your/infrastructure
```

#### 2. Launch TerraX

```bash
terrax
```

#### 3. Navigate the TUI

```text
═══════════════════════════════════════════════════════════════
  🌍 TerraX - Terragrunt eXecutor
═══════════════════════════════════════════════════════════════
📁 /infrastructure/vpc
 ┌─ Commands ─┐  ┌─ Level 1 ────────┐  ╭─ Level 2 ──────╮
 │            │  │                  │  │                │
 │ ► plan     │  │ ► infrastructure │  │ ► vpc 📦       │
 │   apply    │  │   applications   │  │   database 📦  │
 │   destroy  │  │   monitoring     │  │   compute 📦   │
 │   validate │  │                  │  │                │
 │   init     │  │                  │  │                │
 └────────────┘  └──────────────────┘  ╰────────────────╯
↑↓: navigate | ←→: change column | /: filter | enter: confirm | q: quit
```

**Keyboard controls:**

- `↑↓`: Navigate up/down in current column (works while filtering)
- `←→`: Switch between columns (wraps around)
- `/`: Activate filter for current column
- `Esc`: Clear filter and return to title view
- `Enter`: Confirm selection and execute Terragrunt command
- `q` or `Ctrl+C`: Quit without executing

### History viewer

View and manage your execution history:

```bash
terrax --history
```

**History table view:**

```text
═══════════════════════════════════════════════════════════════════════════════════════
                                 📜 Execution History
═══════════════════════════════════════════════════════════════════════════════════════
  #   Timestamp            Command  Stack Path                  Exit Code    Duration
───────────────────────────────────────────────────────────────────────────────────────
▶ 1   2025-12-16 15:30:45  plan     dev/us-east-1/vpc            ✓ 0         12.34s
  2   2025-12-16 14:22:10  apply    dev/us-east-1/database       ✓ 0         45.67s
  3   2025-12-16 13:15:30  destroy  qa/us-west-2/compute         ✗ 1         8.90s

Showing 1-3 of 12 entries | Use ↑/↓ to navigate | Press Enter to re-execute | Press 'q' or 'esc' to exit
```

**History keyboard controls:**

- `↑↓`: Navigate through history entries
- `Enter`: Re-execute selected command at its original path
- `q` or `Esc`: Exit history viewer

**History features:**

- **Project-aware filtering**: Only shows commands from current project
- **Smart path display**: Shows relative paths from project root, truncates with "..." for long paths
- **Coded status**: ✓ for success, ✗ for failures
- **Re-execution**: Press Enter to re-run any command at its original location

### Quick re-execution

Re-run the most recent command instantly:

```bash
terrax --last  # or -l
```

This executes the last command from your project's history without opening the TUI.

---

## 🧩 VS Code Extension

TerraX includes a companion VS Code extension that exposes the CLI as a visual interface directly in your editor.

### Install

```bash
# Build and install the extension
task ext:package
code --install-extension extensions/vscode/terrax-vscode-0.1.0.vsix
```

### Configure

Set the binary path in VS Code settings (`terrax.binaryPath`) if `terrax` is not on your PATH:

```json
{ "terrax.binaryPath": "/usr/local/bin/terrax" }
```

### Panels

| Panel | Description |
|---|---|
| **Stacks** | Directory hierarchy — click to open TUI, `$(play)` button to run plan directly |
| **Dependencies** | What the selected stack depends on (transitive) |
| **Dependents** | What depends on the selected stack (impact analysis) |
| **History** | Recent executions — click to open TUI, `$(run)` button to re-execute |

Stacks in a dependency cycle are shown with a `$(warning)` icon.

---

## 📚 Documentation

### How it works

#### 1. **Pre-scan analysis**

Before launching the TUI, TerraX:

- Scans your directory recursively
- Builds a complete tree structure (`internal/stack/tree.go`)
- Detects stacks by looking for `terragrunt.hcl`
- Calculates maximum hierarchy depth
- Skips common non-stack directories (`.git`, `.terraform`, `vendor`, etc.)

#### 2. **Dynamic navigation**

The Navigator (`internal/stack/navigator.go`) handles:

- **Selection Propagation**: Walks the tree following your selections
- **Column Population**: Fills navigation columns dynamically
- **Depth Calculation**: Determines visible columns
- **Movement Logic**: Bounds-checked navigation with wrapping

#### 3. **Sliding window display**

TerraX displays **3 navigation columns** simultaneously by default (configurable via `max_navigation_columns`):

- As you navigate deeper, earlier columns slide out of view
- The focused column is always visible
- Navigation offset tracks the window position
- **No empty columns** are ever shown

#### 4. **Interactive filtering**

The filtering system (`internal/tui/model.go`, `internal/tui/view.go`):

- **Per-column filters**: Each column maintains its own independent filter
- **Visual replacement**: Filter input replaces column title when active
- **Real-time filtering**: Items filtered as you type (case-insensitive)
- **Navigation aware**: Arrow keys navigate only through filtered results
- **Auto-reactivation**: Returning to a filtered column automatically resumes editing
- **Persistent filters**: Multiple filters can be active simultaneously

#### 5. **Dual execution modes**

**Commands column (full depth execution):**

```text
Selected: Commands > infrastructure > network > subnets
Executes: terragrunt run --all --working-dir /infrastructure/network/subnets -- plan
```

**Navigation column (specific level execution):**

```text
Selected: Level 1 "infrastructure"
Executes: terragrunt run --all --working-dir /infrastructure -- plan
(ignores deeper levels)

Selected: Level 2 "network"
Executes: terragrunt run --all --working-dir /infrastructure/network -- plan
(ignores subnets selection)
```

#### 6. **Execution history system**

TerraX maintains a complete audit trail of all command executions (`internal/history/history.go`):

- **Persistent storage**: All executions logged in JSONL format
- **Dual-path tracking**: Records both absolute paths (for execution) and relative paths (for display)
- **Project filtering**: Automatically filters history by detecting project root via `root_config_file`
- **Rich metadata**: Captures timestamp, user, command, paths, exit code, duration, and summary
- **Automatic trimming**: Maintains configurable max entries (`history.max_entries`)

**History data structure:**

```json
{
  "id": 123,
  "timestamp": "2025-12-16T15:30:45Z",
  "user": "developer",
  "stack_path": "dev/us-east-1/vpc",
  "absolute_path": "/home/user/infra/dev/us-east-1/vpc",
  "command": "plan",
  "exit_code": 0,
  "duration_s": 12.34,
  "summary": "Command completed successfully"
}
```

### Example directory structure

```text
my-infrastructure/
├── infrastructure/
│   ├── vpc/
│   │   └── terragrunt.hcl    # ← Stack (📦)
│   ├── database/
│   │   └── terragrunt.hcl    # ← Stack
│   └── compute/
│       └── terragrunt.hcl    # ← Stack
├── applications/
│   ├── frontend/
│   │   └── terragrunt.hcl    # ← Stack
│   └── backend/
│       └── terragrunt.hcl    # ← Stack
└── monitoring/
    └── prometheus/
        └── terragrunt.hcl    # ← Stack
```

**TerraX navigation result:**

- **Max depth**: 2
- **Level 1**: infrastructure, applications, monitoring
- **Level 2**: Subdirectories under selected Level 1
- **Stacks**: 6 detected (marked with 📦)

### Architecture overview

```text
TerraX/
├── cmd/
│   └── root.go           # CLI coordination (no business logic)
├── internal/
│   ├── config/
│   │   └── defaults.go   # Configuration defaults
│   ├── history/
│   │   ├── history.go    # Execution history tracking & persistence
│   │   └── history_test.go
│   ├── stack/
│   │   ├── tree.go       # Filesystem scanning & tree building
│   │   └── navigator.go  # Navigation business logic (zero UI deps)
│   └── tui/
│       ├── model.go      # Bubble Tea Model-Update-View + filtering logic
│       ├── view.go       # Rendering (LayoutCalculator + Renderer)
│       └── constants.go  # UI configuration
├── main.go               # Entry point
└── Makefile              # Build automation
```

**Design principles:**

- **Separation of Concerns**: Business logic separated from UI
- **Interface-Driven**: Navigator operates on pure data structures
- **Testability**: `afero` for filesystem mocking, `teatest` for TUI testing
- **Error Handling**: Defensive programming with wrapped errors

---

## 🔨 Contributing

We welcome contributions! TerraX follows strict architectural principles and comprehensive testing standards.

We welcome contributions! Please see our [Contributing Guidelines](CONTRIBUTING.md) for details on:

- Setting up your development environment
- Running tests (`make test`)
- Code standards and architectural patterns
- Submitting Pull Requests

For a deep dive into the architecture, see [CLAUDE.md](CLAUDE.md) and the [docs/](docs/) directory.


---

## 📄 License

<a href="https://opensource.org/licenses/Apache-2.0"><img src="https://img.shields.io/badge/License-Apache%202.0-blue.svg?style=for-the-badge" alt="License"></a>

<details>
<summary>Preamble to the Apache License, Version 2.0</summary>
<br/>
<br/>

Complete license is available in the [`LICENSE`](LICENSE) file.

```text
Licensed to the Apache Software Foundation (ASF) under one
or more contributor license agreements.  See the NOTICE file
distributed with this work for additional information
regarding copyright ownership.  The ASF licenses this file
to you under the Apache License, Version 2.0 (the
"License"); you may not use this file except in compliance
with the License.  You may obtain a copy of the License at

  https://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing,
software distributed under the License is distributed on an
"AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
KIND, either express or implied.  See the License for the
specific language governing permissions and limitations
under the License.
```
</details>

---

## 🙏 Acknowledgments

- **[Charm Bracelet](https://charm.sh/)** for exceptional TUI tools (Bubble Tea, Lipgloss)
- **[Atmos](https://atmos.tools/)** for the visual representation that inspired TerraX's hierarchical navigation design
- **[Terragrunt](https://terragrunt.gruntwork.io/)** community for infrastructure patterns
- All contributors who help improve TerraX

---

## ⚠️ How TerraX executes commands

**TerraX executes Terragrunt directly in your terminal.**

After you confirm your selection (pressing Enter), TerraX:

1. **Shows your selection** for review:

   ```text
   ═══════════════════════════════════════
     ✅ Selection confirmed
   ═══════════════════════════════════════
   Command:    plan
   Stack Path: /infrastructure/vpc
   ═══════════════════════════════════════
   ```

2. **Executes Terragrunt** with full passthrough:

   ```bash
   🚀 Executing: terragrunt run --all --working-dir /infrastructure/vpc -- plan
   ```

3. **Maintains full control**: All stdout, stderr, and stdin are connected to your terminal

   - You see all Terragrunt output in real-time
   - You can respond to interactive prompts (like approval requests)
   - You can interrupt with `Ctrl+C` if needed

**This means:**

- ✅ Same experience as running `terragrunt` manually
- ✅ Full interactive control during execution
- ✅ All output visible in your terminal
- ⚠️ Commands execute immediately after confirmation—review your selection carefully

---

<div align="center">

**Built with ❤️ for Infrastructure Engineers**

[Report bug](https://github.com/israoo/terrax/issues) •
[Request feature](https://github.com/israoo/terrax/issues)

</div>
