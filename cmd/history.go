package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/israoo/terrax/internal/config"
	"github.com/israoo/terrax/internal/deps"
	"github.com/israoo/terrax/internal/executor"
	"github.com/israoo/terrax/internal/history"
	"github.com/israoo/terrax/internal/tui"
)

var historyCmd = &cobra.Command{
	Use:   "history",
	Short: "View or export command execution history",
	Long: `Without --json: opens the interactive TUI history viewer.
With --json: prints history for the current project as a JSON array for external tools such as editor extensions.`,
	RunE: runHistoryCmd,
}

func init() {
	historyCmd.Flags().String("dir", "", "Working directory (overrides current directory)")
	historyCmd.Flags().Bool("json", false, "Print history as JSON instead of opening the interactive TUI")
	rootCmd.AddCommand(historyCmd)
}

func runHistoryCmd(cmd *cobra.Command, args []string) error {
	jsonFlag, _ := cmd.Flags().GetBool("json")
	if jsonFlag {
		return runHistoryCmdJSON(cmd, args)
	}
	return runHistoryCmdTUI(cmd, args)
}

// runHistoryCmdJSON prints execution history for the current project as JSON.
// This is the original behavior of the history subcommand, used by external tools.
func runHistoryCmdJSON(cmd *cobra.Command, args []string) error {
	ctx := context.Background()

	dirFlag, _ := cmd.Flags().GetString("dir")
	workDir, err := getWorkingDirectory(dirFlag)
	if err != nil {
		return fmt.Errorf("failed to get working directory: %w", err)
	}

	historyService, err := getHistoryService()
	if err != nil {
		return fmt.Errorf("failed to initialize history service: %w", err)
	}

	entries, err := historyService.LoadAll(ctx)
	if err != nil {
		return fmt.Errorf("failed to load history: %w", err)
	}

	rootConfigFile := viper.GetString("root_config_file")
	if rootConfigFile == "" {
		rootConfigFile = config.DefaultRootConfigFile
	}

	repoRoot := deps.FindRepoRoot(workDir, rootConfigFile)
	if _, err := os.Stat(filepath.Join(repoRoot, rootConfigFile)); err != nil {
		// Not inside a TerraX project — return empty array.
		if _, err := fmt.Fprintln(os.Stdout, "[]"); err != nil {
			return fmt.Errorf("failed to write output: %w", err)
		}
		return nil
	}

	// FilterByCurrentProject detects the project root from os.Getwd().
	// Change to workDir first so detection uses the --dir argument.
	originalDir, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get current directory: %w", err)
	}
	defer func() { _ = os.Chdir(originalDir) }()

	if err := os.Chdir(workDir); err != nil {
		return fmt.Errorf("failed to change directory: %w", err)
	}

	filtered, err := historyService.FilterByCurrentProject(entries)
	if err != nil {
		return fmt.Errorf("failed to filter history: %w", err)
	}

	// Ensure empty slice marshals as [] not null.
	if filtered == nil {
		filtered = []history.ExecutionLogEntry{}
	}

	data, err := json.Marshal(filtered)
	if err != nil {
		return fmt.Errorf("failed to serialize history: %w", err)
	}

	if _, err := fmt.Fprintln(os.Stdout, string(data)); err != nil {
		return fmt.Errorf("failed to write output: %w", err)
	}
	return nil
}

// runHistoryCmdTUI loads and displays the execution history in an interactive TUI.
// It filters the history to show only entries from the current project.
// If the user selects an entry and presses Enter, it re-executes that command.
func runHistoryCmdTUI(cmd *cobra.Command, args []string) error {
	ctx := context.Background()

	historyService, err := getHistoryService()
	if err != nil {
		return err
	}

	entries, err := historyService.LoadAll(ctx)
	if err != nil {
		return fmt.Errorf("failed to load history: %w", err)
	}

	filteredEntries, err := historyService.FilterByCurrentProject(entries)
	if err != nil {
		// Log warning but continue with unfiltered entries.
		fmt.Fprintf(os.Stderr, "Warning: Failed to filter history: %v\n", err)
		filteredEntries = entries
	}

	initialModel := tui.NewHistoryModel(filteredEntries)

	// Note: History viewer uses Stderr specifically, so we don't use the shared runBubbleTeaProgram helper here
	// unless we update it to support custom output.
	p := tea.NewProgram(
		initialModel,
		tea.WithAltScreen(),
		tea.WithOutput(os.Stderr),
	)

	finalModel, err := p.Run()
	if err != nil {
		return fmt.Errorf("history viewer error: %w", err)
	}

	model, ok := finalModel.(tui.Model)
	if !ok {
		return fmt.Errorf("unexpected model type")
	}

	if model.ShouldReExecuteFromHistory() {
		entry := model.GetSelectedHistoryEntry()
		if entry != nil {
			fmt.Fprintf(os.Stderr, "\n🔄 Re-executing command from history...\n")
			fmt.Fprintf(os.Stderr, "Command: %s\n", entry.Command)
			fmt.Fprintf(os.Stderr, "Path: %s\n\n", entry.StackPath)

			absolutePath := entry.AbsolutePath
			if absolutePath == "" {
				// Backward compatibility: old entries only have StackPath (which was absolute).
				absolutePath = entry.StackPath
			}

			if entry.Command == "force-unlock" {
				return runForceUnlock(ctx, historyService, absolutePath)
			}

			repoRoot, filterPaths := collectTransitiveDeps(absolutePath)

			if entry.Command == "plan" && (viper.GetBool("plan.summary_enabled") || viper.GetBool("plan.review_enabled")) {
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
				if err := executor.Run(ctx, historyService, entry.Command, absolutePath, repoRoot, group.Paths, group.EnvVars); err != nil {
					return err
				}
			}
			if entry.Command == "plan" && viper.GetBool("plan.summary_enabled") {
				if err := runPlanSummary(ctx, absolutePath, repoRoot); err != nil {
					fmt.Fprintf(os.Stderr, "Warning: plan summary failed: %v\n", err)
				}
			}
			if entry.Command == "plan" && viper.GetBool("plan.review_enabled") {
				return runPlanReview(ctx, absolutePath)
			}
		}
	}

	return nil
}
