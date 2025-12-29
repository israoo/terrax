package plan

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMapActionsToChangeType(t *testing.T) {
	tests := []struct {
		name     string
		actions  []string
		expected ChangeType
	}{
		{"create", []string{"create"}, ChangeTypeCreate},
		{"delete", []string{"delete"}, ChangeTypeDelete},
		{"update", []string{"update"}, ChangeTypeUpdate},
		{"replace (create, delete)", []string{"delete", "create"}, ChangeTypeReplace},
		{"replace (delete, create)", []string{"create", "delete"}, ChangeTypeReplace},
		{"no-op", []string{"no-op"}, ChangeTypeNoOp},
		{"empty", []string{}, ChangeTypeNoOp},
		{"read", []string{"read"}, ChangeTypeNoOp},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, mapActionsToChangeType(tt.actions))
		})
	}
}

func TestPlanReport_CalculateSummary(t *testing.T) {
	report := &PlanReport{
		Timestamp: time.Now(),
		Stacks: []StackResult{
			{
				HasChanges: true,
				Stats: StackStats{
					Add:     2,
					Change:  1,
					Destroy: 0,
				},
			},
			{
				HasChanges: false,
				Stats: StackStats{
					Add:     0,
					Change:  0,
					Destroy: 0,
				},
			},
			{
				HasChanges: true,
				Stats: StackStats{
					Add:     0,
					Change:  0,
					Destroy: 3,
				},
			},
		},
	}

	report.calculateSummary()

	assert.Equal(t, 3, report.Summary.TotalStacks)
	assert.Equal(t, 2, report.Summary.StacksWithChanges)
	assert.Equal(t, 2, report.Summary.TotalAdd)
	assert.Equal(t, 1, report.Summary.TotalChange)
	assert.Equal(t, 3, report.Summary.TotalDestroy)
}

func TestNewCollector(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "terrax-test-collector")
	assert.NoError(t, err)
	defer func() { _ = os.RemoveAll(tmpDir) }()

	// Create a dummy terrax.yaml to simulate project root
	err = os.WriteFile(filepath.Join(tmpDir, "terrax.yaml"), []byte(""), 0644)
	assert.NoError(t, err)

	viper.Set("root_config_file", "terrax.yaml")
	defer viper.Set("root_config_file", "") // Reset

	// Test case 1: Run inside project root
	c1 := NewCollector(tmpDir)
	assert.NotNil(t, c1)
	assert.Equal(t, tmpDir, c1.projectRoot)
	assert.Equal(t, tmpDir, c1.runDir)

	// Test case 2: Run in subdir
	subDir := filepath.Join(tmpDir, "stacks", "prod")
	err = os.MkdirAll(subDir, 0755)
	assert.NoError(t, err)

	c2 := NewCollector(subDir)
	assert.NotNil(t, c2)
	assert.Equal(t, tmpDir, c2.projectRoot)
	assert.Equal(t, subDir, c2.runDir)
}

func TestCollector_FindPlanFiles(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "terrax-test-find")
	assert.NoError(t, err)
	defer func() { _ = os.RemoveAll(tmpDir) }()

	timestamp := time.Now().Unix()
	viper.Set("terrax.session_timestamp", timestamp)

	c := &Collector{
		projectRoot: tmpDir,
		runDir:      tmpDir,
	}

	// Create valid structure: .terragrunt-cache/.../terrax-tfplan-<ts>.binary
	cacheDir := filepath.Join(tmpDir, "stack1", ".terragrunt-cache", "uuid")
	err = os.MkdirAll(cacheDir, 0755)
	assert.NoError(t, err)

	planName := fmt.Sprintf("terrax-tfplan-%d.binary", timestamp)
	validPlanPath := filepath.Join(cacheDir, planName)
	err = os.WriteFile(validPlanPath, []byte("dummy"), 0644)
	assert.NoError(t, err)

	// Create invalid file (wrong timestamp)
	oldPlanPath := filepath.Join(cacheDir, "terrax-tfplan-123.binary")
	err = os.WriteFile(oldPlanPath, []byte("dummy"), 0644)
	assert.NoError(t, err)

	// Create invalid file (outside cache)
	outsidePath := filepath.Join(tmpDir, "stack1", planName)
	err = os.WriteFile(outsidePath, []byte("dummy"), 0644)
	assert.NoError(t, err)

	files, err := c.findPlanFiles()
	assert.NoError(t, err)
	assert.Len(t, files, 1)
	assert.Equal(t, validPlanPath, files[0])
}

// Mocking support
type MockCmd struct {
	Stdout []byte
	Err    error
}

var mockCmdResponse *MockCmd

// Global var to control mode in fakeExecCommand
var mockExecMode string = ""

func fakeExecCommand(ctx context.Context, command string, args ...string) *exec.Cmd {
	cs := []string{"-test.run=TestHelperProcess", "--", command}
	cs = append(cs, args...)
	cmd := exec.Command(os.Args[0], cs...)
	cmd.Env = []string{
		"GO_WANT_HELPER_PROCESS=1",
		fmt.Sprintf("STDOUT=%s", mockCmdResponse.Stdout),
		"HOME=" + os.TempDir(),
		"GO_HELPER_MODE=" + mockExecMode,
	}
	return cmd
}

func TestHelperProcess(t *testing.T) {
	if os.Getenv("GO_WANT_HELPER_PROCESS") != "1" {
		return
	}

	mode := os.Getenv("GO_HELPER_MODE")
	if mode == "error" {
		os.Exit(1)
	}

	fmt.Print(os.Getenv("STDOUT"))
	os.Exit(0)
}

func TestProcessStack(t *testing.T) {
	// Swap exec command
	oldExec := execCommandContext
	execCommandContext = fakeExecCommand
	defer func() { execCommandContext = oldExec }()

	tmpDir, err := os.MkdirTemp("", "terrax-test-process")
	require.NoError(t, err)
	defer func() { _ = os.RemoveAll(tmpDir) }()

	// Setup config file to define project root for relative path calculation
	viper.Set("root_config_file", "terrax.yaml")
	err = os.WriteFile(filepath.Join(tmpDir, "terrax.yaml"), []byte(""), 0644)
	require.NoError(t, err)

	// Setup Collector
	c := &Collector{
		projectRoot: tmpDir,
		runDir:      tmpDir,
	}

	// Create a dummy plan file path (doesn't need to exist for this test as we match by name/path logic)
	// But we need the directory structure for relative path calculation
	stackDir := filepath.Join(tmpDir, "stacks", "dev")
	cacheDir := filepath.Join(stackDir, ".terragrunt-cache", "xyz")
	err = os.MkdirAll(cacheDir, 0755)
	assert.NoError(t, err)

	planPath := filepath.Join(cacheDir, "plan.binary")

	// Mock Terraform output
	jsonOutput := `{
		"resource_changes": [
			{
				"address": "aws_s3_bucket.test",
				"type": "aws_s3_bucket",
				"name": "test",
				"change": {
					"actions": ["create"],
					"after": {"bucket": "my-bucket"}
				}
			}
		]
	}`
	mockCmdResponse = &MockCmd{Stdout: []byte(jsonOutput)}

	// Test processing
	res, err := c.processStack(context.Background(), planPath)
	require.NoError(t, err)
	require.NotNil(t, res)
	assert.True(t, res.HasChanges)
	assert.Equal(t, 1, res.Stats.Add)
	assert.Equal(t, "stacks/dev", res.StackPath) // Cleaned path
}

func TestProcessStack_Error(t *testing.T) {
	// Swap exec command
	oldExec := execCommandContext
	execCommandContext = fakeExecCommand
	defer func() { execCommandContext = oldExec }()

	tmpDir, err := os.MkdirTemp("", "terrax-test-process-err")
	require.NoError(t, err)
	defer func() { _ = os.RemoveAll(tmpDir) }()

	viper.Set("root_config_file", "terrax.yaml")
	err = os.WriteFile(filepath.Join(tmpDir, "terrax.yaml"), []byte(""), 0644)
	require.NoError(t, err)

	c := &Collector{
		projectRoot: tmpDir,
		runDir:      tmpDir,
	}

	cacheDir := filepath.Join(tmpDir, "stacks/err", ".terragrunt-cache", "xyz")
	err = os.MkdirAll(cacheDir, 0755)
	require.NoError(t, err)
	planPath := filepath.Join(cacheDir, "plan.binary")

	// Test Case 1: Command Error
	mockExecMode = "error"
	mockCmdResponse = &MockCmd{}
	_, err = c.processStack(context.Background(), planPath)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "terraform show failed")
	mockExecMode = "" // Reset

	// Test Case 2: Invalid JSON
	mockCmdResponse = &MockCmd{Stdout: []byte("invalid-json")}
	_, err = c.processStack(context.Background(), planPath)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to parse json")
}

func TestProcessStack_RelativePathFallback(t *testing.T) {
	// Swap exec command
	oldExec := execCommandContext
	execCommandContext = fakeExecCommand
	defer func() { execCommandContext = oldExec }()

	tmpDir, err := os.MkdirTemp("", "terrax-test-process-fallback")
	require.NoError(t, err)
	defer func() { _ = os.RemoveAll(tmpDir) }()

	// Do NOT create terrax.yaml, so GetRelativeStackPath fails to find root
	// and returns error? Or returns absolute path?
	// Actually GetRelativeStackPath checks for root trigger. If not found, it returns error `project root not found`.

	// We need to ensure viper returns a filename that doesn't exist
	viper.Set("root_config_file", "nonexistent.yaml")

	c := &Collector{
		projectRoot: tmpDir,
		runDir:      tmpDir,
	}

	cacheDir := filepath.Join(tmpDir, "stacks/fallback", ".terragrunt-cache", "xyz")
	err = os.MkdirAll(cacheDir, 0755)
	require.NoError(t, err)
	planPath := filepath.Join(cacheDir, "plan.binary")

	mockExecMode = ""
	mockCmdResponse = &MockCmd{Stdout: []byte(`{"resource_changes": []}`)}

	res, err := c.processStack(context.Background(), planPath)
	require.NoError(t, err)
	require.NotNil(t, res)

	// Should fall back to cleanDir which is "stacks/fallback" (relative to runDir?)
	// cleanStackPath returns the path before .terragrunt-cache
	// processStack:
	// cleanDir := cleanStackPath(stackDir) -> /tmp/.../stacks/fallback
	// relPath, err := history.GetRelativeStackPath(cleanDir, configRoot)
	// if err != nil { relPath = cleanDir }

	// Since GetRelativeStackPath errors, relPath should be absolute path of cleanDir
	assert.Equal(t, filepath.Join(tmpDir, "stacks/fallback"), res.StackPath)
}

func TestCollect(t *testing.T) {
	// Swap exec command
	oldExec := execCommandContext
	execCommandContext = fakeExecCommand
	defer func() { execCommandContext = oldExec }()

	tmpDir, err := os.MkdirTemp("", "terrax-test-collect")
	assert.NoError(t, err)
	defer func() { _ = os.RemoveAll(tmpDir) }()

	timestamp := time.Now().Unix()
	viper.Set("terrax.session_timestamp", timestamp)
	viper.Set("root_config_file", "terrax.yaml")

	// Create terrax.yaml
	err = os.WriteFile(filepath.Join(tmpDir, "terrax.yaml"), []byte(""), 0644)
	assert.NoError(t, err)

	c := NewCollector(tmpDir)

	// Create a dummy plan
	cacheDir := filepath.Join(tmpDir, "stack1", ".terragrunt-cache", "uuid")
	err = os.MkdirAll(cacheDir, 0755)
	assert.NoError(t, err)
	planName := fmt.Sprintf("terrax-tfplan-%d.binary", timestamp)
	planPath := filepath.Join(cacheDir, planName)
	err = os.WriteFile(planPath, []byte("dummy"), 0644)
	assert.NoError(t, err)

	// Mock output
	mockCmdResponse = &MockCmd{Stdout: []byte(`{"resource_changes": []}`)} // No changes

	// Run Collect
	ctx := context.Background()
	progressChan := make(chan ProgressMsg, 10)

	// Consumer for progress to prevent blocking
	go func() {
		for range progressChan {
		}
	}()

	report, err := c.Collect(ctx, progressChan)
	assert.NoError(t, err)
	assert.NotNil(t, report)
	assert.Equal(t, 1, report.Summary.TotalStacks)
	assert.Equal(t, 0, report.Summary.StacksWithChanges)
	close(progressChan)
}
