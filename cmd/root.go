package cmd

import (
	"fmt"
	"os"
	"os/exec"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/israoo/terrax/internal/stack"
	"github.com/israoo/terrax/internal/tui"
	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "terrax",
	Short: "Terrax - Terra eXecutor for managing Terragrunt stacks",
	Long: `Terrax is a professional CLI tool for interactive and centralized management
of Terragrunt stacks. It provides a TUI for easy navigation
and selection of infrastructure commands.`,
	RunE: runTUI,
}

// Execute runs the root command.
func Execute() error {
	return rootCmd.Execute()
}

// runTUI starts the TUI application.
func runTUI(cmd *cobra.Command, args []string) error {
	// Get working directory
	workDir, err := getWorkingDirectory()
	if err != nil {
		return fmt.Errorf("failed to get working directory: %w", err)
	}

	// Build stack tree
	stackRoot, maxDepth, err := buildStackTree(workDir)
	if err != nil {
		return fmt.Errorf("failed to build stack tree: %w", err)
	}

	// Run TUI
	model, err := runInteractiveTUI(stackRoot, maxDepth)
	if err != nil {
		return fmt.Errorf("TUI error: %w", err)
	}

	// Display results
	displayResults(model)

	// Execute command if confirmed
	if model.IsConfirmed() {
		return executeTerragruntCommand(model.GetSelectedCommand(), model.GetSelectedStackPath())
	}

	return nil
}

// getWorkingDirectory returns the current working directory.
func getWorkingDirectory() (string, error) {
	workDir, err := os.Getwd()
	if err != nil {
		return "", err
	}
	return workDir, nil
}

// buildStackTree scans and builds the stack tree structure.
func buildStackTree(workDir string) (*stack.Node, int, error) {
	fmt.Println("🔍 Scanning for stacks in:", workDir)

	stackRoot, maxDepth, err := stack.FindAndBuildTree(workDir)
	if err != nil {
		return nil, 0, err
	}

	fmt.Printf("✅ Found stack tree with max depth: %d\n", maxDepth)

	if !stackRoot.HasChildren() {
		fmt.Println("⚠️  No subdirectories found. Make sure you're in the right directory.")
	}

	return stackRoot, maxDepth, nil
}

// runInteractiveTUI starts the Bubble Tea program and returns the final model.
func runInteractiveTUI(stackRoot *stack.Node, maxDepth int) (tui.Model, error) {
	initialModel := tui.NewModel(stackRoot, maxDepth)

	p := tea.NewProgram(
		initialModel,
		tea.WithAltScreen(),
		tea.WithMouseCellMotion(),
	)

	finalModel, err := p.Run()
	if err != nil {
		return tui.Model{}, err
	}

	model, ok := finalModel.(tui.Model)
	if !ok {
		return tui.Model{}, fmt.Errorf("unexpected model type")
	}

	return model, nil
}

// displayResults shows the final selection to the user.
func displayResults(model tui.Model) {
	fmt.Println()

	if !model.IsConfirmed() {
		fmt.Println("⚠️  Selection cancelled")
		return
	}

	fmt.Println("═══════════════════════════════════════")
	fmt.Println("  ✅ Selection Confirmed")
	fmt.Println("═══════════════════════════════════════")
	fmt.Printf("Command:    %s\n", model.GetSelectedCommand())
	fmt.Printf("Stack Path: %s\n", model.GetSelectedStackPath())
	fmt.Println("═══════════════════════════════════════")
	fmt.Println()
}

// executeTerragruntCommand runs the terragrunt command with the selected parameters.
func executeTerragruntCommand(command, stackPath string) error {
	// Build the terragrunt command: terragrunt run --all --working-dir {PATH} -- {command}
	args := []string{"run", "--all", "--working-dir", stackPath, "--", command}

	fmt.Printf("🚀 Executing: terragrunt %v\n\n", args)

	cmd := exec.Command("terragrunt", args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin

	if err := cmd.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "\n❌ Command execution failed: %v\n", err)
		return err
	}

	fmt.Println("\n✅ Command execution completed")
	return nil
}
