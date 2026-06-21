package stack

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/israoo/terrax/internal/config"
	"github.com/israoo/terrax/internal/deps"
)

// FindAndBuildTree scans the filesystem starting from rootDir and builds a tree structure.
// rootConfigFile is used to locate the repository root; if empty, config.DefaultRootConfigFile is used.
// It returns the root node, maximum depth, and any error encountered.
func FindAndBuildTree(rootDir, rootConfigFile string) (*Node, int, error) {
	if rootDir == "" {
		return nil, 0, fmt.Errorf("root directory cannot be empty")
	}

	if rootConfigFile == "" {
		rootConfigFile = config.DefaultRootConfigFile
	}

	absPath, err := filepath.Abs(rootDir)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to resolve absolute path: %w", err)
	}

	info, err := os.Stat(absPath)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to access directory: %w", err)
	}
	if !info.IsDir() {
		return nil, 0, fmt.Errorf("%s is not a directory", absPath)
	}

	repoRoot := deps.FindRepoRoot(absPath, rootConfigFile)

	root := &Node{
		Name:         filepath.Base(absPath),
		Path:         absPath,
		IsStack:      isStackDirectory(absPath),
		Children:     make([]*Node, 0),
		Dependencies: []string{},
		Dependents:   []string{},
		Depth:        0,
	}
	if root.IsStack {
		hclFile := filepath.Join(absPath, "terragrunt.hcl")
		root.Dependencies = deps.ParseDependencies(hclFile, repoRoot)
	}

	maxDepth := 0
	if err := buildTreeRecursive(root, &maxDepth, repoRoot); err != nil {
		return nil, 0, fmt.Errorf("failed to build tree: %w", err)
	}

	AnalyzeGraph(root)
	return root, maxDepth, nil
}

// buildTreeRecursive recursively builds the tree structure.
// Only includes directories that are stacks or contain stacks in their hierarchy.
func buildTreeRecursive(node *Node, maxDepth *int, repoRoot string) error {
	entries, err := os.ReadDir(node.Path)
	if err != nil {
		return nil
	}

	for _, entry := range entries {
		if !entry.IsDir() || strings.HasPrefix(entry.Name(), ".") {
			continue
		}

		if shouldSkipDirectory(entry.Name()) {
			continue
		}

		childPath := filepath.Join(node.Path, entry.Name())
		childNode := &Node{
			Name:         entry.Name(),
			Path:         childPath,
			IsStack:      isStackDirectory(childPath),
			Children:     make([]*Node, 0),
			Dependencies: []string{},
			Dependents:   []string{},
			Depth:        node.Depth + 1,
		}

		if childNode.IsStack {
			hclFile := filepath.Join(childPath, "terragrunt.hcl")
			childNode.Dependencies = deps.ParseDependencies(hclFile, repoRoot)
		}

		// Recursively build children to find nested stacks.
		if err := buildTreeRecursive(childNode, maxDepth, repoRoot); err != nil {
			continue
		}

		// Only add this node if it's a stack or contains stacks.
		if childNode.IsStack || childNode.HasChildren() {
			node.Children = append(node.Children, childNode)
			if childNode.Depth > *maxDepth {
				*maxDepth = childNode.Depth
			}
		}
	}

	return nil
}

// CollectStackPaths returns the absolute paths of all stack directories (those containing
// terragrunt.hcl) found under rootDir, including rootDir itself if it is a stack.
func CollectStackPaths(rootDir string) ([]string, error) {
	absRoot, err := filepath.Abs(rootDir)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve absolute path: %w", err)
	}

	var paths []string
	err = filepath.WalkDir(absRoot, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return nil // Skip unreadable entries.
		}
		if !d.IsDir() {
			return nil
		}
		// Skip hidden and known non-stack directories, but always descend into the root itself.
		if path != absRoot {
			name := d.Name()
			if strings.HasPrefix(name, ".") || shouldSkipDirectory(name) {
				return filepath.SkipDir
			}
		}
		if isStackDirectory(path) {
			paths = append(paths, path)
		}
		return nil
	})
	return paths, err
}

// isStackDirectory checks if a directory contains stack definition files
func isStackDirectory(dirPath string) bool {
	if _, err := os.Stat(filepath.Join(dirPath, "terragrunt.hcl")); err == nil {
		return true
	}

	return false
}

// shouldSkipDirectory returns true for directories that should be skipped during scanning
func shouldSkipDirectory(name string) bool {
	skipList := []string{
		".git",
		".terraform",
		".terragrunt-cache",
		"vendor",
		".idea",
		".vscode",
	}

	for _, skip := range skipList {
		if name == skip {
			return true
		}
	}

	return false
}
