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

	"github.com/spf13/viper"

	"github.com/israoo/terrax/internal/config"
	"github.com/israoo/terrax/internal/history"
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
	nextID, err := historyLogger.GetNextID(ctx)
	if err != nil {
		// Log error but don't fail execution
		fmt.Fprintf(os.Stderr, "Warning: Failed to get history ID: %v\n", err)
		nextID = 0 // Use 0 as fallback
	}

	startTime := time.Now()

	args := buildTerragruntArgs(absoluteStackPath, command)

	fmt.Printf("🚀 Executing: terragrunt %v\n\n", args)

	cmd := exec.CommandContext(ctx, "terragrunt", args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin

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

	duration := time.Since(startTime)

	displayExecutionSummary(command, absoluteStackPath, duration, exitCode, startTime)

	logExecutionToHistory(ctx, historyLogger, nextID, startTime, command, absoluteStackPath, exitCode, duration, summary)

	return execErr
}

// buildTerragruntArgs constructs the full Terragrunt command arguments.
func buildTerragruntArgs(absoluteStackPath, command string) []string {
	args := []string{"run", "--all", "--working-dir", absoluteStackPath}

	args = appendLoggingFlags(args)
	args = appendTerragruntFlags(args)
	args = appendFeatureFlags(args)
	args = appendExtraTerragruntFlags(args)
	args = appendCommandTerragruntFlags(args, command)

	args = append(args, "--", command)

	args = appendTerraformExtraFlags(args)
	args = appendCommandTerraformFlags(args, command)

	// If command is "plan", output to a binary file for later analysis.
	if command == "plan" {
		timestamp := viper.GetInt64("terrax.session_timestamp")
		planFile := fmt.Sprintf("terrax-tfplan-%d.binary", timestamp)
		args = append(args, fmt.Sprintf("-out=%s", planFile))
	}

	return args
}

// appendFeatureFlags appends flags derived from the features.* configuration section.
// Each feature key maps to one or more Terragrunt flags, hiding multi-flag complexity
// behind a single boolean toggle.
func appendFeatureFlags(args []string) []string {
	if viper.GetBool("features.tf_forward_stdout") {
		args = append(args, "--tf-forward-stdout")
	}
	if viper.GetBool("features.summary_per_unit") {
		args = append(args, "--summary-per-unit")
	}
	if viper.GetBool("features.report.enabled") {
		reportFile := viper.GetString("features.report.file")
		if reportFile == "" {
			reportFile = config.DefaultReportFile
		}
		reportFormat := viper.GetString("features.report.format")
		if reportFormat == "" {
			reportFormat = config.DefaultReportFormat
		}
		args = append(args, "--report-file", reportFile, "--report-format", reportFormat)
	}
	return args
}

// appendExtraTerragruntFlags appends global extra Terragrunt flags from terragrunt.extra_flags.
func appendExtraTerragruntFlags(args []string) []string {
	return append(args, viper.GetStringSlice("terragrunt.extra_flags")...)
}

// appendCommandTerragruntFlags appends per-command Terragrunt flags from
// terragrunt.command_flags.<command>. Only the flags for the active command are added.
func appendCommandTerragruntFlags(args []string, command string) []string {
	return append(args, viper.GetStringSlice(fmt.Sprintf("terragrunt.command_flags.%s", command))...)
}

// appendTerraformExtraFlags appends global extra Terraform flags from terraform.extra_flags.
// These flags are passed to Terraform directly, after the -- separator.
func appendTerraformExtraFlags(args []string) []string {
	return append(args, viper.GetStringSlice("terraform.extra_flags")...)
}

// appendCommandTerraformFlags appends per-command Terraform flags from
// terraform.command_flags.<command>. Only the flags for the active command are added.
func appendCommandTerraformFlags(args []string, command string) []string {
	return append(args, viper.GetStringSlice(fmt.Sprintf("terraform.command_flags.%s", command))...)
}

func appendLoggingFlags(args []string) []string {
	logLevel := viper.GetString("log_level")
	if logLevel != "" {
		args = append(args, "--log-level", logLevel)
	}

	logCustomFormat := viper.GetString("log_custom_format")
	if logCustomFormat != "" {
		args = append(args, "--log-custom-format", logCustomFormat)
	} else {
		logFormat := viper.GetString("log_format")
		if logFormat != "" {
			args = append(args, "--log-format", logFormat)
		}
	}
	return args
}

func appendTerragruntFlags(args []string) []string {
	parallelism := viper.GetInt("terragrunt.parallelism")
	if parallelism > 0 {
		args = append(args, "--terragrunt-parallelism", fmt.Sprintf("%d", parallelism))
	}

	if viper.GetBool("terragrunt.no_color") {
		args = append(args, "--terragrunt-no-color")
	}
	if viper.GetBool("terragrunt.non_interactive") {
		args = append(args, "--terragrunt-non-interactive")
	}
	if viper.GetBool("terragrunt.queue_include_external") {
		args = append(args, "--queue-include-external")
	}
	if viper.GetBool("terragrunt.ignore_dependency_errors") {
		args = append(args, "--terragrunt-ignore-dependency-errors")
	}
	if viper.GetBool("terragrunt.ignore_external_dependencies") {
		args = append(args, "--terragrunt-ignore-external-dependencies")
	}
	if viper.GetBool("terragrunt.include_external_dependencies") {
		args = append(args, "--terragrunt-include-external-dependencies")
	}

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
	rootConfigFile := viper.GetString("root_config_file")
	if rootConfigFile == "" {
		rootConfigFile = config.DefaultRootConfigFile
	}

	relativeStackPath, err := history.GetRelativeStackPath(absoluteStackPath, rootConfigFile)
	if err != nil {
		// Log warning but use absolute path as fallback
		fmt.Fprintf(os.Stderr, "Warning: Failed to calculate relative stack path: %v\n", err)
		relativeStackPath = absoluteStackPath
	}

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

	maxEntries := viper.GetInt("history.max_entries")
	if maxEntries < config.MinHistoryMaxEntries {
		maxEntries = config.DefaultHistoryMaxEntries
	}

	if err := logger.TrimHistory(ctx, maxEntries); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: Failed to trim history: %v\n", err)
	}
}
