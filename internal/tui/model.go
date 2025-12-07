package tui

import (
	tea "github.com/charmbracelet/bubbletea"
)

// FocusedColumn represents which column is currently focused
type FocusedColumn int

const (
	CommandColumn FocusedColumn = iota
	StackColumn
)

// Model is the main model for the multi-column TUI
type Model struct {
	// Column states
	stacks          []string // List of available stacks
	selectedStack   int      // Index of the selected stack
	commands        []string // List of available commands
	selectedCommand int      // Index of the selected command

	// Navigation state
	focused FocusedColumn // Currently focused column

	// Window dimensions
	width  int
	height int

	// Selection state
	Confirmed bool // Whether user confirmed selection
}

// NewModel creates a new instance of the TUI model
func NewModel() Model {
	return Model{
		// Mock data for demonstration
		stacks: []string{
			"infrastructure/vpc",
			"infrastructure/database",
			"infrastructure/compute",
			"applications/frontend",
			"applications/backend",
			"monitoring/prometheus",
		},
		commands: []string{
			"plan",
			"apply",
			"destroy",
			"validate",
			"init",
		},
		selectedStack:   FirstItemIndex,
		selectedCommand: FirstItemIndex,
		focused:         CommandColumn,
		Confirmed:       false,
	}
}

// Init initializes the model (required by BubbleTea)
func (m Model) Init() tea.Cmd {
	return nil
}

// Update handles messages and updates state (required by BubbleTea)
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		key := msg.String()

		// Handle quit commands
		if key == KeyCtrlC || key == KeyQ {
			return m, tea.Quit
		}

		// Handle enter confirmation
		if key == KeyEnter {
			m.Confirmed = true
			return m, tea.Quit
		}

		// Handle navigation
		m = m.handleNavigationInput(key)

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
	}

	return m, nil
}

// GetSelectedCommand returns the currently selected command
func (m Model) GetSelectedCommand() string {
	if m.selectedCommand < len(m.commands) {
		return m.commands[m.selectedCommand]
	}
	return ""
}

// GetSelectedStack returns the currently selected stack
func (m Model) GetSelectedStack() string {
	if m.selectedStack < len(m.stacks) {
		return m.stacks[m.selectedStack]
	}
	return ""
}
