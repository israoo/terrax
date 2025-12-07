package cmd

import (
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/israoo/terrax/internal/tui"
	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "terrax",
	Short: "Terrax - Terra eXecutor for managing Terragrunt/Terramate stacks",
	Long: `Terrax is a professional CLI tool for interactive and centralized management
of Terragrunt/Terramate stacks. It provides a TUI for easy navigation
and selection of infrastructure commands.`,
	Run: func(cmd *cobra.Command, args []string) {
		startTUI()
	},
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func startTUI() {
	initialModel := tui.NewModel()

	p := tea.NewProgram(
		initialModel,
		tea.WithAltScreen(),
		tea.WithMouseCellMotion(),
	)

	finalModel, err := p.Run()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error running TUI: %v\n", err)
		os.Exit(1)
	}

	if m, ok := finalModel.(tui.Model); ok && m.Confirmed {
		fmt.Printf("Command: %s\n", m.GetSelectedCommand())
		fmt.Printf("Stack: %s\n", m.GetSelectedStack())
	}
}
