// Package config defines default configuration values and constants for TerraX.
//
// It centralizes all hardcoded defaults used across the application, including
// UI settings, history limits, and Terragrunt flags. This ensures consistency
// and provides a single source of truth for fallback values.
package config

// Default configuration values for TerraX.
const (
	// DefaultMaxNavigationColumns is the default number of navigation columns visible simultaneously.
	DefaultMaxNavigationColumns = 3

	// MinMaxNavigationColumns is the minimum allowed value for max navigation columns.
	MinMaxNavigationColumns = 1

	// DefaultHistoryMaxEntries is the default maximum number of history entries to keep.
	// When the history exceeds this limit, older entries are automatically trimmed.
	DefaultHistoryMaxEntries = 500

	// MinHistoryMaxEntries is the minimum allowed value for history max entries.
	MinHistoryMaxEntries = 10

	// DefaultRootConfigFile is the default name of the root configuration file
	// used to determine the project root directory.
	DefaultRootConfigFile = "root.hcl"

	// DefaultLogFormat is the default terragrunt log format.
	DefaultLogFormat = "pretty"

	// DefaultParallelism is the default number of modules to run in parallel.
	// 0 means use terragrunt's default.
	DefaultParallelism = 0

	// DefaultNoColor controls whether to disable colored output.
	DefaultNoColor = false

	// DefaultOutputDir is the root directory for all TerraX-generated files.
	DefaultOutputDir = ".terrax"

	// DefaultReportFile is the default path for the Terragrunt report file.
	DefaultReportFile = ".terrax/report.json"

	// DefaultReportFormat is the default format for the Terragrunt report file.
	DefaultReportFormat = "json"

	// DefaultStateRegion is the default AWS region for Terraform state operations.
	DefaultStateRegion = "us-east-1"

	// DefaultPlanReviewEnabled controls whether plan file scanning and review TUI are active.
	DefaultPlanReviewEnabled = true

	// DefaultJSONOutDir is the default output directory for Terragrunt JSON plan files.
	DefaultJSONOutDir = ".terrax/plans"

	// DefaultPlanSummaryEnabled controls whether the terminal plan summary is shown after plan execution.
	DefaultPlanSummaryEnabled = false

	// DefaultPlanCleanupEnabled controls whether generated plan files are deleted after summary.
	DefaultPlanCleanupEnabled = false
)

// DefaultCommands is the default list of Terragrunt commands shown in the TUI.
var DefaultCommands = []string{
	"plan",
	"apply",
	"validate",
	"fmt",
	"init",
	"output",
	"refresh",
	"destroy",
}
