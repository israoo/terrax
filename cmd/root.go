// Package cmd implements the CLI commands for TerraX using Cobra.
//
// It serves as the entry point for command-line interactions, handling argument
// parsing, flag validation, and command execution. It orchestrates the TUI
// lifecycle and initializes the necessary services for the application.
package cmd

import (
	"context"
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/israoo/terrax/internal/config"
	"github.com/israoo/terrax/internal/executor"
	"github.com/israoo/terrax/internal/history"
	"github.com/israoo/terrax/internal/plan"
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

	rootCmd.SilenceUsage = true

	rootCmd.Flags().BoolP("last", "l", false, "Execute the last command from history")
	rootCmd.Flags().Bool("history", false, "View execution history interactively")
}

// Execute runs the root command.
func Execute() error {
	return rootCmd.Execute()
}

// initConfig initializes the configuration using Viper.
func initConfig() {
	viper.SetDefault("commands", config.DefaultCommands)
	viper.SetDefault("max_navigation_columns", config.DefaultMaxNavigationColumns)
	viper.SetDefault("history.max_entries", config.DefaultHistoryMaxEntries)
	viper.SetDefault("root_config_file", config.DefaultRootConfigFile)
	viper.SetDefault("log_format", config.DefaultLogFormat)
	viper.SetDefault("terragrunt.parallelism", config.DefaultParallelism)
	viper.SetDefault("terragrunt.no_color", config.DefaultNoColor)

	viper.SetConfigName(".terrax")
	viper.SetConfigType("yaml")

	viper.AddConfigPath(".")
	if home, err := os.UserHomeDir(); err == nil {
		viper.AddConfigPath(home)
	}

	if err := viper.ReadInConfig(); err != nil {
		// Ignore config file not found error - use defaults
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			// Config file was found but another error was produced
			fmt.Fprintf(os.Stderr, "Warning: Error reading config file: %v\n", err)
		}
	}
}

// getHistoryService creates and returns a new history service instance.
func getHistoryService() (*history.Service, error) {
	rootConfigFile := viper.GetString("root_config_file")
	if rootConfigFile == "" {
		rootConfigFile = config.DefaultRootConfigFile
	}

	repo, err := history.NewFileRepository("")
	if err != nil {
		return nil, fmt.Errorf("failed to create history repository: %w", err)
	}

	return history.NewService(repo, rootConfigFile), nil
}

// runTUI starts the TUI application or executes the last command if --last flag is set.
func runTUI(cmd *cobra.Command, args []string) error {
	ctx := context.Background()

	historyService, err := getHistoryService()
	if err != nil {
		return err
	}

	lastFlag, _ := cmd.Flags().GetBool("last")
	if lastFlag {
		return executeLastCommand(ctx, historyService)
	}

	historyFlag, _ := cmd.Flags().GetBool("history")
	if historyFlag {
		return runHistoryViewer(ctx, historyService)
	}

	workDir, err := getWorkingDirectory()
	if err != nil {
		return fmt.Errorf("failed to get working directory: %w", err)
	}

	stackRoot, maxDepth, err := buildStackTree(workDir)
	if err != nil {
		return fmt.Errorf("failed to build stack tree: %w", err)
	}

	commands := viper.GetStringSlice("commands")
	if len(commands) == 0 {
		commands = config.DefaultCommands
	}

	maxNavColumns := viper.GetInt("max_navigation_columns")
	if maxNavColumns < config.MinMaxNavigationColumns {
		maxNavColumns = config.DefaultMaxNavigationColumns
	}

	initialModel := tui.NewModel(stackRoot, maxDepth, commands, maxNavColumns)
	model, err := currentTUIRunner(initialModel)
	if err != nil {
		return fmt.Errorf("TUI error: %w", err)
	}

	displayResults(model)

	if model.IsConfirmed() {
		command := model.GetSelectedCommand()
		stackPath := model.GetSelectedStackPath()

		err := executor.Run(ctx, historyService, command, stackPath)
		if err != nil {
			return err
		}

		if command == "plan" {
			return runPlanReview(ctx, stackPath)
		}

		return nil
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

// executeLastCommand retrieves and executes the most recent command from history for the current project.
func executeLastCommand(ctx context.Context, historyService *history.Service) error {
	lastEntry, err := historyService.GetLastExecutionForProject(ctx)
	if err != nil {
		return fmt.Errorf("failed to get last execution: %w", err)
	}

	if lastEntry == nil {
		fmt.Println("⚠️  No execution history found for this project")
		fmt.Println("Run terrax interactively first to build history")
		return nil
	}

	fmt.Println("═══════════════════════════════════════")
	fmt.Println("  🔄 Re-executing last command")
	fmt.Println("═══════════════════════════════════════")
	fmt.Printf("Command:    %s\n", lastEntry.Command)
	fmt.Printf("Stack Path: %s\n", lastEntry.StackPath)
	fmt.Printf("Previous:   %s (exit code: %d)\n", lastEntry.Timestamp.Format("2006-01-02 15:04:05"), lastEntry.ExitCode)
	fmt.Println("═══════════════════════════════════════")
	fmt.Println()

	// (StackPath is relative for display, AbsolutePath is for execution)
	absolutePath := lastEntry.AbsolutePath
	if absolutePath == "" {
		// Backward compatibility: old entries only have StackPath (which was absolute)
		absolutePath = lastEntry.StackPath
	}

	err = executor.Run(ctx, historyService, lastEntry.Command, absolutePath)
	if err != nil {
		return err
	}

	if lastEntry.Command == "plan" {
		return runPlanReview(ctx, absolutePath)
	}

	return nil
}

// runHistoryViewer loads and displays the execution history in an interactive TUI.
// It filters the history to show only entries from the current project.
// If the user selects an entry and presses Enter, it re-executes that command.
func runHistoryViewer(ctx context.Context, historyService *history.Service) error {
	// FilterByCurrentProject(entries []ExecutionLogEntry) -> requires entries.
	// GetLastExecutionForProject calls repo.LoadAll then filters.

	// We need a method on Service to LoadAll or LoadProjectHistory.
	// Service has: Append, GetLastExecutionForProject, TrimHistory, GetNextID, FilterByCurrentProject(entries), GetRelativeStackPath.
	// It relies on repo for LoadAll but doesn't expose it.

	// Let's create a temporary solution: Access repo directly or add LoadAll to Service.
	// Adding LoadAll to Service is cleaner.
	// For now, I'll assume I can access repo since it's in internal package, but Service struct fields might be private (repo is private).
	// Ah, I cannot access private field `repo` from `cmd` package.

	// I should add `LoadAll` or better `GetProjectHistory` to Service.
	// Or use `GetLastExecutionForProject`... no that's only last.

	// I will add `LoadAll` to Service in the next step. For now in this file I will comment or use a placeholder,
	// BUT wait, I need this to compile.

	// I'll update Service first? No, I'm writing `root.go` now.
	// Use `history.LoadHistory` (facade) for now? No, that defeats DI.

	// I'll add `LoadAll(ctx)` to Service.
	// Let's modify Service.go in next step.
	// Here I will assume `historyService.LoadAll(ctx)` exists.
	// Actually, `history.LoadHistory(ctx)` is the facade.
	// If I strictly want DI, I need `historyService.LoadAll(ctx)`.

	// I will use `historyService.LoadAll(ctx)` and make sure to add it.

	entries, err := historyService.LoadAll(ctx)
	if err != nil {
		return fmt.Errorf("failed to load history: %w", err)
	}

	filteredEntries, err := historyService.FilterByCurrentProject(entries)
	if err != nil {
		// Log warning but continue with unfiltered entries
		fmt.Fprintf(os.Stderr, "Warning: Failed to filter history: %v\n", err)
		filteredEntries = entries
	}

	initialModel := tui.NewHistoryModel(filteredEntries)

	p := tea.NewProgram(
		initialModel,
		tea.WithAltScreen(),
		tea.WithOutput(os.Stderr),
	)

	finalModel, err := p.Run()
	if err != nil {
		return fmt.Errorf("history viewer error: %w", err)
	}

	model, ok := finalModel.(tui.Model)
	if !ok {
		return fmt.Errorf("unexpected model type")
	}

	if model.ShouldReExecuteFromHistory() {
		entry := model.GetSelectedHistoryEntry()
		if entry != nil {
			fmt.Fprintf(os.Stderr, "\n🔄 Re-executing command from history...\n")
			fmt.Fprintf(os.Stderr, "Command: %s\n", entry.Command)
			fmt.Fprintf(os.Stderr, "Path: %s\n\n", entry.StackPath)

			absolutePath := entry.AbsolutePath
			if absolutePath == "" {
				// Backward compatibility: old entries only have StackPath (which was absolute)
				absolutePath = entry.StackPath
			}

			err := executor.Run(ctx, historyService, entry.Command, absolutePath)
			if err != nil {
				return err
			}

			if entry.Command == "plan" {
				return runPlanReview(ctx, absolutePath)
			}

			return nil
		}
	}

	return nil
}

// runPlanReview collects plan results and launches the review TUI.
func runPlanReview(ctx context.Context, stackPath string) error {
	fmt.Println("🔍 Collecting plan results...")

	collector := plan.NewCollector(stackPath)
	report, err := collector.Collect(ctx)
	if err != nil {
		return fmt.Errorf("failed to collect plan results: %w", err)
	}

	if len(report.Stacks) == 0 {
		fmt.Println("⚠️  No plan files found to review.")
		return nil
	}

	fmt.Printf("✅ Found %d stack plans. Launching reviewer...\n", len(report.Stacks))

	initialModel := tui.NewPlanReviewModel(report)

	p := tea.NewProgram(
		initialModel,
		tea.WithAltScreen(),
		tea.WithMouseCellMotion(),
	)

	_, err = p.Run()
	return err
}
