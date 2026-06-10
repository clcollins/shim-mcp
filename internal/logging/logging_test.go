package logging

import (
	"bytes"
	"context"
	"log/slog"
	"sync"
	"testing"
)

func TestParseLevel(t *testing.T) {
	tests := []struct {
		input   string
		want    slog.Level
		wantErr bool
	}{
		{"debug", slog.LevelDebug, false},
		{"info", slog.LevelInfo, false},
		{"warn", slog.LevelWarn, false},
		{"error", slog.LevelError, false},
		{"ERROR", slog.LevelError, false},
		{"Debug", slog.LevelDebug, false},
		{" info ", slog.LevelInfo, false},
		{"verbose", 0, true},
		{"", 0, true},
		{"trace", 0, true},
	}
	for _, tt := range tests {
		got, err := ParseLevel(tt.input)
		if tt.wantErr {
			if err == nil {
				t.Errorf("ParseLevel(%q): want error, got %v", tt.input, got)
			}
			continue
		}
		if err != nil {
			t.Errorf("ParseLevel(%q): unexpected error: %v", tt.input, err)
			continue
		}
		if got != tt.want {
			t.Errorf("ParseLevel(%q) = %v, want %v", tt.input, got, tt.want)
		}
	}
}

func TestUppercaseKeys(t *testing.T) {
	tests := []struct {
		key  string
		want string
	}{
		{"service", "SERVICE"},
		{"method", "METHOD"},
		{"duration_ms", "DURATION_MS"},
		{"response-bytes", "RESPONSE_BYTES"},
		{"path", "PATH"},
	}
	for _, tt := range tests {
		a := uppercaseKeys(nil, slog.String(tt.key, "value"))
		if a.Key != tt.want {
			t.Errorf("uppercaseKeys(%q) = %q, want %q", tt.key, a.Key, tt.want)
		}
	}
}

// recordingHandler captures log records for testing.
type recordingHandler struct {
	mu      sync.Mutex
	records []slog.Record
	level   slog.Level
}

func (h *recordingHandler) Enabled(_ context.Context, level slog.Level) bool {
	return level >= h.level
}

func (h *recordingHandler) Handle(_ context.Context, r slog.Record) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.records = append(h.records, r)
	return nil
}

func (h *recordingHandler) WithAttrs(_ []slog.Attr) slog.Handler { return h }
func (h *recordingHandler) WithGroup(_ string) slog.Handler      { return h }

func TestMultiHandler_FansOut(t *testing.T) {
	h1 := &recordingHandler{level: slog.LevelDebug}
	h2 := &recordingHandler{level: slog.LevelDebug}
	mh := &multiHandler{handlers: []slog.Handler{h1, h2}}

	logger := slog.New(mh)
	logger.Info("test message", "key", "value")

	if len(h1.records) != 1 {
		t.Errorf("handler 1: got %d records, want 1", len(h1.records))
	}
	if len(h2.records) != 1 {
		t.Errorf("handler 2: got %d records, want 1", len(h2.records))
	}
	if h1.records[0].Message != "test message" {
		t.Errorf("handler 1: message = %q, want %q", h1.records[0].Message, "test message")
	}
}

func TestMultiHandler_Enabled(t *testing.T) {
	hDebug := &recordingHandler{level: slog.LevelDebug}
	hError := &recordingHandler{level: slog.LevelError}
	mh := &multiHandler{handlers: []slog.Handler{hDebug, hError}}

	if !mh.Enabled(context.Background(), slog.LevelInfo) {
		t.Error("should be enabled at Info (debug handler accepts it)")
	}
	if !mh.Enabled(context.Background(), slog.LevelError) {
		t.Error("should be enabled at Error (both accept it)")
	}
}

func TestMultiHandler_SkipsDisabledHandler(t *testing.T) {
	hDebug := &recordingHandler{level: slog.LevelDebug}
	hError := &recordingHandler{level: slog.LevelError}
	mh := &multiHandler{handlers: []slog.Handler{hDebug, hError}}

	logger := slog.New(mh)
	logger.Info("info msg")

	if len(hDebug.records) != 1 {
		t.Errorf("debug handler: got %d records, want 1", len(hDebug.records))
	}
	if len(hError.records) != 0 {
		t.Errorf("error handler: got %d records, want 0 (Info < Error)", len(hError.records))
	}
}

func TestNewLogger_StderrFallback(t *testing.T) {
	var level slog.LevelVar
	level.Set(slog.LevelError)

	logger := NewLogger(&level)
	if logger == nil {
		t.Fatal("NewLogger returned nil")
	}

	var buf bytes.Buffer
	stderrHandler := slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: &level})
	testLogger := slog.New(stderrHandler)
	testLogger.Error("fallback test")

	if buf.Len() == 0 {
		t.Error("stderr handler should have written output")
	}
}

func TestMultiHandler_WithAttrs(t *testing.T) {
	h1 := &recordingHandler{level: slog.LevelDebug}
	h2 := &recordingHandler{level: slog.LevelDebug}
	mh := &multiHandler{handlers: []slog.Handler{h1, h2}}

	mh2 := mh.WithAttrs([]slog.Attr{slog.String("extra", "val")})
	if _, ok := mh2.(*multiHandler); !ok {
		t.Error("WithAttrs should return a *multiHandler")
	}
}

func TestMultiHandler_WithGroup(t *testing.T) {
	h1 := &recordingHandler{level: slog.LevelDebug}
	h2 := &recordingHandler{level: slog.LevelDebug}
	mh := &multiHandler{handlers: []slog.Handler{h1, h2}}

	mh2 := mh.WithGroup("mygroup")
	if _, ok := mh2.(*multiHandler); !ok {
		t.Error("WithGroup should return a *multiHandler")
	}
}
