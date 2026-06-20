// Package deps parses Terragrunt HCL files to extract static dependency declarations.
package deps

import (
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

var (
	// configPathRe matches config_path = "some/path" inside dependency blocks.
	configPathRe = regexp.MustCompile(`config_path\s*=\s*"([^"]+)"`)
	// includePathRe matches the path attribute inside include "name" { ... } blocks.
	// [^}] matches newlines in Go's regexp, handling multiline blocks correctly.
	includePathRe = regexp.MustCompile(`include\s+"[^"]+"\s*\{[^}]*\bpath\s*=\s*"([^"]+)"`)
)

// FindRepoRoot walks up from startDir until it finds a directory containing rootConfigFile.
// Returns startDir if not found.
func FindRepoRoot(startDir, rootConfigFile string) string {
	current := startDir
	for {
		if _, err := os.Stat(filepath.Join(current, rootConfigFile)); err == nil {
			return current
		}
		parent := filepath.Dir(current)
		if parent == current {
			return startDir
		}
		current = parent
	}
}

// ParseDependencies reads a terragrunt.hcl file and returns the absolute paths of its direct dependencies.
// It sorts and deduplicates them. It follows include blocks with statically resolvable paths to envcommon files.
// Returns an empty slice if the file does not exist or cannot be read.
func ParseDependencies(hclFilePath, repoRoot string) ([]string, error) {
	raw := parseDepsFromFile(hclFilePath, repoRoot, 0, false)
	seen := make(map[string]bool, len(raw))
	result := make([]string, 0, len(raw))
	for _, p := range raw {
		if !seen[p] {
			seen[p] = true
			result = append(result, p)
		}
	}
	sort.Strings(result)
	return result, nil
}

// parseDepsFromFile extracts dependency paths from a single HCL file and follows statically resolvable include blocks recursively.
// depth prevents infinite loops. fromInclude tracks whether this file was reached via an include block.
func parseDepsFromFile(filePath, repoRoot string, depth int, fromInclude bool) []string {
	if depth > 5 {
		return nil
	}
	content, err := os.ReadFile(filePath)
	if err != nil {
		return nil
	}

	fileDir := filepath.Dir(filePath)
	var result []string

	for _, match := range configPathRe.FindAllStringSubmatch(string(content), -1) {
		if resolved := resolvePath(match[1], fileDir, repoRoot, fromInclude); resolved != "" {
			result = append(result, resolved)
		}
	}

	for _, match := range includePathRe.FindAllStringSubmatch(string(content), -1) {
		includePath := resolvePath(match[1], fileDir, repoRoot, false)
		if includePath == "" {
			continue
		}
		result = append(result, parseDepsFromFile(includePath, repoRoot, depth+1, true)...)
	}

	return result
}

// resolvePath converts a raw Terragrunt path expression to an absolute filesystem path.
// Returns an empty string for expressions that cannot be resolved statically.
// If fromInclude is true, paths starting with ../ will not go above baseDir.
func resolvePath(raw, baseDir, repoRoot string, fromInclude bool) string {
	if strings.Contains(raw, "find_in_parent_folders") || strings.Contains(raw, "get_terragrunt_dir") {
		return ""
	}
	resolved := strings.ReplaceAll(raw, "${get_repo_root()}", repoRoot)
	if strings.Contains(resolved, "${") {
		return ""
	}
	if filepath.IsAbs(resolved) {
		return filepath.Clean(resolved)
	}

	// If this path is in an included file and starts with ../, don't ascend above baseDir.
	if fromInclude && strings.HasPrefix(resolved, "../") {
		resolved = strings.TrimPrefix(resolved, "../")
	}

	return filepath.Join(baseDir, resolved)
}
