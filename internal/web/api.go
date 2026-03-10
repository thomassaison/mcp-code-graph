package web

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
