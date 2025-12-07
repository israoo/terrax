package config

// Default configuration values for TerraX.
const (
	// DefaultMaxNavigationColumns is the default number of navigation columns visible simultaneously.
	DefaultMaxNavigationColumns = 3

	// MinMaxNavigationColumns is the minimum allowed value for max navigation columns.
	MinMaxNavigationColumns = 1
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
