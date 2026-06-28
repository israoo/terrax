package tui

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/israoo/terrax/internal/stack"
)

func makeTree() *stack.Node {
	return &stack.Node{
		Name: "root", Path: "/root",
		Children: []*stack.Node{
			{
				Name: "infra", Path: "/root/infra",
				Children: []*stack.Node{
					{Name: "dev", Path: "/root/infra/dev", IsStack: true},
					{Name: "prod", Path: "/root/infra/prod", IsStack: true},
				},
			},
			{Name: "mgmt", Path: "/root/mgmt", IsStack: true},
		},
	}
}

func TestFlattenDashTree_AllCollapsed(t *testing.T) {
	root := makeTree()
	nodes := flattenDashTree(root, map[string]bool{})
	// Only top-level children of root are visible when collapsed.
	require.Len(t, nodes, 2)
	assert.Equal(t, "infra", nodes[0].Node.Name)
	assert.Equal(t, 0, nodes[0].Depth)
	assert.Equal(t, false, nodes[0].IsLast)
	assert.Equal(t, "mgmt", nodes[1].Node.Name)
	assert.Equal(t, true, nodes[1].IsLast)
}

func TestFlattenDashTree_ExpandedInfra(t *testing.T) {
	root := makeTree()
	expanded := map[string]bool{"/root/infra": true}
	nodes := flattenDashTree(root, expanded)
	require.Len(t, nodes, 4) // infra + dev + prod + mgmt
	assert.Equal(t, "infra", nodes[0].Node.Name)
	assert.Equal(t, "dev", nodes[1].Node.Name)
	assert.Equal(t, 1, nodes[1].Depth)
	assert.Equal(t, false, nodes[1].IsLast)
	assert.Equal(t, "prod", nodes[2].Node.Name)
	assert.Equal(t, true, nodes[2].IsLast)
	assert.Equal(t, "mgmt", nodes[3].Node.Name)
}

func TestFlattenDashTree_PrefixConnectors(t *testing.T) {
	root := makeTree()
	expanded := map[string]bool{"/root/infra": true}
	nodes := flattenDashTree(root, expanded)
	// dev is not last child of infra → its children would use "│  " continuation
	assert.Equal(t, "", nodes[0].Prefix)    // infra: top-level, no parent prefix
	assert.Equal(t, "│  ", nodes[1].Prefix) // dev: infra is not last, so "│  " prefix
}
