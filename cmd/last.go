package cmd

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/israoo/terrax/internal/config"
	"github.com/israoo/terrax/internal/executor"
	"github.com/israoo/terrax/internal/history"
)

var lastCmd = &cobra.Command{
	Use:   "last",
	Short: "Re-execute the last command from history",
	Long:  `Re-execute the most recent command from the execution history for the current project.`,
	RunE:  runLastCmd,
}

func init() {
	rootCmd.AddCommand(lastCmd)
}

func runLastCmd(cmd *cobra.Command, args []string) error {
	ctx := context.Background()

	historyService, err := getHistoryService()
	if err != nil {
		return err
	}

	return executeLastCommand(ctx, historyService)
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

	// (StackPath is relative for display, AbsolutePath is for execution).
	absolutePath := lastEntry.AbsolutePath
	if absolutePath == "" {
		// Backward compatibility: old entries only have StackPath (which was absolute).
		absolutePath = lastEntry.StackPath
	}

	if lastEntry.Command == "force-unlock" {
		return runForceUnlock(ctx, historyService, absolutePath)
	}

	repoRoot, filterPaths := collectTransitiveDeps(absolutePath)

	if lastEntry.Command == "plan" && (viper.GetBool("plan.summary_enabled") || viper.GetBool("plan.review_enabled")) {
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
		if err := executor.Run(ctx, historyService, lastEntry.Command, absolutePath, repoRoot, group.Paths, group.EnvVars); err != nil {
			return err
		}
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
