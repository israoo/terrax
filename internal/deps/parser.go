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
// Relative config_path values inside included files are resolved against the leaf file's directory, matching
// how Terragrunt resolves them at runtime. Returns an empty slice if the file does not exist or cannot be read.
func ParseDependencies(hclFilePath, repoRoot string) []string {
	callerDir := filepath.Dir(hclFilePath)
	raw := parseDepsFromFile(hclFilePath, repoRoot, callerDir, 0)
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

// parseDepsFromFile extracts dependency paths from a single HCL file and follows statically resolvable include
// blocks recursively. callerDir is the directory of the original leaf terragrunt.hcl — relative config_path
// values in included files are resolved against it, which matches Terragrunt's runtime behavior. depth prevents
// infinite loops.
func parseDepsFromFile(filePath, repoRoot, callerDir string, depth int) []string {
	if depth > 5 {
		return nil
	}
	content, err := os.ReadFile(filePath)
	if err != nil {
		return nil
	}

	fileDir := filepath.Dir(filePath)
	var result []string

	// config_path values are always relative to the leaf file (callerDir), not to the file that declares them.
	for _, match := range configPathRe.FindAllStringSubmatch(string(content), -1) {
		if resolved := resolvePath(match[1], callerDir, repoRoot); resolved != "" {
			result = append(result, resolved)
		}
	}

	// Include path attributes are relative to the file that declares the include (fileDir).
	for _, rawPath := range extractIncludePaths(string(content)) {
		includePath := resolvePath(rawPath, fileDir, repoRoot)
		if includePath == "" {
			continue
		}
		result = append(result, parseDepsFromFile(includePath, repoRoot, callerDir, depth+1)...)
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
