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
	rootCmd.SilenceErrors = true // main.go handles error printing to avoid duplicates.

	rootCmd.Flags().Bool("history", false, "View execution history interactively")
	rootCmd.Flags().String("dir", "", "Working directory (overrides current directory)")
}

// Execute runs the root command.
func Execute() error {
	return rootCmd.Execute()
}

// ensureConfigFromWorkDir reloads .terrax.yaml from the project root containing workDir,
// then re-applies any .terrax.local.yaml overrides found alongside it.
// initConfig reads from os.Getwd() at process start, which may differ from the project
// root when commands are invoked via the VS Code extension with --dir flags.
func ensureConfigFromWorkDir(workDir string) {
	rootConfigFile := viper.GetString("root_config_file")
	if rootConfigFile == "" {
		rootConfigFile = config.DefaultRootConfigFile
	}
	repoRoot := deps.FindRepoRoot(workDir, rootConfigFile)
	viper.AddConfigPath(repoRoot)
	if err := viper.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			fmt.Fprintf(os.Stderr, "Warning: error reading config from %s: %v\n", repoRoot, err)
		}
	}
	mergeLocalConfig([]string{repoRoot})
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

	// Merge .terrax.local.yaml on top of the base config. Local config has priority and
	// is intended for machine-specific overrides (gitignored). Deep-merge is used so only
	// the keys present in the local file override their counterparts in the base config.
	mergeLocalConfig([]string{".", func() string {
		if home, err := os.UserHomeDir(); err == nil {
			return home
		}
		return ""
	}()})
}

// mergeLocalConfig loads .terrax.local.yaml from the first path in searchPaths where it exists
// and merges its values into the global viper config with override semantics.
func mergeLocalConfig(searchPaths []string) {
	local := viper.New()
	local.SetConfigName(".terrax.local")
	local.SetConfigType("yaml")
	for _, p := range searchPaths {
		if p != "" {
			local.AddConfigPath(p)
		}
	}
	if err := local.ReadInConfig(); err != nil {
		return // File not found — silently skip.
	}
	if err := viper.MergeConfigMap(local.AllSettings()); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: Error merging local config: %v\n", err)
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

// runTUI starts the TUI application.
func runTUI(cmd *cobra.Command, args []string) error {
	ctx := context.Background()

	historyService, err := getHistoryService()
	if err != nil {
		return err
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
	workDir = resolveWorkDir(workDir)
	ensureConfigFromWorkDir(workDir)

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

		if command == "plan" && (viper.GetBool("plan.summary_enabled") || viper.GetBool("plan.review_enabled")) {
			_ = os.RemoveAll(filepath.Join(repoRoot, config.DefaultOutputDir))
		}

		groups, err := buildGroupedExecution(filterPaths, repoRoot)
		if err != nil {
			return fmt.Errorf("failed to build group execution plan: %w", err)
		}
		for _, group := range groups {
			if group.Skip {
				continue
			}
			if err := executor.Run(ctx, historyService, command, stackPath, repoRoot, group.Paths, group.EnvVars); err != nil {
				return err
			}
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

// resolveWorkDir returns the parent directory when dir is a leaf stack — a directory
// that has a terragrunt.hcl file but no sub-directories that are also stacks.
// TerraX requires sub-directories to navigate, so pointing it at a leaf stack would
// fail; using the parent lets the TUI navigate to the stack as a selectable node.
func resolveWorkDir(dir string) string {
	if _, err := os.Stat(filepath.Join(dir, "terragrunt.hcl")); err != nil {
		return dir
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return dir
	}
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		if _, err := os.Stat(filepath.Join(dir, e.Name(), "terragrunt.hcl")); err == nil {
			return dir
		}
	}
	return filepath.Dir(dir)
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

			if entry.Command == "plan" && (viper.GetBool("plan.summary_enabled") || viper.GetBool("plan.review_enabled")) {
				_ = os.RemoveAll(filepath.Join(repoRoot, config.DefaultOutputDir))
			}

			groups, err := buildGroupedExecution(filterPaths, repoRoot)
			if err != nil {
				return fmt.Errorf("failed to build group execution plan: %w", err)
			}
			for _, group := range groups {
				if group.Skip {
					continue
				}
				if err := executor.Run(ctx, historyService, entry.Command, absolutePath, repoRoot, group.Paths, group.EnvVars); err != nil {
					return err
				}
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
func runPlanSummary(ctx context.Context, stackPath, repoRoot string) error {
	dir := filepath.Join(repoRoot, config.DefaultJSONOutDir)
	_, err := plan.Summarize(ctx, dir, repoRoot)
	return err
}

// PlanReviewRunner is a function type that runs the Plan Review TUI.
type PlanReviewRunner func(initialModel tui.Model) (tui.Model, error)

// currentPlanReviewRunner holds the active Plan Review TUI runner.
var currentPlanReviewRunner PlanReviewRunner = defaultPlanReviewRunner

// runPlanReview reads JSON plan files and launches the review TUI.
func runPlanReview(ctx context.Context, stackPath string) error {
	rootConfigFile := viper.GetString("root_config_file")
	if rootConfigFile == "" {
		rootConfigFile = config.DefaultRootConfigFile
	}
	repoRoot, _ := history.FindProjectRoot(stackPath, rootConfigFile)
	if repoRoot == "" {
		repoRoot = stackPath
	}
	jsonDir := filepath.Join(repoRoot, config.DefaultJSONOutDir)

	if _, err := os.Stat(jsonDir); os.IsNotExist(err) {
		return fmt.Errorf("no plan results found — run a plan first (terrax with plan.review_enabled: true)")
	}

	report, err := plan.CollectFromJSONDir(ctx, jsonDir, stackPath)
	if err != nil {
		return fmt.Errorf("plan collection failed: %w", err)
	}

	if report.Summary.TotalStacks == 0 {
		return fmt.Errorf("no plan results found — run a plan first (terrax with plan.review_enabled: true)")
	}

	if report.Summary.StacksWithChanges == 0 {
		// Only print if summary mode is not active — summary already reported this.
		if !viper.GetBool("plan.summary_enabled") {
			fmt.Println("✅ No changes found in any stack.")
		}
		return nil
	}

	initialModel := tui.NewPlanReviewModel(report)
	_, err = currentPlanReviewRunner(initialModel)
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

// StackGroupConfig holds the configuration for one stack group, loaded from stack_groups in .terrax.yaml.
type StackGroupConfig struct {
	Detect    string            `mapstructure:"detect"`
	DependsOn []string          `mapstructure:"depends_on"`
	Env       map[string]string `mapstructure:"env"`
	Skip      bool              `mapstructure:"skip"` // When true, this group is excluded from local execution.
}

// GroupExecution is one resolved group ready for sequential execution.
type GroupExecution struct {
	Name      string
	DependsOn []string
	Paths     []string
	EnvVars   map[string]string
	Skip      bool // When true, this group was excluded from execution via skip: true in config.
}

// loadStackGroups reads the stack_groups section from viper config.
// Always ensures an implicit "default" group exists.
func loadStackGroups() map[string]StackGroupConfig {
	var groups map[string]StackGroupConfig
	if err := viper.UnmarshalKey("stack_groups", &groups); err != nil || groups == nil {
		groups = map[string]StackGroupConfig{}
	}
	if _, ok := groups["default"]; !ok {
		groups["default"] = StackGroupConfig{}
	}
	return groups
}

// buildGroupedExecution assigns each filter path to a stack group, applies topological
// sorting, and returns the groups in execution order.
func buildGroupedExecution(filterPaths []string, repoRoot string) ([]GroupExecution, error) {
	groups := loadStackGroups()

	detectConfigs := make(map[string]stack.GroupDetectConfig, len(groups))
	for name, cfg := range groups {
		detectConfigs[name] = stack.GroupDetectConfig{
			Detect:    cfg.Detect,
			DependsOn: cfg.DependsOn,
			Env:       cfg.Env,
		}
	}

	pathsByGroup := make(map[string][]string)
	for _, relPath := range filterPaths {
		absPath := filepath.Join(repoRoot, filepath.FromSlash(relPath))
		groupName := stack.DetectGroup(absPath, detectConfigs)
		pathsByGroup[groupName] = append(pathsByGroup[groupName], relPath)
	}

	order, err := stack.TopologicalSort(detectConfigs)
	if err != nil {
		return nil, err
	}

	var result []GroupExecution
	for _, name := range order {
		paths := pathsByGroup[name]
		if len(paths) == 0 {
			continue
		}
		cfg := groups[name]
		deps := cfg.DependsOn
		if deps == nil {
			deps = []string{}
		}
		env := cfg.Env
		if env == nil {
			env = map[string]string{}
		}
		result = append(result, GroupExecution{
			Name:      name,
			DependsOn: deps,
			Paths:     paths,
			EnvVars:   env,
			Skip:      cfg.Skip,
		})
	}
	return result, nil
}
