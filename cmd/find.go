package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/israoo/terrax/internal/changes"
	"github.com/israoo/terrax/internal/stack"
)

var findCmd = &cobra.Command{
	Use:   "find",
	Short: "List stacks, optionally filtered to those affected by a git commit range",
	Long: `List Terragrunt stacks. Without --base, lists all stacks under the working directory.
With --base, lists only stacks affected by changes between <base> and HEAD, including stacks
that consume changed YAML files via mark_as_read() and stacks that transitively depend on
any directly affected stack.`,
	RunE: runFind,
}

func init() {
	findCmd.Flags().String("dir", "", "Working directory (overrides current directory)")
	findCmd.Flags().String("base", "", "Base commit SHA for change detection; omit to list all stacks")
	rootCmd.AddCommand(findCmd)
}

func runFind(cmd *cobra.Command, _ []string) error {
	dirFlag, _ := cmd.Flags().GetString("dir")
	workDir, err := getWorkingDirectory(dirFlag)
	if err != nil {
		return fmt.Errorf("failed to get working directory: %w", err)
	}

	rootConfigFile := viper.GetString("root_config_file")

	baseCommit, _ := cmd.Flags().GetString("base")

	if baseCommit == "" {
		return runFindAll(workDir, rootConfigFile)
	}
	return runFindAffected(workDir, rootConfigFile, baseCommit)
}

func runFindAll(workDir, rootConfigFile string) error {
	paths, err := stack.CollectStackPaths(workDir)
	if err != nil {
		return fmt.Errorf("failed to collect stack paths: %w", err)
	}
	for _, p := range paths {
		if _, err := fmt.Fprintln(os.Stdout, p); err != nil {
			return fmt.Errorf("failed to write output: %w", err)
		}
	}
	return nil
}

func runFindAffected(workDir, rootConfigFile, baseCommit string) error {
	graph, err := changes.BuildFileGraph(workDir, rootConfigFile)
	if err != nil {
		return fmt.Errorf("failed to build file graph: %w", err)
	}

	tree, _, err := stack.FindAndBuildTree(workDir, rootConfigFile)
	if err != nil {
		return fmt.Errorf("failed to build stack tree: %w", err)
	}

	affected, err := changes.AffectedStacks(workDir, baseCommit, graph, tree)
	if err != nil {
		return fmt.Errorf("failed to detect affected stacks: %w", err)
	}

	for _, p := range affected {
		if _, err := fmt.Fprintln(os.Stdout, p); err != nil {
			return fmt.Errorf("failed to write output: %w", err)
		}
	}
	return nil
}
