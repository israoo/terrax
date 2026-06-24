package changes_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/israoo/terrax/internal/changes"
	"github.com/israoo/terrax/internal/stack"
)

// repoFixture sets up a minimal Terragrunt repo structure for testing:
//
//	<root>/
//	  root.hcl
//	  configuration/
//	    database/aurora.yaml
//	    compute/ecs-services.yaml
//	    network/security-groups/alb.yaml
//	  globals/globals.hcl              <- mark_as_read globals.yaml (not used here, for completeness)
//	  _envcommon/
//	    aurora.hcl                     <- mark_as_read aurora.yaml
//	    ecs.hcl                        <- mark_as_read ecs-services.yaml
//	    sg.hcl                         <- mark_as_read security-groups/${local.sg_name}.yaml (dynamic)
//	  workloads/
//	    prod/
//	      aurora/terragrunt.hcl        <- include _envcommon/aurora.hcl
//	      ecs/terragrunt.hcl           <- include _envcommon/ecs.hcl
//	      sg-alb/terragrunt.hcl        <- include _envcommon/sg.hcl
//	      alb/terragrunt.hcl           <- standalone stack (no envcommon)
func repoFixture(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()

	files := map[string]string{
		"root.hcl":                                       "",
		"configuration/database/aurora.yaml":             "aurora: {}",
		"configuration/compute/ecs-services.yaml":        "ecs_services: {}",
		"configuration/network/security-groups/alb.yaml": "rules: []",
		"globals/globals.hcl": `locals {
  _globals_yaml = mark_as_read("${get_repo_root()}/configuration/globals.yaml")
}`,
		"_envcommon/aurora.hcl": `locals {
  _aurora_yaml = mark_as_read("${get_repo_root()}/configuration/database/aurora.yaml")
}`,
		"_envcommon/ecs.hcl": `locals {
  _ecs_yaml = mark_as_read("${get_repo_root()}/configuration/compute/ecs-services.yaml")
}`,
		"_envcommon/sg.hcl": `locals {
  _sg_yaml = mark_as_read("${get_repo_root()}/configuration/network/security-groups/${local.sg_name}.yaml")
}`,
		"workloads/prod/aurora/terragrunt.hcl": `include "envcommon" {
  path = "${get_repo_root()}/_envcommon/aurora.hcl"
}`,
		"workloads/prod/ecs/terragrunt.hcl": `include "envcommon" {
  path = "${get_repo_root()}/_envcommon/ecs.hcl"
}`,
		"workloads/prod/sg-alb/terragrunt.hcl": `include "envcommon" {
  path = "${get_repo_root()}/_envcommon/sg.hcl"
}`,
		"workloads/prod/alb/terragrunt.hcl": `# standalone stack`,
	}

	for rel, content := range files {
		p := filepath.Join(dir, rel)
		require.NoError(t, os.MkdirAll(filepath.Dir(p), 0755))
		require.NoError(t, os.WriteFile(p, []byte(content), 0644))
	}
	return dir
}

// gitRepo wraps repoFixture in a real git repo with an initial commit,
// returning the repo root and the base commit SHA.
func gitRepo(t *testing.T) (root, baseSHA string) {
	t.Helper()
	root = repoFixture(t)

	gitRun(t, root, "init")
	gitRun(t, root, "config", "user.email", "test@test.com")
	gitRun(t, root, "config", "user.name", "Test")
	gitRun(t, root, "add", ".")
	gitRun(t, root, "commit", "-m", "initial")

	out, err := exec.Command("git", "-C", root, "rev-parse", "HEAD").Output()
	require.NoError(t, err)
	baseSHA = string(out[:len(out)-1]) // trim newline
	return root, baseSHA
}

func gitRun(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	require.NoError(t, err, "git %v: %s", args, out)
}

func gitCommitAll(t *testing.T, dir string) {
	t.Helper()
	gitRun(t, dir, "add", ".")
	gitRun(t, dir, "commit", "-m", "change")
}

// BuildFileGraph

func TestBuildFileGraph_MarkAsReadExactMapsYAMLToEnvcommonHCL(t *testing.T) {
	dir := repoFixture(t)

	graph, err := changes.BuildFileGraph(dir, "root.hcl")
	require.NoError(t, err)

	auroraYAML := filepath.Join(dir, "configuration", "database", "aurora.yaml")
	auroraHCL := filepath.Join(dir, "_envcommon", "aurora.hcl")
	assert.Contains(t, graph.MarkAsReadExact[auroraYAML], auroraHCL)
}

func TestBuildFileGraph_MarkAsReadPrefixMapsDirectoryToDynamicHCL(t *testing.T) {
	dir := repoFixture(t)

	graph, err := changes.BuildFileGraph(dir, "root.hcl")
	require.NoError(t, err)

	sgDir := filepath.Join(dir, "configuration", "network", "security-groups")
	sgHCL := filepath.Join(dir, "_envcommon", "sg.hcl")
	assert.Contains(t, graph.MarkAsReadPrefix[sgDir], sgHCL)
}

func TestBuildFileGraph_IncludeReverseMapsEnvcommonToLeafStack(t *testing.T) {
	dir := repoFixture(t)

	graph, err := changes.BuildFileGraph(dir, "root.hcl")
	require.NoError(t, err)

	auroraHCL := filepath.Join(dir, "_envcommon", "aurora.hcl")
	auroraStack := filepath.Join(dir, "workloads", "prod", "aurora", "terragrunt.hcl")
	assert.Contains(t, graph.IncludeReverse[auroraHCL], auroraStack)
}

// AffectedStacks

func TestAffectedStacks_DirectlyChangedStackIsReturned(t *testing.T) {
	root, baseSHA := gitRepo(t)

	albStack := filepath.Join(root, "workloads", "prod", "alb", "terragrunt.hcl")
	require.NoError(t, os.WriteFile(albStack, []byte("# changed"), 0644))
	gitCommitAll(t, root)

	graph, err := changes.BuildFileGraph(root, "root.hcl")
	require.NoError(t, err)
	tree, _, err := stack.FindAndBuildTree(root, "root.hcl")
	require.NoError(t, err)

	affected, err := changes.AffectedStacks(root, baseSHA, graph, tree)
	require.NoError(t, err)

	assert.Contains(t, affected, filepath.Join(root, "workloads", "prod", "alb"))
}

func TestAffectedStacks_ChangedStaticYAMLAffectsConsumingStack(t *testing.T) {
	root, baseSHA := gitRepo(t)

	auroraYAML := filepath.Join(root, "configuration", "database", "aurora.yaml")
	require.NoError(t, os.WriteFile(auroraYAML, []byte("aurora: {changed: true}"), 0644))
	gitCommitAll(t, root)

	graph, err := changes.BuildFileGraph(root, "root.hcl")
	require.NoError(t, err)
	tree, _, err := stack.FindAndBuildTree(root, "root.hcl")
	require.NoError(t, err)

	affected, err := changes.AffectedStacks(root, baseSHA, graph, tree)
	require.NoError(t, err)

	assert.Contains(t, affected, filepath.Join(root, "workloads", "prod", "aurora"))
	assert.NotContains(t, affected, filepath.Join(root, "workloads", "prod", "ecs"))
}

func TestAffectedStacks_ChangedDynamicYAMLPrefixAffectsConservativeMatch(t *testing.T) {
	root, baseSHA := gitRepo(t)

	sgYAML := filepath.Join(root, "configuration", "network", "security-groups", "alb.yaml")
	require.NoError(t, os.WriteFile(sgYAML, []byte("rules: [changed]"), 0644))
	gitCommitAll(t, root)

	graph, err := changes.BuildFileGraph(root, "root.hcl")
	require.NoError(t, err)
	tree, _, err := stack.FindAndBuildTree(root, "root.hcl")
	require.NoError(t, err)

	affected, err := changes.AffectedStacks(root, baseSHA, graph, tree)
	require.NoError(t, err)

	// sg-alb includes sg.hcl which has a dynamic mark_as_read for the security-groups dir.
	assert.Contains(t, affected, filepath.Join(root, "workloads", "prod", "sg-alb"))
}

func TestAffectedStacks_ChangedEnvcommonHCLAffectsIncludingStack(t *testing.T) {
	root, baseSHA := gitRepo(t)

	ecsHCL := filepath.Join(root, "_envcommon", "ecs.hcl")
	require.NoError(t, os.WriteFile(ecsHCL, []byte(`locals { changed = true }`), 0644))
	gitCommitAll(t, root)

	graph, err := changes.BuildFileGraph(root, "root.hcl")
	require.NoError(t, err)
	tree, _, err := stack.FindAndBuildTree(root, "root.hcl")
	require.NoError(t, err)

	affected, err := changes.AffectedStacks(root, baseSHA, graph, tree)
	require.NoError(t, err)

	assert.Contains(t, affected, filepath.Join(root, "workloads", "prod", "ecs"))
	assert.NotContains(t, affected, filepath.Join(root, "workloads", "prod", "aurora"))
}

func TestAffectedStacks_NoChangesReturnsEmpty(t *testing.T) {
	root, baseSHA := gitRepo(t)

	graph, err := changes.BuildFileGraph(root, "root.hcl")
	require.NoError(t, err)
	tree, _, err := stack.FindAndBuildTree(root, "root.hcl")
	require.NoError(t, err)

	affected, err := changes.AffectedStacks(root, baseSHA, graph, tree)
	require.NoError(t, err)

	assert.Empty(t, affected)
}
