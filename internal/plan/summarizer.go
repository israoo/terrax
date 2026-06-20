// Package plan provides plan analysis utilities for TerraX.
package plan

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
)

// execSummarizerContext allows mocking exec.CommandContext in tests.
// Named separately from execCommandContext (used by collector.go) to avoid collision.
var execSummarizerContext = exec.CommandContext

// tfSummarizeJSON represents the output of tf-summarize -json-sum.
type tfSummarizeJSON struct {
	Changes struct {
		Add      int `json:"add"`
		Update   int `json:"update"`
		Delete   int `json:"delete"`
		Recreate int `json:"recreate"`
		Import   int `json:"import"`
		Moved    int `json:"moved"`
	} `json:"changes"`
}

// Summarize scans dir for JSON plan files, prints a count line per stack via
// tf-summarize -json-sum, and returns the number of stacks with changes.
// Returns (0, nil) when dir does not exist or contains no JSON files.
// Returns (0, error) when tf-summarize is not installed.
func Summarize(ctx context.Context, dir string) (int, error) {
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		return 0, nil
	}

	var jsonFiles []string
	err := filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil // Skip inaccessible paths.
		}
		if !d.IsDir() && strings.HasSuffix(d.Name(), ".json") {
			jsonFiles = append(jsonFiles, path)
		}
		return nil
	})
	if err != nil {
		return 0, fmt.Errorf("failed to scan plan directory: %w", err)
	}

	if len(jsonFiles) == 0 {
		return 0, nil
	}

	sort.Strings(jsonFiles)
	fmt.Printf("🔍 Scanning %d JSON plan(s)...\n\n", len(jsonFiles))

	changedCount := 0
	for _, planFile := range jsonFiles {
		rel, _ := filepath.Rel(dir, planFile)
		stackName := filepath.ToSlash(filepath.Dir(rel))

		cmd := execSummarizerContext(ctx, "tf-summarize", "-json-sum", planFile)
		output, err := cmd.Output()
		if err != nil {
			// Distinguish "not installed" from per-file failure.
			var execErr *exec.Error
			if errors.As(err, &execErr) && errors.Is(execErr.Err, exec.ErrNotFound) {
				return 0, fmt.Errorf("tf-summarize not found: install from https://github.com/dineshba/tf-summarize")
			}
			fmt.Fprintf(os.Stderr, "Warning: could not summarize %s\n", stackName)
			continue
		}

		if len(output) == 0 {
			fmt.Fprintf(os.Stderr, "Warning: could not summarize %s\n", stackName)
			continue
		}

		var summary tfSummarizeJSON
		if err := json.Unmarshal(output, &summary); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: could not summarize %s\n", stackName)
			continue
		}

		c := summary.Changes
		fmt.Printf("  %s: +%d ~%d -%d ♻%d\n", stackName, c.Add, c.Update, c.Delete, c.Recreate)

		total := c.Add + c.Update + c.Delete + c.Recreate + c.Import + c.Moved
		if total > 0 {
			changedCount++
		}
	}

	fmt.Println()
	if changedCount > 0 {
		fmt.Printf("%d stack(s) with pending changes\n", changedCount)
	} else {
		fmt.Printf("No changes detected across %d stack(s)\n", len(jsonFiles))
	}

	return changedCount, nil
}
