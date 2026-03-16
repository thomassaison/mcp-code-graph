// anthropic/claude-sonnet-4-6
package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/thomassaison/mcp-code-graph/internal/graph"
)

func (s *Server) handleGetImpact(ctx context.Context, args map[string]any) (string, error) {
	functionID, ok := args["function_id"].(string)
	if !ok {
		return "", fmt.Errorf("function_id must be a string")
	}

	report := s.graph.GetImpact(functionID)

	result := map[string]any{
		"node_id":             report.NodeID,
		"risk_level":          report.RiskLevel,
		"total_reach":         report.TotalReach,
		"direct_callers":      formatNodeList(report.DirectCallers),
		"indirect_callers":    formatNodeList(report.IndirectCallers),
		"interface_contracts": formatNodeList(report.InterfaceContracts),
		"tests":               formatNodeList(report.Tests),
	}

	data, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func (s *Server) handleGetContract(ctx context.Context, args map[string]any) (string, error) {
	functionID, ok := args["function_id"].(string)
	if !ok {
		return "", fmt.Errorf("function_id must be a string")
	}

	contract := s.graph.GetContract(functionID)
	if contract == nil {
		return "", fmt.Errorf("function not found: %s", functionID)
	}

	result := map[string]any{
		"node": map[string]any{
			"id":        contract.Node.ID,
			"name":      contract.Node.Name,
			"package":   contract.Node.Package,
			"signature": contract.Node.Signature,
		},
		"caller_count":    contract.CallerCount,
		"callee_count":    contract.CalleeCount,
		"receiver_type":   contract.ReceiverType,
		"type_interfaces": formatNodeList(contract.TypeInterfaces),
		"returned_types":  formatNodeList(contract.ReturnedTypes),
		"accepted_types":  formatNodeList(contract.AcceptedTypes),
		"test_functions":  formatNodeList(contract.TestFunctions),
	}

	data, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func (s *Server) handleFindTests(ctx context.Context, args map[string]any) (string, error) {
	functionID, ok := args["function_id"].(string)
	if !ok {
		return "", fmt.Errorf("function_id must be a string")
	}

	tests := s.graph.FindTests(functionID)

	data, err := json.MarshalIndent(formatNodeList(tests), "", "  ")
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func (s *Server) handleDiscoverPatterns(ctx context.Context, args map[string]any) (string, error) {
	pkg, ok := args["package"].(string)
	if !ok {
		return "", fmt.Errorf("package must be a string")
	}
	patternType, ok := args["pattern_type"].(string)
	if !ok {
		return "", fmt.Errorf("pattern_type must be a string")
	}

	if !graph.IsValidPatternType(patternType) {
		return "", fmt.Errorf("invalid pattern_type: %s (valid: constructors, error-handling, tests, entrypoints, sinks, sources, hotspots)", patternType)
	}

	report := s.graph.DiscoverPatterns(pkg, graph.PatternType(patternType))

	result := map[string]any{
		"pattern_type": report.PatternType,
		"package":      report.Package,
		"count":        report.Count,
		"functions":    formatNodeList(report.Functions),
	}

	data, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func (s *Server) handleGetFunctionByName(ctx context.Context, args map[string]any) (string, error) {
	name, ok := args["name"].(string)
	if !ok {
		return "", fmt.Errorf("name must be a string")
	}

	pkg, _ := args["package"].(string)
	file, _ := args["file"].(string)

	var nodes []*graph.Node
	if pkg != "" {
		nodes = s.graph.GetNodesByNameAndPackage(name, pkg)
	} else {
		nodes = s.graph.GetNodesByName(name)
	}

	if file != "" {
		var filtered []*graph.Node
		for _, n := range nodes {
			if strings.Contains(n.File, file) {
				filtered = append(filtered, n)
			}
		}
		nodes = filtered
	}

	results := make([]map[string]any, 0)
	for _, n := range nodes {
		if n.Type != graph.NodeTypeFunction && n.Type != graph.NodeTypeMethod {
			continue
		}
		results = append(results, map[string]any{
			"id":        n.ID,
			"name":      n.Name,
			"package":   n.Package,
			"signature": n.Signature,
			"file":      n.File,
			"line":      n.Line,
			"docstring": n.Docstring,
			"summary":   n.SummaryText(),
		})
	}

	data, err := json.MarshalIndent(results, "", "  ")
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func (s *Server) handleGetImplementors(ctx context.Context, args map[string]any) (string, error) {
	interfaceID, ok := args["interface_id"].(string)
	if !ok {
		return "", fmt.Errorf("interface_id must be a string")
	}

	ifaceNode, err := s.graph.GetNode(interfaceID)
	if err != nil {
		return "", fmt.Errorf("interface not found: %s", interfaceID)
	}

	implementors := s.graph.GetImplementors(interfaceID)

	result := map[string]any{
		"interface": map[string]any{
			"id":      ifaceNode.ID,
			"name":    ifaceNode.Name,
			"package": ifaceNode.Package,
			"methods": ifaceNode.Methods,
		},
		"implementors": []map[string]any{},
	}

	implList := make([]map[string]any, 0, len(implementors))
	for _, impl := range implementors {
		implList = append(implList, map[string]any{
			"id":      impl.ID,
			"name":    impl.Name,
			"package": impl.Package,
			"kind":    impl.Metadata["kind"],
		})
	}
	result["implementors"] = implList

	data, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func (s *Server) handleGetInterfaces(ctx context.Context, args map[string]any) (string, error) {
	typeID, ok := args["type_id"].(string)
	if !ok {
		return "", fmt.Errorf("type_id must be a string")
	}

	typeNode, err := s.graph.GetNode(typeID)
	if err != nil {
		return "", fmt.Errorf("type not found: %s", typeID)
	}

	interfaces := s.graph.GetInterfaces(typeID)

	result := map[string]any{
		"type": map[string]any{
			"id":      typeNode.ID,
			"name":    typeNode.Name,
			"package": typeNode.Package,
			"kind":    typeNode.Metadata["kind"],
		},
		"interfaces": []map[string]any{},
	}

	ifaceList := make([]map[string]any, 0, len(interfaces))
	for _, iface := range interfaces {
		pointerReceiver := false
		for _, edge := range s.graph.GetEdgesFrom(typeID) {
			if edge.To == iface.ID && edge.Type == graph.EdgeTypeImplements {
				if pr, ok := edge.Metadata["pointer_receiver"].(bool); ok {
					pointerReceiver = pr
				}
				break
			}
		}

		ifaceList = append(ifaceList, map[string]any{
			"id":               iface.ID,
			"name":             iface.Name,
			"package":          iface.Package,
			"pointer_receiver": pointerReceiver,
		})
	}
	result["interfaces"] = ifaceList

	data, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func (s *Server) handleGetImpactMCP(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {

	functionID, err := req.RequireString("function_id")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	result, err := s.handleGetImpact(ctx, map[string]any{"function_id": functionID})
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	return mcp.NewToolResultText(result), nil
}

func (s *Server) handleGetContractMCP(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {

	functionID, err := req.RequireString("function_id")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	result, err := s.handleGetContract(ctx, map[string]any{"function_id": functionID})
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	return mcp.NewToolResultText(result), nil
}

func (s *Server) handleFindTestsMCP(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {

	functionID, err := req.RequireString("function_id")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	result, err := s.handleFindTests(ctx, map[string]any{"function_id": functionID})
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	return mcp.NewToolResultText(result), nil
}

func (s *Server) handleDiscoverPatternsMCP(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {

	pkg, err := req.RequireString("package")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	patternType, err := req.RequireString("pattern_type")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	result, resultErr := s.handleDiscoverPatterns(ctx, map[string]any{
		"package":      pkg,
		"pattern_type": patternType,
	})
	if resultErr != nil {
		return mcp.NewToolResultError(resultErr.Error()), nil
	}
	return mcp.NewToolResultText(result), nil
}

func (s *Server) handleGetFunctionByNameMCP(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {

	name, err := req.RequireString("name")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	args := map[string]any{
		"name": name,
	}

	if pkg, ok := req.GetArguments()["package"].(string); ok && pkg != "" {
		args["package"] = pkg
	}
	if file, ok := req.GetArguments()["file"].(string); ok && file != "" {
		args["file"] = file
	}

	result, err := s.handleGetFunctionByName(ctx, args)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	return mcp.NewToolResultText(result), nil
}

func (s *Server) handleGetImplementorsMCP(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {

	interfaceID, err := req.RequireString("interface_id")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	args := map[string]any{
		"interface_id": interfaceID,
	}

	result, err := s.handleGetImplementors(ctx, args)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	return mcp.NewToolResultText(result), nil
}

func (s *Server) handleGetInterfacesMCP(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {

	typeID, err := req.RequireString("type_id")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	args := map[string]any{
		"type_id": typeID,
	}

	result, err := s.handleGetInterfaces(ctx, args)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	return mcp.NewToolResultText(result), nil
}
