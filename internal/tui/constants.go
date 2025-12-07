package tui

// UI Layout Constants
const (
	// Column layout
	ColumnOverhead    = 8 // Total overhead for both columns (borders, padding, spacing)
	ColumnPadding     = 4 // Padding within each column
	ColumnBorderWidth = 2 // Border width for each column
	NumberOfColumns   = 2 // Total number of columns in the UI

	// Header
	HeaderHeight    = 1
	DefaultMinWidth = 80 // Minimum terminal width for proper display

	// Footer
	FooterHeight = 1

	// Navigation
	FirstItemIndex = 0 // Index of the first item in a list
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

	// Vim-style navigation
	KeyH = "h"
	KeyJ = "j"
	KeyK = "k"
	KeyL = "l"
)

// UI Text
const (
	AppTitle       = "Terrax - Terra eXecutor"
	CommandsTitle  = "Commands"
	StacksTitle    = "Stacks"
	HelpText       = "↑↓/jk: navigate | ←→/hl: change column | enter: confirm | q: quit"
	NoItemSelected = "None"
)
