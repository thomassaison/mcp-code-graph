package debug_test

import (
	"bytes"
	"log/slog"
	"strings"
	"testing"

	"github.com/thomassaison/mcp-code-graph/internal/debug"
)

func TestSetupLevel1EnablesDebug(t *testing.T) {
	var buf bytes.Buffer
	// After Setup(1, ""), the global logger should accept slog.LevelDebug messages.
	if err := debug.Setup(1, "", &buf); err != nil {
		t.Fatalf("Setup: %v", err)
	}
	slog.Debug("hello from debug")
	if !strings.Contains(buf.String(), "hello from debug") {
		t.Errorf("expected debug message in output, got: %s", buf.String())
	}
}

func TestSetupLevel2EnablesTrace(t *testing.T) {
	var buf bytes.Buffer
	if err := debug.Setup(2, "", &buf); err != nil {
		t.Fatalf("Setup: %v", err)
	}
	slog.Log(nil, debug.LevelTrace, "trace message")
	if !strings.Contains(buf.String(), "trace message") {
		t.Errorf("expected trace message in output, got: %s", buf.String())
	}
}

func TestSetupLevel1SuppressesTrace(t *testing.T) {
	var buf bytes.Buffer
	if err := debug.Setup(1, "", &buf); err != nil {
		t.Fatalf("Setup: %v", err)
	}
	slog.Log(nil, debug.LevelTrace, "should not appear")
	if strings.Contains(buf.String(), "should not appear") {
		t.Errorf("trace message should be suppressed at level 1, got: %s", buf.String())
	}
}

func TestSetupLevel0DisablesAll(t *testing.T) {
	var buf bytes.Buffer
	if err := debug.Setup(0, "", &buf); err != nil {
		t.Fatalf("Setup: %v", err)
	}
	slog.Debug("should not appear")
	if strings.Contains(buf.String(), "should not appear") {
		t.Errorf("debug message should be suppressed at level 0, got: %s", buf.String())
	}
}
