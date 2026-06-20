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
	"os"
	"os/exec"
	"path"
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
// profile selects a named AWS CLI profile; empty string uses the default credential chain.
// configFile overrides the AWS config file path via AWS_CONFIG_FILE; empty string uses the default.
// It returns ("", nil) when no lock exists for the stack.
// It returns ("", error) on AWS CLI failure or JSON parse error.
func GetLockID(ctx context.Context, bucket, project, stackRelPath, region, profile, configFile string) (string, error) {
	if region == "" {
		region = config.DefaultStateRegion
	}

	lockKey := path.Join(project, stackRelPath, "terraform.tfstate.tflock")
	s3URI := fmt.Sprintf("s3://%s/%s", bucket, lockKey)

	var args []string
	if profile != "" {
		args = append(args, "--profile", profile)
	}
	args = append(args, "s3", "cp", s3URI, "-", "--region", region)

	cmd := execCommandContext(ctx, "aws", args...)
	if configFile != "" {
		// Preserve existing cmd.Env (set by mocks in tests) and append the config override.
		base := cmd.Env
		if len(base) == 0 {
			base = os.Environ()
		}
		cmd.Env = append(base, fmt.Sprintf("AWS_CONFIG_FILE=%s", configFile))
	}
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
