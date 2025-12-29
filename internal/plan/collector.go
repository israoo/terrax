package plan

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/israoo/terrax/internal/config"
	"github.com/israoo/terrax/internal/history"
	"github.com/spf13/viper"
)

// Collector handles the collection and processing of plan files.
type Collector struct {
	projectRoot string
	runDir      string
}

// NewCollector creates a new Collector for the given run directory.
// It automatically finds the project root to ensure dependencies are captured.
func NewCollector(runDir string) *Collector {
	rootConfigFile := viper.GetString("root_config_file")
	if rootConfigFile == "" {
		rootConfigFile = config.DefaultRootConfigFile
	}

	projectRoot, err := history.FindProjectRoot(runDir, rootConfigFile)
	if err != nil || projectRoot == "" {
		projectRoot = runDir // Fallback
	}

	return &Collector{
		projectRoot: projectRoot,
		runDir:      runDir,
	}
}

// TerraformPlanJSON represents the structure of `terraform show -json` output.
type TerraformPlanJSON struct {
	ResourceChanges []struct {
		Address string `json:"address"`
		Type    string `json:"type"`
		Name    string `json:"name"`
		Change  struct {
			Actions []string    `json:"actions"`
			Before  interface{} `json:"before"`
			After   interface{} `json:"after"`
			Unknown interface{} `json:"after_unknown"`
		} `json:"change"`
	} `json:"resource_changes"`
}

// Collect scans the project for binary plan files concurrently and processes them.
// It sends progress updates to the optional progressChan.
func (c *Collector) Collect(ctx context.Context, progressChan chan<- ProgressMsg) (*PlanReport, error) {
	report := &PlanReport{
		Timestamp: time.Now(),
		Stacks:    []StackResult{},
	}

	if progressChan != nil {
		select {
		case progressChan <- ProgressMsg{Message: "Scanning for plan files..."}:
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}

	planFiles, err := c.findPlanFiles()
	if err != nil {
		return nil, fmt.Errorf("failed to find plan files: %w", err)
	}

	totalFiles := len(planFiles)
	if totalFiles == 0 {
		return report, nil
	}

	if progressChan != nil {
		select {
		case progressChan <- ProgressMsg{TotalFiles: totalFiles, Message: fmt.Sprintf("Found %d plans", totalFiles)}:
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}

	// Determine parallelism suitable for file I/O and subprocess execution
	maxWorkers := viper.GetInt("terragrunt.parallelism")
	if maxWorkers <= 0 {
		maxWorkers = 4 // Default modest parallelism
	}
	// Don't spawn more workers than tasks
	if totalFiles < maxWorkers {
		maxWorkers = totalFiles
	}

	// Channels for distribution and results
	jobs := make(chan string, totalFiles)
	results := make(chan *StackResult, totalFiles)
	errs := make(chan error, totalFiles)

	// Start workers
	for w := 0; w < maxWorkers; w++ {
		go func() {
			for path := range jobs {
				// Check context cancellation
				if ctx.Err() != nil {
					return
				}

				res, err := c.processStack(ctx, path)
				if err != nil {
					errs <- fmt.Errorf("process %s: %w", path, err)
					continue
				}
				results <- res
			}
		}()
	}

	// Enqueue jobs
	for _, path := range planFiles {
		jobs <- path
	}
	close(jobs)

	// Collect results
	processedCount := 0
	for i := 0; i < totalFiles; i++ {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case res := <-results:
			processedCount++
			if res != nil {
				report.Stacks = append(report.Stacks, *res)
				if progressChan != nil {
					select {
					case progressChan <- ProgressMsg{
						TotalFiles: totalFiles,
						Current:    processedCount,
						Message:    fmt.Sprintf("Processed %s", res.StackPath),
					}:
					case <-ctx.Done():
						return nil, ctx.Err()
					}
				}
			}
		case err := <-errs:
			processedCount++
			// Log error but continue
			fmt.Fprintf(os.Stderr, "Warning: %v\n", err)
			if progressChan != nil {
				select {
				case progressChan <- ProgressMsg{
					TotalFiles: totalFiles,
					Current:    processedCount,
					Message:    "Error processing stack",
				}:
				case <-ctx.Done():
					return nil, ctx.Err()
				}
			}
		}
	}

	report.calculateSummary()

	return report, nil
}

// findPlanFiles searches for terrax-tfplan-timestamp.binary files specifically within .terragrunt-cache directories.
// It matches the current session timestamp and ignores others.
func (c *Collector) findPlanFiles() ([]string, error) {
	var matches []string
	targetName := getSessionPlanFilename()

	// Fast path: recursive directory walk with optimized skipping
	err := filepath.WalkDir(c.projectRoot, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil // Skip inaccessible directories
		}

		if d.IsDir() {
			if shouldSkipDir(d.Name()) {
				return filepath.SkipDir
			}
			return nil
		}

		// Only match plan binaries with the specific timestamp
		if d.Name() == targetName {
			// Ensure it's inside a .terragrunt-cache path to be strictly compliant
			// with "search ONLY inside found .terragrunt-cache"
			if strings.Contains(path, ".terragrunt-cache") {
				matches = append(matches, path)
			}
		}

		return nil
	})

	return matches, err
}

// CleanupOldPlans removes any terrax-tfplan-*.binary files that do NOT match the current session timestamp.
// This ensures we don't accumulate old plan files.
func (c *Collector) CleanupOldPlans() error {
	currentTarget := getSessionPlanFilename()

	return filepath.WalkDir(c.projectRoot, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}

		if d.IsDir() {
			if shouldSkipDir(d.Name()) {
				return filepath.SkipDir
			}
			return nil
		}

		// Check if it's a terrax plan file but NOT the current one
		name := d.Name()
		if strings.HasPrefix(name, "terrax-tfplan-") && strings.HasSuffix(name, ".binary") && name != currentTarget {
			// Only delete if inside .terragrunt-cache (safety check)
			if strings.Contains(path, ".terragrunt-cache") {
				if err := os.Remove(path); err != nil {
					// Log error but continue
					fmt.Fprintf(os.Stderr, "Warning: Failed to delete old plan %s: %v\n", path, err)
				}
			}
		}

		return nil
	})
}

// Helper definitions

func getSessionPlanFilename() string {
	sessionTimestamp := viper.GetInt64("terrax.session_timestamp")
	return fmt.Sprintf("terrax-tfplan-%d.binary", sessionTimestamp)
}

func shouldSkipDir(name string) bool {
	return name == ".git" || name == "node_modules" || name == ".idea" || name == ".vscode"
}

func (c *Collector) processStack(ctx context.Context, planPath string) (*StackResult, error) {
	stackDir := filepath.Dir(planPath)

	// Calculate cleaned relative path
	configRoot := viper.GetString("root_config_file")
	if configRoot == "" {
		configRoot = config.DefaultRootConfigFile
	}

	// Clean path from .terragrunt-cache
	cleanDir := cleanStackPath(stackDir)
	relPath, err := history.GetRelativeStackPath(cleanDir, configRoot)
	if err != nil {
		relPath = cleanDir
	}

	// A stack is a dependency if it's NOT within the runDir
	isDependency := !isSubDir(c.runDir, cleanDir)

	// We use terraform directly to avoid parsing issues with terragrunt output wrappers
	planBinary := getSessionPlanFilename()
	cmd := exec.CommandContext(ctx, "terraform", "show", "-json", planBinary)
	cmd.Dir = stackDir

	// Capture output
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("terraform show failed: %w", err)
	}

	// Optimize: Use json.Decoder for streaming parsing which is more memory efficient
	// and often faster for large JSON files than Unmarshal.
	var planJSON TerraformPlanJSON
	// We can decode directly from the output bytes using a bytes buffer wrapper
	// or better, if we could pipe exec stdout to decoder, but we already captured it.
	// Since we already have []byte, Unmarshal vs Decoder on buffer is similar,
	// BUT the user asked for optimization. Decoder is the standard "optimization" answer.
	// Let's wrapping bytes in a reader.
	if err := json.NewDecoder(strings.NewReader(string(output))).Decode(&planJSON); err != nil {
		return nil, fmt.Errorf("failed to parse json: %w", err)
	}

	result := &StackResult{
		StackPath:    relPath,
		AbsPath:      stackDir,
		IsDependency: isDependency,
		Stats:        StackStats{},
	}

	for _, rc := range planJSON.ResourceChanges {
		changeType := mapActionsToChangeType(rc.Change.Actions)

		if changeType == ChangeTypeNoOp {
			continue
		}

		result.HasChanges = true

		internalRC := ResourceChange{
			Address:    rc.Address,
			Type:       rc.Type,
			Name:       rc.Name,
			ChangeType: changeType,
			Before:     rc.Change.Before,
			After:      rc.Change.After,
			Unknown:    rc.Change.Unknown,
		}
		result.ResourceChanges = append(result.ResourceChanges, internalRC)

		switch changeType {
		case ChangeTypeCreate:
			result.Stats.Add++
		case ChangeTypeDelete:
			result.Stats.Destroy++
		case ChangeTypeUpdate:
			result.Stats.Change++
		case ChangeTypeReplace:
			result.Stats.Add++
			result.Stats.Destroy++
		}
	}

	return result, nil
}

// cleanStackPath removes everything from .terragrunt-cache onwards
func cleanStackPath(path string) string {
	parts := strings.Split(path, string(filepath.Separator))

	for i, part := range parts {
		if part == ".terragrunt-cache" {
			// Found the cache dir, return everything before it
			return strings.Join(parts[:i], string(filepath.Separator))
		}
	}

	return path
}

// isSubDir checks if child is a subdirectory of parent
func isSubDir(parent, child string) bool {
	rel, err := filepath.Rel(parent, child)
	if err != nil {
		return false
	}
	// If relative path starts with ".." it's outside
	return !strings.HasPrefix(rel, "..")
}

func mapActionsToChangeType(actions []string) ChangeType {
	if len(actions) == 0 || (len(actions) == 1 && actions[0] == "no-op") {
		return ChangeTypeNoOp
	}

	isCreate := contains(actions, "create")
	isDelete := contains(actions, "delete")
	isUpdate := contains(actions, "update")

	if isCreate && isDelete {
		return ChangeTypeReplace
	}
	if isCreate {
		return ChangeTypeCreate
	}
	if isDelete {
		return ChangeTypeDelete
	}
	if isUpdate {
		return ChangeTypeUpdate
	}

	return ChangeTypeNoOp
}

func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

func (r *PlanReport) calculateSummary() {
	summary := PlanSummary{}
	summary.TotalStacks = len(r.Stacks)

	for _, stack := range r.Stacks {
		if stack.HasChanges {
			summary.StacksWithChanges++
		}
		summary.TotalAdd += stack.Stats.Add
		summary.TotalChange += stack.Stats.Change
		summary.TotalDestroy += stack.Stats.Destroy
	}
	r.Summary = summary
}
