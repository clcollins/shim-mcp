package filter

import (
	"io"
	"net/http"
	"strings"
	"testing"
)

func TestAutoContentType(t *testing.T) {
	f := &AutoContentType{}

	if f.Name() != "auto_content_type" {
		t.Errorf("Name() = %q, want auto_content_type", f.Name())
	}

	tests := []struct {
		name       string
		body       string
		existingCT string
		wantCT     string
	}{
		{
			name:   "valid JSON object, no CT",
			body:   `{"key":"value"}`,
			wantCT: "application/json",
		},
		{
			name:   "valid JSON array, no CT",
			body:   `[1,2,3]`,
			wantCT: "application/json",
		},
		{
			name:   "invalid JSON, no CT",
			body:   `not json at all`,
			wantCT: "",
		},
		{
			name:   "plain text, no CT",
			body:   `hello world`,
			wantCT: "",
		},
		{
			name:       "CT already set",
			body:       `{"key":"value"}`,
			existingCT: "text/plain",
			wantCT:     "text/plain",
		},
		{
			name:   "no body",
			body:   "",
			wantCT: "",
		},
		{
			name:   "starts with { but invalid JSON",
			body:   `{broken`,
			wantCT: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var body io.Reader
			if tt.body != "" {
				body = strings.NewReader(tt.body)
			}
			req, _ := http.NewRequest(http.MethodPost, "http://example.com", body)
			if tt.existingCT != "" {
				req.Header.Set("Content-Type", tt.existingCT)
			}

			result, err := f.FilterRequest(Context{}, req)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			got := result.Header.Get("Content-Type")
			if got != tt.wantCT {
				t.Errorf("Content-Type = %q, want %q", got, tt.wantCT)
			}

			if result.Body != nil && tt.body != "" {
				readBack, _ := io.ReadAll(result.Body)
				if string(readBack) != tt.body {
					t.Errorf("body changed: got %q, want %q", string(readBack), tt.body)
				}
			}
		})
	}
}
