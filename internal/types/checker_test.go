package types

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCheckerCheck(t *testing.T) {
	tmpDir := t.TempDir()

	goMod := `module testpkg

go 1.21
`
	if err := os.WriteFile(filepath.Join(tmpDir, "go.mod"), []byte(goMod), 0644); err != nil {
		t.Fatal(err)
	}

	goCode := `package testpkg

type Reader interface {
	Read(p []byte) (n int, err error)
}

type Writer interface {
	Write(p []byte) (n int, err error)
}

type File struct{}

func (f *File) Read(p []byte) (n int, err error)  { return 0, nil }
func (f *File) Write(p []byte) (n int, err error) { return 0, nil }
`
	if err := os.WriteFile(filepath.Join(tmpDir, "file.go"), []byte(goCode), 0644); err != nil {
		t.Fatal(err)
	}

	checker := NewChecker()
	result, err := checker.Check(tmpDir)
	if err != nil {
		t.Fatalf("Check failed: %v", err)
	}

	if len(result.Interfaces) < 2 {
		t.Errorf("expected at least 2 interfaces, got %d", len(result.Interfaces))
	}

	if len(result.Types) < 1 {
		t.Errorf("expected at least 1 type, got %d", len(result.Types))
	}

	if len(result.Edges) < 2 {
		t.Errorf("expected at least 2 implementation edges, got %d", len(result.Edges))
	}

	t.Logf("Found %d interfaces, %d types, %d edges", len(result.Interfaces), len(result.Types), len(result.Edges))
}
