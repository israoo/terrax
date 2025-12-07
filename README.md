<div align="center">

# 🌍 TerraX

**The Interactive TUI Executor for Terragrunt Stacks**

[![Go Version](https://img.shields.io/badge/Go-1.25.5-00ADD8?logo=go)](https://go.dev/) [![Build Status](https://img.shields.io/badge/build-passing-brightgreen)](https://github.com/israoo/TerraX) [![License](https://img.shields.io/badge/license-Apache%202.0-blue.svg)](LICENSE) [![PRs Welcome](https://img.shields.io/badge/PRs-welcome-brightgreen.svg)](CONTRIBUTING.md)

Navigate your infrastructure as code hierarchies with an elegant, keyboard-driven terminal interface. **TerraX doesn't execute commands**—it outputs your selection for safe, deliberate execution.

[Features](#-key-features) •
[Installation](#-installation) •
[Quick start](#-quick-start) •
[Documentation](#-documentation) •
[Contributing](#-contributing) •
[License](#-license) •
[Acknowledgments](#-acknowledgments) •

</div>

---

## 🎯 Key features

### Dynamic hierarchical navigation

Automatically detects and builds a complete tree structure from your infrastructure directories with intelligent depth detection.

### Universal stack detection

Identifies Terragrunt (`terragrunt.hcl`) stacks with extensible detection patterns.

### Sliding window navigation

Displays max 3 navigation columns simultaneously (plus commands column) with dynamic sliding as you navigate deeper hierarchies.

### Smart column display

Columns appear/disappear dynamically—no empty columns, keeping the UI clean and focused.

### Dual execution modes

- Commands column: Execute at the full propagated depth (deepest selected level)
- Navigation column: Execute at specific levels only (ignore deeper selections)

### Keyboard-first design

Full keyboard navigation with arrow keys (`↑↓←→`) and Vim-style bindings (`hjkl`), plus Enter for confirmation and `q` to quit.

### Selection-only philosophy

TerraX outputs the selected command and path—it never executes infrastructure changes, ensuring safe, deliberate workflows.

### Professional architecture

Built with strict Separation of Concerns:

- **Business Logic** (`internal/stack/`): Pure Go, zero UI dependencies
- **TUI Layer** (`internal/tui/`): Bubble Tea Model-Update-View pattern
- **CLI Wrapper** (`cmd/`): Minimal coordination layer

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
  🌍 TerraX - Terra eXecutor
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
↑↓/jk: navigate | ←→/hl: change column | enter: confirm | q: quit
```

**Keyboard controls:**

- `↑↓` or `k/j`: Navigate up/down in current column
- `←→` or `h/l`: Switch between columns (wraps around)
- `Enter`: Confirm selection and output path
- `q` or `Ctrl+C`: Quit

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

TerraX displays **max 3 navigation columns** simultaneously:

- As you navigate deeper, earlier columns slide out of view
- The focused column is always visible
- Navigation offset tracks the window position
- **No empty columns** are ever shown

#### 4. **Dual execution modes**

**Commands column (full depth):**

```text
Selected: Commands > infrastructure > network > subnets
Output: /infrastructure/network/subnets  (full path)
```

**Navigation column (specific level):**

```text
Selected: Level 1 "infrastructure"
Output: /infrastructure  (ignores deeper levels)

Selected: Level 2 "network"
Output: /infrastructure/network  (ignores subnets)
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
│       ├── model.go      # Bubble Tea Model-Update-View
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

## 🔧 Contributing

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
| **Language** | Go | 1.25.5 |
| **TUI Framework** | [Bubble Tea](https://github.com/charmbracelet/bubbletea) | 1.3.10 |
| **Styling** | [Lipgloss](https://github.com/charmbracelet/lipgloss) | 1.1.0 |
| **CLI Framework** | [Cobra](https://github.com/spf13/cobra) | 1.10.2 |
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

## ⚠️ Important note

**TerraX is a navigation and selection tool only.**

It does **NOT** execute infrastructure commands. The tool outputs your selection (command + path) to stdout, allowing you to:

- Review before execution
- Integrate with CI/CD pipelines
- Pipe to automation scripts
- Maintain audit trails
- Ensure deliberate, safe infrastructure changes

**Example safe workflow:**

```bash
# 1. Use TerraX to select
TerraX

# 2. Review output
# ✅ Selection confirmed
# Command:    apply
# Stack Path: /infrastructure/vpc

# 3. Execute manually (or in script)
cd /infrastructure/vpc && terragrunt apply
```

---

<div align="center">

**Built with ❤️ for Infrastructure Engineers**

[Report Bug](https://github.com/israoo/TerraX/issues) •
[Request Feature](https://github.com/israoo/TerraX/issues) •

</div>
