package cmd

import (
	"context"
	"fmt"
	"path/filepath"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/israoo/terrax/internal/config"
	"github.com/israoo/terrax/internal/deps"
	"github.com/israoo/terrax/internal/plan"
)

var summaryCmd = &cobra.Command{
	Use:   "summary",
	Short: "Print a terminal summary of pending plan changes",
	Long:  `Print a grouped terminal summary of pending vs. no-change stacks from existing plan files in .terrax/plans/.`,
	RunE:  runSummaryCmd,
}

func init() {
	summaryCmd.Flags().String("dir", "", "Working directory (overrides current directory)")
	rootCmd.AddCommand(summaryCmd)
}

func runSummaryCmd(cmd *cobra.Command, args []string) error {
	ctx := context.Background()

	dirFlag, _ := cmd.Flags().GetString("dir")
	workDir, err := getWorkingDirectory(dirFlag)
	if err != nil {
		return fmt.Errorf("failed to get working directory: %w", err)
	}

	ensureConfigFromWorkDir(workDir)
	workDir = resolveWorkDir(workDir)

	rootConfigFile := viper.GetString("root_config_file")
	if rootConfigFile == "" {
		rootConfigFile = config.DefaultRootConfigFile
	}

	repoRoot := deps.FindRepoRoot(workDir, rootConfigFile)
	jsonDir := filepath.Join(repoRoot, config.DefaultJSONOutDir)

	_, err = plan.Summarize(ctx, jsonDir, repoRoot)
	return err
}
