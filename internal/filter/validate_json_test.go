package filter

import (
	"io"
	"net/http"
	"strings"
	"testing"
)

func TestValidateJSONBody(t *testing.T) {
	f := &ValidateJSONBody{}

	if f.Name() != "validate_json_body" {
		t.Errorf("Name() = %q, want validate_json_body", f.Name())
	}

	tests := []struct {
		name        string
		method      string
		contentType string
		body        string
		wantErr     bool
	}{
		{
			name:        "valid JSON POST",
			method:      http.MethodPost,
			contentType: "application/json",
			body:        `{"key":"value"}`,
			wantErr:     false,
		},
		{
			name:        "invalid JSON POST",
			method:      http.MethodPost,
			contentType: "application/json",
			body:        `{"key":broken}`,
			wantErr:     true,
		},
		{
			name:        "valid JSON PUT",
			method:      http.MethodPut,
			contentType: "application/json",
			body:        `[1,2,3]`,
			wantErr:     false,
		},
		{
			name:        "valid JSON PATCH",
			method:      http.MethodPatch,
			contentType: "application/json",
			body:        `{"op":"add"}`,
			wantErr:     false,
		},
		{
			name:        "GET with body skipped",
			method:      http.MethodGet,
			contentType: "application/json",
			body:        `{invalid}`,
			wantErr:     false,
		},
		{
			name:        "POST no body",
			method:      http.MethodPost,
			contentType: "application/json",
			body:        "",
			wantErr:     false,
		},
		{
			name:        "POST non-JSON content type",
			method:      http.MethodPost,
			contentType: "text/plain",
			body:        `not json`,
			wantErr:     false,
		},
		{
			name:        "POST no content type",
			method:      http.MethodPost,
			contentType: "",
			body:        `{"key":"value"}`,
			wantErr:     false,
		},
		{
			name:        "POST JSON with charset",
			method:      http.MethodPost,
			contentType: "application/json; charset=utf-8",
			body:        `{"key":"value"}`,
			wantErr:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var body io.Reader
			if tt.body != "" {
				body = strings.NewReader(tt.body)
			}
			req, _ := http.NewRequest(tt.method, "http://example.com", body)
			if tt.contentType != "" {
				req.Header.Set("Content-Type", tt.contentType)
			}

			result, err := f.FilterRequest(Context{}, req)
			if tt.wantErr && err == nil {
				t.Fatal("expected error")
			}
			if !tt.wantErr && err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !tt.wantErr {
				// Verify body is still readable after validation
				if result.Body != nil {
					readBack, _ := io.ReadAll(result.Body)
					if tt.body != "" && string(readBack) != tt.body {
						t.Errorf("body changed: got %q, want %q", string(readBack), tt.body)
					}
				}
			}
		})
	}
}
