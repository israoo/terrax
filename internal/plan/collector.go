package plan

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/israoo/terrax/internal/config"
	"github.com/israoo/terrax/internal/history"
	"github.com/spf13/viper"
)

const PlanBinaryName = "tfplan.binary"

// Collector handles the collection and processing of plan files.
type Collector struct {
	projectRoot string
}

// NewCollector creates a new Collector for the given project root.
func NewCollector(projectRoot string) *Collector {
	return &Collector{
		projectRoot: projectRoot,
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

// Collect scans the project for binary plan files, runs `terragrunt show -json`,
// and parses the results into a PlanReport.
func (c *Collector) Collect(ctx context.Context) (*PlanReport, error) {
	report := &PlanReport{
		Timestamp: time.Now(),
		Stacks:    []StackResult{},
	}

	planFiles, err := c.findPlanFiles()
	if err != nil {
		return nil, fmt.Errorf("failed to find plan files: %w", err)
	}

	for _, planPath := range planFiles {
		stackResult, err := c.processStack(ctx, planPath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Warning: failed to process plan for %s: %v\n", planPath, err)
			continue
		}
		if stackResult != nil {
			report.Stacks = append(report.Stacks, *stackResult)
		}
	}

	report.calculateSummary()

	return report, nil
}

func (c *Collector) findPlanFiles() ([]string, error) {
	var matches []string
	err := filepath.Walk(c.projectRoot, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // Skip inaccessible directories
		}
		if !info.IsDir() && info.Name() == PlanBinaryName {
			matches = append(matches, path)
		}
		return nil
	})
	return matches, err
}

func (c *Collector) processStack(ctx context.Context, planPath string) (*StackResult, error) {
	stackDir := filepath.Dir(planPath)

	// Calculate relative path
	rootConfigFile := viper.GetString("root_config_file")
	if rootConfigFile == "" {
		rootConfigFile = config.DefaultRootConfigFile
	}

	relPath, err := history.GetRelativeStackPath(stackDir, rootConfigFile)
	if err != nil {
		relPath = stackDir
	}

	// We use terraform directly to avoid parsing issues with terragrunt output wrappers
	cmd := exec.CommandContext(ctx, "terraform", "show", "-json", PlanBinaryName)
	cmd.Dir = stackDir
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("terraform show failed: %w", err)
	}

	var planJSON TerraformPlanJSON
	if err := json.Unmarshal(output, &planJSON); err != nil {
		return nil, fmt.Errorf("failed to parse json: %w", err)
	}

	result := &StackResult{
		StackPath: relPath,
		AbsPath:   stackDir,
		Stats:     StackStats{},
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
