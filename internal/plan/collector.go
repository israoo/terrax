// Package plan provides plan analysis utilities for TerraX.
//
// It implements collection of Terraform plan results from pre-generated JSON files
// produced by Terragrunt's --json-out-dir flag, and builds a structured PlanReport
// for display in the StatePlanReview TUI.
package plan

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// TerraformPlanJSON represents the structure of terraform show -json output.
type TerraformPlanJSON struct {
	ResourceChanges []struct {
		Address string `json:"address"`
		Type    string `json:"type"`
		Name    string `json:"name"`
		Change  struct {
			Actions   []string    `json:"actions"`
			Before    interface{} `json:"before"`
			After     interface{} `json:"after"`
			Unknown   interface{} `json:"after_unknown"`
			Importing interface{} `json:"importing"`
		} `json:"change"`
	} `json:"resource_changes"`
}

// CollectFromJSONDir reads pre-generated JSON plan files from jsonDir and builds a PlanReport.
// jsonDir is written by Terragrunt's --json-out-dir flag (e.g. <repoRoot>/.terrax/plans).
// runDir is the selected stack path, used to determine which stacks are dependencies.
func CollectFromJSONDir(ctx context.Context, jsonDir, runDir string) (*PlanReport, error) {
	report := &PlanReport{
		Timestamp: time.Now(),
		Stacks:    []StackResult{},
	}

	if _, err := os.Stat(jsonDir); os.IsNotExist(err) {
		return report, nil
	}

	var jsonFiles []string
	err := filepath.WalkDir(jsonDir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if !d.IsDir() && strings.HasSuffix(d.Name(), ".json") {
			jsonFiles = append(jsonFiles, path)
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("failed to scan plan directory: %w", err)
	}

	for _, planFile := range jsonFiles {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		result, err := processPlanJSONFile(planFile, jsonDir, runDir)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Warning: failed to process %s: %v\n", planFile, err)
			continue
		}
		if result != nil {
			report.Stacks = append(report.Stacks, *result)
		}
	}

	report.calculateSummary()
	return report, nil
}

// processPlanJSONFile parses a single JSON plan file and returns a StackResult.
func processPlanJSONFile(planFile, jsonDir, runDir string) (*StackResult, error) {
	rel, _ := filepath.Rel(jsonDir, planFile)
	stackRelPath := filepath.ToSlash(filepath.Dir(rel))

	// jsonDir = <repoRoot>/.terrax/plans — repoRoot is two levels up.
	repoRoot := filepath.Dir(filepath.Dir(jsonDir))
	absPath := filepath.Join(repoRoot, filepath.FromSlash(stackRelPath))
	isDependency := !isSubDir(runDir, absPath)

	data, err := os.ReadFile(planFile)
	if err != nil {
		return nil, fmt.Errorf("failed to read plan file: %w", err)
	}

	var planJSON TerraformPlanJSON
	if err := json.Unmarshal(data, &planJSON); err != nil {
		return nil, fmt.Errorf("failed to parse plan JSON: %w", err)
	}

	result := &StackResult{
		StackPath:    stackRelPath,
		AbsPath:      absPath,
		IsDependency: isDependency,
	}

	for _, rc := range planJSON.ResourceChanges {
		changeType := mapActionsToChangeType(rc.Change.Actions)
		if changeType == ChangeTypeNoOp {
			continue
		}
		result.HasChanges = true
		result.ResourceChanges = append(result.ResourceChanges, ResourceChange{
			Address:    rc.Address,
			Type:       rc.Type,
			Name:       rc.Name,
			ChangeType: changeType,
			Before:     rc.Change.Before,
			After:      rc.Change.After,
			Unknown:    rc.Change.Unknown,
		})
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

// isSubDir returns true when child is within the parent directory tree.
func isSubDir(parent, child string) bool {
	rel, err := filepath.Rel(parent, child)
	if err != nil {
		return false
	}
	return !strings.HasPrefix(rel, "..")
}

// mapActionsToChangeType converts Terraform action strings to a ChangeType.
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

// contains returns true when slice contains item.
func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

// calculateSummary aggregates counts across all stacks in the report.
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
