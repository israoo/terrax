// Package changes detects Terragrunt stacks affected by a git commit range.
// It combines a static HCL file graph (mark_as_read references and include chains)
// with a git diff to produce the set of stacks that need to be re-evaluated.
package changes

import (
	"fmt"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"github.com/israoo/terrax/internal/deps"
	"github.com/israoo/terrax/internal/stack"
)

// FileGraph holds the reverse-edge maps needed to propagate file changes to stacks.
type FileGraph struct {
	// MarkAsReadExact maps an absolute YAML path to the HCL files that declared
	// mark_as_read() with that exact path.
	MarkAsReadExact map[string][]string
	// MarkAsReadPrefix maps an absolute directory prefix to the HCL files that
	// declared mark_as_read() with a dynamic path under that prefix. Any changed
	// file whose path starts with a recorded prefix is conservatively treated as
	// affecting all HCL files associated with that prefix.
	MarkAsReadPrefix map[string][]string
	// IncludeReverse maps an absolute HCL file path to the HCL files (usually
	// terragrunt.hcl leaf stacks) that include it via an include block.
	IncludeReverse map[string][]string
}

// BuildFileGraph scans every .hcl file under repoRoot and builds the three reverse-edge
// maps in two passes:
//
//   - Pass 1: for each HCL file extract mark_as_read references (populating MarkAsReadExact
//     and MarkAsReadPrefix) and include paths (building a forward include map).
//   - Pass 2: invert the forward include map to produce IncludeReverse.
func BuildFileGraph(repoRoot, rootConfigFile string) (*FileGraph, error) {
	if rootConfigFile == "" {
		rootConfigFile = "root.hcl"
	}
	resolvedRoot := deps.FindRepoRoot(repoRoot, rootConfigFile)

	g := &FileGraph{
		MarkAsReadExact:  make(map[string][]string),
		MarkAsReadPrefix: make(map[string][]string),
		IncludeReverse:   make(map[string][]string),
	}

	hclFiles := deps.ScanAllHCLFiles(resolvedRoot)

	// Pass 1: populate mark_as_read maps and build forward include map.
	includeForward := make(map[string][]string) // hcl → []hcl it includes
	for _, hclFile := range hclFiles {
		staticPaths, dynamicPrefixes := deps.ParseMarkAsRead(hclFile, resolvedRoot)
		for _, yamlPath := range staticPaths {
			g.MarkAsReadExact[yamlPath] = appendUnique(g.MarkAsReadExact[yamlPath], hclFile)
		}
		for _, prefix := range dynamicPrefixes {
			g.MarkAsReadPrefix[prefix] = appendUnique(g.MarkAsReadPrefix[prefix], hclFile)
		}

		includes := deps.ParseIncludes(hclFile, resolvedRoot)
		if len(includes) > 0 {
			includeForward[hclFile] = append(includeForward[hclFile], includes...)
		}
	}

	// Pass 2: invert the include forward map → IncludeReverse[included] = []includers.
	for includer, included := range includeForward {
		for _, inc := range included {
			g.IncludeReverse[inc] = appendUnique(g.IncludeReverse[inc], includer)
		}
	}

	return g, nil
}

// AffectedStacks returns the sorted, deduplicated absolute stack directory paths that are
// affected by changes between baseCommit and HEAD. The algorithm:
//
//  1. Run git diff --name-only baseCommit...HEAD to get changed files.
//  2. For each changed file determine which stacks it affects:
//     - terragrunt.hcl: the containing directory is a directly affected stack.
//     - any other .hcl: follow IncludeReverse to find leaf stacks that include it.
//     - .yaml (or any non-HCL): check MarkAsReadExact for exact matches and
//     MarkAsReadPrefix for prefix matches; each matched HCL is then resolved to
//     stacks via IncludeReverse; if the HCL itself is a stack, add it directly.
//     - any other file: walk up the directory tree to find the owning stack (if any).
//  3. Expand the directly affected stacks with their transitive Dependents using the
//     reverse dependency graph already computed by stack.AnalyzeGraph.
func AffectedStacks(repoRoot, baseCommit string, graph *FileGraph, stackTree *stack.Node) ([]string, error) {
	changedFiles, err := gitDiff(repoRoot, baseCommit)
	if err != nil {
		return nil, fmt.Errorf("failed to get git diff: %w", err)
	}

	nodeMap := flattenStackNodes(stackTree)
	directStacks := make(map[string]bool)

	for _, rel := range changedFiles {
		absFile := filepath.Join(repoRoot, rel)

		switch {
		case filepath.Base(absFile) == "terragrunt.hcl":
			// Direct stack change.
			stackDir := filepath.Dir(absFile)
			if _, ok := nodeMap[stackDir]; ok {
				directStacks[stackDir] = true
			}

		case filepath.Ext(absFile) == ".hcl":
			// Non-stack HCL (envcommon, globals, account.hcl, …): resolve via include reverse.
			for _, s := range hclToStacks(absFile, graph.IncludeReverse, nodeMap) {
				directStacks[s] = true
			}

		default:
			// YAML or any other file: check mark_as_read maps first, then fall back to
			// owning-stack walk.
			hclsFromMarkAsRead := hclsForChangedFile(absFile, graph)
			if len(hclsFromMarkAsRead) > 0 {
				for _, hcl := range hclsFromMarkAsRead {
					for _, s := range hclToStacks(hcl, graph.IncludeReverse, nodeMap) {
						directStacks[s] = true
					}
				}
			} else {
				if s := owningStack(absFile, repoRoot, nodeMap); s != "" {
					directStacks[s] = true
				}
			}
		}
	}

	// Expand with transitive dependents.
	allAffected := make(map[string]bool)
	for stackDir := range directStacks {
		allAffected[stackDir] = true
		if node, ok := nodeMap[stackDir]; ok {
			for _, dep := range node.Dependents {
				allAffected[dep] = true
			}
		}
	}

	result := make([]string, 0, len(allAffected))
	for s := range allAffected {
		result = append(result, s)
	}
	sort.Strings(result)
	return result, nil
}

// hclsForChangedFile returns HCL files associated with a changed non-HCL file via
// MarkAsReadExact (exact path match) and MarkAsReadPrefix (directory prefix match).
func hclsForChangedFile(absFile string, graph *FileGraph) []string {
	seen := make(map[string]bool)
	var result []string

	for _, hcl := range graph.MarkAsReadExact[absFile] {
		if !seen[hcl] {
			seen[hcl] = true
			result = append(result, hcl)
		}
	}

	fileDir := filepath.Dir(absFile)
	for prefix, hcls := range graph.MarkAsReadPrefix {
		if fileDir == prefix || strings.HasPrefix(fileDir+string(filepath.Separator), prefix+string(filepath.Separator)) {
			for _, hcl := range hcls {
				if !seen[hcl] {
					seen[hcl] = true
					result = append(result, hcl)
				}
			}
		}
	}
	return result
}

// hclToStacks resolves a single HCL file path to the stack directories that are affected
// by changes to it. If the HCL is itself a terragrunt.hcl, its directory is a direct stack.
// Otherwise, IncludeReverse is traversed (BFS) until terragrunt.hcl leaf files are found.
func hclToStacks(hclFile string, includeReverse map[string][]string, nodeMap map[string]*stack.Node) []string {
	var stacks []string
	visited := make(map[string]bool)
	queue := []string{hclFile}
	for len(queue) > 0 {
		cur := queue[0]
		queue = queue[1:]
		if visited[cur] {
			continue
		}
		visited[cur] = true
		if filepath.Base(cur) == "terragrunt.hcl" {
			if dir := filepath.Dir(cur); nodeMap[dir] != nil {
				stacks = append(stacks, dir)
			}
			continue
		}
		queue = append(queue, includeReverse[cur]...)
	}
	return stacks
}

// owningStack walks up from a file's directory until it finds a directory that is a
// known stack (present in nodeMap). Returns the stack directory path or an empty string.
func owningStack(absFile, repoRoot string, nodeMap map[string]*stack.Node) string {
	dir := filepath.Dir(absFile)
	for {
		if _, ok := nodeMap[dir]; ok {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir || !strings.HasPrefix(dir, repoRoot) {
			return ""
		}
		dir = parent
	}
}

// flattenStackNodes builds an absolute-path → *Node map for all stack nodes in the tree.
func flattenStackNodes(root *stack.Node) map[string]*stack.Node {
	m := make(map[string]*stack.Node)
	if root == nil {
		return m
	}
	var walk func(*stack.Node)
	walk = func(n *stack.Node) {
		if n.IsStack {
			m[n.Path] = n
		}
		for _, c := range n.Children {
			walk(c)
		}
	}
	walk(root)
	return m
}

// gitDiff runs git diff --name-only <baseCommit>...HEAD and returns the list of changed
// relative file paths.
func gitDiff(repoRoot, baseCommit string) ([]string, error) {
	cmd := exec.Command("git", "diff", "--name-only", baseCommit+"...HEAD")
	cmd.Dir = repoRoot
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("git diff failed: %w", err)
	}
	var files []string
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		if line != "" {
			files = append(files, line)
		}
	}
	return files, nil
}

// appendUnique appends value to slice only if not already present.
func appendUnique(slice []string, value string) []string {
	for _, v := range slice {
		if v == value {
			return slice
		}
	}
	return append(slice, value)
}
