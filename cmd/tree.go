package cmd

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/israoo/terrax/internal/stack"
)

var treeCmd = &cobra.Command{
	Use:   "tree",
	Short: "Print the Terragrunt stack tree as JSON",
	Long:  `Print the Terragrunt stack tree as JSON for consumption by external tools such as editor extensions.`,
	RunE:  runTree,
}

func init() {
	treeCmd.Flags().String("dir", "", "Working directory (overrides current directory)")
	rootCmd.AddCommand(treeCmd)
}

func runTree(cmd *cobra.Command, args []string) error {
	dirFlag, _ := cmd.Flags().GetString("dir")
	workDir, err := getWorkingDirectory(dirFlag)
	if err != nil {
		return fmt.Errorf("failed to get working directory: %w", err)
	}

	root, _, err := stack.FindAndBuildTree(workDir, viper.GetString("root_config_file"))
	if err != nil {
		return fmt.Errorf("failed to build stack tree: %w", err)
	}

	data, err := json.Marshal(root)
	if err != nil {
		return fmt.Errorf("failed to serialize tree: %w", err)
	}

	if _, err := fmt.Fprintln(os.Stdout, string(data)); err != nil {
		return fmt.Errorf("failed to write output: %w", err)
	}
	return nil
}
