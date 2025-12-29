package stack

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// FindAndBuildTree scans the filesystem starting from rootDir and builds a tree structure.
// It returns the root node, maximum depth, and any error encountered.
func FindAndBuildTree(rootDir string) (*Node, int, error) {
	if rootDir == "" {
		return nil, 0, fmt.Errorf("root directory cannot be empty")
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

	root := &Node{
		Name:     filepath.Base(absPath),
		Path:     absPath,
		IsStack:  isStackDirectory(absPath),
		Children: make([]*Node, 0),
		Depth:    0,
	}

	maxDepth := 0
	if err := buildTreeRecursive(root, &maxDepth); err != nil {
		return nil, 0, fmt.Errorf("failed to build tree: %w", err)
	}

	return root, maxDepth, nil
}

// buildTreeRecursive recursively builds the tree structure.
// Only includes directories that are stacks or contain stacks in their hierarchy.
func buildTreeRecursive(node *Node, maxDepth *int) error {
	entries, err := os.ReadDir(node.Path)
	if err != nil {
		// Ignore unreadable directories
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
			Name:     entry.Name(),
			Path:     childPath,
			IsStack:  isStackDirectory(childPath),
			Children: make([]*Node, 0),
			Depth:    node.Depth + 1,
		}

		// Recursively build children to find nested stacks
		if err := buildTreeRecursive(childNode, maxDepth); err != nil {
			continue
		}

		// Only add this node if it's a stack or contains stacks
		if childNode.IsStack || len(childNode.Children) > 0 {
			if childNode.Depth > *maxDepth {
				*maxDepth = childNode.Depth
			}

			node.Children = append(node.Children, childNode)
		}
	}

	return nil
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
