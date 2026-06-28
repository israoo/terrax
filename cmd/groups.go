package cmd

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"
)

var groupsCmd = &cobra.Command{
	Use:   "groups",
	Short: "Print stack groups and their filter lists as JSON",
	Long: `Compute stack groups from the stack_groups configuration and print them as JSON.
Useful for CI pipelines that need to orchestrate execution across different runners.`,
	RunE: runGroupsCmd,
}

func init() {
	groupsCmd.Flags().String("dir", "", "Working directory (overrides current directory)")
	rootCmd.AddCommand(groupsCmd)
}

// groupsOutput is the JSON structure for terrax groups.
type groupsOutput struct {
	Groups   []groupEntry `json:"groups"`
	RepoRoot string       `json:"repo_root"`
}

// groupEntry is one resolved group in the JSON output.
type groupEntry struct {
	Name      string            `json:"name"`
	DependsOn []string          `json:"depends_on"`
	Filters   []string          `json:"filters"`
	Env       map[string]string `json:"env"`
	Skip      bool              `json:"skip"` // When true, this group is excluded from local execution.
}

func runGroupsCmd(cmd *cobra.Command, args []string) error {
	dirFlag, _ := cmd.Flags().GetString("dir")
	workDir, err := getWorkingDirectory(dirFlag)
	if err != nil {
		return fmt.Errorf("failed to get working directory: %w", err)
	}
	ensureConfigFromWorkDir(workDir)

	repoRoot, filterPaths := collectTransitiveDeps([]string{workDir})

	groups, err := buildGroupedExecution(filterPaths, repoRoot)
	if err != nil {
		return fmt.Errorf("failed to build groups: %w", err)
	}

	output := groupsOutput{
		RepoRoot: repoRoot,
		Groups:   make([]groupEntry, 0, len(groups)),
	}
	for _, g := range groups {
		output.Groups = append(output.Groups, groupEntry{
			Name:      g.Name,
			DependsOn: g.DependsOn,
			Filters:   g.Paths,
			Env:       g.EnvVars,
			Skip:      g.Skip,
		})
	}

	data, err := json.MarshalIndent(output, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to serialize groups: %w", err)
	}
	fmt.Println(string(data))
	return nil
}
