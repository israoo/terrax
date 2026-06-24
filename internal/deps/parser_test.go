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
	// ../vpc resolved relative to the leaf dir (dev/app/) gives dev/vpc, matching Terragrunt runtime behavior.
	assert.Equal(t, []string{filepath.Join(dir, "dev", "vpc")}, got)
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

// ParseIncludes

func TestParseIncludes_ResolvesStaticGetRepoRoot(t *testing.T) {
	dir := t.TempDir()
	hclPath := filepath.Join(dir, "workloads", "prod", "app", "terragrunt.hcl")
	require.NoError(t, os.MkdirAll(filepath.Dir(hclPath), 0755))
	require.NoError(t, os.WriteFile(hclPath, []byte(`
include "root" {
  path = find_in_parent_folders("root.hcl")
}
include "envcommon" {
  path = "${get_repo_root()}/_envcommon/app.hcl"
}
`), 0644))

	got := ParseIncludes(hclPath, dir)
	assert.Equal(t, []string{filepath.Join(dir, "_envcommon", "app.hcl")}, got)
}

func TestParseIncludes_ResolvesRelativePath(t *testing.T) {
	dir := t.TempDir()
	hclPath := filepath.Join(dir, "workloads", "prod", "app", "terragrunt.hcl")
	require.NoError(t, os.MkdirAll(filepath.Dir(hclPath), 0755))
	require.NoError(t, os.WriteFile(hclPath, []byte(`
include "sibling" {
  path = "../shared.hcl"
}
`), 0644))

	got := ParseIncludes(hclPath, dir)
	assert.Equal(t, []string{filepath.Join(dir, "workloads", "prod", "shared.hcl")}, got)
}

func TestParseIncludes_SkipsDynamicPaths(t *testing.T) {
	dir := t.TempDir()
	hclPath := filepath.Join(dir, "app", "terragrunt.hcl")
	require.NoError(t, os.MkdirAll(filepath.Dir(hclPath), 0755))
	require.NoError(t, os.WriteFile(hclPath, []byte(`
include "root" {
  path = find_in_parent_folders("root.hcl")
}
include "dynamic" {
  path = "${local.some_var}/envcommon.hcl"
}
`), 0644))

	got := ParseIncludes(hclPath, dir)
	assert.Empty(t, got)
}

func TestParseIncludes_MissingFile(t *testing.T) {
	got := ParseIncludes("/nonexistent/terragrunt.hcl", "/repo")
	assert.Empty(t, got)
}

// ParseMarkAsRead

func TestParseMarkAsRead_StaticGetRepoRootPath(t *testing.T) {
	dir := t.TempDir()
	hclPath := filepath.Join(dir, "_envcommon", "aurora.hcl")
	require.NoError(t, os.MkdirAll(filepath.Dir(hclPath), 0755))
	require.NoError(t, os.WriteFile(hclPath, []byte(`
locals {
  _aurora_yaml = mark_as_read("${get_repo_root()}/configuration/database/aurora.yaml")
  config       = yamldecode(file(local._aurora_yaml))
}
`), 0644))

	staticPaths, dynamicPrefixes := ParseMarkAsRead(hclPath, dir)
	assert.Equal(t, []string{filepath.Join(dir, "configuration", "database", "aurora.yaml")}, staticPaths)
	assert.Empty(t, dynamicPrefixes)
}

func TestParseMarkAsRead_DynamicPathExtractsDirectoryPrefix(t *testing.T) {
	dir := t.TempDir()
	hclPath := filepath.Join(dir, "_envcommon", "security-group.hcl")
	require.NoError(t, os.MkdirAll(filepath.Dir(hclPath), 0755))
	require.NoError(t, os.WriteFile(hclPath, []byte(`
locals {
  _sg_yaml = mark_as_read("${get_repo_root()}/configuration/network/security-groups/${local.sg_name}.yaml")
}
`), 0644))

	staticPaths, dynamicPrefixes := ParseMarkAsRead(hclPath, dir)
	assert.Empty(t, staticPaths)
	assert.Equal(t, []string{filepath.Join(dir, "configuration", "network", "security-groups")}, dynamicPrefixes)
}

func TestParseMarkAsRead_MultipleEntriesMixed(t *testing.T) {
	dir := t.TempDir()
	hclPath := filepath.Join(dir, "_envcommon", "ecs.hcl")
	require.NoError(t, os.MkdirAll(filepath.Dir(hclPath), 0755))
	require.NoError(t, os.WriteFile(hclPath, []byte(`
locals {
  _ecs_yaml     = mark_as_read("${get_repo_root()}/configuration/compute/ecs-services.yaml")
  _cluster_yaml = mark_as_read("${get_repo_root()}/configuration/compute/ecs-cluster.yaml")
  _sg_yaml      = mark_as_read("${get_repo_root()}/configuration/network/security-groups/${local.sg_name}.yaml")
}
`), 0644))

	staticPaths, dynamicPrefixes := ParseMarkAsRead(hclPath, dir)
	assert.Equal(t, []string{
		filepath.Join(dir, "configuration", "compute", "ecs-cluster.yaml"),
		filepath.Join(dir, "configuration", "compute", "ecs-services.yaml"),
	}, staticPaths)
	assert.Equal(t, []string{filepath.Join(dir, "configuration", "network", "security-groups")}, dynamicPrefixes)
}

func TestParseMarkAsRead_NoMarkAsRead(t *testing.T) {
	dir := t.TempDir()
	hclPath := filepath.Join(dir, "app", "terragrunt.hcl")
	require.NoError(t, os.MkdirAll(filepath.Dir(hclPath), 0755))
	require.NoError(t, os.WriteFile(hclPath, []byte(`
dependency "vpc" {
  config_path = "../vpc"
}
`), 0644))

	staticPaths, dynamicPrefixes := ParseMarkAsRead(hclPath, dir)
	assert.Empty(t, staticPaths)
	assert.Empty(t, dynamicPrefixes)
}

func TestParseMarkAsRead_MissingFile(t *testing.T) {
	staticPaths, dynamicPrefixes := ParseMarkAsRead("/nonexistent/terragrunt.hcl", "/repo")
	assert.Empty(t, staticPaths)
	assert.Empty(t, dynamicPrefixes)
}

// ScanAllHCLFiles

func TestScanAllHCLFiles_FindsAllHCLFilesExcludingSystemDirs(t *testing.T) {
	dir := t.TempDir()

	wantFiles := []string{
		"root.hcl",
		"globals/globals.hcl",
		"_envcommon/core.hcl",
		"workloads/prod/app/terragrunt.hcl",
	}
	skipFiles := []string{
		".git/config.hcl",
		".terraform/backend.hcl",
		".terragrunt-cache/cached.hcl",
		"vendor/module/main.hcl",
	}
	otherFiles := []string{
		"configuration/globals.yaml",
		"README.md",
	}

	for _, f := range append(append(wantFiles, skipFiles...), otherFiles...) {
		p := filepath.Join(dir, f)
		require.NoError(t, os.MkdirAll(filepath.Dir(p), 0755))
		require.NoError(t, os.WriteFile(p, []byte(""), 0644))
	}

	got := ScanAllHCLFiles(dir)

	var expected []string
	for _, f := range wantFiles {
		expected = append(expected, filepath.Join(dir, f))
	}
	assert.ElementsMatch(t, expected, got)
}
