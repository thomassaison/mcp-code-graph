// Model: anthropic/claude-sonnet-4-6
package mcp

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"go/parser"
	"go/printer"
	"go/token"
	"os"
	"strings"

	mcplib "github.com/mark3labs/mcp-go/mcp"
	"github.com/thomassaison/mcp-code-graph/internal/graph"
)

// ContextEntry is a compact representation of a related function for LLM consumption.
type ContextEntry struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Package   string `json:"package"`
	Signature string `json:"signature"`
	Summary   string `json:"summary,omitempty"`
}

// FunctionContext is an LLM-ready context block returned by get_function_context.
type FunctionContext struct {
	Function *graph.Node     `json:"function"`
	Code     string          `json:"code,omitempty"`
	Callers  []ContextEntry  `json:"callers"`
	Callees  []ContextEntry  `json:"callees"`
	Contract *graph.Contract `json:"contract"`
	Tests    []*graph.Node   `json:"tests"`
	Package  string          `json:"package"`
	File     string          `json:"file"`
}

func (s *Server) handleGetFunctionContext(ctx context.Context, args map[string]any) (string, error) {
	functionID, ok := args["function_id"].(string)
	if !ok {
		return "", fmt.Errorf("function_id must be a string")
	}

	node, err := s.graph.GetNode(functionID)
	if err != nil {
		return "", fmt.Errorf("function not found: %w", err)
	}

	// Lazy-read code from file
	code := readFunctionCode(node.File, node.Line)

	// Build callers context entries
	callers := s.graph.GetCallers(functionID)
	callerEntries := make([]ContextEntry, 0, len(callers))
	for _, c := range callers {
		callerEntries = append(callerEntries, ContextEntry{
			ID:        c.ID,
			Name:      c.Name,
			Package:   c.Package,
			Signature: c.Signature,
			Summary:   c.SummaryText(),
		})
	}

	// Build callees context entries
	callees := s.graph.GetCallees(functionID)
	calleeEntries := make([]ContextEntry, 0, len(callees))
	for _, c := range callees {
		calleeEntries = append(calleeEntries, ContextEntry{
			ID:        c.ID,
			Name:      c.Name,
			Package:   c.Package,
			Signature: c.Signature,
			Summary:   c.SummaryText(),
		})
	}

	// Get contract
	contract := s.graph.GetContract(functionID)

	// Get tests
	tests := s.graph.FindTests(functionID)

	fc := &FunctionContext{
		Function: node,
		Code:     code,
		Callers:  callerEntries,
		Callees:  calleeEntries,
		Contract: contract,
		Tests:    tests,
		Package:  node.Package,
		File:     node.File,
	}

	data, err := json.MarshalIndent(fc, "", "  ")
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func (s *Server) handleGetFunctionContextMCP(ctx context.Context, req mcplib.CallToolRequest) (*mcplib.CallToolResult, error) {
	functionID, err := req.RequireString("function_id")
	if err != nil {
		return mcplib.NewToolResultError(err.Error()), nil
	}

	result, resultErr := s.handleGetFunctionContext(ctx, map[string]any{
		"function_id": functionID,
	})
	if resultErr != nil {
		return mcplib.NewToolResultError(resultErr.Error()), nil
	}

	return mcplib.NewToolResultText(result), nil
}

// readFunctionCode reads a function body from a source file starting at the
// given line. It uses go/parser to accurately extract the function body,
// correctly handling braces in strings, comments, and rune literals.
// Falls back to naive brace-counting if parsing fails.
func readFunctionCode(filePath string, startLine int) string {
	if filePath == "" || startLine <= 0 {
		return ""
	}

	src, err := os.ReadFile(filePath)
	if err != nil {
		return ""
	}

	fset := token.NewFileSet()
	astFile, err := parser.ParseFile(fset, filePath, src, parser.ParseComments)
	if err == nil {
		// Walk all top-level declarations to find a function starting at startLine.
		for _, decl := range astFile.Decls {
			pos := fset.Position(decl.Pos())
			end := fset.Position(decl.End())
			if pos.Line == startLine {
				var buf bytes.Buffer
				if printErr := printer.Fprint(&buf, fset, decl); printErr == nil {
					return buf.String() + "\n"
				}
				// printer failed; fall back to byte-offset slicing
				startOff := fset.File(decl.Pos()).Offset(decl.Pos())
				endOff := fset.File(decl.End()).Offset(decl.End())
				if startOff >= 0 && endOff <= len(src) && startOff <= endOff {
					return string(src[startOff:endOff]) + "\n"
				}
			}
			// Handle functions where the declaration keyword is on startLine
			// but the opening brace may be on a later line (e.g. multiline signatures).
			if pos.Line <= startLine && end.Line >= startLine {
				// Only pick it up if the function starts close to startLine
				// (within a small window) to avoid matching outer declarations.
				if startLine-pos.Line <= 5 {
					var buf bytes.Buffer
					if printErr := printer.Fprint(&buf, fset, decl); printErr == nil {
						return buf.String() + "\n"
					}
					startOff := fset.File(decl.Pos()).Offset(decl.Pos())
					endOff := fset.File(decl.End()).Offset(decl.End())
					if startOff >= 0 && endOff <= len(src) && startOff <= endOff {
						return string(src[startOff:endOff]) + "\n"
					}
				}
			}
		}
	}

	// Fallback: naive brace counting (does not handle braces in strings/comments).
	return readFunctionCodeNaive(src, startLine)
}

// readFunctionCodeNaive is the original brace-counting fallback used when
// go/parser cannot parse the file (e.g. due to syntax errors).
func readFunctionCodeNaive(src []byte, startLine int) string {
	scanner := bufio.NewScanner(strings.NewReader(string(src)))
	lineNum := 0
	var sb strings.Builder
	braceCount := 0
	inFunction := false

	for scanner.Scan() {
		lineNum++
		if lineNum < startLine {
			continue
		}

		line := scanner.Text()
		sb.WriteString(line)
		sb.WriteString("\n")

		for _, ch := range line {
			switch ch {
			case '{':
				braceCount++
				inFunction = true
			case '}':
				braceCount--
			}
		}

		if inFunction && braceCount <= 0 {
			break
		}
	}

	return sb.String()
}
