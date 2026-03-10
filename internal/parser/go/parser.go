package goparser

import (
	"context"
	"fmt"
	"go/ast"
	goparser "go/parser"
	"go/token"
	"log/slog"
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
	result := &parser.ParseResult{}

	file, err := goparser.ParseFile(p.fset, path, nil, goparser.ParseComments)
	if err != nil {
		return nil, fmt.Errorf("parse file: %w", err)
	}

	pkgName := file.Name.Name

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
		node.ID = node.GenerateID()
		result.Nodes = append(result.Nodes, node)

		if fn.Body != nil {
			ast.Inspect(fn.Body, func(n ast.Node) bool {
				call, ok := n.(*ast.CallExpr)
				if !ok {
					return true
				}

				calleeName := p.callName(call)
				if calleeName == "" {
					return true
				}

				edge := &graph.Edge{
					From:     node.ID,
					To:       fmt.Sprintf("func_%s_%s", pkgName, calleeName),
					Type:     graph.EdgeTypeCalls,
					Metadata: make(map[string]any),
				}
				result.Edges = append(result.Edges, edge)

				return true
			})
		}
	}

	slog.Log(context.Background(), debug.LevelTrace, "file parsed", "path", path, "functions", len(result.Nodes))
	return result, nil
}

func (p *GoParser) ParsePackage(dir string) (*parser.ParseResult, error) {
	cfg := &packages.Config{
		Mode: packages.NeedName | packages.NeedFiles | packages.NeedSyntax | packages.NeedTypes,
		Dir:  dir,
		Fset: p.fset,
	}

	pkgs, err := packages.Load(cfg, "./...")
	if err != nil {
		return nil, fmt.Errorf("load packages: %w", err)
	}

	result := &parser.ParseResult{}
	for _, pkg := range pkgs {
		for _, file := range pkg.Syntax {
			pos := p.fset.Position(file.Pos())
			fileResult, err := p.ParseFile(pos.Filename)
			if err != nil {
				continue
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

func (p *GoParser) callName(call *ast.CallExpr) string {
	switch fn := call.Fun.(type) {
	case *ast.Ident:
		return fn.Name
	case *ast.SelectorExpr:
		return fn.Sel.Name
	}
	return ""
}
