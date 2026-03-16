package graph

import "errors"

var ErrNoPath = errors.New("no path found between nodes")

// FindPath finds the shortest call path from fromID to toID using BFS.
// Returns the ordered list of nodes (inclusive) or ErrNoPath if unreachable.
// maxDepth limits the search depth (0 means unlimited).
func (g *Graph) FindPath(fromID, toID string, maxDepth int) ([]*Node, error) {
	g.mu.RLock()
	defer g.mu.RUnlock()

	if _, ok := g.nodes[fromID]; !ok {
		return nil, ErrNodeNotFound
	}
	if _, ok := g.nodes[toID]; !ok {
		return nil, ErrNodeNotFound
	}

	if fromID == toID {
		return []*Node{g.nodes[fromID].Clone()}, nil
	}

	type queueEntry struct {
		id    string
		depth int
	}

	visited := map[string]string{} // child -> parent
	queue := []queueEntry{{id: fromID, depth: 0}}
	visited[fromID] = ""

	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]

		if maxDepth > 0 && current.depth >= maxDepth {
			continue
		}

		for _, edge := range g.edges[current.id] {
			if edge.Type != EdgeTypeCalls {
				continue
			}
			if _, seen := visited[edge.To]; seen {
				continue
			}
			visited[edge.To] = current.id
			if edge.To == toID {
				return reconstructPath(g, visited, fromID, toID), nil
			}
			queue = append(queue, queueEntry{id: edge.To, depth: current.depth + 1})
		}
	}

	return nil, ErrNoPath
}

func reconstructPath(g *Graph, visited map[string]string, fromID, toID string) []*Node {
	// Build path backwards from toID to fromID
	var path []string
	current := toID
	for current != "" {
		path = append(path, current)
		current = visited[current]
	}
	// Reverse
	for i, j := 0, len(path)-1; i < j; i, j = i+1, j-1 {
		path[i], path[j] = path[j], path[i]
	}

	result := make([]*Node, len(path))
	for i, id := range path {
		result[i] = g.nodes[id].Clone()
	}
	return result
}
