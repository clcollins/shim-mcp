package logging

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"strings"

	slogjournal "github.com/systemd/slog-journal"
)

// ParseLevel converts a level name to a slog.Level.
func ParseLevel(s string) (slog.Level, error) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "debug":
		return slog.LevelDebug, nil
	case "info":
		return slog.LevelInfo, nil
	case "warn":
		return slog.LevelWarn, nil
	case "error":
		return slog.LevelError, nil
	default:
		return 0, fmt.Errorf("invalid log level %q (valid: debug, info, warn, error)", s)
	}
}

// NewLogger creates a logger that writes to both journald and stderr when
// journald is available, or stderr only as a fallback.
func NewLogger(level *slog.LevelVar) *slog.Logger {
	stderrHandler := slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: level})

	journalHandler, err := slogjournal.NewHandler(&slogjournal.Options{
		Level:       level,
		ReplaceAttr: uppercaseKeys,
	})
	if err != nil {
		return slog.New(stderrHandler)
	}

	return slog.New(&multiHandler{handlers: []slog.Handler{journalHandler, stderrHandler}})
}

func uppercaseKeys(_ []string, a slog.Attr) slog.Attr {
	a.Key = strings.ReplaceAll(strings.ToUpper(a.Key), "-", "_")
	return a
}

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
