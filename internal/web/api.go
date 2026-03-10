package web

import (
	"encoding/json"
	"net/http"
	"sort"
	"strconv"
)

type PackageNode struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Type string `json:"type"`
}

type NodeResponse struct {
	ID        string       `json:"id"`
	Name      string       `json:"name"`
	Type      string       `json:"type"`
	Package   string       `json:"package"`
	File      string       `json:"file"`
	Line      int          `json:"line"`
	Signature string       `json:"signature,omitempty"`
	Docstring string       `json:"docstring,omitempty"`
	Summary   string       `json:"summary,omitempty"`
	Behaviors []string     `json:"behaviors,omitempty"`
	Methods   []MethodResp `json:"methods,omitempty"`
}

type MethodResp struct {
	Name      string `json:"name"`
	Signature string `json:"signature"`
}

type NeighborhoodResponse struct {
	Center NodeResponse   `json:"center"`
	Nodes  []NodeResponse `json:"nodes"`
	Edges  []EdgeResponse `json:"edges"`
}

type EdgeResponse struct {
	From string `json:"from"`
	To   string `json:"to"`
	Type string `json:"type"`
}

type StatsResponse struct {
	NodeCount int            `json:"node_count"`
	EdgeCount int            `json:"edge_count"`
	ByType    map[string]int `json:"by_type"`
	ByPackage map[string]int `json:"by_package"`
}

type ErrorResponse struct {
	Error string `json:"error"`
}

type GraphResponse struct {
	Nodes []NodeResponse `json:"nodes"`
	Edges []EdgeResponse `json:"edges"`
}

func (h *Handler) handleGraph(w http.ResponseWriter, r *http.Request) {
	allNodes := h.graph.AllNodes()
	allEdges := h.graph.AllEdges()

	nodes := make([]NodeResponse, 0, len(allNodes))
	for _, n := range allNodes {
		resp := NodeResponse{
			ID:        n.ID,
			Name:      n.Name,
			Type:      string(n.Type),
			Package:   n.Package,
			File:      n.File,
			Line:      n.Line,
			Signature: n.Signature,
			Docstring: n.Docstring,
		}
		if n.Summary != nil {
			resp.Summary = n.Summary.Text
		}
		if n.Metadata != nil {
			if beh, ok := n.Metadata["behaviors"].([]string); ok {
				resp.Behaviors = beh
			}
		}
		for _, m := range n.Methods {
			resp.Methods = append(resp.Methods, MethodResp{
				Name:      m.Name,
				Signature: m.Signature,
			})
		}
		nodes = append(nodes, resp)
	}

	edges := make([]EdgeResponse, 0, len(allEdges))
	for _, e := range allEdges {
		edges = append(edges, EdgeResponse{
			From: e.From,
			To:   e.To,
			Type: string(e.Type),
		})
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(GraphResponse{
		Nodes: nodes,
		Edges: edges,
	})
}

func (h *Handler) handlePackages(w http.ResponseWriter, r *http.Request) {
	pkgs := h.graph.AllPackages()
	sort.Strings(pkgs)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(pkgs)
}

func (h *Handler) handlePackageNodes(w http.ResponseWriter, r *http.Request, pkg string) {
	nodes := h.graph.GetNodesByPackage(pkg)

	result := make([]PackageNode, 0, len(nodes))
	for _, n := range nodes {
		result = append(result, PackageNode{
			ID:   n.ID,
			Name: n.Name,
			Type: string(n.Type),
		})
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)
}

func (h *Handler) handleNode(w http.ResponseWriter, r *http.Request, id string) {
	node, err := h.graph.GetNode(id)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(ErrorResponse{Error: "node not found"})
		return
	}

	resp := NodeResponse{
		ID:        node.ID,
		Name:      node.Name,
		Type:      string(node.Type),
		Package:   node.Package,
		File:      node.File,
		Line:      node.Line,
		Signature: node.Signature,
		Docstring: node.Docstring,
	}

	if node.Summary != nil {
		resp.Summary = node.Summary.Text
	}

	if node.Metadata != nil {
		if beh, ok := node.Metadata["behaviors"].([]string); ok {
			resp.Behaviors = beh
		}
	}

	for _, m := range node.Methods {
		resp.Methods = append(resp.Methods, MethodResp{
			Name:      m.Name,
			Signature: m.Signature,
		})
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

func (h *Handler) handleNeighborhood(w http.ResponseWriter, r *http.Request, id string) {
	depth := 1
	if d := r.URL.Query().Get("depth"); d != "" {
		if n, err := strconv.Atoi(d); err == nil && n >= 1 && n <= 3 {
			depth = n
		}
	}

	center, err := h.graph.GetNode(id)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(ErrorResponse{Error: "node not found"})
		return
	}

	nodes, edges := h.graph.GetNeighborhood(id, depth)

	nodeResponses := make([]NodeResponse, 0, len(nodes))
	for _, n := range nodes {
		nodeResponses = append(nodeResponses, NodeResponse{
			ID:      n.ID,
			Name:    n.Name,
			Type:    string(n.Type),
			Package: n.Package,
		})
	}

	edgeResponses := make([]EdgeResponse, 0, len(edges))
	for _, e := range edges {
		edgeResponses = append(edgeResponses, EdgeResponse{
			From: e.From,
			To:   e.To,
			Type: string(e.Type),
		})
	}

	resp := NeighborhoodResponse{
		Center: NodeResponse{
			ID:      center.ID,
			Name:    center.Name,
			Type:    string(center.Type),
			Package: center.Package,
		},
		Nodes: nodeResponses,
		Edges: edgeResponses,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

func (h *Handler) handleSearch(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query().Get("q")
	if query == "" {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode([]PackageNode{})
		return
	}

	nodes := h.graph.GetNodesByName(query)

	result := make([]PackageNode, 0, len(nodes))
	for _, n := range nodes {
		result = append(result, PackageNode{
			ID:   n.ID,
			Name: n.Name,
			Type: string(n.Type),
		})
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)
}

func (h *Handler) handleStats(w http.ResponseWriter, r *http.Request) {
	byType := make(map[string]int)
	for _, node := range h.graph.AllNodes() {
		byType[string(node.Type)]++
	}

	byPackage := make(map[string]int)
	for _, pkg := range h.graph.AllPackages() {
		byPackage[pkg] = len(h.graph.GetNodesByPackage(pkg))
	}

	resp := StatsResponse{
		NodeCount: h.graph.NodeCount(),
		EdgeCount: h.graph.EdgeCount(),
		ByType:    byType,
		ByPackage: byPackage,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}
