package graph

import (
	"sort"
	"strings"
)

// PatternType represents a category of code pattern to discover.
type PatternType string

const (
	PatternConstructors PatternType = "constructors"
	PatternErrorHandle  PatternType = "error-handling"
	PatternTests        PatternType = "tests"
	PatternEntryPoints  PatternType = "entrypoints"
	PatternSinks        PatternType = "sinks"
	PatternSources      PatternType = "sources"
	PatternHotspots     PatternType = "hotspots"
)

var validPatternTypes = map[PatternType]bool{
	PatternConstructors: true,
	PatternErrorHandle:  true,
	PatternTests:        true,
	PatternEntryPoints:  true,
	PatternSinks:        true,
	PatternSources:      true,
	PatternHotspots:     true,
}

// PatternResult holds discovered pattern nodes and metadata.
type PatternResult struct {
	PatternType string  `json:"pattern_type"`
	Package     string  `json:"package"`
	Count       int     `json:"count"`
	Functions   []*Node `json:"functions"`
}

// DiscoverPatterns finds code patterns of the given type within a package.
func (g *Graph) DiscoverPatterns(pkg string, patternType PatternType) *PatternResult {
	g.mu.RLock()
	defer g.mu.RUnlock()

	result := &PatternResult{
		PatternType: string(patternType),
		Package:     pkg,
	}

	pkgNodes, ok := g.byPackage[pkg]
	if !ok {
		return result
	}

	switch patternType {
	case PatternConstructors:
		result.Functions = g.findConstructors(pkgNodes)
	case PatternErrorHandle:
		result.Functions = g.findErrorHandlers(pkgNodes)
	case PatternTests:
		result.Functions = g.findPackageTests(pkgNodes)
	case PatternEntryPoints:
		result.Functions = g.findEntryPoints(pkgNodes)
	case PatternSinks:
		result.Functions = g.findSinks(pkgNodes)
	case PatternSources:
		result.Functions = g.findSources(pkgNodes)
	case PatternHotspots:
		result.Functions = g.findHotspots(pkg, 10)
	}

	result.Count = len(result.Functions)
	return result
}

func IsValidPatternType(pt string) bool {
	return validPatternTypes[PatternType(pt)]
}

func (g *Graph) findConstructors(pkgNodes map[string]*Node) []*Node {
	var result []*Node
	for _, node := range pkgNodes {
		if node.Type != NodeTypeFunction {
			continue
		}
		if strings.HasPrefix(node.Name, "New") && len(node.Name) > 3 && node.Name[3] >= 'A' && node.Name[3] <= 'Z' {
			result = append(result, node.Clone())
		}
	}
	sort.Slice(result, func(i, j int) bool { return result[i].Name < result[j].Name })
	return result
}

func (g *Graph) findErrorHandlers(pkgNodes map[string]*Node) []*Node {
	var result []*Node
	for _, node := range pkgNodes {
		if node.Type != NodeTypeFunction && node.Type != NodeTypeMethod {
			continue
		}

		// Check behavior metadata first
		if behavs := getBehaviorsFromMetadata(node); behavs != nil {
			for _, b := range behavs {
				if b == "error-handle" {
					result = append(result, node.Clone())
					continue
				}
			}
		}

		// Fallback: check signature for "error"
		if node.Signature != "" && strings.Contains(node.Signature, "error") {
			// Avoid duplicates
			found := false
			for _, r := range result {
				if r.ID == node.ID {
					found = true
					break
				}
			}
			if !found {
				result = append(result, node.Clone())
			}
		}
	}
	sort.Slice(result, func(i, j int) bool { return result[i].Name < result[j].Name })
	return result
}

func (g *Graph) findPackageTests(pkgNodes map[string]*Node) []*Node {
	var result []*Node
	for _, node := range pkgNodes {
		if isTestFunction(node) {
			result = append(result, node.Clone())
		}
	}
	sort.Slice(result, func(i, j int) bool { return result[i].Name < result[j].Name })
	return result
}

func (g *Graph) findEntryPoints(pkgNodes map[string]*Node) []*Node {
	pkgName := ""
	for _, node := range pkgNodes {
		pkgName = node.Package
		break
	}

	var result []*Node
	for _, node := range pkgNodes {
		if node.Type != NodeTypeFunction && node.Type != NodeTypeMethod {
			continue
		}
		// Must be exported
		if len(node.Name) == 0 || node.Name[0] < 'A' || node.Name[0] > 'Z' {
			continue
		}
		// Check if any caller is in the same package
		hasInternalCaller := false
		for _, edge := range g.inEdges[node.ID] {
			if edge.Type != EdgeTypeCalls {
				continue
			}
			if caller, ok := g.nodes[edge.From]; ok && caller.Package == pkgName {
				hasInternalCaller = true
				break
			}
		}
		if !hasInternalCaller {
			result = append(result, node.Clone())
		}
	}
	sort.Slice(result, func(i, j int) bool { return result[i].Name < result[j].Name })
	return result
}

func (g *Graph) findSinks(pkgNodes map[string]*Node) []*Node {
	var result []*Node
	for _, node := range pkgNodes {
		if node.Type != NodeTypeFunction && node.Type != NodeTypeMethod {
			continue
		}
		hasCallees := false
		for _, edge := range g.edges[node.ID] {
			if edge.Type == EdgeTypeCalls {
				hasCallees = true
				break
			}
		}
		hasCallers := false
		for _, edge := range g.inEdges[node.ID] {
			if edge.Type == EdgeTypeCalls {
				hasCallers = true
				break
			}
		}
		if hasCallers && !hasCallees {
			result = append(result, node.Clone())
		}
	}
	sort.Slice(result, func(i, j int) bool { return result[i].Name < result[j].Name })
	return result
}

func (g *Graph) findSources(pkgNodes map[string]*Node) []*Node {
	var result []*Node
	for _, node := range pkgNodes {
		if node.Type != NodeTypeFunction && node.Type != NodeTypeMethod {
			continue
		}
		hasCallees := false
		for _, edge := range g.edges[node.ID] {
			if edge.Type == EdgeTypeCalls {
				hasCallees = true
				break
			}
		}
		hasCallers := false
		for _, edge := range g.inEdges[node.ID] {
			if edge.Type == EdgeTypeCalls {
				hasCallers = true
				break
			}
		}
		if hasCallees && !hasCallers {
			result = append(result, node.Clone())
		}
	}
	sort.Slice(result, func(i, j int) bool { return result[i].Name < result[j].Name })
	return result
}

func (g *Graph) findHotspots(pkg string, limit int) []*Node {
	type scored struct {
		node  *Node
		count int
	}

	var scoredList []scored
	for _, node := range g.byPackage[pkg] {
		if node.Type != NodeTypeFunction && node.Type != NodeTypeMethod {
			continue
		}
		count := 0
		for _, edge := range g.inEdges[node.ID] {
			if edge.Type == EdgeTypeCalls {
				count++
			}
		}
		if count > 0 {
			scoredList = append(scoredList, scored{node: node.Clone(), count: count})
		}
	}

	sort.Slice(scoredList, func(i, j int) bool { return scoredList[i].count > scoredList[j].count })

	if len(scoredList) > limit {
		scoredList = scoredList[:limit]
	}

	result := make([]*Node, len(scoredList))
	for i, s := range scoredList {
		result[i] = s.node
	}
	return result
}
