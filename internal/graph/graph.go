package graph

import (
	"errors"
	"sync"
)

var (
	ErrNodeNotFound = errors.New("node not found")
)

type Graph struct {
	mu sync.RWMutex

	nodes   map[string]*Node
	edges   map[string][]*Edge
	inEdges map[string][]*Edge

	byType    map[NodeType]map[string]*Node
	byPackage map[string]map[string]*Node
}

func New() *Graph {
	return &Graph{
		nodes:     make(map[string]*Node),
		edges:     make(map[string][]*Edge),
		inEdges:   make(map[string][]*Edge),
		byType:    make(map[NodeType]map[string]*Node),
		byPackage: make(map[string]map[string]*Node),
	}
}

func (g *Graph) AddNode(node *Node) {
	g.mu.Lock()
	defer g.mu.Unlock()

	g.nodes[node.ID] = node

	if g.byType[node.Type] == nil {
		g.byType[node.Type] = make(map[string]*Node)
	}
	g.byType[node.Type][node.ID] = node

	if g.byPackage[node.Package] == nil {
		g.byPackage[node.Package] = make(map[string]*Node)
	}
	g.byPackage[node.Package][node.ID] = node
}

func (g *Graph) GetNode(id string) (*Node, error) {
	g.mu.RLock()
	defer g.mu.RUnlock()

	node, ok := g.nodes[id]
	if !ok {
		return nil, ErrNodeNotFound
	}
	return node, nil
}

func (g *Graph) AddEdge(edge *Edge) {
	g.mu.Lock()
	defer g.mu.Unlock()

	g.edges[edge.From] = append(g.edges[edge.From], edge)
	g.inEdges[edge.To] = append(g.inEdges[edge.To], edge)
}

func (g *Graph) NodeCount() int {
	g.mu.RLock()
	defer g.mu.RUnlock()
	return len(g.nodes)
}

func (g *Graph) EdgeCount() int {
	g.mu.RLock()
	defer g.mu.RUnlock()
	count := 0
	for _, edges := range g.edges {
		count += len(edges)
	}
	return count
}

func (g *Graph) GetCallers(nodeID string) []*Node {
	g.mu.RLock()
	defer g.mu.RUnlock()

	var callers []*Node
	for _, edge := range g.inEdges[nodeID] {
		if edge.Type == EdgeTypeCalls {
			if node, ok := g.nodes[edge.From]; ok {
				callers = append(callers, node)
			}
		}
	}
	return callers
}

func (g *Graph) GetCallees(nodeID string) []*Node {
	g.mu.RLock()
	defer g.mu.RUnlock()

	var callees []*Node
	for _, edge := range g.edges[nodeID] {
		if edge.Type == EdgeTypeCalls {
			if node, ok := g.nodes[edge.To]; ok {
				callees = append(callees, node)
			}
		}
	}
	return callees
}

func (g *Graph) GetNodesByType(nodeType NodeType) []*Node {
	g.mu.RLock()
	defer g.mu.RUnlock()

	var nodes []*Node
	for _, node := range g.byType[nodeType] {
		nodes = append(nodes, node)
	}
	return nodes
}

func (g *Graph) GetNodesByPackage(pkg string) []*Node {
	g.mu.RLock()
	defer g.mu.RUnlock()

	var nodes []*Node
	for _, node := range g.byPackage[pkg] {
		nodes = append(nodes, node)
	}
	return nodes
}

func (g *Graph) RemoveNodesForPackage(pkg string) {
	g.mu.Lock()
	defer g.mu.Unlock()

	nodeIDs := make(map[string]bool)
	for id := range g.byPackage[pkg] {
		nodeIDs[id] = true
	}

	for id := range nodeIDs {
		delete(g.edges, id)
		delete(g.inEdges, id)
	}

	for fromID, edges := range g.edges {
		filtered := edges[:0]
		for _, e := range edges {
			if !nodeIDs[e.To] {
				filtered = append(filtered, e)
			}
		}
		g.edges[fromID] = filtered
	}

	for id := range nodeIDs {
		node := g.nodes[id]
		delete(g.nodes, id)
		delete(g.byType[node.Type], id)
	}
	delete(g.byPackage, pkg)
}

func (g *Graph) AllNodes() []*Node {
	g.mu.RLock()
	defer g.mu.RUnlock()

	nodes := make([]*Node, 0, len(g.nodes))
	for _, node := range g.nodes {
		nodes = append(nodes, node)
	}
	return nodes
}
