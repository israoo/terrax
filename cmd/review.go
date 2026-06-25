package cmd

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var reviewCmd = &cobra.Command{
	Use:   "review",
	Short: "Open the plan review TUI from the last plan execution",
	Long:  `Open the plan review TUI without re-running the plan. Reads plan output from .terrax/plans/.`,
	RunE:  runReviewCmd,
}

func init() {
	reviewCmd.Flags().String("dir", "", "Working directory (overrides current directory)")
	reviewCmd.Flags().String("plans-dir", "", "Directory for JSON plan output files (overrides plan.json_out_dir in config)")
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

	if plansDir, _ := cmd.Flags().GetString("plans-dir"); plansDir != "" {
		viper.Set("plan.json_out_dir", plansDir)
	}

	workDir = resolveWorkDir(workDir)

	return runPlanReview(ctx, workDir)
}
