package types

import (
	"fmt"
	"go/token"
	"go/types"
	"log/slog"

	"github.com/thomassaison/mcp-code-graph/internal/graph"
	"golang.org/x/tools/go/packages"
)

type Checker struct {
	fset *token.FileSet
}

type CheckResult struct {
	Interfaces []*graph.Node
	Types      []*graph.Node
	Edges      []*graph.Edge
}

func NewChecker() *Checker {
	return &Checker{
		fset: token.NewFileSet(),
	}
}

func (c *Checker) Check(root string) (*CheckResult, error) {
	cfg := &packages.Config{
		Mode: packages.NeedName | packages.NeedFiles |
			packages.NeedSyntax | packages.NeedTypes |
			packages.NeedImports | packages.NeedDeps,
		Dir:  root,
		Fset: c.fset,
	}

	pkgs, err := packages.Load(cfg, "./...")
	if err != nil {
		return nil, fmt.Errorf("load packages: %w", err)
	}

	packages.Visit(pkgs, nil, func(pkg *packages.Package) {
		for _, err := range pkg.Errors {
			slog.Warn("package error", "pkg", pkg.PkgPath, "error", err)
		}
	})

	result := &CheckResult{
		Interfaces: make([]*graph.Node, 0),
		Types:      make([]*graph.Node, 0),
		Edges:      make([]*graph.Edge, 0),
	}

	interfaces := make(map[string]*types.Interface)
	allTypes := make(map[string]types.Type)

	packages.Visit(pkgs, nil, func(pkg *packages.Package) {
		scope := pkg.Types.Scope()
		for _, name := range scope.Names() {
			obj := scope.Lookup(name)

			if obj.Type() == nil {
				continue
			}

			if iface, ok := obj.Type().Underlying().(*types.Interface); ok {
				node := c.interfaceToNode(pkg.PkgPath, name, iface)
				result.Interfaces = append(result.Interfaces, node)
				interfaces[node.ID] = iface
			}

			if _, ok := obj.(*types.TypeName); ok {
				if !types.IsInterface(obj.Type()) {
					node := c.typeToNode(pkg.PkgPath, name, obj.Type())
					result.Types = append(result.Types, node)
					allTypes[node.ID] = obj.Type()
				}
			}
		}
	})

	for typeID, typ := range allTypes {
		for ifaceID, iface := range interfaces {
			if types.Implements(typ, iface) {
				result.Edges = append(result.Edges, &graph.Edge{
					From: typeID,
					To:   ifaceID,
					Type: graph.EdgeTypeImplements,
					Metadata: map[string]any{
						"pointer_receiver": false,
					},
				})
			}
			ptrType := types.NewPointer(typ)
			if types.Implements(ptrType, iface) {
				result.Edges = append(result.Edges, &graph.Edge{
					From: typeID,
					To:   ifaceID,
					Type: graph.EdgeTypeImplements,
					Metadata: map[string]any{
						"pointer_receiver": true,
					},
				})
			}
		}
	}

	slog.Info("type check complete", "interfaces", len(result.Interfaces), "types", len(result.Types), "edges", len(result.Edges))
	return result, nil
}

func (c *Checker) interfaceToNode(pkgPath, name string, iface *types.Interface) *graph.Node {
	methods := make([]graph.Method, iface.NumMethods())
	for i := 0; i < iface.NumMethods(); i++ {
		m := iface.Method(i)
		methods[i] = graph.Method{
			Name:      m.Name(),
			Signature: m.Type().String(),
		}
	}

	return &graph.Node{
		ID:      fmt.Sprintf("type_%s_%s", pkgPath, name),
		Type:    graph.NodeTypeInterface,
		Package: pkgPath,
		Name:    name,
		Methods: methods,
	}
}

func (c *Checker) typeToNode(pkgPath, name string, typ types.Type) *graph.Node {
	kind := "type"
	switch typ.Underlying().(type) {
	case *types.Struct:
		kind = "struct"
	}

	return &graph.Node{
		ID:       fmt.Sprintf("type_%s_%s", pkgPath, name),
		Type:     graph.NodeTypeType,
		Package:  pkgPath,
		Name:     name,
		Metadata: map[string]any{"kind": kind},
	}
}
