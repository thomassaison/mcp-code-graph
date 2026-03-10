package types

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/thomassaison/mcp-code-graph/internal/graph"
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

func TestCheckerReturnsAcceptsEdges(t *testing.T) {
	tmpDir := t.TempDir()

	goMod := `module example.com/edgetest

go 1.21
`
	if err := os.WriteFile(filepath.Join(tmpDir, "go.mod"), []byte(goMod), 0644); err != nil {
		t.Fatal(err)
	}

	goCode := `package edgetest

type Config struct {
	Name string
}

type Server struct {
	cfg Config
}

// NewServer accepts a Config and returns a Server.
func NewServer(cfg Config) *Server {
	return &Server{cfg: cfg}
}

// GetConfig is a method that returns Config.
func (s *Server) GetConfig() Config {
	return s.cfg
}

// ProcessSlice accepts a slice of Configs.
func ProcessSlice(cfgs []Config) {}

// NoCustomTypes only uses built-ins — should produce no accepts/returns edges.
func NoCustomTypes(name string) (int, error) {
	return 0, nil
}
`
	if err := os.WriteFile(filepath.Join(tmpDir, "server.go"), []byte(goCode), 0644); err != nil {
		t.Fatal(err)
	}

	checker := NewChecker()
	result, err := checker.Check(tmpDir)
	if err != nil {
		t.Fatalf("Check failed: %v", err)
	}

	// Count edge types
	accepts := 0
	returns := 0
	acceptTargets := make(map[string][]string) // typeID -> []funcName (from edge ID)
	returnTargets := make(map[string][]string)

	for _, edge := range result.Edges {
		switch edge.Type {
		case graph.EdgeTypeAccepts:
			accepts++
			acceptTargets[edge.To] = append(acceptTargets[edge.To], edge.From)
		case graph.EdgeTypeReturns:
			returns++
			returnTargets[edge.To] = append(returnTargets[edge.To], edge.From)
		}
	}

	t.Logf("accepts edges: %d, returns edges: %d", accepts, returns)
	for to, froms := range acceptTargets {
		t.Logf("  accepts -> %s from %v", to, froms)
	}
	for to, froms := range returnTargets {
		t.Logf("  returns -> %s from %v", to, froms)
	}

	// NewServer accepts Config
	configID := "type_example.com/edgetest_Config"
	serverID := "type_example.com/edgetest_Server"

	if len(acceptTargets[configID]) < 2 {
		// NewServer(cfg Config) + ProcessSlice(cfgs []Config) + GetConfig receiver
		t.Errorf("expected at least 2 accepts->Config edges, got %d", len(acceptTargets[configID]))
	}

	// NewServer returns *Server
	if len(returnTargets[serverID]) < 1 {
		t.Errorf("expected at least 1 returns->Server edge (from NewServer), got %d", len(returnTargets[serverID]))
	}

	// GetConfig returns Config
	if len(returnTargets[configID]) < 1 {
		t.Errorf("expected at least 1 returns->Config edge (from GetConfig), got %d", len(returnTargets[configID]))
	}

	// NoCustomTypes should NOT produce any accepts/returns edges to our types
	for _, edge := range result.Edges {
		if edge.Type == graph.EdgeTypeAccepts || edge.Type == graph.EdgeTypeReturns {
			// Edges should only point to Config or Server
			if edge.To != configID && edge.To != serverID {
				t.Errorf("unexpected edge target: %s (type %s)", edge.To, edge.Type)
			}
		}
	}
}

func TestNamedTypeIDs(t *testing.T) {
	// Test that namedTypeIDs returns nil for built-in types
	// (We can't easily construct types.Named in tests without go/types machinery,
	// but we can verify nil/basic returns nil)
	ids := namedTypeIDs(nil)
	if ids != nil {
		t.Errorf("expected nil for nil type, got %v", ids)
	}
}
