package plan

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// summarizer mock controls
var (
	summarizerMockStdout string
	summarizerMockExit   int
)

// fakeExecSummarizer returns a *exec.Cmd running TestHelperProcessSummarizer.
func fakeExecSummarizer(ctx context.Context, command string, args ...string) *exec.Cmd {
	cs := []string{"-test.run=TestHelperProcessSummarizer", "--", command}
	cs = append(cs, args...)
	cmd := exec.Command(os.Args[0], cs...)
	cmd.Env = []string{
		"GO_WANT_HELPER_PROCESS=1",
		fmt.Sprintf("GO_SUMMARIZER_STDOUT=%s", summarizerMockStdout),
		fmt.Sprintf("GO_SUMMARIZER_EXIT=%d", summarizerMockExit),
		"HOME=" + os.TempDir(),
	}
	return cmd
}

// TestHelperProcessSummarizer is invoked as a subprocess by fakeExecSummarizer.
func TestHelperProcessSummarizer(t *testing.T) {
	if os.Getenv("GO_WANT_HELPER_PROCESS") != "1" {
		return
	}
	fmt.Print(os.Getenv("GO_SUMMARIZER_STDOUT"))
	if code := os.Getenv("GO_SUMMARIZER_EXIT"); code != "" && code != "0" {
		exitCode, _ := strconv.Atoi(code)
		os.Exit(exitCode)
	}
	os.Exit(0)
}

func TestSummarize_DirectoryNotExist(t *testing.T) {
	count, err := Summarize(context.Background(), "/nonexistent/path/xyz")
	require.NoError(t, err)
	assert.Equal(t, 0, count)
}

func TestSummarize_EmptyDirectory(t *testing.T) {
	dir := t.TempDir()
	count, err := Summarize(context.Background(), dir)
	require.NoError(t, err)
	assert.Equal(t, 0, count)
}

func TestSummarize_TFSummarizeNotFound(t *testing.T) {
	oldExec := execSummarizerContext
	execSummarizerContext = func(ctx context.Context, name string, args ...string) *exec.Cmd {
		// Running a nonexistent binary triggers exec.ErrNotFound.
		return exec.CommandContext(ctx, "tf-summarize-not-installed-xyz123")
	}
	defer func() { execSummarizerContext = oldExec }()

	dir := t.TempDir()
	stackDir := filepath.Join(dir, "workloads", "dev", "acm")
	require.NoError(t, os.MkdirAll(stackDir, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(stackDir, "plan.json"), []byte("{}"), 0644))

	_, err := Summarize(context.Background(), dir)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "tf-summarize not found")
}

func TestSummarize_StackWithChanges(t *testing.T) {
	oldExec := execSummarizerContext
	execSummarizerContext = fakeExecSummarizer
	defer func() { execSummarizerContext = oldExec }()

	summarizerMockStdout = `{"changes":{"add":2,"update":1,"delete":0,"recreate":0,"import":0,"moved":0}}`
	summarizerMockExit = 0

	dir := t.TempDir()
	stackDir := filepath.Join(dir, "workloads", "dev", "acm")
	require.NoError(t, os.MkdirAll(stackDir, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(stackDir, "plan.json"), []byte("{}"), 0644))

	count, err := Summarize(context.Background(), dir)
	require.NoError(t, err)
	assert.Equal(t, 1, count)
}

func TestSummarize_StackNoChanges(t *testing.T) {
	oldExec := execSummarizerContext
	execSummarizerContext = fakeExecSummarizer
	defer func() { execSummarizerContext = oldExec }()

	summarizerMockStdout = `{"changes":{"add":0,"update":0,"delete":0,"recreate":0,"import":0,"moved":0}}`
	summarizerMockExit = 0

	dir := t.TempDir()
	stackDir := filepath.Join(dir, "workloads", "dev", "acm")
	require.NoError(t, os.MkdirAll(stackDir, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(stackDir, "plan.json"), []byte("{}"), 0644))

	count, err := Summarize(context.Background(), dir)
	require.NoError(t, err)
	assert.Equal(t, 0, count)
}

func TestSummarize_MultipleStacksPartialChanges(t *testing.T) {
	oldExec := execSummarizerContext
	execSummarizerContext = fakeExecSummarizer
	defer func() { execSummarizerContext = oldExec }()

	// First call has changes, second does not (both use same mock stdout — fine for count test).
	summarizerMockStdout = `{"changes":{"add":1,"update":0,"delete":0,"recreate":0,"import":0,"moved":0}}`
	summarizerMockExit = 0

	dir := t.TempDir()
	for _, stack := range []string{"workloads/dev/acm", "workloads/dev/ecr"} {
		stackDir := filepath.Join(dir, filepath.FromSlash(stack))
		require.NoError(t, os.MkdirAll(stackDir, 0755))
		require.NoError(t, os.WriteFile(filepath.Join(stackDir, "plan.json"), []byte("{}"), 0644))
	}

	count, err := Summarize(context.Background(), dir)
	require.NoError(t, err)
	assert.Equal(t, 2, count) // both stacks have add:1
}
