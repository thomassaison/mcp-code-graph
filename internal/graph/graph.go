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
	edgeSet map[string]bool // dedup set: "from|to|type" → true

	byType    map[NodeType]map[string]*Node
	byPackage map[string]map[string]*Node
	byName    map[string]map[string]*Node

	byInterface map[string][]*Node
	byTypeImpl  map[string][]*Node
}

func New() *Graph {
	return &Graph{
		nodes:       make(map[string]*Node),
		edges:       make(map[string][]*Edge),
		inEdges:     make(map[string][]*Edge),
		edgeSet:     make(map[string]bool),
		byType:      make(map[NodeType]map[string]*Node),
		byPackage:   make(map[string]map[string]*Node),
		byName:      make(map[string]map[string]*Node),
		byInterface: make(map[string][]*Node),
		byTypeImpl:  make(map[string][]*Node),
	}
}

// edgeKey returns a unique key for an edge used for deduplication.
func edgeKey(from, to string, edgeType EdgeType) string {
	return from + "|" + to + "|" + string(edgeType)
}

func (g *Graph) AddNode(node *Node) {
	g.mu.Lock()
	defer g.mu.Unlock()

	if existing, ok := g.nodes[node.ID]; ok {
		delete(g.byType[existing.Type], node.ID)
		if len(g.byType[existing.Type]) == 0 {
			delete(g.byType, existing.Type)
		}
		delete(g.byPackage[existing.Package], node.ID)
		if len(g.byPackage[existing.Package]) == 0 {
			delete(g.byPackage, existing.Package)
		}
		delete(g.byName[existing.Name], node.ID)
		if len(g.byName[existing.Name]) == 0 {
			delete(g.byName, existing.Name)
		}
	}

	g.nodes[node.ID] = node

	if g.byType[node.Type] == nil {
		g.byType[node.Type] = make(map[string]*Node)
	}
	g.byType[node.Type][node.ID] = node

	if g.byPackage[node.Package] == nil {
		g.byPackage[node.Package] = make(map[string]*Node)
	}
	g.byPackage[node.Package][node.ID] = node

	if g.byName[node.Name] == nil {
		g.byName[node.Name] = make(map[string]*Node)
	}
	g.byName[node.Name][node.ID] = node
}

func (g *Graph) SetNodeSummary(id string, summary *Summary) error {
	g.mu.Lock()
	defer g.mu.Unlock()

	node, ok := g.nodes[id]
	if !ok {
		return ErrNodeNotFound
	}
	node.Summary = summary
	return nil
}

func (g *Graph) GetNode(id string) (*Node, error) {
	g.mu.RLock()
	defer g.mu.RUnlock()

	node, ok := g.nodes[id]
	if !ok {
		return nil, ErrNodeNotFound
	}
	return node.Clone(), nil
}

func (g *Graph) AddEdge(edge *Edge) {
	g.mu.Lock()
	defer g.mu.Unlock()

	key := edgeKey(edge.From, edge.To, edge.Type)
	if g.edgeSet[key] {
		return // already exists, skip duplicate
	}
	g.edgeSet[key] = true

	g.edges[edge.From] = append(g.edges[edge.From], edge)
	g.inEdges[edge.To] = append(g.inEdges[edge.To], edge)

	if edge.Type == EdgeTypeImplements {
		if fromNode, ok := g.nodes[edge.From]; ok {
			g.byInterface[edge.To] = append(g.byInterface[edge.To], fromNode)
		}
		if toNode, ok := g.nodes[edge.To]; ok {
			g.byTypeImpl[edge.From] = append(g.byTypeImpl[edge.From], toNode)
		}
	}
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
				callers = append(callers, node.Clone())
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
				callees = append(callees, node.Clone())
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
		nodes = append(nodes, node.Clone())
	}
	return nodes
}

func (g *Graph) GetNodesByPackage(pkg string) []*Node {
	g.mu.RLock()
	defer g.mu.RUnlock()

	var nodes []*Node
	for _, node := range g.byPackage[pkg] {
		nodes = append(nodes, node.Clone())
	}
	return nodes
}

func (g *Graph) GetNodesByName(name string) []*Node {
	g.mu.RLock()
	defer g.mu.RUnlock()

	var nodes []*Node
	for _, node := range g.byName[name] {
		nodes = append(nodes, node.Clone())
	}
	return nodes
}

func (g *Graph) GetNodesByNameAndPackage(name, pkg string) []*Node {
	g.mu.RLock()
	defer g.mu.RUnlock()

	var nodes []*Node
	for _, node := range g.byName[name] {
		if node.Package == pkg {
			nodes = append(nodes, node.Clone())
		}
	}
	return nodes
}

func (g *Graph) RemoveNodesForPackage(pkg string) {
	g.mu.Lock()
	defer g.mu.Unlock()

	g.removeNodesLocked(func(id string) bool {
		node, ok := g.byPackage[pkg][id]
		return ok && node != nil
	})

	delete(g.byPackage, pkg)
}

// removeNodesLocked removes nodes (and their edges) matching the predicate.
// Caller must hold g.mu write lock.
func (g *Graph) removeNodesLocked(match func(id string) bool) {
	nodeIDs := make(map[string]bool)
	for id := range g.nodes {
		if match(id) {
			nodeIDs[id] = true
		}
	}

	// Remove edges from deleted nodes and clean edgeSet
	for id := range nodeIDs {
		for _, e := range g.edges[id] {
			delete(g.edgeSet, edgeKey(e.From, e.To, e.Type))
		}
		delete(g.edges, id)
		for _, e := range g.inEdges[id] {
			delete(g.edgeSet, edgeKey(e.From, e.To, e.Type))
		}
		delete(g.inEdges, id)
	}

	// Remove edges pointing to deleted nodes from surviving nodes
	for fromID, edges := range g.edges {
		filtered := edges[:0]
		for _, e := range edges {
			if nodeIDs[e.To] {
				delete(g.edgeSet, edgeKey(e.From, e.To, e.Type))
			} else {
				filtered = append(filtered, e)
			}
		}
		g.edges[fromID] = filtered
	}

	// Clean up inEdges for surviving nodes
	for toID, edges := range g.inEdges {
		filtered := edges[:0]
		for _, e := range edges {
			if !nodeIDs[e.From] {
				filtered = append(filtered, e)
			}
		}
		g.inEdges[toID] = filtered
	}

	for id := range nodeIDs {
		node := g.nodes[id]
		delete(g.nodes, id)
		delete(g.byType[node.Type], id)
		if len(g.byType[node.Type]) == 0 {
			delete(g.byType, node.Type)
		}
		delete(g.byName[node.Name], id)
		if len(g.byName[node.Name]) == 0 {
			delete(g.byName, node.Name)
		}
	}

	for ifaceID, impls := range g.byInterface {
		filtered := impls[:0]
		for _, impl := range impls {
			if !nodeIDs[impl.ID] {
				filtered = append(filtered, impl)
			}
		}
		if len(filtered) == 0 {
			delete(g.byInterface, ifaceID)
		} else {
			g.byInterface[ifaceID] = filtered
		}
	}

	for typeID := range nodeIDs {
		delete(g.byTypeImpl, typeID)
	}
}

// RemoveNodesForFile removes all nodes (and their edges) that belong to the given file.
// This is used for incremental re-indexing of a single file without
// destroying sibling files in the same package.
func (g *Graph) RemoveNodesForFile(file string) {
	g.mu.Lock()
	defer g.mu.Unlock()

	// Collect nodes belonging to this file
	nodeIDs := make(map[string]bool)
	for id, node := range g.nodes {
		if node.File == file {
			nodeIDs[id] = true
		}
	}

	if len(nodeIDs) == 0 {
		return
	}

	// Determine the package for byPackage cleanup
	var pkg string
	for id := range nodeIDs {
		pkg = g.nodes[id].Package
		break
	}

	// Remove edges from/to deleted nodes and clean edgeSet
	for id := range nodeIDs {
		for _, e := range g.edges[id] {
			delete(g.edgeSet, edgeKey(e.From, e.To, e.Type))
		}
		delete(g.edges, id)
		for _, e := range g.inEdges[id] {
			delete(g.edgeSet, edgeKey(e.From, e.To, e.Type))
		}
		delete(g.inEdges, id)
	}

	for fromID, edges := range g.edges {
		filtered := edges[:0]
		for _, e := range edges {
			if nodeIDs[e.To] {
				delete(g.edgeSet, edgeKey(e.From, e.To, e.Type))
			} else {
				filtered = append(filtered, e)
			}
		}
		g.edges[fromID] = filtered
	}

	for toID, edges := range g.inEdges {
		filtered := edges[:0]
		for _, e := range edges {
			if !nodeIDs[e.From] {
				filtered = append(filtered, e)
			}
		}
		g.inEdges[toID] = filtered
	}

	// Remove nodes from indexes
	for id := range nodeIDs {
		node := g.nodes[id]
		delete(g.nodes, id)
		delete(g.byType[node.Type], id)
		if len(g.byType[node.Type]) == 0 {
			delete(g.byType, node.Type)
		}
		delete(g.byName[node.Name], id)
		if len(g.byName[node.Name]) == 0 {
			delete(g.byName, node.Name)
		}
		if pkg != "" {
			delete(g.byPackage[pkg], id)
			if len(g.byPackage[pkg]) == 0 {
				delete(g.byPackage, pkg)
			}
		}
	}

	// Clean up interface indexes
	for ifaceID, impls := range g.byInterface {
		filtered := impls[:0]
		for _, impl := range impls {
			if !nodeIDs[impl.ID] {
				filtered = append(filtered, impl)
			}
		}
		if len(filtered) == 0 {
			delete(g.byInterface, ifaceID)
		} else {
			g.byInterface[ifaceID] = filtered
		}
	}

	for typeID := range nodeIDs {
		delete(g.byTypeImpl, typeID)
	}
}

func (g *Graph) ReplaceAll(other *Graph) {
	// Guard against self-replacement which would deadlock
	// because RWMutex is not reentrant
	if g == other {
		return
	}

	g.mu.Lock()
	defer g.mu.Unlock()

	other.mu.RLock()
	defer other.mu.RUnlock()

	g.nodes = other.nodes
	g.edges = other.edges
	g.inEdges = other.inEdges
	g.edgeSet = other.edgeSet
	g.byType = other.byType
	g.byPackage = other.byPackage
	g.byName = other.byName
	g.byInterface = other.byInterface
	g.byTypeImpl = other.byTypeImpl
}

func (g *Graph) AllNodes() []*Node {
	g.mu.RLock()
	defer g.mu.RUnlock()

	nodes := make([]*Node, 0, len(g.nodes))
	for _, node := range g.nodes {
		nodes = append(nodes, node.Clone())
	}
	return nodes
}

func (g *Graph) AllEdges() []*Edge {
	g.mu.RLock()
	defer g.mu.RUnlock()

	var edges []*Edge
	for _, edgeList := range g.edges {
		edges = append(edges, edgeList...)
	}
	return edges
}

func (g *Graph) AllPackages() []string {
	g.mu.RLock()
	defer g.mu.RUnlock()

	pkgs := make(map[string]bool)
	for pkg := range g.byPackage {
		pkgs[pkg] = true
	}

	result := make([]string, 0, len(pkgs))
	for pkg := range pkgs {
		result = append(result, pkg)
	}
	return result
}

func (g *Graph) GetNeighborhood(nodeID string, depth int) ([]*Node, []*Edge) {
	g.mu.RLock()
	defer g.mu.RUnlock()

	visited := make(map[string]bool)
	nodes := make(map[string]*Node)
	edges := make(map[string]*Edge)

	var visit func(id string, d int)
	visit = func(id string, d int) {
		if d < 0 || visited[id] {
			return
		}
		visited[id] = true

		if node, ok := g.nodes[id]; ok {
			nodes[id] = node.Clone()
		}

		for _, edge := range g.edges[id] {
			edgeKey := edge.From + "->" + edge.To
			edges[edgeKey] = edge
			visit(edge.To, d-1)
		}

		for _, edge := range g.inEdges[id] {
			edgeKey := edge.From + "->" + edge.To
			edges[edgeKey] = edge
			visit(edge.From, d-1)
		}
	}

	visit(nodeID, depth)

	result := make([]*Node, 0, len(nodes))
	for _, node := range nodes {
		result = append(result, node)
	}

	edgeResult := make([]*Edge, 0, len(edges))
	for _, edge := range edges {
		edgeResult = append(edgeResult, edge)
	}

	return result, edgeResult
}

func (g *Graph) GetImplementors(interfaceID string) []*Node {
	g.mu.RLock()
	defer g.mu.RUnlock()
	impls := g.byInterface[interfaceID]
	result := make([]*Node, len(impls))
	for i, impl := range impls {
		result[i] = impl.Clone()
	}
	return result
}

func (g *Graph) GetInterfaces(typeID string) []*Node {
	g.mu.RLock()
	defer g.mu.RUnlock()
	ifaces := g.byTypeImpl[typeID]
	result := make([]*Node, len(ifaces))
	for i, iface := range ifaces {
		result[i] = iface.Clone()
	}
	return result
}

func (g *Graph) GetNodesByFile(file string) []*Node {
	g.mu.RLock()
	defer g.mu.RUnlock()

	var nodes []*Node
	for _, node := range g.nodes {
		if node.File == file {
			nodes = append(nodes, node.Clone())
		}
	}
	return nodes
}

func (g *Graph) GetNodesByPackageAndType(pkg string, nodeType NodeType) []*Node {
	g.mu.RLock()
	defer g.mu.RUnlock()

	var nodes []*Node
	for _, node := range g.byPackage[pkg] {
		if node.Type == nodeType {
			nodes = append(nodes, node.Clone())
		}
	}
	return nodes
}

func (g *Graph) GetNeighborsByEdgeType(nodeID string, edgeType EdgeType) []*Node {
	g.mu.RLock()
	defer g.mu.RUnlock()

	var result []*Node
	for _, edge := range g.edges[nodeID] {
		if edge.Type == edgeType {
			if node, ok := g.nodes[edge.To]; ok {
				result = append(result, node.Clone())
			}
		}
	}
	return result
}

func (g *Graph) FindTests(nodeID string) []*Node {
	g.mu.RLock()
	defer g.mu.RUnlock()

	return g.findTestsLocked(nodeID)
}

func (g *Graph) GetEdgesFrom(nodeID string) []*Edge {
	g.mu.RLock()
	defer g.mu.RUnlock()
	edges := g.edges[nodeID]
	result := make([]*Edge, len(edges))
	copy(result, edges)
	return result
}

func (g *Graph) GetNodesByBehaviors(behaviors []string) []*Node {
	g.mu.RLock()
	defer g.mu.RUnlock()

	if len(behaviors) == 0 {
		result := make([]*Node, 0, len(g.nodes))
		for _, node := range g.nodes {
			if node.Type == NodeTypeFunction || node.Type == NodeTypeMethod {
				result = append(result, node.Clone())
			}
		}
		return result
	}

	var result []*Node
	for _, node := range g.nodes {
		if node.Type != NodeTypeFunction && node.Type != NodeTypeMethod {
			continue
		}

		nodeBehaviors := getBehaviorsFromMetadata(node)
		if hasAllBehaviors(nodeBehaviors, behaviors) {
			result = append(result, node.Clone())
		}
	}

	return result
}

func getBehaviorsFromMetadata(node *Node) []string {
	if node.Metadata == nil {
		return nil
	}

	behaviorsRaw, ok := node.Metadata["behaviors"]
	if !ok {
		return nil
	}

	behaviors, ok := behaviorsRaw.([]string)
	if !ok {
		return nil
	}

	return behaviors
}

func hasAllBehaviors(nodeBehaviors, requiredBehaviors []string) bool {
	for _, required := range requiredBehaviors {
		found := false
		for _, b := range nodeBehaviors {
			if b == required {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	return true
}
