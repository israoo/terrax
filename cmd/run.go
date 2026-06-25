package cmd

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"slices"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/israoo/terrax/internal/config"
	"github.com/israoo/terrax/internal/executor"
)

var runCmd = &cobra.Command{
	Use:   "run <command>",
	Short: "Execute a Terragrunt command directly without the TUI",
	Long:  `Execute a Terragrunt command on a directory directly, without opening the interactive TUI.`,
	Args:  cobra.ExactArgs(1),
	RunE:  runCommand,
}

func init() {
	runCmd.Flags().String("dir", "", "Working directory (overrides current directory)")
	runCmd.Flags().String("plans-dir", "", "Directory for JSON plan output files (overrides plan.json_out_dir in config)")
	rootCmd.AddCommand(runCmd)
}

func runCommand(cmd *cobra.Command, args []string) error {
	ctx := context.Background()
	command := args[0]

	validCommands := viper.GetStringSlice("commands")
	if len(validCommands) == 0 {
		validCommands = config.DefaultCommands
	}
	if !slices.Contains(validCommands, command) {
		return fmt.Errorf("unknown command %q: must be one of %v", command, validCommands)
	}

	dirFlag, _ := cmd.Flags().GetString("dir")
	workDir, err := getWorkingDirectory(dirFlag)
	if err != nil {
		return fmt.Errorf("failed to get working directory: %w", err)
	}
	ensureConfigFromWorkDir(workDir)

	if plansDir, _ := cmd.Flags().GetString("plans-dir"); plansDir != "" {
		viper.Set("plan.json_out_dir", plansDir)
	}

	historyService, err := getHistoryService()
	if err != nil {
		return fmt.Errorf("failed to initialize history service: %w", err)
	}

	repoRoot, filterPaths := collectTransitiveDeps(workDir)

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
		if err := executor.Run(ctx, historyService, command, workDir, repoRoot, group.Paths, group.EnvVars); err != nil {
			return err
		}
	}

	if command == "plan" && viper.GetBool("plan.summary_enabled") {
		if err := runPlanSummary(ctx, workDir, repoRoot); err != nil {
			return err
		}
	}
	if command == "plan" && viper.GetBool("plan.review_enabled") {
		return runPlanReview(ctx, workDir)
	}
	return nil
}
