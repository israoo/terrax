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

// FindProjectRoot searches for the project root by looking for the root config file.
// It starts from the given path and walks up the directory tree until it finds
// the specified root config file or reaches the filesystem root.
// Returns the directory containing the root config file, or empty string if not found.
func FindProjectRoot(startPath, rootConfigFile string) (string, error) {
	absPath, err := filepath.Abs(startPath)
	if err != nil {
		return "", fmt.Errorf("failed to get absolute path: %w", err)
	}

	currentDir := absPath
	// If startPath is a file, start from its directory
	if info, err := os.Stat(currentDir); err == nil && !info.IsDir() {
		currentDir = filepath.Dir(currentDir)
	}

	// Walk up the directory tree
	for {
		rootConfigPath := filepath.Join(currentDir, rootConfigFile)
		if _, err := os.Stat(rootConfigPath); err == nil {
			// Found the root config file
			return currentDir, nil
		}

		// Move to parent directory
		parentDir := filepath.Dir(currentDir)

		// Check if we've reached the root of the filesystem
		if parentDir == currentDir {
			// We've reached the filesystem root without finding the config file
			// Return empty string to indicate not found
			return "", nil
		}

		currentDir = parentDir
	}
}

// GetRelativeStackPath calculates the relative path from the project root to the stack path.
// If the root config file is not found, it returns the absolute path as fallback.
func GetRelativeStackPath(absolutePath, rootConfigFile string) (string, error) {
	absPath, err := filepath.Abs(absolutePath)
	if err != nil {
		return absolutePath, err
	}

	projectRoot, err := FindProjectRoot(absPath, rootConfigFile)
	if err != nil {
		return absolutePath, err
	}

	// If project root is empty, the root config file was not found
	// Return absolute path as fallback
	if projectRoot == "" {
		return absolutePath, nil
	}

	// Calculate relative path from project root
	relPath, err := filepath.Rel(projectRoot, absPath)
	if err != nil {
		// If we can't calculate relative path, return absolute as fallback
		return absolutePath, err
	}

	// If the relative path starts with "..", it means the stack is outside the project
	// In that case, return the absolute path
	if len(relPath) >= 2 && relPath[0:2] == ".." {
		return absolutePath, nil
	}

	return relPath, nil
}

// FilterHistoryByProject filters history entries to show only those belonging to the current project.
// It determines the current project root and returns only entries whose AbsolutePath starts with it.
// If the project root cannot be determined, it returns all entries (no filtering).
func FilterHistoryByProject(entries []ExecutionLogEntry, rootConfigFile string) ([]ExecutionLogEntry, error) {
	// Get current working directory
	currentDir, err := os.Getwd()
	if err != nil {
		// If we can't get working directory, return all entries without filtering
		return entries, nil
	}

	// Find project root from current directory
	projectRoot, err := FindProjectRoot(currentDir, rootConfigFile)
	if err != nil {
		// If error finding root, return all entries without filtering
		return entries, nil
	}

	// If project root not found (empty string), return all entries
	if projectRoot == "" {
		return entries, nil
	}

	// Filter entries that belong to this project
	var filtered []ExecutionLogEntry
	for _, entry := range entries {
		// Skip entries without AbsolutePath (shouldn't happen with new entries)
		if entry.AbsolutePath == "" {
			continue
		}

		// Check if the entry's absolute path starts with the project root
		if hasPrefix(entry.AbsolutePath, projectRoot) {
			filtered = append(filtered, entry)
		}
	}

	return filtered, nil
}

// hasPrefix checks if path starts with prefix, handling filepath separators correctly
func hasPrefix(path, prefix string) bool {
	// Resolve symlinks to get real paths (important on macOS where /var -> /private/var)
	realPath, err := filepath.EvalSymlinks(path)
	if err != nil {
		// If we can't resolve, use the original path
		realPath = path
	}

	realPrefix, err := filepath.EvalSymlinks(prefix)
	if err != nil {
		// If we can't resolve, use the original prefix
		realPrefix = prefix
	}

	// Clean both paths to normalize separators
	cleanPath := filepath.Clean(realPath)
	cleanPrefix := filepath.Clean(realPrefix)

	// If they're equal, path has the prefix
	if cleanPath == cleanPrefix {
		return true
	}

	// Check if path starts with prefix followed by separator
	// This ensures /path/to/project matches /path/to/project/subdir
	// but not /path/to/project2
	prefixWithSep := cleanPrefix + string(filepath.Separator)
	return len(cleanPath) >= len(prefixWithSep) && cleanPath[:len(prefixWithSep)] == prefixWithSep
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

	// Read all lines
	var lines []string
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}

	if err := scanner.Err(); err != nil {
		_ = file.Close()
		return fmt.Errorf("failed to read history file: %w", err)
	}

	// Close the file BEFORE attempting rename (critical for Windows)
	if err := file.Close(); err != nil {
		return fmt.Errorf("failed to close history file: %w", err)
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

	// On Windows, os.Rename() cannot replace an existing file.
	// We need to remove the target file first.
	// This is safe because:
	// 1. Original file is already closed
	// 2. Temp file is fully written and synced
	// 3. If remove fails, we keep the original file
	_ = os.Remove(historyPath)

	// Replace original file with trimmed version
	if err := os.Rename(tempPath, historyPath); err != nil {
		_ = os.Remove(tempPath)
		return fmt.Errorf("failed to replace history file: %w", err)
	}

	return nil
}

// LoadHistory reads all execution entries from the history file and returns them
// in chronological order (oldest first). Returns an empty slice if the file doesn't exist.
func LoadHistory(ctx context.Context) ([]ExecutionLogEntry, error) {
	historyPath, err := GetHistoryFilePath()
	if err != nil {
		return nil, fmt.Errorf("failed to get history file path: %w", err)
	}

	// Check if file exists
	if _, err := os.Stat(historyPath); os.IsNotExist(err) {
		return []ExecutionLogEntry{}, nil
	}

	file, err := os.Open(historyPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open history file: %w", err)
	}
	defer func() {
		if closeErr := file.Close(); closeErr != nil {
			fmt.Fprintf(os.Stderr, "Warning: failed to close history file: %v\n", closeErr)
		}
	}()

	var entries []ExecutionLogEntry
	scanner := bufio.NewScanner(file)

	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			continue
		}

		var entry ExecutionLogEntry
		if err := json.Unmarshal([]byte(line), &entry); err != nil {
			// Skip invalid lines but continue reading
			continue
		}

		// Handle backward compatibility: if AbsolutePath is empty, use StackPath
		// (old entries stored absolute path in StackPath field)
		if entry.AbsolutePath == "" && entry.StackPath != "" {
			entry.AbsolutePath = entry.StackPath
		}

		entries = append(entries, entry)
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("error reading history file: %w", err)
	}

	// Reverse entries to show most recent first
	for i, j := 0, len(entries)-1; i < j; i, j = i+1, j-1 {
		entries[i], entries[j] = entries[j], entries[i]
	}

	return entries, nil
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

// GetLastExecution reads the history file and returns the most recent execution entry.
// Returns nil if the history is empty or doesn't exist.
func GetLastExecution(ctx context.Context) (*ExecutionLogEntry, error) {
	historyPath, err := GetHistoryFilePath()
	if err != nil {
		return nil, fmt.Errorf("failed to get history file path: %w", err)
	}

	file, err := os.Open(historyPath)
	if err != nil {
		if os.IsNotExist(err) {
			// File doesn't exist, no history
			return nil, nil
		}
		return nil, fmt.Errorf("failed to open history file: %w", err)
	}
	defer func() {
		if closeErr := file.Close(); closeErr != nil && err == nil {
			err = fmt.Errorf("failed to close history file: %w", closeErr)
		}
	}()

	// Read all lines and keep track of the last valid entry
	var lastEntry *ExecutionLogEntry
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		var entry ExecutionLogEntry
		if err := json.Unmarshal([]byte(scanner.Text()), &entry); err != nil {
			// Skip invalid lines
			continue
		}
		// Keep track of entry with highest ID (most recent)
		if lastEntry == nil || entry.ID > lastEntry.ID {
			lastEntry = &entry
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("failed to read history file: %w", err)
	}

	return lastEntry, nil
}

// GetLastExecutionForProject reads the history file and returns the most recent execution entry
// for the current project. It filters entries by the project root.
// Returns nil if no matching history is found.
func GetLastExecutionForProject(ctx context.Context, rootConfigFile string) (*ExecutionLogEntry, error) {
	// Load all history entries
	entries, err := LoadHistory(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to load history: %w", err)
	}

	// Filter by current project
	filteredEntries, err := FilterHistoryByProject(entries, rootConfigFile)
	if err != nil {
		// If filtering fails, fall back to unfiltered entries
		filteredEntries = entries
	}

	// No entries for this project
	if len(filteredEntries) == 0 {
		return nil, nil
	}

	// Return the first entry (already sorted most recent first after reverse in LoadHistory)
	return &filteredEntries[0], nil
}
