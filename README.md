<div align="center">

# 🌍 TerraX

**The interactive TUI executor for Terragrunt stacks**

[![Go Version](https://img.shields.io/badge/Go-1.25.5-00ADD8?logo=go)](https://go.dev/) [![Build Status](https://img.shields.io/badge/build-passing-brightgreen)](https://github.com/israoo/TerraX) [![Coverage](https://img.shields.io/badge/coverage-89.4%25-brightgreen)](https://github.com/israoo/TerraX) [![License](https://img.shields.io/badge/license-Apache%202.0-blue.svg)](LICENSE) [![PRs Welcome](https://img.shields.io/badge/PRs-welcome-brightgreen.svg)](CONTRIBUTING.md)

Navigate your infrastructure as code hierarchies with an elegant, keyboard-driven terminal interface. Select your stack and command, and **TerraX executes Terragrunt directly in your terminal** with full interactive control.

[Features](#-key-features) •
[Installation](#-installation) •
[Configuration](#-configuration) •
[Quick start](#-quick-start) •
[Documentation](#-documentation) •
[Contributing](#-contributing) •
[License](#-license)

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

### ✔︎ Keyboard-first design

Full keyboard navigation with arrow keys (`↑↓←→`), Enter for confirmation, and `q` to quit.

### ✔︎ Direct Terragrunt execution

Once you confirm your selection, TerraX executes `terragrunt` directly in your terminal with full stdout/stderr/stdin passthrough. You maintain complete control and can interact with prompts as if running Terragrunt manually.

---

## 📦 Installation

### From binary (recommended)

Download the latest release for your platform:

```bash
# macOS (Intel)
curl -Lo TerraX https://github.com/israoo/TerraX/releases/latest/download/TerraX-darwin-amd64
chmod +x TerraX
sudo mv TerraX /usr/local/bin/

# macOS (Apple Silicon)
curl -Lo TerraX https://github.com/israoo/TerraX/releases/latest/download/TerraX-darwin-arm64
chmod +x TerraX
sudo mv TerraX /usr/local/bin/

# Linux (AMD64)
curl -Lo TerraX https://github.com/israoo/TerraX/releases/latest/download/TerraX-linux-amd64
chmod +x TerraX
sudo mv TerraX /usr/local/bin/
```

### From source

Requires **Go 1.25.5** or higher:

```bash
# Install directly
go install github.com/israoo/TerraX@latest

# Or clone and build
git clone https://github.com/israoo/TerraX.git
cd TerraX
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

# Maximum number of navigation columns visible simultaneously
# Increase for deeper hierarchies or larger terminals
max_navigation_columns: 3
```

### Configuration options

| Option | Type | Default | Description |
|--------|------|---------|-------------|
| `max_navigation_columns` | integer | `3` | Maximum navigation columns visible in sliding window |
| `commands` | list | 8 commands | Terragrunt commands shown in TUI (in order) |

**Notes:**

- Commands appear in the TUI in the order specified
- `max_navigation_columns` must be at least 1 (falls back to 3 if invalid)
- Empty or missing `commands` key falls back to defaults
- Configuration is loaded once at startup

---

## 🚀 Quick start

### 1. Navigate to your infrastructure directory

```bash
cd /path/to/your/infrastructure
```

### 2. Launch TerraX

```bash
terrax
```

### 3. Navigate the TUI

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

### Development setup

```bash
# Clone repository
git clone https://github.com/israoo/TerraX.git
cd TerraX

# Initialize (install dependencies + verify setup)
make init

# Run tests
make test

# Run with coverage report
make test-coverage
```

### Tech stack

| Component | Technology | Version |
|-----------|------------|---------|
| **Language** | [Go](https://go.dev/) | 1.25.5 |
| **TUI Framework** | [Bubble Tea](https://github.com/charmbracelet/bubbletea) | 1.3.10 |
| **UI Components** | [Bubbles](https://github.com/charmbracelet/bubbles) | 0.21.0 |
| **Styling** | [Lipgloss](https://github.com/charmbracelet/lipgloss) | 1.1.0 |
| **CLI Framework** | [Cobra](https://github.com/spf13/cobra) | 1.10.2 |
| **Configuration** | [Viper](https://github.com/spf13/viper) | 1.21.0 |
| **Testing** | [Testify](https://github.com/stretchr/testify) | 1.11.1 |
| **Filesystem Mocking** | [Afero](https://github.com/spf13/afero) | 1.15.0 |

### Testing strategy

TerraX employs a multi-layered testing approach:

1. **Unit Tests** (`internal/stack/`):
   - Pure business logic, zero I/O dependencies
   - `afero.MemMapFs` for filesystem isolation
   - Table-driven test patterns

2. **TUI Tests** (`internal/tui/`):
   - Bubble Tea Model-Update-View validation
   - State transition testing
   - Layout calculation verification

3. **Integration Tests** (`cmd/`):
   - CLI coordination and output formatting
   - Error handling paths

### Code quality standards

- **Separation of Concerns**: Business logic isolated from UI
- **Interface-Driven Design**: Navigator operates on pure data structures
- **Table-Driven Tests**: Comprehensive scenario coverage
- **Defensive Programming**: Nil checks, error wrapping, bounds validation
- **Comment Style**: All comments end with periods
- **Import Organization**: stdlib → third-party → internal

### Contributing workflow

1. **Fork** the repository
2. **Create** a feature branch (`git checkout -b feature/amazing-feature`)
3. **Write** tests for new functionality
4. **Ensure** tests pass (`make test`)
5. **Format** code (`make fmt`)
6. **Commit** changes (`git commit -m 'Add amazing feature'`)
7. **Push** to branch (`git push origin feature/amazing-feature`)
8. **Open** a Pull Request

### Architecture documentation

For deep architectural guidance, see:

- **[CLAUDE.md](CLAUDE.md)**: Comprehensive architectural patterns and testing strategies
- **[.github/copilot-instructions.md](.github/copilot-instructions.md)**: Quick orientation for AI agents
- **[.claude/agents/](claude/agents/)**: Agent-specific governance and patterns

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
- **Terragrunt** community for infrastructure patterns
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

[Report bug](https://github.com/israoo/TerraX/issues) •
[Request feature](https://github.com/israoo/TerraX/issues)

</div>
