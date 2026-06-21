package cmd

import (
	"context"
	"fmt"
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

	historyService, err := getHistoryService()
	if err != nil {
		return fmt.Errorf("failed to initialize history service: %w", err)
	}

	repoRoot, filterPaths := collectTransitiveDeps(workDir)
	return executor.Run(ctx, historyService, command, workDir, repoRoot, filterPaths)
}
