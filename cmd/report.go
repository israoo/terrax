package cmd

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/israoo/terrax/internal/config"
	"github.com/israoo/terrax/internal/deps"
	"github.com/israoo/terrax/internal/plan"
)

var reportCmd = &cobra.Command{
	Use:   "report",
	Short: "Generate a per-resource diff report from JSON plan files",
	Long: `Read Terraform JSON plan files written by Terragrunt's --json-out-dir and render
a full attribute-level diff per stack — the equivalent of 'terraform show' without
requiring the plan binary or .terragrunt-cache.

By default only stacks with pending changes are shown. Use --all to include stacks
with no changes.`,
	RunE: runReportCmd,
}

func init() {
	reportCmd.Flags().String("dir", "", "Working directory (overrides current directory)")
	reportCmd.Flags().String("plans-dir", "", "Directory for JSON plan output files (overrides plan.json_out_dir in config)")
	reportCmd.Flags().String("format", "text", "Output format: text or markdown")
	reportCmd.Flags().String("output", "", "Output file path (default: stdout)")
	reportCmd.Flags().Bool("all", false, "Include stacks with no changes")
	rootCmd.AddCommand(reportCmd)
}

func runReportCmd(cmd *cobra.Command, _ []string) error {
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

	report, err := plan.CollectFromJSONDir(ctx, jsonDir, workDir)
	if err != nil {
		return fmt.Errorf("failed to collect plan files: %w", err)
	}

	if len(report.Stacks) == 0 {
		_, _ = fmt.Fprintf(cmd.OutOrStdout(), "No plan files found in %s\n", jsonDir)
		return nil
	}

	formatFlag, _ := cmd.Flags().GetString("format")
	showAll, _ := cmd.Flags().GetBool("all")
	outputFlag, _ := cmd.Flags().GetString("output")

	var fmt_ plan.Format
	switch formatFlag {
	case "text":
		fmt_ = plan.FormatText
	case "markdown":
		fmt_ = plan.FormatMarkdown
	default:
		return fmt.Errorf("unknown format %q: use text or markdown", formatFlag)
	}

	w := cmd.OutOrStdout()
	if outputFlag != "" {
		f, err := os.Create(outputFlag)
		if err != nil {
			return fmt.Errorf("failed to create output file: %w", err)
		}
		defer func() { _ = f.Close() }()
		w = f
	}

	return plan.Report(report, plan.ReportOptions{
		Format:  fmt_,
		ShowAll: showAll,
		Writer:  w,
	})
}
