package cmd

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/israoo/terrax/internal/config"
	"github.com/israoo/terrax/internal/history"
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

	// Add --last flag for executing the most recent command
	rootCmd.Flags().BoolP("last", "l", false, "Execute the last command from history")
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
	viper.SetDefault("history.max_entries", config.DefaultHistoryMaxEntries)

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

// runTUI starts the TUI application or executes the last command if --last flag is set.
func runTUI(cmd *cobra.Command, args []string) error {
	ctx := context.Background()

	// Check if --last flag is set
	lastFlag, _ := cmd.Flags().GetBool("last")
	if lastFlag {
		return executeLastCommand(ctx)
	}

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
// It also logs the execution to the history file for audit and replay purposes.
func executeTerragruntCommand(command, stackPath string) error {
	ctx := context.Background()

	// Get next ID for this execution
	nextID, err := history.GetNextID(ctx)
	if err != nil {
		// Log error but don't fail execution
		fmt.Fprintf(os.Stderr, "Warning: Failed to get history ID: %v\n", err)
		nextID = 0 // Use 0 as fallback
	}

	// Record execution start time
	startTime := time.Now()

	// Build the terragrunt command: terragrunt run --all --working-dir {PATH} -- {command}
	args := []string{"run", "--all", "--working-dir", stackPath, "--", command}

	fmt.Printf("🚀 Executing: terragrunt %v\n\n", args)

	cmd := exec.Command("terragrunt", args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin

	// Execute command and capture exit code
	execErr := cmd.Run()
	exitCode := 0
	summary := "Command completed successfully"

	if execErr != nil {
		fmt.Fprintf(os.Stderr, "\n❌ Command execution failed: %v\n", execErr)
		// Extract exit code from error
		if exitErr, ok := execErr.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		} else {
			exitCode = 1 // Generic error code
		}
		summary = fmt.Sprintf("Command failed: %v", execErr)
	} else {
		fmt.Println("\n✅ Command execution completed")
	}

	// Calculate duration
	duration := time.Since(startTime)

	// Log execution to history
	entry := history.ExecutionLogEntry{
		ID:        nextID,
		Timestamp: startTime,
		User:      history.GetCurrentUser(),
		StackPath: stackPath,
		Command:   command,
		ExitCode:  exitCode,
		DurationS: duration.Seconds(),
		Summary:   summary,
	}

	if err := history.AppendToHistory(ctx, entry); err != nil {
		// Log error but don't fail the overall execution
		fmt.Fprintf(os.Stderr, "Warning: Failed to append to history: %v\n", err)
	}

	// Trim history if configured
	maxEntries := viper.GetInt("history.max_entries")
	if maxEntries < config.MinHistoryMaxEntries {
		maxEntries = config.DefaultHistoryMaxEntries
	}

	if err := history.TrimHistory(ctx, maxEntries); err != nil {
		// Log error but don't fail the overall execution
		fmt.Fprintf(os.Stderr, "Warning: Failed to trim history: %v\n", err)
	}

	return execErr
}

// executeLastCommand retrieves and executes the most recent command from history.
func executeLastCommand(ctx context.Context) error {
	// Get last execution from history
	lastEntry, err := history.GetLastExecution(ctx)
	if err != nil {
		return fmt.Errorf("failed to get last execution: %w", err)
	}

	if lastEntry == nil {
		fmt.Println("⚠️  No execution history found")
		fmt.Println("Run terrax interactively first to build history")
		return nil
	}

	// Display what will be executed
	fmt.Println("═══════════════════════════════════════")
	fmt.Println("  🔄 Re-executing last command")
	fmt.Println("═══════════════════════════════════════")
	fmt.Printf("Command:    %s\n", lastEntry.Command)
	fmt.Printf("Stack Path: %s\n", lastEntry.StackPath)
	fmt.Printf("Previous:   %s (exit code: %d)\n", lastEntry.Timestamp.Format("2006-01-02 15:04:05"), lastEntry.ExitCode)
	fmt.Println("═══════════════════════════════════════")
	fmt.Println()

	// Execute the command
	return executeTerragruntCommand(lastEntry.Command, lastEntry.StackPath)
}
