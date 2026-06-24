package history

import (
	"time"
)

// ExecutionLogEntry represents a single command execution record in the history log.
// Each entry is persisted as a single line in JSONL format for easy appending and parsing.
type ExecutionLogEntry struct {
	ID           int       `json:"id"`            // Unique incremental identifier
	Timestamp    time.Time `json:"timestamp"`     // Execution start time
	User         string    `json:"user"`          // OS user who executed the command (for audit)
	StackPath    string    `json:"stack_path"`    // Relative stack path from project root (for display)
	AbsolutePath string    `json:"absolute_path"` // Absolute path to stack directory (for execution)
	Command      string    `json:"command"`       // Terragrunt command executed (plan, apply, etc.)
	ExitCode     int       `json:"exit_code"`     // Process exit code (0 = success)
	DurationS    float64   `json:"duration_s"`    // Execution duration in seconds
	Summary      string    `json:"summary"`       // Brief result summary (e.g., "3 added, 0 changed")
}
