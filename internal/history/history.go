// Package history provides execution history tracking and persistence.
//
// It manages the storage and retrieval of command execution logs, allowing users
// to review past actions and re-execute commands. The package implements a
// persistent JSON-based storage using the XDG Base Directory specification.
package history

import (
	"context"
)

// Global default service for backward compatibility
var DefaultService *Service

func init() {
	// Initialize default service with default repository
	repo, err := NewFileRepository("")
	if err != nil {
		// This primarily fails if home directory cannot be determined.
		// In that unlikely case, we can't persist history properly.
		// Since init() cannot return error, and this is a fallback global,
		// we accept a potentially nil repo or broken state here, but ideally we'd log it.
		// However, standard logger might interfere with TUI. Use nil safely?
		// Better approach: NewFileRepository returns error only on critical failure.
		// We'll panic here as the app likely won't work well without a home dir.
		panic("failed to initialize default history repository: " + err.Error())
	}
	DefaultService = NewService(repo, "terragrunt.hcl")
}

// GetHistoryFilePath returns the history file path using the default repository logic.
func GetHistoryFilePath() (string, error) {
	return GetDefaultHistoryFilePath()
}

// GetNextID wraps the service GetNextID.
func GetNextID(ctx context.Context) (int, error) {
	return DefaultService.GetNextID(ctx)
}

// AppendToHistory wraps the service Append.
func AppendToHistory(ctx context.Context, entry ExecutionLogEntry) error {
	return DefaultService.Append(ctx, entry)
}

// LoadHistory wraps repo LoadAll directly (as it was logic-less).
func LoadHistory(ctx context.Context) ([]ExecutionLogEntry, error) {
	return DefaultService.repo.LoadAll(ctx)
}

// TrimHistory wraps the service TrimHistory.
func TrimHistory(ctx context.Context, maxEntries int) error {
	return DefaultService.TrimHistory(ctx, maxEntries)
}

// GetLastExecutionForProject wraps the service.
func GetLastExecutionForProject(ctx context.Context, rootConfigFile string) (*ExecutionLogEntry, error) {
	// Temporarily override config file for this call if it differs,
	// or create a temp service.
	// For simplicity, we can just instantiate a new service since it's cheap.
	svc := NewService(DefaultService.repo, rootConfigFile)
	return svc.GetLastExecutionForProject(ctx)
}

// FilterHistoryByProject wraps the service.
func FilterHistoryByProject(entries []ExecutionLogEntry, rootConfigFile string) ([]ExecutionLogEntry, error) {
	svc := NewService(DefaultService.repo, rootConfigFile)
	return svc.FilterByCurrentProject(entries)
}
