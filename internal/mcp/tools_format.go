// anthropic/claude-sonnet-4-6
package mcp

import "github.com/thomassaison/mcp-code-graph/internal/graph"

// formatNodeList converts a slice of Nodes into a JSON-friendly format.
func formatNodeList(nodes []*graph.Node) []map[string]any {
	result := make([]map[string]any, 0, len(nodes))
	for _, n := range nodes {
		entry := map[string]any{
			"id":        n.ID,
			"name":      n.Name,
			"package":   n.Package,
			"signature": n.Signature,
		}
		if n.File != "" {
			entry["file"] = n.File
		}
		if n.Line > 0 {
			entry["line"] = n.Line
		}
		if s := n.SummaryText(); s != "" {
			entry["summary"] = s
		}
		result = append(result, entry)
	}
	return result
}
