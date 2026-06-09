package filter

import (
	"net/http"
	"strings"
	"testing"
)

func TestRejectEmptyBody(t *testing.T) {
	f := &RejectEmptyBody{}

	if f.Name() != "reject_empty_body" {
		t.Errorf("Name() = %q, want reject_empty_body", f.Name())
	}

	tests := []struct {
		name    string
		method  string
		body    string
		wantErr bool
	}{
		{"POST with body", http.MethodPost, `{"data":"ok"}`, false},
		{"POST empty body", http.MethodPost, "", true},
		{"PUT empty body", http.MethodPut, "", true},
		{"PATCH empty body", http.MethodPatch, "", true},
		{"GET no body", http.MethodGet, "", false},
		{"DELETE no body", http.MethodDelete, "", false},
		{"HEAD no body", http.MethodHead, "", false},
		{"POST with whitespace-only body", http.MethodPost, "   ", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var req *http.Request
			if tt.body != "" {
				req, _ = http.NewRequest(tt.method, "http://example.com", strings.NewReader(tt.body))
			} else {
				req, _ = http.NewRequest(tt.method, "http://example.com", nil)
			}

			_, err := f.FilterRequest(Context{}, req)
			if tt.wantErr && err == nil {
				t.Fatal("expected error")
			}
			if !tt.wantErr && err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}
