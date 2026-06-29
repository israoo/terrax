package cmd

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
)

var groupsCmd = &cobra.Command{
	Use:   "groups",
	Short: "Print stack groups and their filter lists as JSON",
	Long: `Compute stack groups from the stack_groups configuration and print them as JSON.
Useful for CI pipelines that need to orchestrate execution across different runners.

Without --stack, all stacks under the working directory are classified.
With --stack, only the provided paths are classified — useful for filtering to
an already-computed set of affected stacks.`,
	RunE: runGroupsCmd,
}

func init() {
	groupsCmd.Flags().String("dir", "", "Working directory (overrides current directory)")
	groupsCmd.Flags().StringArray("stack", nil, "Explicit stack path to classify (repeatable). When set, skips directory scan.")
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

	stackFlags, _ := cmd.Flags().GetStringArray("stack")

	var seeds []string
	if len(stackFlags) > 0 {
		// Resolve each provided path to absolute before passing to collectTransitiveDeps.
		for _, s := range stackFlags {
			if filepath.IsAbs(s) || strings.HasPrefix(s, "/") {
				seeds = append(seeds, s)
			} else {
				seeds = append(seeds, filepath.Join(workDir, s))
			}
		}
	} else {
		seeds = []string{workDir}
	}

	repoRoot, filterPaths := collectTransitiveDeps(seeds)

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
	_, _ = fmt.Fprintln(cmd.OutOrStdout(), string(data))
	return nil
}
