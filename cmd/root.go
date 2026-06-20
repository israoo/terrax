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
	"path/filepath"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/israoo/terrax/internal/config"
	"github.com/israoo/terrax/internal/deps"
	"github.com/israoo/terrax/internal/executor"
	"github.com/israoo/terrax/internal/history"
	"github.com/israoo/terrax/internal/plan"
	"github.com/israoo/terrax/internal/stack"
	"github.com/israoo/terrax/internal/state"
	"github.com/israoo/terrax/internal/tui"
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
		viper.Set("terrax.session_timestamp", time.Now().UnixNano())
	},
	RunE: runTUI,
}

func init() {
	// Assign the version to rootCmd to enable --version flag
	rootCmd.Version = Version

	rootCmd.SilenceUsage = true

	rootCmd.Flags().BoolP("last", "l", false, "Execute the last command from history")
	rootCmd.Flags().Bool("history", false, "View execution history interactively")
	rootCmd.Flags().String("dir", "", "Working directory (overrides current directory)")
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
	viper.SetDefault("plan.review_enabled", config.DefaultPlanReviewEnabled)
	viper.SetDefault("plan.summary_enabled", config.DefaultPlanSummaryEnabled)
	viper.SetDefault("plan.cleanup_enabled", config.DefaultPlanCleanupEnabled)
	viper.SetDefault("include_dependencies", config.DefaultIncludeDependencies)

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

	dirFlag, _ := cmd.Flags().GetString("dir")
	workDir, err := getWorkingDirectory(dirFlag)
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

		if command == "force-unlock" {
			return runForceUnlock(ctx, historyService, stackPath)
		}

		repoRoot, filterPaths := collectTransitiveDeps(stackPath)
		if err := executor.Run(ctx, historyService, command, stackPath, repoRoot, filterPaths); err != nil {
			return err
		}
		if command == "plan" && viper.GetBool("plan.summary_enabled") {
			if err := runPlanSummary(ctx, stackPath, repoRoot); err != nil {
				fmt.Fprintf(os.Stderr, "Warning: plan summary failed: %v\n", err)
			}
		}
		if command == "plan" && viper.GetBool("plan.review_enabled") {
			return runPlanReview(ctx, stackPath)
		}

		return nil
	}

	return nil
}

// getWorkingDirectory returns dir if non-empty, otherwise the current working directory.
func getWorkingDirectory(dir string) (string, error) {
	if dir != "" {
		return dir, nil
	}
	workDir, err := os.Getwd()
	if err != nil {
		return "", err
	}
	return workDir, nil
}

// buildStackTree scans and builds the stack tree structure.
func buildStackTree(workDir string) (*stack.Node, int, error) {
	fmt.Println("🔍 Scanning for stacks in:", workDir)

	stackRoot, maxDepth, err := stack.FindAndBuildTree(workDir, viper.GetString("root_config_file"))
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
	return runBubbleTeaProgram(initialModel)
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

	if lastEntry.Command == "force-unlock" {
		return runForceUnlock(ctx, historyService, absolutePath)
	}

	repoRoot, filterPaths := collectTransitiveDeps(absolutePath)
	if err := executor.Run(ctx, historyService, lastEntry.Command, absolutePath, repoRoot, filterPaths); err != nil {
		return err
	}
	if lastEntry.Command == "plan" && viper.GetBool("plan.summary_enabled") {
		if err := runPlanSummary(ctx, absolutePath, repoRoot); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: plan summary failed: %v\n", err)
		}
	}
	if lastEntry.Command == "plan" && viper.GetBool("plan.review_enabled") {
		return runPlanReview(ctx, absolutePath)
	}

	return nil
}

// runHistoryViewer loads and displays the execution history in an interactive TUI.
// It filters the history to show only entries from the current project.
// If the user selects an entry and presses Enter, it re-executes that command.
func runHistoryViewer(ctx context.Context, historyService *history.Service) error {
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

	// Note: History viewer uses Stderr specifically, so we don't use the shared runBubbleTeaProgram helper here
	// unless we update it to support custom output.
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

			if entry.Command == "force-unlock" {
				return runForceUnlock(ctx, historyService, absolutePath)
			}

			repoRoot, filterPaths := collectTransitiveDeps(absolutePath)
			if err := executor.Run(ctx, historyService, entry.Command, absolutePath, repoRoot, filterPaths); err != nil {
				return err
			}
			if entry.Command == "plan" && viper.GetBool("plan.summary_enabled") {
				if err := runPlanSummary(ctx, absolutePath, repoRoot); err != nil {
					fmt.Fprintf(os.Stderr, "Warning: plan summary failed: %v\n", err)
				}
			}
			if entry.Command == "plan" && viper.GetBool("plan.review_enabled") {
				return runPlanReview(ctx, absolutePath)
			}

			return nil
		}
	}

	return nil
}

// runForceUnlock discovers the state lock ID from S3 and executes force-unlock.
// It scans all Terragrunt stacks under absoluteStackPath (recursively), checks each
// for an active lock, and unlocks every locked stack. Returns nil if no locks are found.
func runForceUnlock(ctx context.Context, historyService *history.Service, absoluteStackPath string) error {
	bucket := viper.GetString("state.bucket")
	project := viper.GetString("state.project")
	region := viper.GetString("state.region")
	profile := viper.GetString("state.aws_profile")
	configFile := viper.GetString("state.aws_config_file")

	if bucket == "" || project == "" {
		return fmt.Errorf("state.bucket and state.project must be set in .terrax.yaml to use force-unlock")
	}

	rootConfigFile := viper.GetString("root_config_file")
	if rootConfigFile == "" {
		rootConfigFile = config.DefaultRootConfigFile
	}

	stackPaths, err := stack.CollectStackPaths(absoluteStackPath)
	if err != nil {
		return fmt.Errorf("failed to scan stacks: %w", err)
	}

	// Fallback: treat the path itself as a single stack when no terragrunt.hcl is found below it.
	if len(stackPaths) == 0 {
		stackPaths = []string{absoluteStackPath}
	}

	var unlockErrs []string
	locksFound := 0

	for _, stackPath := range stackPaths {
		stackRelPath, err := history.GetRelativeStackPath(stackPath, rootConfigFile)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Warning: failed to determine relative path for %s: %v\n", stackPath, err)
			continue
		}

		lockID, err := state.GetLockID(ctx, bucket, project, stackRelPath, region, profile, configFile)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Warning: failed to get lock ID for %s: %v\n", stackRelPath, err)
			continue
		}

		if lockID == "" {
			fmt.Printf("No lock found for %s\n", stackRelPath)
			continue
		}

		locksFound++
		fmt.Printf("🔓 Unlocking %s (lock: %s)\n", stackRelPath, lockID)
		if err := executor.RunForceUnlock(ctx, historyService, lockID, stackPath); err != nil {
			unlockErrs = append(unlockErrs, fmt.Sprintf("%s: %v", stackRelPath, err))
		}
	}

	if locksFound == 0 {
		fmt.Println("No locks found.")
	}

	if len(unlockErrs) > 0 {
		return fmt.Errorf("force-unlock failed for %d stack(s): %s", len(unlockErrs), strings.Join(unlockErrs, "; "))
	}

	return nil
}

// collectTransitiveDeps computes the filter list for summary mode.
// When include_dependencies is true, transitive dependencies are resolved
// via static HCL parsing and included in the filter list.
// When false, only the selected stack(s) are included — no dependency traversal.
// Non-leaf directories are expanded to all leaf stacks they contain via CollectStackPaths.
func collectTransitiveDeps(stackPath string) (repoRoot string, filterPaths []string) {
	rootConfigFile := viper.GetString("root_config_file")
	if rootConfigFile == "" {
		rootConfigFile = config.DefaultRootConfigFile
	}
	repoRoot = deps.FindRepoRoot(stackPath, rootConfigFile)
	includeExternal := viper.GetBool("include_dependencies")

	// Seed the queue: if the path is a leaf stack, start with it alone.
	// If it is a directory containing multiple stacks, seed with all of them.
	var seeds []string
	hclFile := filepath.Join(stackPath, "terragrunt.hcl")
	if _, err := os.Stat(hclFile); err == nil {
		seeds = []string{stackPath}
	} else {
		leafPaths, err := stack.CollectStackPaths(stackPath)
		if err != nil || len(leafPaths) == 0 {
			seeds = []string{stackPath} // fallback
		} else {
			seeds = leafPaths
		}
	}

	visited := map[string]bool{}
	queue := seeds

	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]

		if visited[current] {
			continue
		}
		visited[current] = true

		rel, err := filepath.Rel(repoRoot, current)
		if err == nil {
			filterPaths = append(filterPaths, filepath.ToSlash(rel))
		}

		// Only resolve transitive dependencies when include_dependencies is enabled.
		if includeExternal {
			depHCL := filepath.Join(current, "terragrunt.hcl")
			for _, dep := range deps.ParseDependencies(depHCL, repoRoot) {
				if !visited[dep] {
					queue = append(queue, dep)
				}
			}
		}
	}

	return repoRoot, filterPaths
}

// runPlanSummary reads JSON plan files from repoRoot/.terrax/plans and prints a terminal count summary.
// When plan.cleanup_enabled is true, the entire .terrax/ output directory is removed after the summary.
func runPlanSummary(ctx context.Context, stackPath, repoRoot string) error {
	dir := filepath.Join(repoRoot, config.DefaultJSONOutDir)

	_, err := plan.Summarize(ctx, dir, repoRoot)
	if viper.GetBool("plan.cleanup_enabled") {
		outputDir := filepath.Join(repoRoot, config.DefaultOutputDir)
		if removeErr := os.RemoveAll(outputDir); removeErr != nil {
			fmt.Fprintf(os.Stderr, "Warning: failed to clean up plan files: %v\n", removeErr)
		}
	}
	return err
}

// PlanReviewRunner is a function type that runs the Plan Review TUI.
type PlanReviewRunner func(initialModel tui.Model) (tui.Model, error)

// currentPlanReviewRunner holds the active Plan Review TUI runner.
var currentPlanReviewRunner PlanReviewRunner = defaultPlanReviewRunner

// runPlanReview collects plan results and launches the review TUI.
func runPlanReview(ctx context.Context, stackPath string) error {
	collector := plan.NewCollector(stackPath)
	progressChan := make(chan plan.ProgressMsg, 10) // Buffered channel

	// Error channel to capture collection error from goroutine
	errChan := make(chan error, 1)
	var report *plan.PlanReport

	// Start collection in background
	go func() {
		defer close(errChan)
		defer close(progressChan)

		r, err := collector.Collect(ctx, progressChan)
		if err != nil {
			errChan <- err
			return
		}
		report = r
	}()

	// CLI Progress Loop
	fmt.Println() // Start with a newline
	for msg := range progressChan {
		if msg.TotalFiles > 0 && msg.Current > 0 {
			// [1/10] Processed path/to/stack
			fmt.Printf("[%d/%d] %s\n", msg.Current, msg.TotalFiles, msg.Message)
		} else {
			// Initial messages (Scanning..., Found X plans)
			fmt.Printf("🔍 %s\n", msg.Message)
		}
	}
	fmt.Println() // End progress line

	// Check for collection error
	if err := <-errChan; err != nil {
		return fmt.Errorf("plan collection failed: %w", err)
	}

	if report == nil {
		return fmt.Errorf("no plan results collected")
	}

	// Check if there are any changes to display
	if report.Summary.StacksWithChanges == 0 {
		fmt.Println("✅ No changes found in any stack.")
		// We still perform cleanup
		if cleanupErr := collector.CleanupOldPlans(); cleanupErr != nil {
			fmt.Fprintf(os.Stderr, "Warning: Failed to cleanup old plans: %v\n", cleanupErr)
		}
		return nil
	}

	// Launch Review TUI with ready report
	initialModel := tui.NewPlanReviewModel(report)

	_, err := currentPlanReviewRunner(initialModel)

	// Best-effort cleanup after review
	if cleanupErr := collector.CleanupOldPlans(); cleanupErr != nil {
		fmt.Fprintf(os.Stderr, "Warning: Failed to cleanup old plans: %v\n", cleanupErr)
	}

	return err
}

// defaultPlanReviewRunner is the default implementation that runs Bubble Tea interactively.
func defaultPlanReviewRunner(initialModel tui.Model) (tui.Model, error) {
	return runBubbleTeaProgram(initialModel)
}

// setPlanReviewRunner allows tests to inject a custom Plan Review runner.
func setPlanReviewRunner(runner PlanReviewRunner) func() {
	original := currentPlanReviewRunner
	currentPlanReviewRunner = runner
	return func() {
		currentPlanReviewRunner = original
	}
}

// runBubbleTeaProgram runs a Bubble Tea program with standard options.
func runBubbleTeaProgram(model tui.Model) (tui.Model, error) {
	p := tea.NewProgram(
		model,
		tea.WithAltScreen(),
		tea.WithMouseCellMotion(),
	)

	finalModel, err := p.Run()
	if err != nil {
		return tui.Model{}, err
	}

	resultModel, ok := finalModel.(tui.Model)
	if !ok {
		return tui.Model{}, fmt.Errorf("unexpected model type")
	}

	return resultModel, nil
}
