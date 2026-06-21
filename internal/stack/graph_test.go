package stack

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

func makeTestNode(path string, isStack bool, deps []string) *Node {
	if deps == nil {
		deps = []string{}
	}
	return &Node{
		Name:         filepath.Base(path),
		Path:         path,
		IsStack:      isStack,
		Children:     []*Node{},
		Depth:        0,
		Dependencies: deps,
		Dependents:   []string{},
		InCycle:      false,
	}
}

func TestAnalyzeGraph_BuildsDependents(t *testing.T) {
	root := makeTestNode("/root", false, nil)
	a := makeTestNode("/a", true, []string{"/b"})
	b := makeTestNode("/b", true, nil)
	root.Children = []*Node{a, b}

	AnalyzeGraph(root)

	assert.Equal(t, []string{"/a"}, b.Dependents)
	assert.Empty(t, a.Dependents)
	assert.False(t, a.InCycle)
	assert.False(t, b.InCycle)
}

func TestAnalyzeGraph_DetectsCycle(t *testing.T) {
	root := makeTestNode("/root", false, nil)
	a := makeTestNode("/a", true, []string{"/b"})
	b := makeTestNode("/b", true, []string{"/c"})
	c := makeTestNode("/c", true, []string{"/a"})
	root.Children = []*Node{a, b, c}

	AnalyzeGraph(root)

	assert.True(t, a.InCycle)
	assert.True(t, b.InCycle)
	assert.True(t, c.InCycle)
}

func TestAnalyzeGraph_NoCycle(t *testing.T) {
	root := makeTestNode("/root", false, nil)
	a := makeTestNode("/a", true, []string{"/b"})
	b := makeTestNode("/b", true, []string{"/c"})
	c := makeTestNode("/c", true, nil)
	root.Children = []*Node{a, b, c}

	AnalyzeGraph(root)

	assert.False(t, a.InCycle)
	assert.False(t, b.InCycle)
	assert.False(t, c.InCycle)
}

func TestAnalyzeGraph_PartialCycle(t *testing.T) {
	// A → B → C → B (B and C form a cycle, A does not).
	root := makeTestNode("/root", false, nil)
	a := makeTestNode("/a", true, []string{"/b"})
	b := makeTestNode("/b", true, []string{"/c"})
	c := makeTestNode("/c", true, []string{"/b"})
	root.Children = []*Node{a, b, c}

	AnalyzeGraph(root)

	assert.False(t, a.InCycle)
	assert.True(t, b.InCycle)
	assert.True(t, c.InCycle)
}

func TestAnalyzeGraph_NonStackNodesUntouched(t *testing.T) {
	root := makeTestNode("/root", false, nil)
	dir := makeTestNode("/dir", false, nil)
	stack := makeTestNode("/dir/stack", true, nil)
	dir.Children = []*Node{stack}
	root.Children = []*Node{dir}

	AnalyzeGraph(root)

	assert.False(t, dir.InCycle)
	assert.Empty(t, dir.Dependents)
}
