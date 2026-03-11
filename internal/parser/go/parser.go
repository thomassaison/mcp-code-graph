package goparser

import (
	"context"
	"fmt"
	"go/ast"
	goparser "go/parser"
	"go/token"
	"go/types"
	"log/slog"
	"os"
	"strings"

	"github.com/thomassaison/mcp-code-graph/internal/debug"
	"github.com/thomassaison/mcp-code-graph/internal/graph"
	"github.com/thomassaison/mcp-code-graph/internal/parser"
	"golang.org/x/tools/go/packages"
)

type GoParser struct {
	fset *token.FileSet
}

func New() *GoParser {
	return &GoParser{
		fset: token.NewFileSet(),
	}
}

func (p *GoParser) ParseFile(path string) (*parser.ParseResult, error) {
	slog.Log(context.Background(), debug.LevelTrace, "parsing file", "path", path)

	src, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read file: %w", err)
	}
	file, err := goparser.ParseFile(p.fset, path, src, goparser.ParseComments)
	if err != nil {
		return nil, fmt.Errorf("parse file: %w", err)
	}

	result := p.extractFromFile(file, src, file.Name.Name, nil)

	slog.Log(context.Background(), debug.LevelTrace, "file parsed", "path", path, "functions", len(result.Nodes))
	return result, nil
}

func (p *GoParser) extractFromFile(file *ast.File, src []byte, pkgName string, typesInfo *types.Info) *parser.ParseResult {
	result := &parser.ParseResult{}

	// Derive path from fset
	path := p.fset.Position(file.Pos()).Filename

	// Build import alias -> full import path map for resolving cross-package calls
	importMap := make(map[string]string)
	for _, imp := range file.Imports {
		importPath := strings.Trim(imp.Path.Value, `"`)
		var alias string
		if imp.Name != nil {
			alias = imp.Name.Name
		} else {
			parts := strings.Split(importPath, "/")
			alias = parts[len(parts)-1]
		}
		importMap[alias] = importPath
	}

	for _, decl := range file.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok {
			continue
		}

		node := &graph.Node{
			Type:      graph.NodeTypeFunction,
			Package:   pkgName,
			Name:      fn.Name.Name,
			File:      path,
			Line:      p.fset.Position(fn.Pos()).Line,
			Signature: p.signature(fn),
			Docstring: p.docstring(fn),
			Metadata:  make(map[string]any),
		}

		// Detect method receiver
		if fn.Recv != nil && len(fn.Recv.List) > 0 {
			node.Type = graph.NodeTypeMethod
			node.Metadata["receiver"] = p.typeString(fn.Recv.List[0].Type)
		}

		node.ID = node.GenerateID()

		// Populate Code only for functions — methods are excluded intentionally
		// (this iteration; they are not processed by the embedding pipeline).
		if node.Type == graph.NodeTypeFunction && src != nil {
			start := p.fset.Position(fn.Pos()).Offset
			end := p.fset.Position(fn.End()).Offset
			if start >= 0 && end <= len(src) && start < end {
				node.Code = string(src[start:end])
			}
		}

		result.Nodes = append(result.Nodes, node)

		if fn.Body != nil {
			ast.Inspect(fn.Body, func(n ast.Node) bool {
				call, ok := n.(*ast.CallExpr)
				if !ok {
					return true
				}

				edgeTo := p.callEdgeTarget(call, pkgName, importMap, typesInfo)
				if edgeTo == "" {
					return true
				}

				edge := &graph.Edge{
					From:     node.ID,
					To:       edgeTo,
					Type:     graph.EdgeTypeCalls,
					Metadata: make(map[string]any),
				}
				result.Edges = append(result.Edges, edge)

				return true
			})
		}
	}

	return result
}

func (p *GoParser) ParsePackage(dir string) (*parser.ParseResult, error) {
	cfg := &packages.Config{
		Mode: packages.NeedName | packages.NeedFiles | packages.NeedSyntax | packages.NeedTypes | packages.NeedTypesInfo,
		Dir:  dir,
		Fset: p.fset,
	}

	pkgs, err := packages.Load(cfg, "./...")
	if err != nil {
		return nil, fmt.Errorf("load packages: %w", err)
	}

	result := &parser.ParseResult{}
	for _, pkg := range pkgs {
		shortName := pkg.Name
		fullPath := pkg.PkgPath

		for _, file := range pkg.Syntax {
			pos := p.fset.Position(file.Pos())
			src, _ := os.ReadFile(pos.Filename) // for Code extraction; ok if read fails

			fileResult := p.extractFromFile(file, src, shortName, pkg.TypesInfo)

			// Rewrite short package names to full import paths so that
			// function nodes match the convention used by the type checker.
			if shortName != fullPath {
				oldPlaceholderPrefix := fmt.Sprintf("func_%s_", shortName)
				newPlaceholderPrefix := fmt.Sprintf("func_%s_", fullPath)

				// Build old-ID -> new-ID mapping for edge rewriting
				idMap := make(map[string]string, len(fileResult.Nodes))
				for _, node := range fileResult.Nodes {
					oldID := node.ID
					node.Package = fullPath
					node.ID = node.GenerateID()
					idMap[oldID] = node.ID
				}

				for _, edge := range fileResult.Edges {
					if newID, ok := idMap[edge.From]; ok {
						edge.From = newID
					}
					// Rewrite placeholder targets (func_<shortName>_X -> func_<fullPath>_X)
					if strings.HasPrefix(edge.To, oldPlaceholderPrefix) {
						edge.To = newPlaceholderPrefix + strings.TrimPrefix(edge.To, oldPlaceholderPrefix)
					}
					if newID, ok := idMap[edge.To]; ok {
						edge.To = newID
					}
				}
			}

			result.Nodes = append(result.Nodes, fileResult.Nodes...)
			result.Edges = append(result.Edges, fileResult.Edges...)
		}
	}

	return result, nil
}

func (p *GoParser) ParseModule(root string) (*parser.ParseResult, error) {
	return p.ParsePackage(root)
}

func (p *GoParser) signature(fn *ast.FuncDecl) string {
	var sb strings.Builder
	sb.WriteString("func ")
	sb.WriteString(fn.Name.Name)
	sb.WriteString("(")

	if fn.Type.Params != nil {
		for i, param := range fn.Type.Params.List {
			if i > 0 {
				sb.WriteString(", ")
			}
			for _, name := range param.Names {
				sb.WriteString(name.Name)
				sb.WriteString(" ")
			}
			sb.WriteString(p.typeString(param.Type))
		}
	}

	sb.WriteString(")")

	if fn.Type.Results != nil {
		sb.WriteString(" ")
		if len(fn.Type.Results.List) == 1 && len(fn.Type.Results.List[0].Names) == 0 {
			sb.WriteString(p.typeString(fn.Type.Results.List[0].Type))
		} else {
			sb.WriteString("(")
			for i, res := range fn.Type.Results.List {
				if i > 0 {
					sb.WriteString(", ")
				}
				for _, name := range res.Names {
					sb.WriteString(name.Name)
					sb.WriteString(" ")
				}
				sb.WriteString(p.typeString(res.Type))
			}
			sb.WriteString(")")
		}
	}

	return sb.String()
}

func (p *GoParser) docstring(fn *ast.FuncDecl) string {
	if fn.Doc == nil {
		return ""
	}
	return strings.TrimSpace(fn.Doc.Text())
}

func (p *GoParser) typeString(expr ast.Expr) string {
	switch t := expr.(type) {
	case *ast.Ident:
		return t.Name
	case *ast.SelectorExpr:
		return p.typeString(t.X) + "." + t.Sel.Name
	case *ast.StarExpr:
		return "*" + p.typeString(t.X)
	case *ast.ArrayType:
		return "[]" + p.typeString(t.Elt)
	case *ast.MapType:
		return "map[" + p.typeString(t.Key) + "]" + p.typeString(t.Value)
	case *ast.InterfaceType:
		return "interface{}"
	case *ast.ChanType:
		return "chan " + p.typeString(t.Value)
	default:
		return "any"
	}
}

func (p *GoParser) callEdgeTarget(call *ast.CallExpr, currentPkg string, importMap map[string]string, typesInfo *types.Info) string {
	switch fn := call.Fun.(type) {
	case *ast.Ident:
		return fmt.Sprintf("func_%s_%s", currentPkg, fn.Name)
	case *ast.SelectorExpr:
		// Try TypesInfo first — resolves method calls on struct variables and chained selectors
		if typesInfo != nil {
			if sel, ok := typesInfo.Selections[fn]; ok {
				if obj := sel.Obj(); obj.Pkg() != nil {
					return fmt.Sprintf("func_%s_%s", obj.Pkg().Path(), obj.Name())
				}
			}
		}
		// Fallback: check if X is an import alias (package-qualified call)
		if ident, ok := fn.X.(*ast.Ident); ok {
			if importPath, ok := importMap[ident.Name]; ok {
				return fmt.Sprintf("func_%s_%s", importPath, fn.Sel.Name)
			}
		}
		return fmt.Sprintf("func_%s_%s", currentPkg, fn.Sel.Name)
	}
	return ""
}
