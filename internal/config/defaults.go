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
