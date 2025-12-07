package tui

// UI Layout Constants
const (
	// Column layout
	ColumnOverhead    = 8  // Total overhead per column (borders, padding, spacing)
	ColumnPadding     = 4  // Padding within each column
	ColumnBorderWidth = 2  // Border width for each column
	MinColumnWidth    = 20 // Minimum width for a column

	// Header
	HeaderHeight    = 1
	DefaultMinWidth = 80 // Minimum terminal width for proper display

	// Footer
	FooterHeight = 1

	// Navigation
	FirstItemIndex = 0 // Index of the first item in a list
)

// Column types
const (
	CommandColumnType = iota
	NavigationColumnType
)

// Key bindings
const (
	KeyUp    = "up"
	KeyDown  = "down"
	KeyLeft  = "left"
	KeyRight = "right"
	KeyEnter = "enter"
	KeyCtrlC = "ctrl+c"
	KeyQ     = "q"
	KeyEsc   = "esc"
	KeySlash = "/"
)

// UI Text
const (
	AppTitle       = "TerraX - Terragrunt eXecutor"
	CommandsTitle  = "Commands"
	StacksTitle    = "Stacks"
	HelpText       = "↑↓: navigate | ←→: change column | enter: select/confirm | q: quit"
	NoItemSelected = "None"
	Initializing   = "Initializing..."
	ScanningStacks = "Scanning stacks..."
)
