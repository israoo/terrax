package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
)

var (
	dashOutputHeaderStyle  = lipgloss.NewStyle().Bold(true).Foreground(secondaryColor)
	dashOutputSuccessStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#00FF87"))
	dashOutputFailStyle    = lipgloss.NewStyle().Foreground(accentColor)
	dashOutputLineStyle    = lipgloss.NewStyle().Foreground(textColor)
)

// renderDashboardOutput renders the output panel (header + scrollable line buffer).
//
//nolint:unused
func (m Model) renderDashboardOutput(width, height int) string {
	header := m.dashOutputHeader()
	var sb strings.Builder
	sb.WriteString(header + "\n")
	sb.WriteString(strings.Repeat("─", width) + "\n")

	contentHeight := height - 2 // header line + separator
	if contentHeight < 1 {
		contentHeight = 1
	}

	start := m.dashOutputOffset
	end := min(start+contentHeight, len(m.dashOutputLines))

	for i := start; i < end; i++ {
		line := m.dashOutputLines[i]
		if len(line) > width {
			line = line[:width]
		}
		sb.WriteString(dashOutputLineStyle.Render(line) + "\n")
	}

	// Fill remaining lines with empty space for consistent height.
	rendered := end - start
	for range contentHeight - rendered {
		sb.WriteString("\n")
	}

	return sb.String()
}

// dashOutputHeader returns the one-line header for the output panel.
func (m Model) dashOutputHeader() string {
	if m.dashRunning {
		spinner := spinnerFrame()
		return dashOutputHeaderStyle.Render(fmt.Sprintf("%s terragrunt %s", spinner, m.dashRunCmd))
	}
	if m.dashLastRun != nil && m.dashLastRun.Command == m.dashRunCmd && m.dashRunCmd != "" {
		dur := time.Duration(m.dashLastRun.DurationS * float64(time.Second)).Round(time.Millisecond)
		if m.dashLastRun.ExitCode == 0 {
			return dashOutputSuccessStyle.Render(fmt.Sprintf("✓ %s — %s", m.dashRunCmd, dur))
		}
		return dashOutputFailStyle.Render(fmt.Sprintf("✗ %s — %s", m.dashRunCmd, dur))
	}
	return dashOutputHeaderStyle.Render("⚡ Output")
}

// spinnerFrame returns a simple spinner character cycling on time.
func spinnerFrame() string {
	frames := []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}
	idx := (time.Now().UnixMilli() / 80) % int64(len(frames))
	return frames[idx]
}
