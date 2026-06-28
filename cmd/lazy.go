// Package cmd implements the CLI commands for TerraX using Cobra.
package cmd

import (
	"context"
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/israoo/terrax/internal/deps"
	"github.com/israoo/terrax/internal/stack"
	"github.com/israoo/terrax/internal/tui"
)

var lazyCmd = &cobra.Command{
	Use:   "lazy",
	Short: "Open the multi-panel dashboard TUI",
	Long: `terrax lazy opens a persistent multi-panel dashboard:
- Top-left: indented stack tree with expand/collapse
- Top-right: real-time streaming command output
- Bottom: tabbed context (dependencies, last run, plan diff)

Use p/a/i/v/d for quick commands or Space to open the command palette.`,
	RunE: runLazy,
}

func init() {
	lazyCmd.Flags().String("dir", "", "Initial stack or directory path")
	rootCmd.AddCommand(lazyCmd)
}

func runLazy(cmd *cobra.Command, _ []string) error {
	ctx := cmd.Context()
	if ctx == nil {
		ctx = context.Background()
	}

	dirFlag, _ := cmd.Flags().GetString("dir")
	workDir, err := getWorkingDirectory(dirFlag)
	if err != nil {
		return fmt.Errorf("failed to get working directory: %w", err)
	}

	rootCfgFile := viper.GetString("root_config_file")
	repoRoot := deps.FindRepoRoot(workDir, rootCfgFile)

	root, maxDepth, err := stack.FindAndBuildTree(workDir, rootCfgFile)
	if err != nil {
		return fmt.Errorf("failed to build stack tree: %w", err)
	}

	commands := viper.GetStringSlice("commands")
	maxNavCols := viper.GetInt("max_navigation_columns")

	historyService, err := getHistoryService()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Warning: history unavailable: %v\n", err)
		historyService = nil
	}

	filterFn := tui.FilterPathsFn(func(ctx context.Context, stackPath string) (string, []string, error) {
		includeDeps := viper.GetBool("include_dependencies")
		if !includeDeps {
			return repoRoot, []string{stackPath}, nil
		}
		_, filterPaths := collectTransitiveDeps([]string{stackPath})
		return repoRoot, filterPaths, nil
	})

	m := tui.NewDashboardModel(root, maxDepth, commands, maxNavCols, ctx, filterFn, historyService)

	p := tea.NewProgram(m, tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		return fmt.Errorf("dashboard error: %w", err)
	}
	return nil
}
