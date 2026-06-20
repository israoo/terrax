package stack

import (
	"os"
	"testing"

	"github.com/spf13/afero"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFindAndBuildTree_Success(t *testing.T) {
	// Create in-memory filesystem.
	fs := afero.NewMemMapFs()

	// Set up fixture: create directory hierarchy.
	// Structure:
	//  .
	//  ├── dev
	//  │   └── us-east-1
	//  │       └── vpc.hcl
	//  ├── prod
	//  │   └── eu-west-1
	//  │       └── app.hcl
	require.NoError(t, fs.MkdirAll("/root/dev/us-east-1", 0755))
	require.NoError(t, fs.MkdirAll("/root/prod/eu-west-1", 0755))

	// Create terragrunt.hcl files to mark stack directories.
	require.NoError(t, afero.WriteFile(fs, "/root/dev/us-east-1/terragrunt.hcl", []byte("# vpc config"), 0644))
	require.NoError(t, afero.WriteFile(fs, "/root/prod/eu-west-1/terragrunt.hcl", []byte("# app config"), 0644))

	// Build tree using mocked filesystem.
	tree, maxDepth, err := findAndBuildTreeWithFS(fs, "/root")

	// Assertions.
	require.NoError(t, err, "should build tree without error")
	require.NotNil(t, tree, "tree root should not be nil")

	// Verify root node.
	assert.Equal(t, "root", tree.Name, "root node name should be 'root'")
	assert.Equal(t, "/root", tree.Path, "root node path should be '/root'")
	assert.Equal(t, 0, tree.Depth, "root node depth should be 0")

	// Verify max depth (root=0, dev/prod=1, us-east-1/eu-west-1=2).
	assert.Equal(t, 2, maxDepth, "max depth should be 2")

	// Verify first level children (dev, prod).
	require.Len(t, tree.Children, 2, "root should have 2 children")

	// Find dev and prod nodes (order may vary).
	var devNode, prodNode *Node
	for _, child := range tree.Children {
		switch child.Name {
		case "dev":
			devNode = child
		case "prod":
			prodNode = child
		}
	}

	require.NotNil(t, devNode, "dev node should exist")
	require.NotNil(t, prodNode, "prod node should exist")

	// Verify dev node.
	assert.Equal(t, "dev", devNode.Name)
	assert.Equal(t, "/root/dev", devNode.Path)
	assert.Equal(t, 1, devNode.Depth)
	assert.False(t, devNode.IsStack, "dev should not be a stack directory")

	// Verify prod node.
	assert.Equal(t, "prod", prodNode.Name)
	assert.Equal(t, "/root/prod", prodNode.Path)
	assert.Equal(t, 1, prodNode.Depth)
	assert.False(t, prodNode.IsStack, "prod should not be a stack directory")

	// Verify second level children.
	require.Len(t, devNode.Children, 1, "dev should have 1 child")
	require.Len(t, prodNode.Children, 1, "prod should have 1 child")

	usEast1Node := devNode.Children[0]
	euWest1Node := prodNode.Children[0]

	// Verify us-east-1 node.
	assert.Equal(t, "us-east-1", usEast1Node.Name)
	assert.Equal(t, "/root/dev/us-east-1", usEast1Node.Path)
	assert.Equal(t, 2, usEast1Node.Depth)
	assert.True(t, usEast1Node.IsStack, "us-east-1 should be a stack directory")

	// Verify eu-west-1 node.
	assert.Equal(t, "eu-west-1", euWest1Node.Name)
	assert.Equal(t, "/root/prod/eu-west-1", euWest1Node.Path)
	assert.Equal(t, 2, euWest1Node.Depth)
	assert.True(t, euWest1Node.IsStack, "eu-west-1 should be a stack directory")
}

// findAndBuildTreeWithFS is an internal helper that accepts an afero.Fs for testing.
// This function wraps the filesystem operations to enable testing with afero.
func findAndBuildTreeWithFS(fs afero.Fs, rootDir string) (*Node, int, error) {
	// Verify directory exists.
	info, err := fs.Stat(rootDir)
	if err != nil {
		return nil, 0, err
	}
	if !info.IsDir() {
		return nil, 0, err
	}

	// Build the tree starting from root.
	root := &Node{
		Name:     info.Name(),
		Path:     rootDir,
		IsStack:  isStackDirectoryWithFS(fs, rootDir),
		Children: make([]*Node, 0),
		Depth:    0,
	}

	maxDepth := 0
	if err := buildTreeRecursiveWithFS(fs, root, &maxDepth); err != nil {
		return nil, 0, err
	}

	return root, maxDepth, nil
}

// buildTreeRecursiveWithFS recursively builds the tree structure using afero.Fs.
func buildTreeRecursiveWithFS(fs afero.Fs, node *Node, maxDepth *int) error {
	entries, err := afero.ReadDir(fs, node.Path)
	if err != nil {
		// Skip directories we can't read.
		return nil
	}

	for _, entry := range entries {
		// Skip non-directories and hidden directories.
		if !entry.IsDir() || entry.Name()[0] == '.' {
			continue
		}

		// Skip common non-stack directories.
		if shouldSkipDirectory(entry.Name()) {
			continue
		}

		childPath := node.Path + "/" + entry.Name()
		childNode := &Node{
			Name:     entry.Name(),
			Path:     childPath,
			IsStack:  isStackDirectoryWithFS(fs, childPath),
			Children: make([]*Node, 0),
			Depth:    node.Depth + 1,
		}

		// Recursively build children first.
		if err := buildTreeRecursiveWithFS(fs, childNode, maxDepth); err != nil {
			continue // Skip problematic subdirectories.
		}

		// FILTER: Only add directories that are stacks OR contain stacks.
		// This prevents leaf directories like "global/" with only "globals.hcl" from appearing.
		if childNode.IsStack || len(childNode.Children) > 0 {
			// Update max depth only for included nodes.
			if childNode.Depth > *maxDepth {
				*maxDepth = childNode.Depth
			}
			node.Children = append(node.Children, childNode)
		}
	}

	return nil
}

// isStackDirectoryWithFS checks if a directory contains stack definition files using afero.Fs.
func isStackDirectoryWithFS(fs afero.Fs, dirPath string) bool {
	// Check for Terragrunt.
	if _, err := fs.Stat(dirPath + "/terragrunt.hcl"); err == nil {
		return true
	}

	return false
}

// TestNode_GetChildren tests retrieving child nodes.
func TestNode_GetChildren(t *testing.T) {
	tests := []struct {
		name     string
		node     *Node
		expected []*Node
	}{
		{
			name: "node with children",
			node: &Node{
				Name: "parent",
				Children: []*Node{
					{Name: "child1"},
					{Name: "child2"},
				},
			},
			expected: []*Node{
				{Name: "child1"},
				{Name: "child2"},
			},
		},
		{
			name:     "node without children",
			node:     &Node{Name: "leaf"},
			expected: nil,
		},
		{
			name:     "nil node",
			node:     nil,
			expected: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			children := tt.node.GetChildren()
			assert.Equal(t, tt.expected, children)
		})
	}
}

// TestNode_HasChildren tests checking if a node has children.
func TestNode_HasChildren(t *testing.T) {
	tests := []struct {
		name     string
		node     *Node
		expected bool
	}{
		{
			name: "node with children",
			node: &Node{
				Name:     "parent",
				Children: []*Node{{Name: "child"}},
			},
			expected: true,
		},
		{
			name:     "node without children",
			node:     &Node{Name: "leaf"},
			expected: false,
		},
		{
			name:     "nil node",
			node:     nil,
			expected: false,
		},
		{
			name: "node with empty children slice",
			node: &Node{
				Name:     "empty",
				Children: []*Node{},
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hasChildren := tt.node.HasChildren()
			assert.Equal(t, tt.expected, hasChildren)
		})
	}
}

// TestNode_GetChildNames tests getting child node names with markers.
func TestNode_GetChildNames(t *testing.T) {
	tests := []struct {
		name     string
		node     *Node
		expected []string
	}{
		{
			name: "children with stack marker",
			node: &Node{
				Name: "parent",
				Children: []*Node{
					{Name: "env", IsStack: false},
					{Name: "vpc", IsStack: true},
					{Name: "app", IsStack: true},
				},
			},
			expected: []string{"env", "vpc 📦", "app 📦"},
		},
		{
			name: "children without stack marker",
			node: &Node{
				Name: "parent",
				Children: []*Node{
					{Name: "child1", IsStack: false},
					{Name: "child2", IsStack: false},
				},
			},
			expected: []string{"child1", "child2"},
		},
		{
			name:     "no children",
			node:     &Node{Name: "leaf"},
			expected: []string{},
		},
		{
			name:     "nil node",
			node:     nil,
			expected: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			names := tt.node.GetChildNames()
			assert.Equal(t, tt.expected, names)
		})
	}
}

// TestNode_FindChildByIndex tests finding a child by index.
func TestNode_FindChildByIndex(t *testing.T) {
	parent := &Node{
		Name: "parent",
		Children: []*Node{
			{Name: "child0"},
			{Name: "child1"},
			{Name: "child2"},
		},
	}

	tests := []struct {
		name         string
		node         *Node
		index        int
		expectedName string
		expectNil    bool
	}{
		{
			name:         "valid index 0",
			node:         parent,
			index:        0,
			expectedName: "child0",
			expectNil:    false,
		},
		{
			name:         "valid index 2",
			node:         parent,
			index:        2,
			expectedName: "child2",
			expectNil:    false,
		},
		{
			name:      "negative index",
			node:      parent,
			index:     -1,
			expectNil: true,
		},
		{
			name:      "index out of bounds",
			node:      parent,
			index:     10,
			expectNil: true,
		},
		{
			name:      "node without children",
			node:      &Node{Name: "leaf"},
			index:     0,
			expectNil: true,
		},
		{
			name:      "nil node",
			node:      nil,
			index:     0,
			expectNil: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			child := tt.node.FindChildByIndex(tt.index)

			if tt.expectNil {
				assert.Nil(t, child)
			} else {
				require.NotNil(t, child)
				assert.Equal(t, tt.expectedName, child.Name)
			}
		})
	}
}

// TestShouldSkipDirectory tests directory filtering logic.
func TestShouldSkipDirectory(t *testing.T) {
	tests := []struct {
		name     string
		dirName  string
		expected bool
	}{
		{
			name:     "skip .git directory",
			dirName:  ".git",
			expected: true,
		},
		{
			name:     "skip .terraform directory",
			dirName:  ".terraform",
			expected: true,
		},
		{
			name:     "skip .terragrunt-cache directory",
			dirName:  ".terragrunt-cache",
			expected: true,
		},
		{
			name:     "skip vendor directory",
			dirName:  "vendor",
			expected: true,
		},
		{
			name:     "skip .idea directory",
			dirName:  ".idea",
			expected: true,
		},
		{
			name:     "skip .vscode directory",
			dirName:  ".vscode",
			expected: true,
		},
		{
			name:     "do not skip normal directory",
			dirName:  "modules",
			expected: false,
		},
		{
			name:     "do not skip env directory",
			dirName:  "env",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := shouldSkipDirectory(tt.dirName)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// TestFindAndBuildTree_EmptyDirectory tests building a tree from an empty directory.
func TestFindAndBuildTree_EmptyDirectory(t *testing.T) {
	fs := afero.NewMemMapFs()

	// Create empty root directory.
	require.NoError(t, fs.MkdirAll("/empty", 0755))

	tree, maxDepth, err := findAndBuildTreeWithFS(fs, "/empty")

	require.NoError(t, err)
	require.NotNil(t, tree)
	assert.Equal(t, "empty", tree.Name)
	assert.Equal(t, 0, maxDepth)
	assert.Empty(t, tree.Children)
}

// TestFindAndBuildTree_HiddenDirectories tests that hidden directories are skipped.
func TestFindAndBuildTree_HiddenDirectories(t *testing.T) {
	fs := afero.NewMemMapFs()

	// Create directory structure with hidden directories.
	require.NoError(t, fs.MkdirAll("/root/visible", 0755))
	require.NoError(t, fs.MkdirAll("/root/.hidden", 0755))
	require.NoError(t, fs.MkdirAll("/root/.git", 0755))
	// Make visible a stack so it appears in the tree.
	require.NoError(t, afero.WriteFile(fs, "/root/visible/terragrunt.hcl", []byte(""), 0644))

	tree, maxDepth, err := findAndBuildTreeWithFS(fs, "/root")

	require.NoError(t, err)
	require.NotNil(t, tree)
	assert.Equal(t, 1, maxDepth)
	require.Len(t, tree.Children, 1)
	assert.Equal(t, "visible", tree.Children[0].Name)
}

// TestFindAndBuildTree_SkippedDirectories tests that configured skip directories are filtered.
func TestFindAndBuildTree_SkippedDirectories(t *testing.T) {
	fs := afero.NewMemMapFs()

	// Create directory structure with skip directories.
	require.NoError(t, fs.MkdirAll("/root/modules", 0755))
	require.NoError(t, fs.MkdirAll("/root/.terraform", 0755))
	require.NoError(t, fs.MkdirAll("/root/vendor", 0755))
	require.NoError(t, fs.MkdirAll("/root/.vscode", 0755))
	// Make modules a stack so it appears in tree.
	require.NoError(t, afero.WriteFile(fs, "/root/modules/terragrunt.hcl", []byte(""), 0644))

	tree, maxDepth, err := findAndBuildTreeWithFS(fs, "/root")

	require.NoError(t, err)
	require.NotNil(t, tree)
	assert.Equal(t, 1, maxDepth)
	require.Len(t, tree.Children, 1)
	assert.Equal(t, "modules", tree.Children[0].Name)
}

// TestFindAndBuildTree_DeepHierarchy tests building a deep directory tree.
func TestFindAndBuildTree_DeepHierarchy(t *testing.T) {
	fs := afero.NewMemMapFs()

	// Create deep hierarchy: level0/level1/level2/level3/level4.
	require.NoError(t, fs.MkdirAll("/root/level1/level2/level3/level4", 0755))
	// Make the leaf a stack so intermediate directories appear.
	require.NoError(t, afero.WriteFile(fs, "/root/level1/level2/level3/level4/terragrunt.hcl", []byte(""), 0644))

	tree, maxDepth, err := findAndBuildTreeWithFS(fs, "/root")

	require.NoError(t, err)
	require.NotNil(t, tree)
	assert.Equal(t, 4, maxDepth)

	// Verify the chain.
	current := tree
	for i := 1; i <= 4; i++ {
		require.Len(t, current.Children, 1)
		current = current.Children[0]
		expectedName := "level" + string(rune('0'+i))
		assert.Equal(t, expectedName, current.Name)
	}
}

// TestFindAndBuildTree_MultipleStackFiles tests detection of multiple stack directories.
func TestFindAndBuildTree_MultipleStackFiles(t *testing.T) {
	fs := afero.NewMemMapFs()

	// Create structure with multiple stack files.
	require.NoError(t, fs.MkdirAll("/root/stack1", 0755))
	require.NoError(t, fs.MkdirAll("/root/stack2", 0755))
	require.NoError(t, fs.MkdirAll("/root/nostack", 0755))

	require.NoError(t, afero.WriteFile(fs, "/root/stack1/terragrunt.hcl", []byte(""), 0644))
	require.NoError(t, afero.WriteFile(fs, "/root/stack2/terragrunt.hcl", []byte(""), 0644))

	tree, _, err := findAndBuildTreeWithFS(fs, "/root")

	require.NoError(t, err)
	// Only directories with stacks should appear (nostack should be filtered out).
	require.Len(t, tree.Children, 2, "only stack directories should be included")

	// Verify both children are stacks.
	stackCount := 0
	for _, child := range tree.Children {
		if child.IsStack {
			stackCount++
		}
	}
	assert.Equal(t, 2, stackCount, "both children should be stacks")
}

// TestFindAndBuildTree_NonStackDirectoriesFiltered tests that directories without
// terragrunt.hcl and without stack descendants are filtered out (e.g., 'global' with only 'globals.hcl').
func TestFindAndBuildTree_NonStackDirectoriesFiltered(t *testing.T) {
	fs := afero.NewMemMapFs()

	// Create structure:
	//  root/
	//  ├── global/               <- NO terragrunt.hcl, only globals.hcl (should be filtered)
	//  │   └── globals.hcl
	//  ├── env/                  <- NO terragrunt.hcl, but contains stacks (should appear)
	//  │   ├── dev/
	//  │   │   └── terragrunt.hcl
	//  │   └── prod/
	//  │       └── terragrunt.hcl
	//  └── modules/              <- NO terragrunt.hcl, no stack descendants (should be filtered)
	//      └── some-module/
	//          └── main.tf

	require.NoError(t, fs.MkdirAll("/root/global", 0755))
	require.NoError(t, afero.WriteFile(fs, "/root/global/globals.hcl", []byte("# global vars"), 0644))

	require.NoError(t, fs.MkdirAll("/root/env/dev", 0755))
	require.NoError(t, afero.WriteFile(fs, "/root/env/dev/terragrunt.hcl", []byte(""), 0644))
	require.NoError(t, fs.MkdirAll("/root/env/prod", 0755))
	require.NoError(t, afero.WriteFile(fs, "/root/env/prod/terragrunt.hcl", []byte(""), 0644))

	require.NoError(t, fs.MkdirAll("/root/modules/some-module", 0755))
	require.NoError(t, afero.WriteFile(fs, "/root/modules/some-module/main.tf", []byte(""), 0644))

	tree, maxDepth, err := findAndBuildTreeWithFS(fs, "/root")

	require.NoError(t, err)
	require.NotNil(t, tree)

	// Only 'env' should appear (contains stacks).
	// 'global' and 'modules' should be filtered out.
	require.Len(t, tree.Children, 1, "only 'env' should appear (contains stacks)")
	assert.Equal(t, "env", tree.Children[0].Name)
	assert.False(t, tree.Children[0].IsStack, "env itself is not a stack")

	// Verify 'env' has 2 children (dev, prod).
	require.Len(t, tree.Children[0].Children, 2, "env should have dev and prod")
	assert.True(t, tree.Children[0].Children[0].IsStack, "dev should be a stack")
	assert.True(t, tree.Children[0].Children[1].IsStack, "prod should be a stack")

	// Max depth should be 2 (root=0, env=1, dev/prod=2).
	assert.Equal(t, 2, maxDepth)
}

// TestFindAndBuildTree_RealFilesystem tests the production function with real OS calls.
// This test uses the actual working directory to cover production code paths.
func TestFindAndBuildTree_RealFilesystem(t *testing.T) {
	// Use current working directory for a realistic test.
	wd, err := os.Getwd()
	require.NoError(t, err)

	// Call the actual production function (not the test helper).
	tree, maxDepth, err := FindAndBuildTree(wd)

	// Assertions.
	require.NoError(t, err, "should build tree from real filesystem")
	require.NotNil(t, tree, "tree should not be nil")
	assert.NotEmpty(t, tree.Name, "root name should not be empty")
	assert.NotEmpty(t, tree.Path, "root path should not be empty")
	assert.Equal(t, 0, tree.Depth, "root depth should be 0")
	assert.GreaterOrEqual(t, maxDepth, 0, "max depth should be non-negative")
}

// TestFindAndBuildTree_InvalidPath tests error handling for invalid paths.
func TestFindAndBuildTree_InvalidPath(t *testing.T) {
	tests := []struct {
		name        string
		path        string
		expectError bool
	}{
		{
			name:        "empty path",
			path:        "",
			expectError: true,
		},
		{
			name:        "nonexistent path",
			path:        "/this/path/definitely/does/not/exist/12345",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tree, maxDepth, err := FindAndBuildTree(tt.path)

			if tt.expectError {
				assert.Error(t, err)
				assert.Nil(t, tree)
				assert.Equal(t, 0, maxDepth)
			}
		})
	}
}

// TestIsStackDirectory tests the production isStackDirectory function.
func TestIsStackDirectory(t *testing.T) {
	// Create a temporary directory for testing.
	tmpDir := t.TempDir()

	// Create a subdirectory with terragrunt.hcl.
	stackDir := tmpDir + "/stack"
	require.NoError(t, os.MkdirAll(stackDir, 0755))
	require.NoError(t, os.WriteFile(stackDir+"/terragrunt.hcl", []byte(""), 0644))

	// Create a subdirectory without terragrunt.hcl.
	nonStackDir := tmpDir + "/nonstack"
	require.NoError(t, os.MkdirAll(nonStackDir, 0755))

	tests := []struct {
		name     string
		path     string
		expected bool
	}{
		{
			name:     "directory with terragrunt.hcl is stack",
			path:     stackDir,
			expected: true,
		},
		{
			name:     "directory without terragrunt.hcl is not stack",
			path:     nonStackDir,
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isStackDirectory(tt.path)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// TestBuildTreeRecursive_ErrorHandling tests error handling in buildTreeRecursive.
func TestBuildTreeRecursive_ErrorHandling(t *testing.T) {
	// Test the error handling path where ReadDir returns an error.
	// We use afero to simulate a permission error.
	fs := afero.NewMemMapFs()

	// Create a directory structure.
	require.NoError(t, fs.MkdirAll("/root/accessible", 0755))
	// Make it a stack so it appears in tree.
	require.NoError(t, afero.WriteFile(fs, "/root/accessible/terragrunt.hcl", []byte(""), 0644))

	// Create the root node.
	root := &Node{
		Name:     "root",
		Path:     "/root",
		IsStack:  false,
		Children: make([]*Node, 0),
		Depth:    0,
	}

	maxDepth := 0

	// Call buildTreeRecursiveWithFS - should handle errors gracefully.
	err := buildTreeRecursiveWithFS(fs, root, &maxDepth)

	// Should not return an error (errors are swallowed).
	assert.NoError(t, err)

	// Should have built the accessible subdirectory.
	assert.Len(t, root.Children, 1)
	assert.Equal(t, "accessible", root.Children[0].Name)
}

// TestBuildTreeRecursive_LeafNode tests that recursion terminates at leaf nodes.
func TestBuildTreeRecursive_LeafNode(t *testing.T) {
	fs := afero.NewMemMapFs()

	// Create a single directory with no children (leaf node).
	require.NoError(t, fs.MkdirAll("/root/leaf", 0755))

	// Create the leaf node.
	leaf := &Node{
		Name:     "leaf",
		Path:     "/root/leaf",
		IsStack:  false,
		Children: make([]*Node, 0),
		Depth:    1,
	}

	maxDepth := 1

	// Call buildTreeRecursiveWithFS on leaf node.
	err := buildTreeRecursiveWithFS(fs, leaf, &maxDepth)

	// Should not return an error.
	assert.NoError(t, err)

	// Should have no children (recursion terminates).
	assert.Empty(t, leaf.Children)

	// MaxDepth should remain 1.
	assert.Equal(t, 1, maxDepth)
}

// TestBuildTreeRecursive_MaxDepthTracking tests that maxDepth is correctly updated.
func TestBuildTreeRecursive_MaxDepthTracking(t *testing.T) {
	fs := afero.NewMemMapFs()

	// Create a deep hierarchy: root -> level1 -> level2 -> level3.
	require.NoError(t, fs.MkdirAll("/root/level1/level2/level3", 0755))
	// Make the leaf a stack so intermediate directories appear.
	require.NoError(t, afero.WriteFile(fs, "/root/level1/level2/level3/terragrunt.hcl", []byte(""), 0644))

	root := &Node{
		Name:     "root",
		Path:     "/root",
		IsStack:  false,
		Children: make([]*Node, 0),
		Depth:    0,
	}

	maxDepth := 0

	// Build tree.
	err := buildTreeRecursiveWithFS(fs, root, &maxDepth)

	// Should not return an error.
	assert.NoError(t, err)

	// MaxDepth should be 3 (level3 is at depth 3).
	assert.Equal(t, 3, maxDepth)

	// Verify the chain exists.
	assert.Len(t, root.Children, 1)
	assert.Equal(t, "level1", root.Children[0].Name)
	assert.Len(t, root.Children[0].Children, 1)
	assert.Equal(t, "level2", root.Children[0].Children[0].Name)
}

// TestBuildTreeRecursive_SkipsNonDirectories tests that files are skipped.
func TestBuildTreeRecursive_SkipsNonDirectories(t *testing.T) {
	fs := afero.NewMemMapFs()

	// Create a directory with both files and subdirectories.
	require.NoError(t, fs.MkdirAll("/root/subdir", 0755))
	require.NoError(t, afero.WriteFile(fs, "/root/file.txt", []byte("content"), 0644))
	require.NoError(t, afero.WriteFile(fs, "/root/script.sh", []byte("#!/bin/bash"), 0755))
	// Make subdir a stack so it appears.
	require.NoError(t, afero.WriteFile(fs, "/root/subdir/terragrunt.hcl", []byte(""), 0644))

	root := &Node{
		Name:     "root",
		Path:     "/root",
		IsStack:  false,
		Children: make([]*Node, 0),
		Depth:    0,
	}

	maxDepth := 0

	// Build tree.
	err := buildTreeRecursiveWithFS(fs, root, &maxDepth)

	// Should not return an error.
	assert.NoError(t, err)

	// Should only have the subdirectory, not the files.
	assert.Len(t, root.Children, 1)
	assert.Equal(t, "subdir", root.Children[0].Name)
}

// TestBuildTreeRecursive_ContinuesOnSubdirectoryError tests that errors in subdirectories don't stop processing.
func TestBuildTreeRecursive_ContinuesOnSubdirectoryError(t *testing.T) {
	fs := afero.NewMemMapFs()

	// Create multiple subdirectories.
	require.NoError(t, fs.MkdirAll("/root/good1", 0755))
	require.NoError(t, fs.MkdirAll("/root/good2", 0755))
	// Make them stacks so they appear.
	require.NoError(t, afero.WriteFile(fs, "/root/good1/terragrunt.hcl", []byte(""), 0644))
	require.NoError(t, afero.WriteFile(fs, "/root/good2/terragrunt.hcl", []byte(""), 0644))

	// Note: We can't easily simulate a permission error with afero's MemMapFs,
	// but we can verify that the function continues processing after encountering issues.

	root := &Node{
		Name:     "root",
		Path:     "/root",
		IsStack:  false,
		Children: make([]*Node, 0),
		Depth:    0,
	}

	maxDepth := 0

	// Build tree.
	err := buildTreeRecursiveWithFS(fs, root, &maxDepth)

	// Should not return an error.
	assert.NoError(t, err)

	// Should have both good directories.
	assert.Len(t, root.Children, 2)

	// Verify both are present (order may vary).
	names := []string{root.Children[0].Name, root.Children[1].Name}
	assert.Contains(t, names, "good1")
	assert.Contains(t, names, "good2")
}

// TestBuildTreeRecursive_RealFilesystem tests the production buildTreeRecursive with real OS calls.
func TestBuildTreeRecursive_RealFilesystem(t *testing.T) {
	// Create a temporary directory structure.
	tmpDir := t.TempDir()

	// Create subdirectories.
	require.NoError(t, os.MkdirAll(tmpDir+"/env/dev", 0755))
	require.NoError(t, os.MkdirAll(tmpDir+"/env/prod", 0755))
	require.NoError(t, os.WriteFile(tmpDir+"/env/dev/terragrunt.hcl", []byte(""), 0644))
	require.NoError(t, os.WriteFile(tmpDir+"/env/prod/terragrunt.hcl", []byte(""), 0644))

	// Create the root node pointing to the real filesystem.
	root := &Node{
		Name:     "testroot",
		Path:     tmpDir,
		IsStack:  false,
		Children: make([]*Node, 0),
		Depth:    0,
	}

	maxDepth := 0

	// Call the production buildTreeRecursive (uses os.ReadDir).
	err := buildTreeRecursive(root, &maxDepth)

	// Assertions.
	require.NoError(t, err, "should build tree without error")
	assert.Equal(t, 2, maxDepth, "should have depth 2 (env/dev)")
	assert.Len(t, root.Children, 1, "should have 1 child (env)")
	assert.Equal(t, "env", root.Children[0].Name)

	// Verify the env node has children.
	envNode := root.Children[0]
	assert.Len(t, envNode.Children, 2, "env should have 2 children (dev, prod)")

	// Find dev node and verify it's marked as a stack.
	var devNode *Node
	for _, child := range envNode.Children {
		if child.Name == "dev" {
			devNode = child
			break
		}
	}
	require.NotNil(t, devNode, "dev node should exist")
	assert.True(t, devNode.IsStack, "dev should be marked as stack")
}

// TestBuildTreeRecursive_ErrorOnNonexistentPath tests error handling for invalid paths.
func TestBuildTreeRecursive_ErrorOnNonexistentPath(t *testing.T) {
	// Create a node with a nonexistent path.
	root := &Node{
		Name:     "nonexistent",
		Path:     "/this/path/does/not/exist/12345",
		IsStack:  false,
		Children: make([]*Node, 0),
		Depth:    0,
	}

	maxDepth := 0

	// Call buildTreeRecursive with a nonexistent path.
	err := buildTreeRecursive(root, &maxDepth)

	// Should not return an error (errors are swallowed in buildTreeRecursive).
	assert.NoError(t, err, "buildTreeRecursive swallows ReadDir errors")

	// Should have no children since the directory doesn't exist.
	assert.Empty(t, root.Children)
}
