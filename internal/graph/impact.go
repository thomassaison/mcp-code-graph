package graph

import "sort"

// ImpactReport describes the blast radius of changing a function.
type ImpactReport struct {
	NodeID             string  `json:"node_id"`
	DirectCallers      []*Node `json:"direct_callers"`
	IndirectCallers    []*Node `json:"indirect_callers"`
	InterfaceContracts []*Node `json:"interface_contracts"`
	Tests              []*Node `json:"tests"`
	RiskLevel          string  `json:"risk_level"`
	TotalReach         int     `json:"total_reach"`
}

// GetImpact performs a reverse BFS from the given node and returns the full
// blast radius: direct callers, indirect callers, interfaces that declare this
// method, and tests that exercise any affected function.
func (g *Graph) GetImpact(nodeID string) *ImpactReport {
	g.mu.RLock()
	defer g.mu.RUnlock()

	report := &ImpactReport{
		NodeID: nodeID,
	}

	if _, ok := g.nodes[nodeID]; !ok {
		return report
	}

	// BFS reverse traversal using inEdges
	visited := make(map[string]int) // nodeID -> depth (0 = start, 1 = direct, >1 = indirect)
	queue := []string{nodeID}
	visited[nodeID] = 0

	directSet := make(map[string]bool)

	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]
		currentDepth := visited[current]

		for _, edge := range g.inEdges[current] {
			if edge.Type != EdgeTypeCalls {
				continue
			}
			if _, seen := visited[edge.From]; seen {
				continue
			}
			visited[edge.From] = currentDepth + 1
			queue = append(queue, edge.From)

			if currentDepth == 0 {
				directSet[edge.From] = true
			}
		}
	}

	// Check if this node is a method — find interfaces that declare it
	var ifaceNodes []*Node
	for _, edge := range g.inEdges[nodeID] {
		if edge.Type == EdgeTypeImplements {
			if ifaceNode, ok := g.nodes[edge.From]; ok {
				ifaceNodes = append(ifaceNodes, ifaceNode.Clone())
			}
		}
	}
	report.InterfaceContracts = ifaceNodes

	// Collect direct and indirect callers + tests
	var directCallers, indirectCallers, tests []*Node
	for id, depth := range visited {
		if depth == 0 {
			continue // skip the original node
		}
		node, ok := g.nodes[id]
		if !ok {
			continue
		}

		if isTestFunction(node) {
			tests = append(tests, node.Clone())
		}

		if depth == 1 {
			directCallers = append(directCallers, node.Clone())
		} else {
			indirectCallers = append(indirectCallers, node.Clone())
		}
	}

	sortByName := func(nodes []*Node) {
		sort.Slice(nodes, func(i, j int) bool { return nodes[i].Name < nodes[j].Name })
	}
	sortByName(directCallers)
	sortByName(indirectCallers)
	sortByName(tests)

	report.DirectCallers = directCallers
	report.IndirectCallers = indirectCallers
	report.Tests = tests
	report.TotalReach = len(visited) - 1 // exclude original node

	// Risk level heuristic
	switch {
	case report.TotalReach >= 20:
		report.RiskLevel = "high"
	case report.TotalReach >= 5:
		report.RiskLevel = "medium"
	default:
		report.RiskLevel = "low"
	}

	return report
}

func isTestFunction(node *Node) bool {
	if len(node.Name) >= 4 && node.Name[:4] == "Test" {
		return true
	}
	if len(node.Name) >= 9 && node.Name[:9] == "Benchmark" {
		return true
	}
	if len(node.Name) >= 7 && node.Name[:7] == "Example" {
		return true
	}
	if len(node.File) >= 8 && node.File[len(node.File)-8:] == "_test.go" {
		return true
	}
	return false
}
