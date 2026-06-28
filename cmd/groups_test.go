package cmd_test

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// groupsTestRepo creates a minimal repo with two stacks:
//   - workloads/default/vpc    — no stack.hcl (default group)
//   - workloads/private/db     — stack.hcl with require_private_connection = true
//
// .terrax.yaml configures two groups: default and private_connection.
func groupsTestRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()

	files := map[string]string{
		"root.hcl": "",
		".terrax.yaml": `
stack_groups:
  default:
    depends_on: []
  private_connection:
    detect: "require_private_connection = true"
    depends_on: [default]
`,
		"workloads/default/vpc/terragrunt.hcl": "# vpc stack",
		"workloads/private/db/terragrunt.hcl":  "# db stack",
		"workloads/private/db/stack.hcl":       "require_private_connection = true",
	}
	for rel, content := range files {
		p := filepath.Join(dir, rel)
		require.NoError(t, os.MkdirAll(filepath.Dir(p), 0755))
		require.NoError(t, os.WriteFile(p, []byte(content), 0644))
	}
	return dir
}

func TestGroupsCmd_NoStack_AllStacks(t *testing.T) {
	bin := buildTerrax(t)
	root := groupsTestRepo(t)

	out, err := exec.Command(bin, "groups", "--dir", root).Output()
	require.NoError(t, err)

	var result struct {
		Groups []struct {
			Name    string   `json:"name"`
			Filters []string `json:"filters"`
		} `json:"groups"`
	}
	require.NoError(t, json.Unmarshal(out, &result))

	groups := map[string][]string{}
	for _, g := range result.Groups {
		groups[g.Name] = g.Filters
	}

	assert.Contains(t, groups, "default")
	assert.Contains(t, groups, "private_connection")

	assert.Contains(t, groups["default"], "workloads/default/vpc")
	assert.Contains(t, groups["private_connection"], "workloads/private/db")
}

func TestGroupsCmd_StackFlag_OnlyProvidedStacks(t *testing.T) {
	bin := buildTerrax(t)
	root := groupsTestRepo(t)

	vpcAbs := filepath.Join(root, "workloads", "default", "vpc")
	out, err := exec.Command(bin, "groups", "--dir", root, "--stack", vpcAbs).Output()
	require.NoError(t, err)

	var result struct {
		Groups []struct {
			Name    string   `json:"name"`
			Filters []string `json:"filters"`
		} `json:"groups"`
	}
	require.NoError(t, json.Unmarshal(out, &result))

	groups := map[string][]string{}
	for _, g := range result.Groups {
		groups[g.Name] = g.Filters
	}

	assert.Contains(t, groups["default"], "workloads/default/vpc")

	// db stack was not passed — must not appear in any group.
	for name, filters := range groups {
		for _, f := range filters {
			assert.NotContains(t, f, "db", "group %q should not contain db stack", name)
		}
	}
}

func TestGroupsCmd_StackFlag_PrivateStack(t *testing.T) {
	bin := buildTerrax(t)
	root := groupsTestRepo(t)

	dbAbs := filepath.Join(root, "workloads", "private", "db")
	out, err := exec.Command(bin, "groups", "--dir", root, "--stack", dbAbs).Output()
	require.NoError(t, err)

	var result struct {
		Groups []struct {
			Name    string   `json:"name"`
			Filters []string `json:"filters"`
		} `json:"groups"`
	}
	require.NoError(t, json.Unmarshal(out, &result))

	groups := map[string][]string{}
	for _, g := range result.Groups {
		groups[g.Name] = g.Filters
	}

	assert.Contains(t, groups["private_connection"], "workloads/private/db")

	for _, f := range groups["default"] {
		assert.NotContains(t, f, "vpc", "default group should not contain vpc when only db was passed")
	}
}

func TestGroupsCmd_StackFlag_MultipleStacks(t *testing.T) {
	bin := buildTerrax(t)
	root := groupsTestRepo(t)

	vpcAbs := filepath.Join(root, "workloads", "default", "vpc")
	dbAbs := filepath.Join(root, "workloads", "private", "db")
	out, err := exec.Command(bin, "groups", "--dir", root,
		"--stack", vpcAbs, "--stack", dbAbs).Output()
	require.NoError(t, err)

	var result struct {
		Groups []struct {
			Name    string   `json:"name"`
			Filters []string `json:"filters"`
		} `json:"groups"`
	}
	require.NoError(t, json.Unmarshal(out, &result))

	groups := map[string][]string{}
	for _, g := range result.Groups {
		groups[g.Name] = g.Filters
	}

	assert.Contains(t, groups["default"], "workloads/default/vpc")
	assert.Contains(t, groups["private_connection"], "workloads/private/db")
}
