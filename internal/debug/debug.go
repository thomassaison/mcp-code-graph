package debug

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
)

// LevelTrace is a custom slog level below LevelDebug, used for level 2 verbosity.
const LevelTrace = slog.Level(-8)

type multiHandler struct {
	handlers []slog.Handler
}

func (m *multiHandler) Enabled(ctx context.Context, level slog.Level) bool {
	for _, h := range m.handlers {
		if h.Enabled(ctx, level) {
			return true
		}
	}
	return false
}

func (m *multiHandler) Handle(ctx context.Context, r slog.Record) error {
	for _, h := range m.handlers {
		if h.Enabled(ctx, r.Level) {
			if err := h.Handle(ctx, r.Clone()); err != nil {
				return err
			}
		}
	}
	return nil
}

func (m *multiHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	handlers := make([]slog.Handler, len(m.handlers))
	for i, h := range m.handlers {
		handlers[i] = h.WithAttrs(attrs)
	}
	return &multiHandler{handlers: handlers}
}

func (m *multiHandler) WithGroup(name string) slog.Handler {
	handlers := make([]slog.Handler, len(m.handlers))
	for i, h := range m.handlers {
		handlers[i] = h.WithGroup(name)
	}
	return &multiHandler{handlers: handlers}
}

// Setup configures the global slog logger for debug mode.
// level 0 = off, 1 = debug, 2 = trace.
// filePath is optional; if non-empty, logs are also written to that file (appended).
// w is an optional override writer for testing (pass nil to use os.Stderr).
func Setup(level int, filePath string, w io.Writer) error {
	if level == 0 {
		// Discard all debug output — set logger to warn level.
		h := slog.NewTextHandler(io.Discard, &slog.HandlerOptions{Level: slog.LevelWarn})
		slog.SetDefault(slog.New(h))
		return nil
	}

	var minLevel slog.Level
	if level >= 2 {
		minLevel = LevelTrace
	} else {
		minLevel = slog.LevelDebug
	}

	opts := &slog.HandlerOptions{Level: minLevel}

	var writers []io.Writer
	if w != nil {
		writers = append(writers, w)
	} else {
		writers = append(writers, os.Stderr)
	}

	if filePath != "" {
		f, err := os.OpenFile(filePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		if err != nil {
			return fmt.Errorf("open debug file: %w", err)
		}
		writers = append(writers, f)
	}

	var handlers []slog.Handler
	for _, wr := range writers {
		handlers = append(handlers, slog.NewTextHandler(wr, opts))
	}

	var h slog.Handler
	if len(handlers) == 1 {
		h = handlers[0]
	} else {
		h = &multiHandler{handlers: handlers}
	}

	slog.SetDefault(slog.New(h))
	return nil
}
