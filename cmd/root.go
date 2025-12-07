package cmd

import (
	"fmt"
	"os"
	"os/exec"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/israoo/terrax/internal/config"
	"github.com/israoo/terrax/internal/stack"
	"github.com/israoo/terrax/internal/tui"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// This variable is injected by GoReleaser during builds via ldflags.
// Default value is "dev" for local development builds.
var Version = "dev"

// TUIRunner is a function type that runs a TUI program and returns the final model.
// This allows dependency injection for testing.
type TUIRunner func(initialModel tui.Model) (tui.Model, error)

// currentTUIRunner holds the active TUI runner (can be overridden in tests).
var currentTUIRunner TUIRunner = defaultTUIRunner

var rootCmd = &cobra.Command{
	Use:   "terrax",
	Short: "TerraX - Terragrunt eXecutor for managing Terragrunt stacks",
	Long: `TerraX is a professional CLI tool for interactive and centralized management
of Terragrunt stacks. It provides a TUI for easy navigation
and selection of infrastructure commands.`,
	PersistentPreRun: func(cmd *cobra.Command, args []string) {
		initConfig()
	},
	RunE: runTUI,
}

func init() {
	// Assign the version to rootCmd to enable --version flag
	rootCmd.Version = Version

	// Configure professional CLI behavior
	rootCmd.SilenceUsage = true
}

// Execute runs the root command.
func Execute() error {
	return rootCmd.Execute()
}

// initConfig initializes the configuration using Viper.
func initConfig() {
	// Set default values
	viper.SetDefault("commands", config.DefaultCommands)
	viper.SetDefault("max_navigation_columns", config.DefaultMaxNavigationColumns)

	// Configure config file search paths
	viper.SetConfigName(".terrax")
	viper.SetConfigType("yaml")

	// Search in current directory first, then home directory
	viper.AddConfigPath(".")
	if home, err := os.UserHomeDir(); err == nil {
		viper.AddConfigPath(home)
	}

	// Read config file (if exists)
	if err := viper.ReadInConfig(); err != nil {
		// Ignore config file not found error - use defaults
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			// Config file was found but another error was produced
			fmt.Fprintf(os.Stderr, "Warning: Error reading config file: %v\n", err)
		}
	}
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

	// Get commands from config (with defaults fallback)
	commands := viper.GetStringSlice("commands")
	if len(commands) == 0 {
		// Fallback to defaults if config is empty
		commands = config.DefaultCommands
	}

	// Get max navigation columns from config and validate
	maxNavColumns := viper.GetInt("max_navigation_columns")
	if maxNavColumns < config.MinMaxNavigationColumns {
		maxNavColumns = config.DefaultMaxNavigationColumns
	}

	// Run TUI using the current runner (injectable for tests)
	initialModel := tui.NewModel(stackRoot, maxDepth, commands, maxNavColumns)
	model, err := currentTUIRunner(initialModel)
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
		return nil, 0, fmt.Errorf("no terragrunt directories found")
	}

	return stackRoot, maxDepth, nil
}

// defaultTUIRunner is the default implementation that runs Bubble Tea interactively.
func defaultTUIRunner(initialModel tui.Model) (tui.Model, error) {
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

// setTUIRunner allows tests to inject a custom TUI runner.
// Returns a cleanup function to restore the original runner.
func setTUIRunner(runner TUIRunner) func() {
	original := currentTUIRunner
	currentTUIRunner = runner
	return func() {
		currentTUIRunner = original
	}
}

// displayResults shows the final selection to the user.
func displayResults(model tui.Model) {
	fmt.Println()

	if !model.IsConfirmed() {
		fmt.Println("⚠️  Selection cancelled")
		return
	}

	fmt.Println("═══════════════════════════════════════")
	fmt.Println("  ✅ Selection confirmed")
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
