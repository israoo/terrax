package stack

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// GroupDetectConfig holds the classification and execution config for a single stack group.
type GroupDetectConfig struct {
	Detect    string            // Exact string to grep in stack.hcl; empty = default group.
	DependsOn []string          // Group names that must complete before this group.
	Env       map[string]string // Environment variables injected for this group's execution.
}

// DetectGroup returns the name of the first group whose detect pattern is found in
// <stackPath>/stack.hcl. Returns "default" if no pattern matches or the file does not exist.
func DetectGroup(stackPath string, groups map[string]GroupDetectConfig) string {
	hclPath := filepath.Join(stackPath, "stack.hcl")
	data, err := os.ReadFile(hclPath)
	if err != nil {
		return "default"
	}
	content := string(data)

	// Sort group names for deterministic matching order.
	names := make([]string, 0, len(groups))
	for name := range groups {
		names = append(names, name)
	}
	sort.Strings(names)

	for _, name := range names {
		cfg := groups[name]
		if cfg.Detect != "" && strings.Contains(content, cfg.Detect) {
			return name
		}
	}
	return "default"
}

// TopologicalSort returns group names in a valid execution order respecting all
// depends_on relationships. Groups referenced in depends_on but not defined in
// the map are treated as already complete. Returns an error if a cycle is detected.
func TopologicalSort(groups map[string]GroupDetectConfig) ([]string, error) {
	inDegree := make(map[string]int, len(groups))
	successors := make(map[string][]string, len(groups))

	for name := range groups {
		inDegree[name] = 0
	}

	for name, cfg := range groups {
		for _, dep := range cfg.DependsOn {
			if _, exists := groups[dep]; !exists {
				continue // Undefined group — treat as already complete.
			}
			inDegree[name]++
			successors[dep] = append(successors[dep], name)
		}
	}

	// Seed with groups that have no dependencies.
	queue := make([]string, 0, len(groups))
	for name := range groups {
		if inDegree[name] == 0 {
			queue = append(queue, name)
		}
	}
	sort.Strings(queue)

	result := make([]string, 0, len(groups))
	for len(queue) > 0 {
		node := queue[0]
		queue = queue[1:]
		result = append(result, node)

		succs := successors[node]
		sort.Strings(succs)
		for _, succ := range succs {
			inDegree[succ]--
			if inDegree[succ] == 0 {
				queue = append(queue, succ)
				sort.Strings(queue)
			}
		}
	}

	if len(result) != len(groups) {
		cycleMembers := make([]string, 0, len(groups))
		for name, deg := range inDegree {
			if deg > 0 {
				cycleMembers = append(cycleMembers, name)
			}
		}
		sort.Strings(cycleMembers)
		return nil, fmt.Errorf("cycle detected in stack_groups depends_on involving: %v", cycleMembers)
	}
	return result, nil
}
