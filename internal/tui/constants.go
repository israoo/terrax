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

	// Item rendering
	CursorWidth         = 2  // Width of cursor and space: "► "
	ItemStylePadding    = 2  // Item style padding (left + right)
	ColumnStylePadding  = 6  // Column padding (unfocused: 2,3 = 6 total)
	EllipsisWidth       = 3  // Width of truncation ellipsis "..."
	MinItemTextWidth    = 10 // Minimum width for item text
	BreadcrumbLineCount = 1  // Number of lines for breadcrumb bar

	// Plan Review Layout
	PlanMasterWidthRatio = 3  // 1/3 of screen width
	PlanDetailMargin     = 7  // Overhead for detail view width calculation
	PlanMinDetailWidth   = 10 // Minimum width for detail view
	PlanVerticalFrame    = 2  // Vertical border overhead
	PlanContentFrame     = 6  // Vertical overhead for content (header + padding)
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
	HelpText       = "↑↓: navigate | ←→: change column | enter: select/confirm | q/esc: quit"
	PlanHelpText   = "↑↓: navigate | ←→: change column | PgUp/PgDn: scroll | q/esc: quit"
	NoItemSelected = "None"
	Initializing   = "Initializing..."
	ScanningStacks = "Scanning stacks..."
)
