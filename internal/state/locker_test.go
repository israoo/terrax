package state

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeExecCommand returns a *exec.Cmd that runs this test binary's TestHelperProcessLocker.
func fakeExecCommand(ctx context.Context, command string, args ...string) *exec.Cmd {
	cs := []string{"-test.run=TestHelperProcessLocker", "--", command}
	cs = append(cs, args...)
	cmd := exec.Command(os.Args[0], cs...)
	cmd.Env = []string{
		"GO_WANT_HELPER_PROCESS=1",
		fmt.Sprintf("GO_LOCKER_STDOUT=%s", lockerMockStdout),
		fmt.Sprintf("GO_LOCKER_STDERR=%s", lockerMockStderr),
		fmt.Sprintf("GO_LOCKER_EXIT=%d", lockerMockExit),
	}
	return cmd
}

// TestHelperProcessLocker is invoked as a subprocess by fakeExecCommand.
func TestHelperProcessLocker(t *testing.T) {
	if os.Getenv("GO_WANT_HELPER_PROCESS") != "1" {
		return
	}
	fmt.Print(os.Getenv("GO_LOCKER_STDOUT"))
	if s := os.Getenv("GO_LOCKER_STDERR"); s != "" {
		fmt.Fprint(os.Stderr, s)
	}
	if os.Getenv("GO_LOCKER_EXIT") == "1" {
		os.Exit(1)
	}
	os.Exit(0)
}

var (
	lockerMockStdout string
	lockerMockStderr string
	lockerMockExit   int
)

func TestGetLockID(t *testing.T) {
	oldExec := execCommandContext
	execCommandContext = fakeExecCommand
	defer func() { execCommandContext = oldExec }()

	tests := []struct {
		name         string
		bucket       string
		project      string
		stackRelPath string
		region       string
		mockStdout   string
		mockStderr   string
		mockExit     int
		wantID       string
		wantErr      bool
	}{
		{
			name:         "lock found returns ID",
			bucket:       "my-bucket",
			project:      "caas",
			stackRelPath: "workloads/dev/us-east-1/core/acm",
			region:       "us-east-1",
			mockStdout:   `{"ID":"abc-123","Who":"user@host","Operation":"OperationTypePlan","Created":"2026-06-20T00:00:00Z"}`,
			mockExit:     0,
			wantID:       "abc-123",
		},
		{
			name:         "no lock returns empty string",
			bucket:       "my-bucket",
			project:      "caas",
			stackRelPath: "workloads/dev/us-east-1/core/acm",
			region:       "us-east-1",
			mockStderr:   "An error occurred (NoSuchKey) when calling the GetObject operation",
			mockExit:     1,
			wantID:       "",
		},
		{
			name:         "aws CLI error returns error",
			bucket:       "my-bucket",
			project:      "caas",
			stackRelPath: "workloads/dev/us-east-1/core/acm",
			region:       "us-east-1",
			mockStderr:   "An error occurred (AccessDenied)",
			mockExit:     1,
			wantErr:      true,
		},
		{
			name:         "empty region falls back to default",
			bucket:       "my-bucket",
			project:      "caas",
			stackRelPath: "workloads/dev/us-east-1/core/acm",
			region:       "",
			mockStdout:   `{"ID":"def-456","Who":"ci@runner","Operation":"OperationTypeApply","Created":"2026-06-20T00:00:00Z"}`,
			mockExit:     0,
			wantID:       "def-456",
		},
		{
			name:         "malformed JSON returns error",
			bucket:       "my-bucket",
			project:      "caas",
			stackRelPath: "workloads/dev/us-east-1/core/acm",
			region:       "us-east-1",
			mockStdout:   `not-valid-json`,
			mockExit:     0,
			wantErr:      true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			lockerMockStdout = tt.mockStdout
			lockerMockStderr = tt.mockStderr
			lockerMockExit = tt.mockExit

			id, err := GetLockID(context.Background(), tt.bucket, tt.project, tt.stackRelPath, tt.region)

			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.wantID, id)
		})
	}
}
