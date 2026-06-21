package stack

import "sort"

// AnalyzeGraph computes Dependents and InCycle for all nodes in the tree.
// It must be called after FindAndBuildTree has populated Dependencies on all nodes.
// Non-stack nodes are left with Dependents: []string{} and InCycle: false.
func AnalyzeGraph(root *Node) {
	nodeMap := make(map[string]*Node)
	flattenNodes(root, nodeMap)
	buildReverseGraph(nodeMap)
	detectCycles(nodeMap)
}

// flattenNodes recursively builds a path→node map for all nodes in the tree.
func flattenNodes(node *Node, nodeMap map[string]*Node) {
	nodeMap[node.Path] = node
	for _, child := range node.Children {
		flattenNodes(child, nodeMap)
	}
}

// buildReverseGraph populates Dependents by inverting the Dependencies edges.
// Nodes with no dependents keep their existing empty slice.
func buildReverseGraph(nodeMap map[string]*Node) {
	for _, node := range nodeMap {
		for _, depPath := range node.Dependencies {
			if dep, ok := nodeMap[depPath]; ok {
				dep.Dependents = append(dep.Dependents, node.Path)
			}
		}
	}
	for _, node := range nodeMap {
		sort.Strings(node.Dependents)
	}
}

// detectCycles runs DFS from every unvisited node to mark cycle membership.
// All nodes that are part of any dependency cycle have InCycle set to true.
func detectCycles(nodeMap map[string]*Node) {
	visited := make(map[string]bool)
	inStack := make(map[string]bool)
	stackPath := []string{}

	var dfs func(path string)
	dfs = func(path string) {
		visited[path] = true
		inStack[path] = true
		stackPath = append(stackPath, path)

		if node, ok := nodeMap[path]; ok {
			for _, depPath := range node.Dependencies {
				if inStack[depPath] {
					// Found a cycle — mark all nodes from depPath to current.
					marking := false
					for _, p := range stackPath {
						if p == depPath {
							marking = true
						}
						if marking {
							if n, ok2 := nodeMap[p]; ok2 {
								n.InCycle = true
							}
						}
					}
				} else if !visited[depPath] {
					dfs(depPath)
				}
			}
		}

		stackPath = stackPath[:len(stackPath)-1]
		inStack[path] = false
	}

	for path := range nodeMap {
		if !visited[path] {
			dfs(path)
		}
	}
}
