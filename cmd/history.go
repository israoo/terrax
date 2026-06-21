package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/israoo/terrax/internal/history"
)

var historyCmd = &cobra.Command{
	Use:   "history",
	Short: "Print command execution history as JSON",
	Long:  `Print command execution history for the current project as JSON for consumption by external tools such as editor extensions.`,
	RunE:  runHistoryCmd,
}

func init() {
	historyCmd.Flags().String("dir", "", "Working directory (overrides current directory)")
	rootCmd.AddCommand(historyCmd)
}

func runHistoryCmd(cmd *cobra.Command, args []string) error {
	ctx := context.Background()

	dirFlag, _ := cmd.Flags().GetString("dir")
	workDir, err := getWorkingDirectory(dirFlag)
	if err != nil {
		return fmt.Errorf("failed to get working directory: %w", err)
	}

	historyService, err := getHistoryService()
	if err != nil {
		return fmt.Errorf("failed to initialize history service: %w", err)
	}

	entries, err := historyService.LoadAll(ctx)
	if err != nil {
		return fmt.Errorf("failed to load history: %w", err)
	}

	// FilterByCurrentProject detects the project root from os.Getwd().
	// Change to workDir first so detection uses the --dir argument.
	if err := os.Chdir(workDir); err != nil {
		return fmt.Errorf("failed to change directory: %w", err)
	}

	filtered, err := historyService.FilterByCurrentProject(entries)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Warning: failed to filter history: %v\n", err)
		filtered = entries
	}

	// Reverse to most-recent-first order.
	for i, j := 0, len(filtered)-1; i < j; i, j = i+1, j-1 {
		filtered[i], filtered[j] = filtered[j], filtered[i]
	}

	// Ensure empty slice marshals as [] not null.
	if filtered == nil {
		filtered = []history.ExecutionLogEntry{}
	}

	data, err := json.Marshal(filtered)
	if err != nil {
		return fmt.Errorf("failed to serialize history: %w", err)
	}

	if _, err := fmt.Fprintln(os.Stdout, string(data)); err != nil {
		return fmt.Errorf("failed to write output: %w", err)
	}
	return nil
}
