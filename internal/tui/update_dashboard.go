package tui

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/israoo/terrax/internal/executor"
	"github.com/israoo/terrax/internal/history"
)

// CancelledMsg is sent internally when the user quits while a command is running,
// signalling that the quit should proceed after the command finishes.
type CancelledMsg struct{}

// OutputLineMsg carries one line of streaming command output.
type OutputLineMsg struct{ Line string }

// CommandFinishedMsg is sent when a streaming command completes.
type CommandFinishedMsg struct {
	ExitCode int
	Duration time.Duration
	Err      error
}

// commandResult is passed from the streaming goroutine to readNextLine via a channel.
type commandResult struct {
	exitCode int
	duration time.Duration
	err      error
}

// lineScanner wraps a bufio.Scanner and the done channel for a streaming command.
type lineScanner struct {
	scanner *bufio.Scanner
	done    chan commandResult
}

// readNextLine returns a tea.Cmd that reads one line from ls.
// When the scanner is exhausted it reads the commandResult and returns CommandFinishedMsg.
func readNextLine(ls *lineScanner) tea.Cmd {
	return func() tea.Msg {
		if ls.scanner.Scan() {
			return OutputLineMsg{Line: ls.scanner.Text()}
		}
		result := <-ls.done
		return CommandFinishedMsg{
			ExitCode: result.exitCode,
			Duration: result.duration,
			Err:      result.err,
		}
	}
}

// handleDashboardUpdate handles all Bubble Tea messages for StateDashboard.
func (m Model) handleDashboardUpdate(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.ready = true
		if len(m.dashTreeNodes) == 0 {
			m.dashTreeNodes = flattenDashTree(m.navigator.GetRoot(), m.dashTreeExpanded)
		}
		return m, nil

	case OutputLineMsg:
		m.dashOutputLines = append(m.dashOutputLines, msg.Line)
		if m.dashAutoScroll {
			m.dashOutputOffset = max(0, len(m.dashOutputLines)-m.dashOutputVisibleLines())
		}
		return m, readNextLine(m.dashScanner)

	case CommandFinishedMsg:
		m = m.finishCommand(msg)
		if m.dashQuitPending {
			return m, tea.Quit
		}
		return m, nil

	case tea.KeyMsg:
		return m.handleDashboardKey(msg)
	}
	return m, nil
}

// handleDashboardKey routes key events based on focused panel.
func (m Model) handleDashboardKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// Global keys.
	switch msg.String() {
	case "q", "esc":
		if m.dashPaletteOpen {
			m.dashPaletteOpen = false
			return m, nil
		}
		if m.dashRunning && m.dashCancelCmd != nil {
			m.dashCancelCmd()
			m.dashQuitPending = true
			return m, nil // wait for CommandFinishedMsg to drain and log history
		}
		return m, tea.Quit
	case "tab":
		m.dashFocusedPanel = (m.dashFocusedPanel + 1) % 3
		return m, nil
	}

	if m.dashPaletteOpen {
		return m.handlePaletteKey(msg)
	}

	switch m.dashFocusedPanel {
	case 0:
		return m.handleTreeKey(msg)
	case 1:
		return m.handleOutputKey(msg)
	case 2:
		return m.handleBottomKey(msg)
	}
	return m, nil
}

// handleTreeKey handles key events when the tree panel is focused.
func (m Model) handleTreeKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "up":
		if m.dashTreeCursor > 0 {
			m.dashTreeCursor--
		}
	case "down":
		if m.dashTreeCursor < len(m.dashTreeNodes)-1 {
			m.dashTreeCursor++
		}
	case "enter":
		if len(m.dashTreeNodes) == 0 {
			break
		}
		node := m.dashTreeNodes[m.dashTreeCursor].Node
		if node.HasChildren() {
			if m.dashTreeExpanded[node.Path] {
				delete(m.dashTreeExpanded, node.Path)
			} else {
				m.dashTreeExpanded[node.Path] = true
			}
			m.dashTreeNodes = flattenDashTree(m.navigator.GetRoot(), m.dashTreeExpanded)
			// Keep cursor in bounds after re-flatten.
			if m.dashTreeCursor >= len(m.dashTreeNodes) {
				m.dashTreeCursor = len(m.dashTreeNodes) - 1
			}
		}
	case " ":
		m.dashPaletteOpen = true
		m.dashPaletteCursor = 0
	default:
		if !m.dashRunning {
			return m.handleCommandKey(msg.String())
		}
	}
	return m, nil
}

// handleCommandKey dispatches a command shortcut key from the tree panel.
func (m Model) handleCommandKey(key string) (Model, tea.Cmd) {
	cmdMap := map[string]string{
		"p": "plan",
		"a": "apply",
		"i": "init",
		"v": "validate",
		"d": "destroy",
	}
	command, ok := cmdMap[key]
	if !ok {
		return m, nil
	}
	return m.dispatchCommand(command)
}

// handleOutputKey handles key events when the output panel is focused.
func (m Model) handleOutputKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	visLines := m.dashOutputVisibleLines()
	switch msg.String() {
	case "up":
		if m.dashOutputOffset > 0 {
			m.dashOutputOffset--
			m.dashAutoScroll = false
		}
	case "down":
		newOffset := m.dashOutputOffset + 1
		maxOffset := max(0, len(m.dashOutputLines)-visLines)
		if newOffset <= maxOffset {
			m.dashOutputOffset = newOffset
		}
		if m.dashOutputOffset >= maxOffset {
			m.dashAutoScroll = true
		}
	case "pgup":
		m.dashOutputOffset = max(0, m.dashOutputOffset-visLines)
		m.dashAutoScroll = false
	case "pgdown":
		maxOffset := max(0, len(m.dashOutputLines)-visLines)
		m.dashOutputOffset = min(m.dashOutputOffset+visLines, maxOffset)
		if m.dashOutputOffset >= maxOffset {
			m.dashAutoScroll = true
		}
	case "c":
		m.dashOutputLines = nil
		m.dashOutputOffset = 0
	case "ctrl+c":
		if m.dashCancelCmd != nil {
			m.dashCancelCmd()
		}
	}
	return m, nil
}

// handleBottomKey handles key events when the bottom panel is focused.
func (m Model) handleBottomKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "[":
		m.dashActiveTab = max(0, m.dashActiveTab-1)
		m.dashBottomOffset = 0
	case "]":
		m.dashActiveTab = min(m.dashActiveTab+1, 2)
		m.dashBottomOffset = 0
	case "up":
		if m.dashBottomOffset > 0 {
			m.dashBottomOffset--
		}
	case "down":
		m.dashBottomOffset++
	}
	return m, nil
}

// handlePaletteKey handles key events when the command palette is open.
func (m Model) handlePaletteKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.dashPaletteOpen = false
	case "up":
		if m.dashPaletteCursor > 0 {
			m.dashPaletteCursor--
		}
	case "down":
		if m.dashPaletteCursor < len(m.commands)-1 {
			m.dashPaletteCursor++
		}
	case "enter":
		if !m.dashRunning && m.dashPaletteCursor < len(m.commands) {
			command := m.commands[m.dashPaletteCursor]
			m.dashPaletteOpen = false
			newM, cmd := m.dispatchCommand(command)
			return newM, cmd
		}
	}
	return m, nil
}

// dispatchCommand starts a streaming Terragrunt command on the currently selected stack.
// No-op if the cursor is on a directory node or a command is already running.
func (m Model) dispatchCommand(command string) (Model, tea.Cmd) {
	if m.dashRunning || len(m.dashTreeNodes) == 0 {
		return m, nil
	}
	node := m.dashTreeNodes[m.dashTreeCursor].Node
	if node.HasChildren() {
		return m, nil // directories are not executable
	}

	repoRoot, filterPaths, err := m.dashFilterFn(m.dashCtx, node.Path)
	if err != nil {
		m.dashOutputLines = []string{fmt.Sprintf("Error: %v", err)}
		return m, nil
	}

	pr, pw := io.Pipe()
	done := make(chan commandResult, 1)

	cmdCtx, cancel := context.WithCancel(m.dashCtx)
	m.dashCancelCmd = cancel
	m.dashRunning = true
	m.dashRunCmd = command
	m.dashOutputLines = nil
	m.dashOutputOffset = 0
	m.dashAutoScroll = true
	m.dashStartTime = time.Now()

	ls := &lineScanner{
		scanner: bufio.NewScanner(pr),
		done:    done,
	}
	m.dashScanner = ls

	absStackPath := node.Path
	startTime := m.dashStartTime

	go func() {
		defer func() { _ = pw.Close() }()
		runErr := executor.RunStreaming(cmdCtx, pw, command, absStackPath, repoRoot, filterPaths, nil)
		exitCode := 0
		if runErr != nil {
			var exitErr *exec.ExitError
			if errors.As(runErr, &exitErr) {
				exitCode = exitErr.ExitCode()
			} else if errors.Is(runErr, context.Canceled) {
				exitCode = -1
			} else {
				exitCode = 1
			}
		}
		done <- commandResult{
			exitCode: exitCode,
			duration: time.Since(startTime),
			err:      runErr,
		}
	}()

	return m, readNextLine(ls)
}

// finishCommand handles a CommandFinishedMsg: clears running state and logs to history.
func (m Model) finishCommand(msg CommandFinishedMsg) Model {
	m.dashRunning = false
	m.dashCancelCmd = nil
	m.dashScanner = nil
	m.dashPlanReport = nil // cleared on each new run; repopulated by Task 5 plan collector

	if len(m.dashTreeNodes) == 0 {
		return m
	}
	node := m.dashTreeNodes[m.dashTreeCursor].Node

	summary := "completed"
	switch {
	case msg.ExitCode < 0:
		summary = "cancelled"
	case msg.ExitCode != 0:
		summary = fmt.Sprintf("failed (exit %d)", msg.ExitCode)
	}

	entry := history.ExecutionLogEntry{
		Timestamp:    time.Now().Add(-msg.Duration),
		User:         history.GetCurrentUser(),
		StackPath:    node.Path,
		AbsolutePath: node.Path,
		Command:      m.dashRunCmd,
		ExitCode:     msg.ExitCode,
		DurationS:    msg.Duration.Seconds(),
		Summary:      summary,
	}

	if m.dashHistoryLogger != nil {
		nextID, err := m.dashHistoryLogger.GetNextID(m.dashCtx)
		if err == nil {
			entry.ID = nextID
		}
		_ = m.dashHistoryLogger.Append(m.dashCtx, entry)
	}

	m.dashLastRun = &entry
	return m
}

// dashOutputVisibleLines returns the number of lines that fit in the output panel.
func (m Model) dashOutputVisibleLines() int {
	bottomH := m.height * 30 / 100
	// Header(1) + header-line(1) + footer(1).
	outputH := m.height - bottomH - HeaderHeight - 3
	if outputH < 1 {
		return 1
	}
	return outputH
}
