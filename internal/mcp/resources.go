package mcp

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/thomas-saison/mcp-code-graph/internal/graph"
)

type Resource struct {
	URI         string
	Name        string
	Description string
	MimeType    string
}

func (s *Server) GetResources() []Resource {
	var resources []Resource

	functions := s.graph.GetNodesByType(graph.NodeTypeFunction)
	for _, fn := range functions {
		resources = append(resources, Resource{
			URI:         fmt.Sprintf("function://%s/%s", fn.Package, fn.Name),
			Name:        fn.Name,
			Description: fmt.Sprintf("Function %s in package %s", fn.Name, fn.Package),
			MimeType:    "application/json",
		})
	}

	packages := make(map[string]bool)
	for _, fn := range functions {
		packages[fn.Package] = true
	}

	for pkg := range packages {
		resources = append(resources, Resource{
			URI:         fmt.Sprintf("package://%s", pkg),
			Name:        pkg,
			Description: fmt.Sprintf("Package %s overview", pkg),
			MimeType:    "application/json",
		})
	}

	return resources
}

func (s *Server) ReadResource(uri string) (string, error) {
	if strings.HasPrefix(uri, "function://") {
		return s.readFunctionResource(uri)
	}

	if strings.HasPrefix(uri, "package://") {
		return s.readPackageResource(uri)
	}

	return "", fmt.Errorf("unknown resource URI: %s", uri)
}

func (s *Server) readFunctionResource(uri string) (string, error) {
	uriPart := strings.TrimPrefix(uri, "function://")
	lastSlash := strings.LastIndex(uriPart, "/")
	if lastSlash == -1 {
		return "", fmt.Errorf("invalid function URI: %s", uri)
	}
	pkg := uriPart[:lastSlash]
	name := uriPart[lastSlash+1:]

	functions := s.graph.GetNodesByType(graph.NodeTypeFunction)
	var fn *graph.Node
	for _, f := range functions {
		if f.Package == pkg && f.Name == name {
			fn = f
			break
		}
	}

	if fn == nil {
		return "", fmt.Errorf("function not found: %s/%s", pkg, name)
	}

	callers := s.graph.GetCallers(fn.ID)
	callees := s.graph.GetCallees(fn.ID)

	var callerNames []string
	for _, c := range callers {
		callerNames = append(callerNames, c.Name)
	}

	var calleeNames []string
	for _, c := range callees {
		calleeNames = append(calleeNames, c.Name)
	}

	result := map[string]any{
		"id":        fn.ID,
		"name":      fn.Name,
		"package":   fn.Package,
		"file":      fn.File,
		"line":      fn.Line,
		"signature": fn.Signature,
		"docstring": fn.Docstring,
		"callers":   callerNames,
		"callees":   calleeNames,
	}

	if fn.Summary != nil {
		result["summary"] = fn.Summary.Text
	}

	data, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return "", err
	}

	return string(data), nil
}

func (s *Server) readPackageResource(uri string) (string, error) {
	pkg := strings.TrimPrefix(uri, "package://")

	nodes := s.graph.GetNodesByPackage(pkg)
	if len(nodes) == 0 {
		return "", fmt.Errorf("package not found: %s", pkg)
	}

	var functions []map[string]any
	for _, node := range nodes {
		if node.Type == graph.NodeTypeFunction || node.Type == graph.NodeTypeMethod {
			fnInfo := map[string]any{
				"id":        node.ID,
				"name":      node.Name,
				"signature": node.Signature,
			}
			if node.Summary != nil {
				fnInfo["summary"] = node.Summary.Text
			}
			functions = append(functions, fnInfo)
		}
	}

	result := map[string]any{
		"package":   pkg,
		"functions": functions,
	}

	data, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return "", err
	}

	return string(data), nil
}
