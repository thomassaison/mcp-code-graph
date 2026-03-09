package goparser

import (
	"os"
	"path/filepath"
	"testing"
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
