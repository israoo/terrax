package history

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/user"
	"path/filepath"
	"time"

	"github.com/adrg/xdg"
)

// ExecutionLogEntry represents a single command execution record in the history log.
// Each entry is persisted as a single line in JSONL format for easy appending and parsing.
type ExecutionLogEntry struct {
	ID        int       `json:"id"`         // Unique incremental identifier
	Timestamp time.Time `json:"timestamp"`  // Execution start time
	User      string    `json:"user"`       // OS user who executed the command (for audit)
	StackPath string    `json:"stack_path"` // Terragrunt stack directory path (for replay)
	Command   string    `json:"command"`    // Terragrunt command executed (plan, apply, etc.)
	ExitCode  int       `json:"exit_code"`  // Process exit code (0 = success)
	DurationS float64   `json:"duration_s"` // Execution duration in seconds
	Summary   string    `json:"summary"`    // Brief result summary (e.g., "3 added, 0 changed")
}

const (
	// HistoryFileName is the name of the history log file
	HistoryFileName = "history.log"
	// ConfigDirName is the application configuration directory name
	ConfigDirName = "terrax"
)

// historyFilePathFunc is a variable that holds the function to get the history file path.
// This allows for easy testing by overriding the function in tests.
var historyFilePathFunc = getHistoryFilePathImpl

// GetHistoryFilePath constructs and returns the full path to the history log file.
// The file is located in the user's configuration directory following XDG Base Directory spec:
//   - Linux/BSD: ~/.config/terrax/history.log
//   - macOS: ~/Library/Application Support/terrax/history.log
//   - Windows: %LOCALAPPDATA%\terrax\history.log
//
// The directory is created if it doesn't exist.
func GetHistoryFilePath() (string, error) {
	return historyFilePathFunc()
}

// getHistoryFilePathImpl is the actual implementation of GetHistoryFilePath.
func getHistoryFilePathImpl() (string, error) {
	// Construct config directory path using XDG base directory spec
	configDir := filepath.Join(xdg.ConfigHome, ConfigDirName)

	// Ensure the directory exists
	if err := os.MkdirAll(configDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create config directory: %w", err)
	}

	return filepath.Join(configDir, HistoryFileName), nil
}

// AppendToHistory appends a single execution log entry to the history file in JSONL format.
// Each entry is written as a single line of JSON followed by a newline character.
//
// JSONL (JSON Lines) format benefits:
//   - Easy to append new entries without parsing the entire file
//   - Each line is independently parseable
//   - Resistant to corruption (one bad line doesn't break the whole file)
//   - Efficient for streaming and tail operations
//
// The function:
//  1. Opens the history file in append mode (creates if doesn't exist)
//  2. Serializes the entry to JSON
//  3. Writes the JSON followed by \n
//  4. Syncs to disk and closes the file
func AppendToHistory(ctx context.Context, entry ExecutionLogEntry) error {
	historyPath, err := GetHistoryFilePath()
	if err != nil {
		return fmt.Errorf("failed to get history file path: %w", err)
	}

	// Open file in append mode, create if doesn't exist
	// 0644 = rw-r--r-- (owner can read/write, others can read)
	file, err := os.OpenFile(historyPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("failed to open history file: %w", err)
	}
	defer func() {
		if closeErr := file.Close(); closeErr != nil && err == nil {
			err = fmt.Errorf("failed to close history file: %w", closeErr)
		}
	}()

	// Serialize entry to JSON
	jsonData, err := json.Marshal(entry)
	if err != nil {
		return fmt.Errorf("failed to marshal entry to JSON: %w", err)
	}

	// Write JSON line + newline (JSONL format)
	if _, err := file.Write(append(jsonData, '\n')); err != nil {
		return fmt.Errorf("failed to write entry to history: %w", err)
	}

	// Ensure data is written to disk
	if err := file.Sync(); err != nil {
		return fmt.Errorf("failed to sync history file: %w", err)
	}

	return nil
}

// TrimHistory implements a simple log rotation strategy by keeping only the most recent N entries.
// This prevents the history file from growing unbounded.
//
// Algorithm:
//  1. Read all lines from history.log
//  2. If total lines > maxEntries, keep only the last maxEntries lines
//  3. Overwrite history.log with the trimmed content
//
// Trade-offs:
//   - Simple to implement and understand
//   - Requires reading entire file into memory (acceptable for history logs)
//   - Atomic replacement prevents partial corruption
//
// For very large maxEntries (>100k), consider alternative strategies:
//   - File rotation with multiple numbered files (history.log.1, history.log.2, etc.)
//   - SQLite database for efficient querying and indexing
func TrimHistory(ctx context.Context, maxEntries int) error {
	if maxEntries <= 0 {
		return fmt.Errorf("maxEntries must be positive, got: %d", maxEntries)
	}

	historyPath, err := GetHistoryFilePath()
	if err != nil {
		return fmt.Errorf("failed to get history file path: %w", err)
	}

	// Open file for reading
	file, err := os.Open(historyPath)
	if err != nil {
		if os.IsNotExist(err) {
			// File doesn't exist yet, nothing to trim
			return nil
		}
		return fmt.Errorf("failed to open history file: %w", err)
	}
	defer func() {
		if closeErr := file.Close(); closeErr != nil && err == nil {
			err = fmt.Errorf("failed to close history file: %w", closeErr)
		}
	}()

	// Read all lines
	var lines []string
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}

	if err := scanner.Err(); err != nil {
		return fmt.Errorf("failed to read history file: %w", err)
	}

	// Check if trimming is needed
	totalLines := len(lines)
	if totalLines <= maxEntries {
		// No trimming needed
		return nil
	}

	// Keep only the last maxEntries lines
	trimmedLines := lines[totalLines-maxEntries:]

	// Write trimmed content back to file (atomic replace)
	// Use temp file + rename for atomic operation
	tempPath := historyPath + ".tmp"
	tempFile, err := os.Create(tempPath)
	if err != nil {
		return fmt.Errorf("failed to create temp file: %w", err)
	}

	writer := bufio.NewWriter(tempFile)
	for _, line := range trimmedLines {
		if _, err := writer.WriteString(line + "\n"); err != nil {
			_ = tempFile.Close()
			_ = os.Remove(tempPath)
			return fmt.Errorf("failed to write to temp file: %w", err)
		}
	}

	if err := writer.Flush(); err != nil {
		_ = tempFile.Close()
		_ = os.Remove(tempPath)
		return fmt.Errorf("failed to flush temp file: %w", err)
	}

	if err := tempFile.Sync(); err != nil {
		_ = tempFile.Close()
		_ = os.Remove(tempPath)
		return fmt.Errorf("failed to sync temp file: %w", err)
	}

	if err := tempFile.Close(); err != nil {
		_ = os.Remove(tempPath)
		return fmt.Errorf("failed to close temp file: %w", err)
	}

	// Atomically replace original file with trimmed version
	if err := os.Rename(tempPath, historyPath); err != nil {
		_ = os.Remove(tempPath)
		return fmt.Errorf("failed to replace history file: %w", err)
	}

	return nil
}

// GetCurrentUser returns the current OS username for audit purposes.
// Falls back to "unknown" if user cannot be determined.
func GetCurrentUser() string {
	currentUser, err := user.Current()
	if err != nil {
		return "unknown"
	}
	return currentUser.Username
}

// GetNextID reads the history file and returns the next available ID.
// Returns 1 if the file is empty or doesn't exist.
func GetNextID(ctx context.Context) (int, error) {
	historyPath, err := GetHistoryFilePath()
	if err != nil {
		return 0, fmt.Errorf("failed to get history file path: %w", err)
	}

	file, err := os.Open(historyPath)
	if err != nil {
		if os.IsNotExist(err) {
			// File doesn't exist, start with ID 1
			return 1, nil
		}
		return 0, fmt.Errorf("failed to open history file: %w", err)
	}
	defer func() {
		if closeErr := file.Close(); closeErr != nil && err == nil {
			err = fmt.Errorf("failed to close history file: %w", closeErr)
		}
	}()

	// Read the last line to get the highest ID
	var lastID int
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		var entry ExecutionLogEntry
		if err := json.Unmarshal([]byte(scanner.Text()), &entry); err != nil {
			// Skip invalid lines
			continue
		}
		if entry.ID > lastID {
			lastID = entry.ID
		}
	}

	if err := scanner.Err(); err != nil {
		return 0, fmt.Errorf("failed to read history file: %w", err)
	}

	return lastID + 1, nil
}
