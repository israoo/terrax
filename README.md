# Terrax - Terra eXecutor

🌍 **Terrax** is a professional CLI for interactive and centralized management of Terragrunt/Terramate stacks.

## ✨ Features

- 🎯 **Two-Column TUI**: Terminal user interface with two interactive columns
  - Command selection (plan, apply, destroy, validate, init)
  - Stack navigation
- 🎨 **Professional Design**: Attractive interface using Charm Bracelet (Bubble Tea + Lipgloss)
- ⌨️ **Intuitive Navigation**: Easy keyboard-based navigation and selection

## 🚀 Quick Start

### Installation

```bash
git clone https://github.com/israoo/terrax.git
cd terrax
go mod tidy
go build -o terrax
```

### Usage

Start the interactive TUI:

```bash
./terrax
```

## ⌨️ Controls

- `←` / `→` or `h` / `l`: Switch between columns
- `↑` / `↓` or `k` / `j`: Navigate in the active column
- `Enter`: Confirm selection and exit
- `q` / `Ctrl+C`: Quit

## 🏗️ Project Structure

```text
terrax/
├── cmd/
│   └── root.go        # Cobra commands and TUI entry point
├── internal/
│   └── tui/           # BubbleTea TUI models
│       └── model.go
├── main.go            # Entry point
└── go.mod
```

## 🛠️ Tech Stack

- **CLI**: [Cobra](https://github.com/spf13/cobra)
- **TUI**: [Bubble Tea](https://github.com/charmbracelet/bubbletea), [Lipgloss](https://github.com/charmbracelet/lipgloss)

## 🤝 Contributing

This is an actively developed project. Contributions are welcome.

## 📄 License

MIT License

---

Developed with ❤️ using [Charm Bracelet](https://charm.sh)
