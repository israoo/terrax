// Package executor handles the execution of external Terragrunt commands.
//
// It provides a high-level interface for building and running Terragrunt CLI
// commands, managing input/output streams, and integrating with the history
// tracking system.
package executor

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"time"

	"github.com/israoo/terrax/internal/config"
	"github.com/israoo/terrax/internal/history"
	"github.com/spf13/viper"
)

// HistoryLogger defines the interface for logging execution history.
type HistoryLogger interface {
	GetNextID(ctx context.Context) (int, error)
	Append(ctx context.Context, entry history.ExecutionLogEntry) error
	TrimHistory(ctx context.Context, maxEntries int) error
}

// Run executes the terragrunt command with the selected parameters.
// It also logs the execution to the history file using the provided logger.
func Run(ctx context.Context, historyLogger HistoryLogger, command, absoluteStackPath string) error {
	// Get next ID for this execution
	nextID, err := historyLogger.GetNextID(ctx)
	if err != nil {
		// Log error but don't fail execution
		fmt.Fprintf(os.Stderr, "Warning: Failed to get history ID: %v\n", err)
		nextID = 0 // Use 0 as fallback
	}

	// Record execution start time
	startTime := time.Now()

	// Build the terragrunt command
	args := buildTerragruntArgs(absoluteStackPath, command)

	fmt.Printf("🚀 Executing: terragrunt %v\n\n", args)

	cmd := exec.CommandContext(ctx, "terragrunt", args...)
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

	// Display execution summary
	displayExecutionSummary(command, absoluteStackPath, duration, exitCode, startTime)

	// Log execution to history
	logExecutionToHistory(ctx, historyLogger, nextID, startTime, command, absoluteStackPath, exitCode, duration, summary)

	return execErr
}

// buildTerragruntArgs constructs the terragrunt command arguments with logging configuration.
func buildTerragruntArgs(absoluteStackPath, command string) []string {
	// Start with base arguments
	args := []string{"run", "--all", "--working-dir", absoluteStackPath}

	// Add log level if configured
	logLevel := viper.GetString("log_level")
	if logLevel != "" {
		args = append(args, "--log-level", logLevel)
	}

	// Add log format arguments
	logCustomFormat := viper.GetString("log_custom_format")
	if logCustomFormat != "" {
		args = append(args, "--log-custom-format", logCustomFormat)
	} else {
		logFormat := viper.GetString("log_format")
		if logFormat != "" {
			args = append(args, "--log-format", logFormat)
		}
	}

	// Add parallelism if configured (non-zero)
	parallelism := viper.GetInt("terragrunt.parallelism")
	if parallelism > 0 {
		args = append(args, "--terragrunt-parallelism", fmt.Sprintf("%d", parallelism))
	}

	// Add no-color flag if configured
	noColor := viper.GetBool("terragrunt.no_color")
	if noColor {
		args = append(args, "--terragrunt-no-color")
	}

	// Add non-interactive flag if configured
	nonInteractive := viper.GetBool("terragrunt.non_interactive")
	if nonInteractive {
		args = append(args, "--terragrunt-non-interactive")
	}

	// Add ignore-dependency-errors flag if configured
	ignoreDependencyErrors := viper.GetBool("terragrunt.ignore_dependency_errors")
	if ignoreDependencyErrors {
		args = append(args, "--terragrunt-ignore-dependency-errors")
	}

	// Add ignore-external-dependencies flag if configured
	ignoreExternalDependencies := viper.GetBool("terragrunt.ignore_external_dependencies")
	if ignoreExternalDependencies {
		args = append(args, "--terragrunt-ignore-external-dependencies")
	}

	// Add include-external-dependencies flag if configured
	includeExternalDependencies := viper.GetBool("terragrunt.include_external_dependencies")
	if includeExternalDependencies {
		args = append(args, "--terragrunt-include-external-dependencies")
	}

	// Add extra flags
	extraFlags := viper.GetStringSlice("terragrunt.extra_flags")
	if len(extraFlags) > 0 {
		args = append(args, extraFlags...)
	}

	// Add separator and command
	args = append(args, "--", command)

	return args
}

// displayExecutionSummary prints the summary of the execution.
func displayExecutionSummary(command, path string, duration time.Duration, exitCode int, timestamp time.Time) {
	fmt.Println()
	fmt.Println("═══════════════════════════════════════")
	fmt.Println("  📊 Execution Summary")
	fmt.Println("═══════════════════════════════════════")
	fmt.Printf("Command:    %s\n", command)
	fmt.Printf("Stack Path: %s\n", path)
	fmt.Printf("Duration:   %.2fs\n", duration.Seconds())
	fmt.Printf("Exit Code:  %d\n", exitCode)
	fmt.Printf("Timestamp:  %s\n", timestamp.Format("2006-01-02 15:04:05"))
	fmt.Println("═══════════════════════════════════════")
	fmt.Println()
}

// logExecutionToHistory handles the details of recording the execution to the history file.
func logExecutionToHistory(ctx context.Context, logger HistoryLogger, id int, timestamp time.Time, command, absoluteStackPath string, exitCode int, duration time.Duration, summary string) {
	// Get root config file from configuration
	rootConfigFile := viper.GetString("root_config_file")
	if rootConfigFile == "" {
		rootConfigFile = config.DefaultRootConfigFile
	}

	// Calculate relative stack path
	relativeStackPath, err := history.GetRelativeStackPath(absoluteStackPath, rootConfigFile)
	if err != nil {
		// Log warning but use absolute path as fallback
		fmt.Fprintf(os.Stderr, "Warning: Failed to calculate relative stack path: %v\n", err)
		relativeStackPath = absoluteStackPath
	}

	// Log execution to history
	entry := history.ExecutionLogEntry{
		ID:           id,
		Timestamp:    timestamp,
		User:         history.GetCurrentUser(),
		StackPath:    relativeStackPath,
		AbsolutePath: absoluteStackPath,
		Command:      command,
		ExitCode:     exitCode,
		DurationS:    duration.Seconds(),
		Summary:      summary,
	}

	if err := logger.Append(ctx, entry); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: Failed to append to history: %v\n", err)
	}

	// Trim history if configured
	maxEntries := viper.GetInt("history.max_entries")
	if maxEntries < config.MinHistoryMaxEntries {
		maxEntries = config.DefaultHistoryMaxEntries
	}

	if err := logger.TrimHistory(ctx, maxEntries); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: Failed to trim history: %v\n", err)
	}
}
