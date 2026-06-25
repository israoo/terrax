package cmd

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"
)

var reviewCmd = &cobra.Command{
	Use:   "review",
	Short: "Open the plan review TUI from the last plan execution",
	Long:  `Open the plan review TUI without re-running the plan. Reads plan output from .terrax/plans/.`,
	RunE:  runReviewCmd,
}

func init() {
	reviewCmd.Flags().String("dir", "", "Working directory (overrides current directory)")
	rootCmd.AddCommand(reviewCmd)
}

func runReviewCmd(cmd *cobra.Command, args []string) error {
	ctx := context.Background()

	dirFlag, _ := cmd.Flags().GetString("dir")
	workDir, err := getWorkingDirectory(dirFlag)
	if err != nil {
		return fmt.Errorf("failed to get working directory: %w", err)
	}

	ensureConfigFromWorkDir(workDir)

	return runPlanReview(ctx, workDir)
}
