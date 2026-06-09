package proxy

import (
	"errors"
	"net/http"
	"testing"
)

func TestScrubHeaders(t *testing.T) {
	h := http.Header{}
	h.Set("Authorization", "Bearer secret-token")
	h.Set("X-Api-Key", "api-key-123")
	h.Set("Content-Type", "application/json")
	h.Set("Date", "Mon, 08 Jun 2026 00:00:00 GMT")
	h.Set("X-Custom", "custom-value")

	scrubbed := ScrubHeaders(h)

	if _, ok := scrubbed["Authorization"]; ok {
		t.Error("Authorization header should be scrubbed")
	}
	if _, ok := scrubbed["X-Api-Key"]; ok {
		t.Error("X-Api-Key header should be scrubbed")
	}
	if scrubbed["Content-Type"] != "application/json" {
		t.Errorf("Content-Type = %q, want %q", scrubbed["Content-Type"], "application/json")
	}
	if scrubbed["Date"] != "Mon, 08 Jun 2026 00:00:00 GMT" {
		t.Errorf("Date = %q, want preserved", scrubbed["Date"])
	}
}

func TestScrubHeaders_Nil(t *testing.T) {
	scrubbed := ScrubHeaders(nil)
	if scrubbed == nil {
		t.Error("expected non-nil map for nil input")
	}
}

func TestScrubHeaders_Empty(t *testing.T) {
	scrubbed := ScrubHeaders(http.Header{})
	if len(scrubbed) != 0 {
		t.Errorf("expected empty map, got %d entries", len(scrubbed))
	}
}

func TestScrubError(t *testing.T) {
	err := errors.New("connection failed: Authorization: Bearer secret-token-xyz")
	scrubbed := ScrubError(err, []string{"secret-token-xyz"})
	if scrubbed == nil {
		t.Fatal("expected non-nil error")
	}
	msg := scrubbed.Error()
	if msg == err.Error() {
		t.Error("error should have been scrubbed")
	}
	if contains(msg, "secret-token-xyz") {
		t.Errorf("scrubbed error still contains credential: %q", msg)
	}
}

func TestScrubError_NilError(t *testing.T) {
	scrubbed := ScrubError(nil, []string{"token"})
	if scrubbed != nil {
		t.Error("expected nil for nil error input")
	}
}

func TestScrubError_NoCredentials(t *testing.T) {
	err := errors.New("connection refused")
	scrubbed := ScrubError(err, nil)
	if scrubbed.Error() != err.Error() {
		t.Errorf("got %q, want %q", scrubbed.Error(), err.Error())
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 ||
		(len(s) > 0 && len(substr) > 0 && containsStr(s, substr)))
}

func containsStr(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
