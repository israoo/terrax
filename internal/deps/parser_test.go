package deps

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseDependencies_StaticRelative(t *testing.T) {
	dir := t.TempDir()
	hclPath := filepath.Join(dir, "app", "terragrunt.hcl")
	require.NoError(t, os.MkdirAll(filepath.Dir(hclPath), 0755))
	require.NoError(t, os.WriteFile(hclPath, []byte(`
dependency "vpc" {
  config_path = "../vpc"
  mock_outputs = {
    id = "vpc-123"
  }
}
dependency "sg" {
  config_path = "../security-groups"
}
`), 0644))

	got := ParseDependencies(hclPath, dir)
	assert.Equal(t, []string{
		filepath.Join(dir, "security-groups"),
		filepath.Join(dir, "vpc"),
	}, got)
}

func TestParseDependencies_GetRepoRoot(t *testing.T) {
	dir := t.TempDir()
	hclPath := filepath.Join(dir, "infra", "s3", "terragrunt.hcl")
	require.NoError(t, os.MkdirAll(filepath.Dir(hclPath), 0755))
	require.NoError(t, os.WriteFile(hclPath, []byte(`
dependency "iam" {
  config_path = "${get_repo_root()}/management/global/iam"
}
`), 0644))

	got := ParseDependencies(hclPath, dir)
	assert.Equal(t, []string{filepath.Join(dir, "management", "global", "iam")}, got)
}

func TestParseDependencies_FollowsInclude(t *testing.T) {
	dir := t.TempDir()
	envcommonPath := filepath.Join(dir, "_envcommon", "app.hcl")
	require.NoError(t, os.MkdirAll(filepath.Dir(envcommonPath), 0755))
	require.NoError(t, os.WriteFile(envcommonPath, []byte(`
dependency "vpc" {
  config_path = "../vpc"
}
`), 0644))

	leafPath := filepath.Join(dir, "dev", "app", "terragrunt.hcl")
	require.NoError(t, os.MkdirAll(filepath.Dir(leafPath), 0755))
	require.NoError(t, os.WriteFile(leafPath, []byte(`
include "envcommon" {
  path = "${get_repo_root()}/_envcommon/app.hcl"
}
`), 0644))

	got := ParseDependencies(leafPath, dir)
	// ../vpc resolved relative to _envcommon/ means one level up from _envcommon, giving dir/vpc.
	assert.Equal(t, []string{filepath.Join(dir, "vpc")}, got)
}

func TestParseDependencies_SkipsFindInParentFolders(t *testing.T) {
	dir := t.TempDir()
	hclPath := filepath.Join(dir, "app", "terragrunt.hcl")
	require.NoError(t, os.MkdirAll(filepath.Dir(hclPath), 0755))
	require.NoError(t, os.WriteFile(hclPath, []byte(`
include "root" {
  path = find_in_parent_folders("root.hcl")
}
`), 0644))

	got := ParseDependencies(hclPath, dir)
	assert.Empty(t, got)
}

func TestParseDependencies_MissingFile(t *testing.T) {
	got := ParseDependencies("/nonexistent/path/terragrunt.hcl", "/repo")
	assert.Equal(t, []string{}, got)
}

func TestFindRepoRoot_FindsRoot(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "root.hcl"), []byte(""), 0644))
	nested := filepath.Join(dir, "a", "b", "c")
	require.NoError(t, os.MkdirAll(nested, 0755))

	got := FindRepoRoot(nested, "root.hcl")
	assert.Equal(t, dir, got)
}

func TestFindRepoRoot_FallsBackToStartDir(t *testing.T) {
	dir := t.TempDir()
	got := FindRepoRoot(dir, "root.hcl")
	assert.Equal(t, dir, got)
}
