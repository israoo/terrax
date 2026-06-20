// Package plan provides plan analysis utilities for TerraX.
package plan

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// planSummaryStats holds the change counts for a single plan file.
type planSummaryStats struct {
	Add      int
	Update   int
	Delete   int
	Recreate int
	Import   int
}

func (s planSummaryStats) total() int {
	return s.Add + s.Update + s.Delete + s.Recreate + s.Import
}

// countChanges parses a TerraformPlanJSON and returns change counts by type.
func countChanges(p TerraformPlanJSON) planSummaryStats {
	var stats planSummaryStats
	for _, rc := range p.ResourceChanges {
		actions := rc.Change.Actions
		hasCreate := contains(actions, "create")
		hasDelete := contains(actions, "delete")
		hasUpdate := contains(actions, "update")

		switch {
		case hasCreate && hasDelete:
			stats.Recreate++
		case hasCreate:
			if rc.Change.Importing != nil {
				stats.Import++
			} else {
				stats.Add++
			}
		case hasUpdate:
			stats.Update++
		case hasDelete:
			stats.Delete++
		}
	}
	return stats
}

// Summarize scans dir for JSON plan files, prints a count line per stack, and
// returns the number of stacks with changes.
// projectRoot is used to display full relative paths instead of basenames.
// Returns (0, nil) when dir does not exist or contains no JSON files.
// No external tools required — parses Terraform plan JSON directly.
func Summarize(_ context.Context, dir, projectRoot string) (int, error) {
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

	// stackBase is the working directory Terragrunt used (parent of <dir>/<.terrax>/plans).
	// dir = <stackBase>/.terrax/plans
	stackBase := filepath.Dir(filepath.Dir(dir))

	changedCount := 0
	for _, planFile := range jsonFiles {
		rel, _ := filepath.Rel(dir, planFile)
		unitDir := filepath.ToSlash(filepath.Dir(rel))

		// Construct the full display path relative to the project root.
		var unitAbsPath string
		if unitDir == "." {
			unitAbsPath = stackBase
		} else {
			unitAbsPath = filepath.Join(stackBase, filepath.FromSlash(unitDir))
		}
		stackName := unitDir // fallback to basename if project root is unknown
		if projectRoot != "" {
			if fullRel, err := filepath.Rel(projectRoot, unitAbsPath); err == nil {
				stackName = filepath.ToSlash(fullRel)
			}
		}

		data, err := os.ReadFile(planFile)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Warning: could not read %s.\n", stackName)
			continue
		}

		var planJSON TerraformPlanJSON
		if err := json.Unmarshal(data, &planJSON); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: could not parse %s.\n", stackName)
			continue
		}

		stats := countChanges(planJSON)
		fmt.Printf("  %s: +%d ~%d -%d ♻%d\n", stackName, stats.Add, stats.Update, stats.Delete, stats.Recreate)

		if stats.total() > 0 {
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
