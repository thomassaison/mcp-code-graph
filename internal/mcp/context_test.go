// anthropic/claude-sonnet-4-6
package mcp

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeGoFile(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "test.go")
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("write temp go file: %v", err)
	}
	return path
}

func TestReadFunctionCode_simpleFunction(t *testing.T) {
	t.Parallel()
	src := `package example

func Add(a, b int) int {
	return a + b
}
`
	path := writeGoFile(t, src)

	code := readFunctionCode(path, 3) // "func Add" is on line 3
	if code == "" {
		t.Fatal("expected non-empty code, got empty string")
	}
	if !strings.Contains(code, "func Add") {
		t.Errorf("expected 'func Add' in code, got: %s", code)
	}
	if !strings.Contains(code, "return a + b") {
		t.Errorf("expected 'return a + b' in code, got: %s", code)
	}
}

func TestReadFunctionCode_bracesInsideStrings(t *testing.T) {
	t.Parallel()
	// This tests the bug-fix: braces inside string literals must not confuse extraction.
	src := `package example

func Template() string {
	return "hello { world }"
}
`
	path := writeGoFile(t, src)

	code := readFunctionCode(path, 3)
	if code == "" {
		t.Fatal("expected non-empty code, got empty string")
	}
	if !strings.Contains(code, "func Template") {
		t.Errorf("expected 'func Template' in code, got: %s", code)
	}
	if !strings.Contains(code, `"hello { world }"`) {
		t.Errorf("expected string literal with braces in code, got: %s", code)
	}
}

func TestReadFunctionCode_nonexistentFile(t *testing.T) {
	t.Parallel()
	code := readFunctionCode("/nonexistent/path/to/file.go", 1)
	if code != "" {
		t.Errorf("expected empty string for nonexistent file, got: %s", code)
	}
}

func TestReadFunctionCode_lineZero(t *testing.T) {
	t.Parallel()
	src := `package example

func SomeFunc() {}
`
	path := writeGoFile(t, src)

	code := readFunctionCode(path, 0)
	if code != "" {
		t.Errorf("expected empty string for line 0, got: %s", code)
	}
}

func TestReadFunctionCode_bracesInComments(t *testing.T) {
	t.Parallel()
	// Braces in comments must not confuse extraction.
	src := `package example

// DoThing does a thing { with braces in the comment }
func DoThing() int {
	// another { comment }
	return 42
}
`
	path := writeGoFile(t, src)

	// "func DoThing" is on line 4
	code := readFunctionCode(path, 4)
	if code == "" {
		t.Fatal("expected non-empty code, got empty string")
	}
	if !strings.Contains(code, "func DoThing") {
		t.Errorf("expected 'func DoThing' in code, got: %s", code)
	}
	if !strings.Contains(code, "return 42") {
		t.Errorf("expected 'return 42' in code, got: %s", code)
	}
}

func TestReadFunctionCode_emptyFilePath(t *testing.T) {
	t.Parallel()
	code := readFunctionCode("", 5)
	if code != "" {
		t.Errorf("expected empty string for empty file path, got: %s", code)
	}
}
