// Package deps parses Terragrunt HCL files to extract static dependency declarations.
package deps

import (
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

var (
	// configPathRe matches config_path = "some/path" inside dependency blocks.
	configPathRe = regexp.MustCompile(`config_path\s*=\s*"([^"]+)"`)
	// markAsReadRe matches mark_as_read("some/path") calls.
	markAsReadRe = regexp.MustCompile(`mark_as_read\s*\(\s*"([^"]+)"\s*\)`)
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

// ParseIncludes reads an HCL file and returns the absolute paths of all statically resolvable
// include blocks. Paths containing unresolvable expressions (e.g. find_in_parent_folders,
// unresolved ${...} interpolations) are silently skipped. Returns an empty slice on error.
func ParseIncludes(hclFilePath, repoRoot string) []string {
	content, err := os.ReadFile(hclFilePath)
	if err != nil {
		return []string{}
	}
	fileDir := filepath.Dir(hclFilePath)
	var result []string
	for _, raw := range extractIncludePaths(string(content)) {
		if resolved := resolvePath(raw, fileDir, repoRoot); resolved != "" {
			result = append(result, resolved)
		}
	}
	return result
}

// ParseMarkAsRead reads an HCL file and extracts mark_as_read() file references.
// Returns two slices: staticPaths contains absolute paths for fully resolvable expressions;
// dynamicPrefixes contains absolute directory prefixes for expressions with unresolvable
// interpolations (conservative: any file under that prefix is treated as a potential match).
// Both slices are sorted and deduplicated. Returns empty slices on error or no matches.
func ParseMarkAsRead(hclFilePath, repoRoot string) (staticPaths []string, dynamicPrefixes []string) {
	content, err := os.ReadFile(hclFilePath)
	if err != nil {
		return []string{}, []string{}
	}
	fileDir := filepath.Dir(hclFilePath)
	seenStatic := make(map[string]bool)
	seenPrefix := make(map[string]bool)
	for _, match := range markAsReadRe.FindAllStringSubmatch(string(content), -1) {
		raw := match[1]
		if resolved := resolvePath(raw, fileDir, repoRoot); resolved != "" {
			if !seenStatic[resolved] {
				seenStatic[resolved] = true
				staticPaths = append(staticPaths, resolved)
			}
			continue
		}
		if prefix := extractStaticPrefix(raw, repoRoot); prefix != "" {
			if !seenPrefix[prefix] {
				seenPrefix[prefix] = true
				dynamicPrefixes = append(dynamicPrefixes, prefix)
			}
		}
	}
	sort.Strings(staticPaths)
	sort.Strings(dynamicPrefixes)
	if staticPaths == nil {
		staticPaths = []string{}
	}
	if dynamicPrefixes == nil {
		dynamicPrefixes = []string{}
	}
	return staticPaths, dynamicPrefixes
}

// ScanAllHCLFiles walks the repo and returns the absolute paths of every .hcl file found,
// skipping hidden directories and known non-Terragrunt directories (.git, .terraform,
// .terragrunt-cache, vendor). Unlike FindAndBuildTree this includes non-stack HCL files
// such as _envcommon, globals, and account/region configs.
func ScanAllHCLFiles(repoRoot string) []string {
	var files []string
	_ = filepath.WalkDir(repoRoot, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			name := d.Name()
			if strings.HasPrefix(name, ".") || shouldSkipHCLScanDir(name) {
				return filepath.SkipDir
			}
			return nil
		}
		if filepath.Ext(path) == ".hcl" {
			files = append(files, path)
		}
		return nil
	})
	return files
}

// extractStaticPrefix returns the absolute directory prefix of a mark_as_read path that
// contains unresolvable interpolations. It substitutes ${get_repo_root()} and takes
// everything up to the first remaining ${, then returns filepath.Dir of that prefix.
// Returns an empty string if no static prefix can be determined.
func extractStaticPrefix(raw, repoRoot string) string {
	substituted := strings.ReplaceAll(raw, "${get_repo_root()}", repoRoot)
	idx := strings.Index(substituted, "${")
	if idx < 0 {
		return ""
	}
	prefix := substituted[:idx]
	prefix = strings.TrimRight(prefix, "/\\")
	if !filepath.IsAbs(prefix) {
		return ""
	}
	return filepath.Clean(prefix)
}

// shouldSkipHCLScanDir returns true for directories that should be skipped when scanning
// for HCL files. Mirrors the skip list in internal/stack/builder.go.
func shouldSkipHCLScanDir(name string) bool {
	switch name {
	case ".git", ".terraform", ".terragrunt-cache", "vendor", ".idea", ".vscode":
		return true
	}
	return false
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
