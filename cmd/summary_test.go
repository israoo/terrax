package cmd

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSummaryCmd_NoPlanDir(t *testing.T) {
	tmpDir := t.TempDir()

	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	done := make(chan string, 1)
	go func() {
		var buf bytes.Buffer
		_, _ = io.Copy(&buf, r)
		done <- buf.String()
	}()

	cmd := &cobra.Command{}
	cmd.Flags().String("dir", "", "")
	require.NoError(t, cmd.ParseFlags([]string{"--dir", tmpDir}))

	err := runSummaryCmd(cmd, []string{})

	_ = w.Close()
	os.Stdout = oldStdout
	output := <-done

	require.NoError(t, err)
	assert.Empty(t, output, "no plan dir must produce no output")
}

func TestSummaryCmd_WithPlanFiles(t *testing.T) {
	tmpDir := t.TempDir()

	// Write a minimal root config file so FindRepoRoot finds this dir as root.
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "root.hcl"), []byte(""), 0644))

	// Write a minimal plan JSON with one "create" resource change.
	planDir := filepath.Join(tmpDir, ".terrax", "plans", "env", "dev", "vpc")
	require.NoError(t, os.MkdirAll(planDir, 0755))
	planJSON := `{
		"resource_changes": [
			{
				"address": "aws_vpc.main",
				"type": "aws_vpc",
				"name": "main",
				"change": {
					"actions": ["create"],
					"before": null,
					"after": {}
				}
			}
		]
	}`
	require.NoError(t, os.WriteFile(filepath.Join(planDir, "tfplan.json"), []byte(planJSON), 0644))

	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	done := make(chan string, 1)
	go func() {
		var buf bytes.Buffer
		_, _ = io.Copy(&buf, r)
		done <- buf.String()
	}()

	cmd := &cobra.Command{}
	cmd.Flags().String("dir", "", "")
	require.NoError(t, cmd.ParseFlags([]string{"--dir", tmpDir}))

	err := runSummaryCmd(cmd, []string{})

	_ = w.Close()
	os.Stdout = oldStdout
	output := <-done

	require.NoError(t, err)
	assert.Contains(t, output, "Pending changes", "output must report pending changes")
	assert.Contains(t, output, "env/dev/vpc", "output must include the stack path")
}
