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

	// Collect all known type node IDs for edge target validation
	knownTypes := make(map[string]bool, len(allTypes)+len(interfaces))
	for id := range allTypes {
		knownTypes[id] = true
	}
	for id := range interfaces {
		knownTypes[id] = true
	}

	// Generate returns/accepts edges from function signatures
	packages.Visit(pkgs, nil, func(pkg *packages.Package) {
		c.extractSignatureEdges(pkg, knownTypes, result)
	})

	slog.Info("type check complete", "interfaces", len(result.Interfaces), "types", len(result.Types), "edges", len(result.Edges))
	return result, nil
}

// extractSignatureEdges walks all functions and methods in a package,
// emitting accepts/returns edges for named types in their signatures.
func (c *Checker) extractSignatureEdges(pkg *packages.Package, knownTypes map[string]bool, result *CheckResult) {
	scope := pkg.Types.Scope()

	// Package-level functions
	for _, name := range scope.Names() {
		obj := scope.Lookup(name)
		fn, ok := obj.(*types.Func)
		if !ok {
			continue
		}
		funcID := c.buildFuncID(pkg.PkgPath, fn)
		sig := fn.Type().(*types.Signature)
		c.emitSignatureEdges(funcID, sig, knownTypes, result)
	}

	// Methods on named types
	for _, name := range scope.Names() {
		obj := scope.Lookup(name)
		tn, ok := obj.(*types.TypeName)
		if !ok {
			continue
		}
		named, ok := tn.Type().(*types.Named)
		if !ok {
			continue
		}
		for i := 0; i < named.NumMethods(); i++ {
			method := named.Method(i)
			funcID := c.buildFuncID(pkg.PkgPath, method)
			sig := method.Type().(*types.Signature)
			c.emitSignatureEdges(funcID, sig, knownTypes, result)
		}
	}
}

// emitSignatureEdges emits accepts/returns edges for a single function signature.
func (c *Checker) emitSignatureEdges(funcID string, sig *types.Signature, knownTypes map[string]bool, result *CheckResult) {
	seen := make(map[string]bool)

	// Parameter types → accepts edges
	params := sig.Params()
	for i := 0; i < params.Len(); i++ {
		for _, typeID := range namedTypeIDs(params.At(i).Type()) {
			key := "accepts:" + typeID
			if !seen[key] && knownTypes[typeID] {
				seen[key] = true
				result.Edges = append(result.Edges, &graph.Edge{
					From:     funcID,
					To:       typeID,
					Type:     graph.EdgeTypeAccepts,
					Metadata: map[string]any{},
				})
			}
		}
	}

	// Return types → returns edges
	results := sig.Results()
	for i := 0; i < results.Len(); i++ {
		for _, typeID := range namedTypeIDs(results.At(i).Type()) {
			key := "returns:" + typeID
			if !seen[key] && knownTypes[typeID] {
				seen[key] = true
				result.Edges = append(result.Edges, &graph.Edge{
					From:     funcID,
					To:       typeID,
					Type:     graph.EdgeTypeReturns,
					Metadata: map[string]any{},
				})
			}
		}
	}
}

// buildFuncID constructs a function node ID matching the parser's GenerateID convention.
func (c *Checker) buildFuncID(pkgPath string, fn *types.Func) string {
	pos := c.fset.Position(fn.Pos())
	return fmt.Sprintf("func_%s_%s_%s:%d", pkgPath, fn.Name(), pos.Filename, pos.Line)
}

// namedTypeIDs extracts type node IDs from a types.Type, unwrapping
// pointers, slices, maps, and channels to find named types.
// Returns zero or more IDs (maps can yield two).
func namedTypeIDs(t types.Type) []string {
	switch typ := t.(type) {
	case *types.Named:
		obj := typ.Obj()
		pkg := obj.Pkg()
		if pkg == nil {
			return nil // built-in like error
		}
		return []string{fmt.Sprintf("type_%s_%s", pkg.Path(), obj.Name())}
	case *types.Pointer:
		return namedTypeIDs(typ.Elem())
	case *types.Slice:
		return namedTypeIDs(typ.Elem())
	case *types.Array:
		return namedTypeIDs(typ.Elem())
	case *types.Map:
		ids := namedTypeIDs(typ.Key())
		ids = append(ids, namedTypeIDs(typ.Elem())...)
		return ids
	case *types.Chan:
		return namedTypeIDs(typ.Elem())
	default:
		return nil
	}
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
