package filter

import (
	"io"
	"net/http"
	"strings"
	"testing"
)

func makeResponse(body string) *http.Response {
	return &http.Response{
		StatusCode: 200,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(strings.NewReader(body)),
	}
}

func TestStripFields(t *testing.T) {
	f := NewStripFields([]string{"expand", "self", "schema"})

	if f.Name() != "strip_fields" {
		t.Errorf("Name() = %q, want strip_fields", f.Name())
	}

	tests := []struct {
		name     string
		body     string
		fields   []string
		wantKeys []string
		noKeys   []string
	}{
		{
			name:     "strip configured fields",
			body:     `{"expand":"all","self":"http://x","key":"SREP-1","summary":"test"}`,
			fields:   []string{"expand", "self"},
			wantKeys: []string{"key", "summary"},
			noKeys:   []string{"expand", "self"},
		},
		{
			name:     "no matching fields",
			body:     `{"key":"SREP-1","summary":"test"}`,
			fields:   []string{"expand", "self"},
			wantKeys: []string{"key", "summary"},
			noKeys:   []string{},
		},
		{
			name:     "nested fields preserved",
			body:     `{"fields":{"status":{"name":"In Progress"}},"expand":"all"}`,
			fields:   []string{"expand"},
			wantKeys: []string{"fields"},
			noKeys:   []string{"expand"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sf := NewStripFields(tt.fields)
			resp := makeResponse(tt.body)
			defer func() { _ = resp.Body.Close() }()

			result, err := sf.FilterResponse(Context{}, resp)
			if result != nil && result != resp {
				defer func() { _ = result.Body.Close() }()
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			body, _ := io.ReadAll(result.Body)
			bodyStr := string(body)

			for _, key := range tt.wantKeys {
				if !strings.Contains(bodyStr, `"`+key+`"`) {
					t.Errorf("expected key %q in response: %s", key, bodyStr)
				}
			}
			for _, key := range tt.noKeys {
				if strings.Contains(bodyStr, `"`+key+`"`) {
					t.Errorf("key %q should be stripped from response: %s", key, bodyStr)
				}
			}
		})
	}
}

func TestStripFields_NonJSON(t *testing.T) {
	f := NewStripFields([]string{"expand"})
	resp := &http.Response{
		StatusCode: 200,
		Header:     http.Header{"Content-Type": []string{"text/plain"}},
		Body:       io.NopCloser(strings.NewReader("plain text response")),
	}
	defer func() { _ = resp.Body.Close() }()

	result, err := f.FilterResponse(Context{}, resp)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer func() { _ = result.Body.Close() }()

	body, _ := io.ReadAll(result.Body)
	if string(body) != "plain text response" {
		t.Errorf("non-JSON body changed: %q", string(body))
	}
}

func TestStripFields_EmptyFieldList(t *testing.T) {
	f := NewStripFields(nil)
	resp := makeResponse(`{"key":"value"}`)
	defer func() { _ = resp.Body.Close() }()

	result, err := f.FilterResponse(Context{}, resp)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer func() { _ = result.Body.Close() }()

	body, _ := io.ReadAll(result.Body)
	if !strings.Contains(string(body), `"key"`) {
		t.Error("body should be unchanged with empty field list")
	}
}

func TestStripFields_Array(t *testing.T) {
	f := NewStripFields([]string{"expand", "self"})
	resp := makeResponse(`{"issues":[{"expand":"all","self":"http://x","key":"A"},{"expand":"all","self":"http://y","key":"B"}],"total":2}`)
	defer func() { _ = resp.Body.Close() }()

	result, err := f.FilterResponse(Context{}, resp)
	defer func() {
		if result != nil {
			_ = result.Body.Close()
		}
	}()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	body, _ := io.ReadAll(result.Body)
	bodyStr := string(body)

	if strings.Contains(bodyStr, `"expand"`) {
		t.Error("expand should be stripped from nested array elements")
	}
	if !strings.Contains(bodyStr, `"key"`) {
		t.Error("key should be preserved")
	}
	if !strings.Contains(bodyStr, `"total"`) {
		t.Error("total should be preserved (top-level)")
	}
}
