// Package state handles Terraform state operations for TerraX.
//
// It provides utilities for inspecting and managing Terraform state backends,
// including lock detection via the AWS CLI for force-unlock operations.
package state

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
	"strings"

	"github.com/israoo/terrax/internal/config"
)

// execCommandContext allows mocking exec.CommandContext in tests.
var execCommandContext = exec.CommandContext

// tfLockJSON represents the structure of a Terraform state lock file.
type tfLockJSON struct {
	ID        string `json:"ID"`
	Who       string `json:"Who"`
	Operation string `json:"Operation"`
	Created   string `json:"Created"`
}

// GetLockID fetches the Terraform state lock ID from S3 for the given stack.
// It returns ("", nil) when no lock exists for the stack.
// It returns ("", error) on AWS CLI failure or JSON parse error.
func GetLockID(ctx context.Context, bucket, project, stackRelPath, region string) (string, error) {
	if region == "" {
		region = config.DefaultStateRegion
	}

	lockKey := fmt.Sprintf("%s/%s/terraform.tfstate.tflock", project, stackRelPath)
	s3URI := fmt.Sprintf("s3://%s/%s", bucket, lockKey)

	cmd := execCommandContext(ctx, "aws", "s3", "cp", s3URI, "-", "--region", region)
	output, err := cmd.Output()
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) && strings.Contains(string(exitErr.Stderr), "NoSuchKey") {
			return "", nil
		}
		return "", fmt.Errorf("aws s3 cp failed: %w", err)
	}

	var lock tfLockJSON
	if err := json.Unmarshal(output, &lock); err != nil {
		return "", fmt.Errorf("failed to parse lock file: %w", err)
	}

	return lock.ID, nil
}
