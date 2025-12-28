package history

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/adrg/xdg"
)

const (
	// HistoryFileName is the name of the history log file
	HistoryFileName = "history.log"
	// ConfigDirName is the application configuration directory name
	ConfigDirName = "terrax"
)

// Repository defines the interface for history persistence.
type Repository interface {
	// Append adds an entry to the history.
	Append(ctx context.Context, entry ExecutionLogEntry) error
	// LoadAll returns all history entries sorted by most recent first.
	LoadAll(ctx context.Context) ([]ExecutionLogEntry, error)
	// Trim retains only the most recent maxEntries.
	Trim(ctx context.Context, maxEntries int) error
	// GetNextID returns the next available ID for a new entry.
	GetNextID(ctx context.Context) (int, error)
}

// FileRepository implements Repository using a JSONL file.
type FileRepository struct {
	filePath string
}

// NewFileRepository creates a new FileRepository.
// If filePath is empty, it uses the default XDG location.
func NewFileRepository(filePath string) (*FileRepository, error) {
	if filePath == "" {
		var err error
		filePath, err = GetDefaultHistoryFilePath()
		if err != nil {
			return nil, err
		}
	}
	return &FileRepository{filePath: filePath}, nil
}

// Append adds an entry to the history file.
func (r *FileRepository) Append(ctx context.Context, entry ExecutionLogEntry) error {
	// Open file in append mode, create if doesn't exist
	// 0644 = rw-r--r-- (owner can read/write, others can read)
	file, err := os.OpenFile(r.filePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("failed to open history file: %w", err)
	}
	defer func() {
		_ = file.Close()
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

	return nil
}

// LoadAll returns all history entries sorted by most recent first.
func (r *FileRepository) LoadAll(ctx context.Context) ([]ExecutionLogEntry, error) {
	// Check if file exists
	if _, err := os.Stat(r.filePath); os.IsNotExist(err) {
		return []ExecutionLogEntry{}, nil
	}

	file, err := os.Open(r.filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to open history file: %w", err)
	}
	defer func() {
		_ = file.Close()
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

// Trim retains only the most recent maxEntries.
func (r *FileRepository) Trim(ctx context.Context, maxEntries int) error {
	if maxEntries <= 0 {
		return fmt.Errorf("maxEntries must be positive, got: %d", maxEntries)
	}

	// Open file for reading
	file, err := os.Open(r.filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil // Nothing to trim
		}
		return fmt.Errorf("failed to open history file: %w", err)
	}

	var lines []string
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}
	_ = file.Close() // Close before writing

	if err := scanner.Err(); err != nil {
		return fmt.Errorf("failed to read history file: %w", err)
	}

	if len(lines) <= maxEntries {
		return nil // No trimming needed
	}

	// Keep only the last maxEntries lines
	trimmedLines := lines[len(lines)-maxEntries:]

	// Atomic write using temp file
	tempPath := r.filePath + ".tmp"
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

	if err := tempFile.Close(); err != nil {
		_ = os.Remove(tempPath)
		return fmt.Errorf("failed to close temp file: %w", err)
	}

	if err := os.Rename(tempPath, r.filePath); err != nil {
		// Fallback for Windows or cross-device link errors: remove target first
		_ = os.Remove(r.filePath)
		if err := os.Rename(tempPath, r.filePath); err != nil {
			return fmt.Errorf("failed to replace history file: %w", err)
		}
	}

	return nil
}

// GetNextID returns the next available ID.
func (r *FileRepository) GetNextID(ctx context.Context) (int, error) {
	file, err := os.Open(r.filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return 1, nil
		}
		return 0, fmt.Errorf("failed to open history file: %w", err)
	}
	defer func() {
		_ = file.Close()
	}()

	var lastID int
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		var entry ExecutionLogEntry
		if err := json.Unmarshal([]byte(scanner.Text()), &entry); err == nil {
			if entry.ID > lastID {
				lastID = entry.ID
			}
		}
	}

	return lastID + 1, nil
}

// GetDefaultHistoryFilePath returns the standard XDG path for the history file.
func GetDefaultHistoryFilePath() (string, error) {
	configDir := filepath.Join(xdg.ConfigHome, ConfigDirName)
	if err := os.MkdirAll(configDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create config directory: %w", err)
	}
	return filepath.Join(configDir, HistoryFileName), nil
}
