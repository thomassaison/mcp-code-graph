package web

import (
	"io/fs"
	"net/http"
	"strings"

	"github.com/thomassaison/mcp-code-graph/internal/graph"
)

// Handler serves the web visualization UI and API endpoints.
type Handler struct {
	graph *graph.Graph
}

// NewHandler creates a new web handler with the given graph.
func NewHandler(g *graph.Graph) *Handler {
	return &Handler{graph: g}
}

// ServeHTTP routes requests to API handlers or serves static files.
func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Path

	switch {
	case path == "/api/graph":
		h.handleGraph(w, r)
	case path == "/api/packages":
		h.handlePackages(w, r)
	case strings.HasPrefix(path, "/api/packages/") && strings.HasSuffix(path, "/nodes"):
		pkg := strings.TrimPrefix(path, "/api/packages/")
		pkg = strings.TrimSuffix(pkg, "/nodes")
		h.handlePackageNodes(w, r, pkg)
	case strings.HasPrefix(path, "/api/nodes/") && strings.HasSuffix(path, "/neighborhood"):
		id := strings.TrimPrefix(path, "/api/nodes/")
		id = strings.TrimSuffix(id, "/neighborhood")
		h.handleNeighborhood(w, r, id)
	case strings.HasPrefix(path, "/api/nodes/"):
		id := strings.TrimPrefix(path, "/api/nodes/")
		h.handleNode(w, r, id)
	case path == "/api/search":
		h.handleSearch(w, r)
	case path == "/api/stats":
		h.handleStats(w, r)
	default:
		// Serve static files
		sub, err := fs.Sub(staticFS, "static")
		if err != nil {
			http.NotFound(w, r)
			return
		}
		http.FileServer(http.FS(sub)).ServeHTTP(w, r)
	}
}
