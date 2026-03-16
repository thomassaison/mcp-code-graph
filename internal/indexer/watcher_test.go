package indexer

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/thomassaison/mcp-code-graph/internal/graph"
	"github.com/thomassaison/mcp-code-graph/internal/parser"
	goparser "github.com/thomassaison/mcp-code-graph/internal/parser/go"
)

func TestNewWatcher(t *testing.T) {
	t.Parallel()
	g := newTestGraph()
	idx := New(g, newTestParser())

	w, err := NewWatcher(idx, 100*time.Millisecond, nil, nil)
	if err != nil {
		t.Fatalf("NewWatcher() error = %v", err)
	}
	if w == nil {
		t.Fatal("NewWatcher() returned nil")
	}
	w.Close()
}

func TestWatcher_Watch(t *testing.T) {
	tmpDir := t.TempDir()

	g := newTestGraph()
	idx := New(g, newTestParser())

	w, err := NewWatcher(idx, 100*time.Millisecond, nil, nil)
	if err != nil {
		t.Fatalf("NewWatcher() error = %v", err)
	}
	defer w.Close()

	if err := w.Watch(tmpDir); err != nil {
		t.Fatalf("Watch() error = %v", err)
	}
}

func TestWatcher_WatchSkipsDotDirs(t *testing.T) {
	tmpDir := t.TempDir()

	hiddenDir := filepath.Join(tmpDir, ".hidden")
	if err := os.MkdirAll(hiddenDir, 0755); err != nil {
		t.Fatal(err)
	}

	g := newTestGraph()
	idx := New(g, newTestParser())

	w, err := NewWatcher(idx, 100*time.Millisecond, nil, nil)
	if err != nil {
		t.Fatalf("NewWatcher() error = %v", err)
	}
	defer w.Close()

	// Watch should not error even with dot directories present
	if err := w.Watch(tmpDir); err != nil {
		t.Fatalf("Watch() error = %v", err)
	}
}

func TestWatcher_WatchSkipsVendor(t *testing.T) {
	tmpDir := t.TempDir()

	vendorDir := filepath.Join(tmpDir, "vendor")
	if err := os.MkdirAll(vendorDir, 0755); err != nil {
		t.Fatal(err)
	}

	g := newTestGraph()
	idx := New(g, newTestParser())

	w, err := NewWatcher(idx, 100*time.Millisecond, nil, nil)
	if err != nil {
		t.Fatalf("NewWatcher() error = %v", err)
	}
	defer w.Close()

	if err := w.Watch(tmpDir); err != nil {
		t.Fatalf("Watch() error = %v", err)
	}
}

func TestWatcher_Close(t *testing.T) {
	t.Parallel()
	g := newTestGraph()
	idx := New(g, newTestParser())

	w, err := NewWatcher(idx, 100*time.Millisecond, nil, nil)
	if err != nil {
		t.Fatalf("NewWatcher() error = %v", err)
	}

	w.Close()
}

func TestWatcher_WatchNonExistent(t *testing.T) {
	t.Parallel()
	g := newTestGraph()
	idx := New(g, newTestParser())

	w, err := NewWatcher(idx, 100*time.Millisecond, nil, nil)
	if err != nil {
		t.Fatalf("NewWatcher() error = %v", err)
	}
	defer w.Close()

	err = w.Watch("/nonexistent/path")
	if err == nil {
		t.Error("Watch on nonexistent path should return error")
	}
}

// helpers
func newTestGraph() *graph.Graph {
	return graph.New()
}

func newTestParser() parser.Parser {
	return goparser.New()
}
