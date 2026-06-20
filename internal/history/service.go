package history

import (
	"context"
	"fmt"
	"os"
	"os/user"
	"path/filepath"
)

// Service handles business logic for execution history.
type Service struct {
	repo           Repository
	rootConfigFile string
}

// NewService creates a new history service.
func NewService(repo Repository, rootConfigFile string) *Service {
	return &Service{
		repo:           repo,
		rootConfigFile: rootConfigFile,
	}
}

// Append adds a new execution entry to the history.
func (s *Service) Append(ctx context.Context, entry ExecutionLogEntry) error {
	return s.repo.Append(ctx, entry)
}

// LoadAll returns all history entries sorted by most recent first.
func (s *Service) LoadAll(ctx context.Context) ([]ExecutionLogEntry, error) {
	return s.repo.LoadAll(ctx)
}

// GetLastExecutionForProject returns the most recent execution entry for the current project.
func (s *Service) GetLastExecutionForProject(ctx context.Context) (*ExecutionLogEntry, error) {
	entries, err := s.repo.LoadAll(ctx)
	if err != nil {
		return nil, err
	}

	filtered, err := s.FilterByCurrentProject(entries)
	if err != nil {
		// Log error but attempt to use unfiltered? Or fail?
		// For safety, let's just return what we have if filtering fails strictly,
		// but typically we'd want strictly filtered or nothing.
		// Let's stick to previous behavior: fallback to all entries is too broad,
		// but previous implementation did fallback.
		// "If filtering fails, fall back to unfiltered entries" (from previous Code).
		filtered = entries
	}

	if len(filtered) == 0 {
		return nil, nil
	}

	// entries are already sorted by repository LoadAll (most recent first)
	return &filtered[0], nil
}

// TrimHistory trims the history to the specified number of entries.
func (s *Service) TrimHistory(ctx context.Context, maxEntries int) error {
	return s.repo.Trim(ctx, maxEntries)
}

// GetNextID returns the next ID from the repository.
func (s *Service) GetNextID(ctx context.Context) (int, error) {
	return s.repo.GetNextID(ctx)
}

// FilterByCurrentProject filters entries belonging to the project in the current working directory.
func (s *Service) FilterByCurrentProject(entries []ExecutionLogEntry) ([]ExecutionLogEntry, error) {
	currentDir, err := os.Getwd()
	if err != nil {
		return entries, nil
	}

	projectRoot, err := FindProjectRoot(currentDir, s.rootConfigFile)
	if err != nil || projectRoot == "" {
		return entries, nil
	}

	var filtered []ExecutionLogEntry
	for _, entry := range entries {
		if entry.AbsolutePath == "" {
			continue
		}
		if hasPrefix(entry.AbsolutePath, projectRoot) {
			filtered = append(filtered, entry)
		}
	}
	return filtered, nil
}

// GetRelativeStackPath calculates the relative path from the project root to the stack path.
func GetRelativeStackPath(absolutePath, rootConfigFile string) (string, error) {
	absPath, err := filepath.Abs(absolutePath)
	if err != nil {
		return absolutePath, err
	}

	projectRoot, err := FindProjectRoot(absPath, rootConfigFile)
	if err != nil {
		return absolutePath, err
	}

	if projectRoot == "" {
		return absolutePath, nil
	}

	relPath, err := filepath.Rel(projectRoot, absPath)
	if err != nil {
		return absolutePath, err
	}

	if len(relPath) >= 2 && relPath[0:2] == ".." {
		return absolutePath, nil
	}

	return relPath, nil
}

// FindProjectRoot searches for the project root by looking for the root config file.
func FindProjectRoot(startPath, rootConfigFile string) (string, error) {
	absPath, err := filepath.Abs(startPath)
	if err != nil {
		return "", fmt.Errorf("failed to get absolute path: %w", err)
	}

	currentDir := absPath
	if info, err := os.Stat(currentDir); err == nil && !info.IsDir() {
		currentDir = filepath.Dir(currentDir)
	}

	for {
		rootConfigPath := filepath.Join(currentDir, rootConfigFile)
		if _, err := os.Stat(rootConfigPath); err == nil {
			return currentDir, nil
		}

		parentDir := filepath.Dir(currentDir)
		if parentDir == currentDir {
			return "", nil
		}
		currentDir = parentDir
	}
}

// GetCurrentUser returns the current OS username.
func GetCurrentUser() string {
	currentUser, err := user.Current()
	if err != nil {
		return "unknown"
	}
	return currentUser.Username
}

// hasPrefix checks if path starts with prefix, handling filepath separators correctly.
func hasPrefix(path, prefix string) bool {
	// Resolve symlinks for accurate path comparison
	realPath, err := filepath.EvalSymlinks(path)
	if err != nil {
		// Fallback to original path if resolution fails (e.g. file doesn't exist yet)
		realPath = path
	}

	realPrefix, err := filepath.EvalSymlinks(prefix)
	if err != nil {
		realPrefix = prefix
	}

	cleanPath := filepath.Clean(realPath)
	cleanPrefix := filepath.Clean(realPrefix)

	if cleanPath == cleanPrefix {
		return true
	}

	prefixWithSep := cleanPrefix + string(filepath.Separator)
	return len(cleanPath) >= len(prefixWithSep) && cleanPath[:len(prefixWithSep)] == prefixWithSep
}
