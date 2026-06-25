package cmd

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/viper"

	"github.com/israoo/terrax/internal/config"
	"github.com/israoo/terrax/internal/executor"
	"github.com/israoo/terrax/internal/history"
)

// reExecuteHistoryEntry runs the command stored in entry, resolving deps and
// handling the plan summary/review flow.
func reExecuteHistoryEntry(ctx context.Context, historyService *history.Service, entry *history.ExecutionLogEntry) error {
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
		jsonOutDir := viper.GetString("plan.json_out_dir")
		if jsonOutDir == "" {
			jsonOutDir = config.DefaultJSONOutDir
		}
		var absPlansDir string
		if filepath.IsAbs(jsonOutDir) {
			absPlansDir = jsonOutDir
		} else {
			absPlansDir = filepath.Join(repoRoot, jsonOutDir)
		}
		_ = os.RemoveAll(absPlansDir)
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

	return nil
}
