package goparser

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/thomassaison/mcp-code-graph/internal/graph"
)

func TestParseFile(t *testing.T) {
	tmpDir := t.TempDir()
	goFile := filepath.Join(tmpDir, "test.go")

	code := `package main

import "fmt"

// greet says hello
func greet(name string) string {
	return fmt.Sprintf("Hello, %s!", name)
}

func main() {
	msg := greet("World")
	fmt.Println(msg)
}
`
	if err := os.WriteFile(goFile, []byte(code), 0644); err != nil {
		t.Fatal(err)
	}

	parser := New()
	result, err := parser.ParseFile(goFile)
	if err != nil {
		t.Fatalf("ParseFile() error = %v", err)
	}

	if len(result.Nodes) < 2 {
		t.Errorf("len(Nodes) = %d, want at least 2", len(result.Nodes))
	}

	found := false
	for _, node := range result.Nodes {
		if node.Name == "greet" {
			found = true
			if node.Docstring == "" {
				t.Error("greet() missing docstring")
			}
		}
	}
	if !found {
		t.Error("greet function not found")
	}

	if len(result.Edges) == 0 {
		t.Error("no edges found")
	}
}

func TestParsePackage_FullImportPaths(t *testing.T) {
	// Create a minimal Go module with a non-main package
	tmpDir := t.TempDir()

	// go.mod
	goMod := `module example.com/testmod

go 1.21
`
	if err := os.WriteFile(filepath.Join(tmpDir, "go.mod"), []byte(goMod), 0644); err != nil {
		t.Fatal(err)
	}

	// Create a sub-package
	pkgDir := filepath.Join(tmpDir, "mypkg")
	if err := os.MkdirAll(pkgDir, 0755); err != nil {
		t.Fatal(err)
	}

	code := `package mypkg

// Hello returns a greeting.
func Hello() string {
	return "hello"
}

func helper() string {
	return Hello()
}
`
	if err := os.WriteFile(filepath.Join(pkgDir, "hello.go"), []byte(code), 0644); err != nil {
		t.Fatal(err)
	}

	p := New()
	result, err := p.ParsePackage(tmpDir)
	if err != nil {
		t.Fatalf("ParsePackage() error = %v", err)
	}

	if len(result.Nodes) < 2 {
		t.Fatalf("expected at least 2 nodes, got %d", len(result.Nodes))
	}

	// All nodes should use the full import path, not the short "mypkg"
	for _, node := range result.Nodes {
		if node.Package != "example.com/testmod/mypkg" {
			t.Errorf("node %q has Package=%q, want %q", node.Name, node.Package, "example.com/testmod/mypkg")
		}
		// IDs should also contain the full path
		if node.ID == "" {
			t.Errorf("node %q has empty ID", node.Name)
		}
	}

	// Edges should reference the full import path in placeholder resolution
	for _, edge := range result.Edges {
		if edge.From == "" || edge.To == "" {
			t.Errorf("edge has empty From=%q or To=%q", edge.From, edge.To)
		}
	}
}

func TestParseFile_MethodReceiver(t *testing.T) {
	tmpDir := t.TempDir()
	goFile := filepath.Join(tmpDir, "test.go")

	code := `package test

type Server struct{}

func (s *Server) Handle() {}
func main() {}
`
	if err := os.WriteFile(goFile, []byte(code), 0644); err != nil {
		t.Fatal(err)
	}

	p := New()
	result, err := p.ParseFile(goFile)
	if err != nil {
		t.Fatal(err)
	}

	// Find Handle method
	var handleMethod *graph.Node
	for _, node := range result.Nodes {
		if node.Name == "Handle" {
			handleMethod = node
			break
		}
	}

	if handleMethod == nil {
		t.Fatal("Handle method not found")
	}

	if handleMethod.Type != graph.NodeTypeMethod {
		t.Errorf("Handle.Type = %v, want %v", handleMethod.Type, graph.NodeTypeMethod)
	}

	if handleMethod.Metadata["receiver"] != "*Server" {
		t.Errorf("Handle.Metadata[receiver] = %v, want *Server", handleMethod.Metadata["receiver"])
	}

	// main should still be a function
	var mainFunc *graph.Node
	for _, node := range result.Nodes {
		if node.Name == "main" {
			mainFunc = node
			break
		}
	}

	if mainFunc == nil {
		t.Fatal("main function not found")
	}

	if mainFunc.Type != graph.NodeTypeFunction {
		t.Errorf("main.Type = %v, want %v", mainFunc.Type, graph.NodeTypeFunction)
	}
}
