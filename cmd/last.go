package cmd

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"

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
		return fmt.Errorf("failed to initialize history service: %w", err)
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

	return reExecuteHistoryEntry(ctx, historyService, lastEntry)
}
