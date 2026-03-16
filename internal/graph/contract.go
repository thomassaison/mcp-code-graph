package graph

// Contract describes the "what you must not break" summary for a function.
type Contract struct {
	Node           *Node   `json:"node"`
	CallerCount    int     `json:"caller_count"`
	CalleeCount    int     `json:"callee_count"`
	ReceiverType   string  `json:"receiver_type,omitempty"`
	TypeInterfaces []*Node `json:"type_interfaces,omitempty"`
	ReturnedTypes  []*Node `json:"returned_types,omitempty"`
	AcceptedTypes  []*Node `json:"accepted_types,omitempty"`
	TestFunctions  []*Node `json:"test_functions,omitempty"`
}

// GetContract builds a complete contract for the given function/method node.
func (g *Graph) GetContract(nodeID string) *Contract {
	g.mu.RLock()
	defer g.mu.RUnlock()

	node, ok := g.nodes[nodeID]
	if !ok {
		return nil
	}

	contract := &Contract{
		Node: node.Clone(),
	}

	// Caller and callee counts
	for _, edge := range g.inEdges[nodeID] {
		if edge.Type == EdgeTypeCalls {
			contract.CallerCount++
		}
	}
	for _, edge := range g.edges[nodeID] {
		if edge.Type == EdgeTypeCalls {
			contract.CalleeCount++
		}
	}

	// For methods: find interfaces the receiver type implements
	if node.Type == NodeTypeMethod && node.Metadata != nil {
		if recv, ok := node.Metadata["receiver"].(string); ok {
			contract.ReceiverType = recv

			// Find the type node by name (strip pointer prefix if present)
			typeName := recv
			if len(typeName) > 0 && typeName[0] == '*' {
				typeName = typeName[1:]
			}

			// Search byName index for the receiver type
			if typeNodes, ok := g.byName[typeName]; ok {
				for _, tn := range typeNodes {
					if tn.Package == node.Package && (tn.Type == NodeTypeType || tn.Type == NodeTypeInterface) {
						if ifaces, exists := g.byTypeImpl[tn.ID]; exists {
							for _, iface := range ifaces {
								contract.TypeInterfaces = append(contract.TypeInterfaces, iface.Clone())
							}
						}
					}
				}
			}
		}
	}

	// Returned types via EdgeTypeReturns edges
	for _, edge := range g.edges[nodeID] {
		if edge.Type == EdgeTypeReturns {
			if typeNode, ok := g.nodes[edge.To]; ok {
				contract.ReturnedTypes = append(contract.ReturnedTypes, typeNode.Clone())
			}
		}
	}

	// Accepted types via EdgeTypeAccepts edges
	for _, edge := range g.edges[nodeID] {
		if edge.Type == EdgeTypeAccepts {
			if typeNode, ok := g.nodes[edge.To]; ok {
				contract.AcceptedTypes = append(contract.AcceptedTypes, typeNode.Clone())
			}
		}
	}

	// Find tests that exercise this function (caller-direction BFS)
	tests := g.findTestsLocked(nodeID)
	contract.TestFunctions = tests

	return contract
}

// findTestsLocked finds test functions that exercise the given node.
// Must be called with g.mu held for reading.
func (g *Graph) findTestsLocked(nodeID string) []*Node {
	visited := make(map[string]bool)
	queue := []string{nodeID}
	visited[nodeID] = true

	var tests []*Node

	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]

		for _, edge := range g.inEdges[current] {
			if edge.Type != EdgeTypeCalls {
				continue
			}
			if visited[edge.From] {
				continue
			}
			visited[edge.From] = true
			queue = append(queue, edge.From)

			if node, ok := g.nodes[edge.From]; ok && isTestFunction(node) {
				tests = append(tests, node.Clone())
			}
		}
	}

	return tests
}
