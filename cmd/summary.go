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
	summaryCmd.Flags().String("plans-dir", "", "Directory for JSON plan output files (overrides plan.json_out_dir in config)")
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

	if plansDir, _ := cmd.Flags().GetString("plans-dir"); plansDir != "" {
		viper.Set("plan.json_out_dir", plansDir)
	}

	rootConfigFile := viper.GetString("root_config_file")
	if rootConfigFile == "" {
		rootConfigFile = config.DefaultRootConfigFile
	}

	repoRoot := deps.FindRepoRoot(workDir, rootConfigFile)

	jsonOutDir := viper.GetString("plan.json_out_dir")
	if jsonOutDir == "" {
		jsonOutDir = config.DefaultJSONOutDir
	}
	var jsonDir string
	if filepath.IsAbs(jsonOutDir) {
		jsonDir = jsonOutDir
	} else {
		jsonDir = filepath.Join(repoRoot, jsonOutDir)
	}

	_, err = plan.Summarize(ctx, jsonDir, repoRoot)
	return err
}
