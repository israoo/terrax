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
)

// FindRepoRoot walks up from startDir until it finds a directory containing
// rootConfigFile, then returns that directory. Returns startDir if not found.
// Note: history.FindProjectRoot performs the same walk but returns "" on failure.
// The two implementations are kept separate to avoid a circular import dependency.
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
func ParseDependencies(hclFilePath, repoRoot string) []string {
	raw := parseDepsFromFile(hclFilePath, repoRoot, 0)
	seen := make(map[string]bool, len(raw))
	result := make([]string, 0, len(raw))
	for _, p := range raw {
		if !seen[p] {
			seen[p] = true
			result = append(result, p)
		}
	}
	sort.Strings(result)
	return result
}

// parseDepsFromFile extracts dependency paths from a single HCL file and follows statically resolvable include blocks recursively.
// depth prevents infinite loops.
func parseDepsFromFile(filePath, repoRoot string, depth int) []string {
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
		if resolved := resolvePath(match[1], fileDir, repoRoot); resolved != "" {
			result = append(result, resolved)
		}
	}

	for _, rawPath := range extractIncludePaths(string(content)) {
		includePath := resolvePath(rawPath, fileDir, repoRoot)
		if includePath == "" {
			continue
		}
		result = append(result, parseDepsFromFile(includePath, repoRoot, depth+1)...)
	}

	return result
}

// extractIncludePaths finds path attribute values inside include blocks,
// correctly handling nested braces (e.g. locals maps) within the block.
func extractIncludePaths(content string) []string {
	var paths []string
	includeStartRe := regexp.MustCompile(`include\s+"[^"]+"\s*\{`)
	pathAttrRe := regexp.MustCompile(`\bpath\s*=\s*"([^"]+)"`)
	for _, loc := range includeStartRe.FindAllStringIndex(content, -1) {
		depth := 0
		end := -1
		for i, ch := range content[loc[0]:] {
			if ch == '{' {
				depth++
			} else if ch == '}' {
				depth--
				if depth == 0 {
					end = loc[0] + i
					break
				}
			}
		}
		if end < 0 {
			continue
		}
		block := content[loc[0] : end+1]
		if m := pathAttrRe.FindStringSubmatch(block); len(m) > 1 {
			paths = append(paths, m[1])
		}
	}
	return paths
}

// resolvePath converts a raw Terragrunt path expression to an absolute filesystem path.
// Returns an empty string for expressions that cannot be resolved statically.
func resolvePath(raw, baseDir, repoRoot string) string {
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
	return filepath.Join(baseDir, resolved)
}
