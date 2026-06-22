package cmd_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// buildTerrax compiles the terrax binary into a temp dir and returns its path.
func buildTerrax(t *testing.T) string {
	t.Helper()
	bin := filepath.Join(t.TempDir(), "terrax")
	out, err := exec.Command("go", "build", "-o", bin, "../main.go").CombinedOutput()
	require.NoError(t, err, "go build: %s", out)
	return bin
}

// findTestRepo creates a minimal git repo for find subcommand tests:
//
//	<root>/
//	  root.hcl
//	  configuration/db/db.yaml
//	  _envcommon/db.hcl         <- mark_as_read db.yaml, included by prod/db
//	  workloads/prod/db/terragrunt.hcl
//	  workloads/prod/api/terragrunt.hcl  <- standalone
func findTestRepo(t *testing.T) (root, baseSHA string) {
	t.Helper()
	dir := t.TempDir()

	files := map[string]string{
		"root.hcl":                 "",
		"configuration/db/db.yaml": "db: {}",
		"_envcommon/db.hcl": `locals {
  _db_yaml = mark_as_read("${get_repo_root()}/configuration/db/db.yaml")
}`,
		"workloads/prod/db/terragrunt.hcl": `include "envcommon" {
  path = "${get_repo_root()}/_envcommon/db.hcl"
}`,
		"workloads/prod/api/terragrunt.hcl": `# standalone`,
	}
	for rel, content := range files {
		p := filepath.Join(dir, rel)
		require.NoError(t, os.MkdirAll(filepath.Dir(p), 0755))
		require.NoError(t, os.WriteFile(p, []byte(content), 0644))
	}

	run := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		out, err := cmd.CombinedOutput()
		require.NoError(t, err, "git %v: %s", args, out)
	}
	run("init")
	run("config", "user.email", "test@test.com")
	run("config", "user.name", "Test")
	run("add", ".")
	run("commit", "-m", "initial")

	out, err := exec.Command("git", "-C", dir, "rev-parse", "HEAD").Output()
	require.NoError(t, err)
	baseSHA = string(out[:len(out)-1])
	return dir, baseSHA
}

func TestFindCmd_WithBase_DirectStack(t *testing.T) {
	bin := buildTerrax(t)
	root, baseSHA := findTestRepo(t)

	apiHCL := filepath.Join(root, "workloads", "prod", "api", "terragrunt.hcl")
	require.NoError(t, os.WriteFile(apiHCL, []byte("# changed"), 0644))
	exec.Command("git", "-C", root, "add", ".").Run()          //nolint
	exec.Command("git", "-C", root, "commit", "-m", "c").Run() //nolint

	cmd := exec.Command(bin, "find", "--base", baseSHA, "--dir", root)
	out, err := cmd.Output()
	require.NoError(t, err)

	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	assert.Contains(t, lines, filepath.Join(root, "workloads", "prod", "api"))
	assert.NotContains(t, lines, filepath.Join(root, "workloads", "prod", "db"))
}

func TestFindCmd_WithBase_YAMLChangeViaMarkAsRead(t *testing.T) {
	bin := buildTerrax(t)
	root, baseSHA := findTestRepo(t)

	dbYAML := filepath.Join(root, "configuration", "db", "db.yaml")
	require.NoError(t, os.WriteFile(dbYAML, []byte("db: {changed: true}"), 0644))
	exec.Command("git", "-C", root, "add", ".").Run()          //nolint
	exec.Command("git", "-C", root, "commit", "-m", "c").Run() //nolint

	cmd := exec.Command(bin, "find", "--base", baseSHA, "--dir", root)
	out, err := cmd.Output()
	require.NoError(t, err)

	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	assert.Contains(t, lines, filepath.Join(root, "workloads", "prod", "db"))
	assert.NotContains(t, lines, filepath.Join(root, "workloads", "prod", "api"))
}

func TestFindCmd_WithoutBase_ListsAllStacks(t *testing.T) {
	bin := buildTerrax(t)
	root, _ := findTestRepo(t)

	cmd := exec.Command(bin, "find", "--dir", root)
	out, err := cmd.Output()
	require.NoError(t, err)

	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	assert.Contains(t, lines, filepath.Join(root, "workloads", "prod", "db"))
	assert.Contains(t, lines, filepath.Join(root, "workloads", "prod", "api"))
}

func TestFindCmd_WithBase_NoChanges_EmptyOutput(t *testing.T) {
	bin := buildTerrax(t)
	root, baseSHA := findTestRepo(t)

	cmd := exec.Command(bin, "find", "--base", baseSHA, "--dir", root)
	out, err := cmd.Output()
	require.NoError(t, err)
	assert.Empty(t, strings.TrimSpace(string(out)))
}
