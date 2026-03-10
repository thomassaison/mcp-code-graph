package web

import (
	"net/http"

	"github.com/thomassaison/mcp-code-graph/internal/graph"
)

type Handler struct {
	graph *graph.Graph
}

func NewHandler(g *graph.Graph) *Handler {
	return &Handler{graph: g}
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path == "/" {
		http.ServeFileFS(w, r, staticFS, "static/index.html")
		return
	}
	http.NotFound(w, r)
}
