package proxy

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/clcollins/shim-mcp/internal/config"
	"github.com/clcollins/shim-mcp/internal/filter"
)

type mockAuthProvider struct {
	name  string
	token string
	err   error
}

func (m *mockAuthProvider) Name() string { return m.name }
func (m *mockAuthProvider) Authenticate(req *http.Request) error {
	if m.err != nil {
		return m.err
	}
	req.Header.Set("Authorization", "Bearer "+m.token)
	return nil
}

func TestProxy_BasicRequest(t *testing.T) {
	var receivedMethod, receivedPath, receivedAuth string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedMethod = r.Method
		receivedPath = r.URL.Path
		receivedAuth = r.Header.Get("Authorization")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = fmt.Fprint(w, `{"status":"ok"}`)
	}))
	defer ts.Close()

	p := newTestProxy(t, ts.URL, "test-token-123")

	resp, err := p.Do(context.Background(), &Request{
		Service: "test",
		Method:  "GET",
		Path:    "/api/v1/resource",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if receivedMethod != "GET" {
		t.Errorf("method = %q, want GET", receivedMethod)
	}
	if receivedPath != "/api/v1/resource" {
		t.Errorf("path = %q, want /api/v1/resource", receivedPath)
	}
	if receivedAuth != "Bearer test-token-123" {
		t.Errorf("auth = %q, want Bearer test-token-123", receivedAuth)
	}
	if resp.StatusCode != 200 {
		t.Errorf("status = %d, want 200", resp.StatusCode)
	}
	if resp.Body != `{"status":"ok"}` {
		t.Errorf("body = %q, want {\"status\":\"ok\"}", resp.Body)
	}
}

func TestProxy_HeaderMerging(t *testing.T) {
	var receivedAccept, receivedCustom string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedAccept = r.Header.Get("Accept")
		receivedCustom = r.Header.Get("X-Custom")
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	services := map[string]config.ServiceConfig{
		"test": {
			BaseURL: ts.URL,
			Auth:    config.AuthConfig{Type: "bearer"},
			Headers: map[string]string{
				"accept": "application/json",
			},
		},
	}
	authProviders := map[string]authProvider{
		"test": &mockAuthProvider{name: "bearer", token: "tok"},
	}
	p := &Proxy{
		services:        services,
		auth:            authProviders,
		client:          &http.Client{Timeout: 10 * time.Second},
		requestFilters:  make(map[string][]filter.RequestFilter),
		responseFilters: make(map[string][]filter.ResponseFilter),
		logger:          slog.New(slog.NewTextHandler(io.Discard, nil)),
	}

	_, err := p.Do(context.Background(), &Request{
		Service: "test",
		Method:  "GET",
		Path:    "/test",
		Headers: map[string]string{"X-Custom": "custom-val"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if receivedAccept != "application/json" {
		t.Errorf("Accept = %q, want application/json", receivedAccept)
	}
	if receivedCustom != "custom-val" {
		t.Errorf("X-Custom = %q, want custom-val", receivedCustom)
	}
}

func TestProxy_SSRFPrevention(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	p := newTestProxy(t, "https://api.example.com", "token")

	_, err := p.Do(context.Background(), &Request{
		Service: "test",
		Method:  "GET",
		Path:    "http://evil.com/steal",
	})
	if err == nil {
		t.Fatal("expected SSRF prevention error")
	}
}

func TestProxy_UnknownService(t *testing.T) {
	p := &Proxy{
		services:        map[string]config.ServiceConfig{},
		auth:            map[string]authProvider{},
		client:          &http.Client{Timeout: 10 * time.Second},
		requestFilters:  make(map[string][]filter.RequestFilter),
		responseFilters: make(map[string][]filter.ResponseFilter),
		logger:          slog.New(slog.NewTextHandler(io.Discard, nil)),
	}

	_, err := p.Do(context.Background(), &Request{
		Service: "nonexistent",
		Method:  "GET",
		Path:    "/test",
	})
	if err == nil {
		t.Fatal("expected error for unknown service")
	}
}

func TestProxy_InvalidMethod(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	p := newTestProxy(t, ts.URL, "token")

	_, err := p.Do(context.Background(), &Request{
		Service: "test",
		Method:  "INVALID",
		Path:    "/test",
	})
	if err == nil {
		t.Fatal("expected error for invalid HTTP method")
	}
}

func TestProxy_QueryParams(t *testing.T) {
	var receivedQuery string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedQuery = r.URL.RawQuery
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	p := newTestProxy(t, ts.URL, "token")

	_, err := p.Do(context.Background(), &Request{
		Service:     "test",
		Method:      "GET",
		Path:        "/search",
		QueryParams: map[string]string{"q": "test query", "page": "1"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(receivedQuery, "q=test+query") && !strings.Contains(receivedQuery, "q=test%20query") {
		t.Errorf("query = %q, expected q=test+query or q=test%%20query", receivedQuery)
	}
	if !strings.Contains(receivedQuery, "page=1") {
		t.Errorf("query = %q, expected page=1", receivedQuery)
	}
}

func TestProxy_PostWithBody(t *testing.T) {
	var receivedBody string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		buf := make([]byte, r.ContentLength)
		_, _ = r.Body.Read(buf)
		receivedBody = string(buf)
		w.WriteHeader(http.StatusCreated)
	}))
	defer ts.Close()

	p := newTestProxy(t, ts.URL, "token")
	body := `{"name":"test"}`

	resp, err := p.Do(context.Background(), &Request{
		Service: "test",
		Method:  "POST",
		Path:    "/items",
		Body:    body,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if receivedBody != body {
		t.Errorf("body = %q, want %q", receivedBody, body)
	}
	if resp.StatusCode != 201 {
		t.Errorf("status = %d, want 201", resp.StatusCode)
	}
}

func TestProxy_BodySizeLimit(t *testing.T) {
	largeBody := strings.Repeat("x", 1024*1024+1)
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = fmt.Fprint(w, largeBody)
	}))
	defer ts.Close()

	p := newTestProxy(t, ts.URL, "token")
	p.maxResponseSize = 1024

	resp, err := p.Do(context.Background(), &Request{
		Service: "test",
		Method:  "GET",
		Path:    "/large",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(resp.Body) > 1025 {
		t.Errorf("response body too large: %d bytes", len(resp.Body))
	}
}

func TestProxy_Timeout(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(2 * time.Second)
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	p := newTestProxy(t, ts.URL, "token")
	p.client.Timeout = 100 * time.Millisecond

	_, err := p.Do(context.Background(), &Request{
		Service: "test",
		Method:  "GET",
		Path:    "/slow",
	})
	if err == nil {
		t.Fatal("expected timeout error")
	}
}

func TestProxy_AuthError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	services := map[string]config.ServiceConfig{
		"test": {BaseURL: ts.URL, Auth: config.AuthConfig{Type: "bearer"}},
	}
	authProviders := map[string]authProvider{
		"test": &mockAuthProvider{name: "bearer", err: fmt.Errorf("credential error")},
	}
	p := &Proxy{
		services:        services,
		auth:            authProviders,
		client:          &http.Client{Timeout: 10 * time.Second},
		requestFilters:  make(map[string][]filter.RequestFilter),
		responseFilters: make(map[string][]filter.ResponseFilter),
		logger:          slog.New(slog.NewTextHandler(io.Discard, nil)),
	}

	_, err := p.Do(context.Background(), &Request{
		Service: "test",
		Method:  "GET",
		Path:    "/test",
	})
	if err == nil {
		t.Fatal("expected auth error")
	}
}

func TestProxy_ResponseHeadersScrubbed(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("X-Secret-Internal", "should-pass-through")
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	p := newTestProxy(t, ts.URL, "token")

	resp, err := p.Do(context.Background(), &Request{
		Service: "test",
		Method:  "GET",
		Path:    "/test",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if _, ok := resp.Headers["Authorization"]; ok {
		t.Error("response should not contain Authorization header")
	}
	if resp.Headers["Content-Type"] != "application/json" {
		t.Errorf("Content-Type = %q, want application/json", resp.Headers["Content-Type"])
	}
}

func newTestProxy(t *testing.T, baseURL, token string) *Proxy {
	t.Helper()
	services := map[string]config.ServiceConfig{
		"test": {
			BaseURL: baseURL,
			Auth:    config.AuthConfig{Type: "bearer"},
		},
	}
	authProviders := map[string]authProvider{
		"test": &mockAuthProvider{name: "bearer", token: token},
	}
	return &Proxy{
		services:        services,
		auth:            authProviders,
		client:          &http.Client{Timeout: 30 * time.Second},
		maxResponseSize: 10 * 1024 * 1024,
		requestFilters:  make(map[string][]filter.RequestFilter),
		responseFilters: make(map[string][]filter.ResponseFilter),
		logger:          slog.New(slog.NewTextHandler(io.Discard, nil)),
	}
}

func TestProxy_ValidateJSONBodyRejectsInvalid(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	p := newTestProxy(t, ts.URL, "token")
	p.requestFilters["test"] = []filter.RequestFilter{&filter.ValidateJSONBody{}}

	_, err := p.Do(context.Background(), &Request{
		Service: "test",
		Method:  "POST",
		Path:    "/test",
		Headers: map[string]string{"Content-Type": "application/json"},
		Body:    `{"broken":}`,
	})
	if err == nil {
		t.Fatal("expected error for invalid JSON body")
	}
	if !strings.Contains(err.Error(), "validate_json_body") {
		t.Errorf("error should mention filter name: %v", err)
	}
}

func TestProxy_AutoContentTypeSetsHeader(t *testing.T) {
	var receivedCT string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedCT = r.Header.Get("Content-Type")
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	p := newTestProxy(t, ts.URL, "token")
	p.requestFilters["test"] = []filter.RequestFilter{&filter.AutoContentType{}}

	_, err := p.Do(context.Background(), &Request{
		Service: "test",
		Method:  "POST",
		Path:    "/test",
		Body:    `{"key":"value"}`,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if receivedCT != "application/json" {
		t.Errorf("Content-Type = %q, want application/json", receivedCT)
	}
}

func TestProxy_StripFieldsRemovesConfiguredFields(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprint(w, `{"expand":"all","self":"http://x","key":"SREP-1","summary":"test"}`)
	}))
	defer ts.Close()

	p := newTestProxy(t, ts.URL, "token")
	p.responseFilters["test"] = []filter.ResponseFilter{
		filter.NewStripFields([]string{"expand", "self"}),
	}

	resp, err := p.Do(context.Background(), &Request{
		Service: "test",
		Method:  "GET",
		Path:    "/test",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var result map[string]any
	if err := json.Unmarshal([]byte(resp.Body), &result); err != nil {
		t.Fatalf("parsing response: %v", err)
	}
	if _, ok := result["expand"]; ok {
		t.Error("expand should be stripped")
	}
	if _, ok := result["self"]; ok {
		t.Error("self should be stripped")
	}
	if _, ok := result["key"]; !ok {
		t.Error("key should be preserved")
	}
}

func TestProxy_FiltersOnlyApplyToConfiguredService(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	p := newTestProxy(t, ts.URL, "token")
	p.requestFilters["other-service"] = []filter.RequestFilter{&filter.RejectEmptyBody{}}

	_, err := p.Do(context.Background(), &Request{
		Service: "test",
		Method:  "POST",
		Path:    "/test",
	})
	if err != nil {
		t.Fatalf("should not error — reject_empty_body is on different service: %v", err)
	}
}

func TestProxy_NoFiltersConfigured(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = fmt.Fprint(w, `{"ok":true}`)
	}))
	defer ts.Close()

	p := newTestProxy(t, ts.URL, "token")

	resp, err := p.Do(context.Background(), &Request{
		Service: "test",
		Method:  "GET",
		Path:    "/test",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.StatusCode != 200 {
		t.Errorf("status = %d, want 200", resp.StatusCode)
	}
}
