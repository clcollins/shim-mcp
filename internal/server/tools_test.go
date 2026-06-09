package server

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/clcollins/shim-mcp/internal/config"
	"github.com/clcollins/shim-mcp/internal/proxy"
	"github.com/modelcontextprotocol/go-sdk/mcp"

	"net/http"
	"net/http/httptest"
)

func connectTestClient(t *testing.T, ts *httptest.Server) *mcp.ClientSession {
	t.Helper()

	cfg := &config.Config{
		Services: map[string]config.ServiceConfig{
			"testapi": {
				BaseURL: ts.URL,
				Auth: config.AuthConfig{
					Type:  "bearer",
					Token: config.CredentialRef{Env: "TEST_AUTH_TOKEN"},
				},
				Headers: map[string]string{
					"accept": "application/json",
				},
			},
		},
	}

	t.Setenv("TEST_AUTH_TOKEN", "test-bearer-token")

	p, err := proxy.New(cfg)
	if err != nil {
		t.Fatalf("creating proxy: %v", err)
	}

	server := New(cfg, p)
	clientTransport, serverTransport := mcp.NewInMemoryTransports()

	go func() {
		_ = server.Run(context.Background(), serverTransport)
	}()

	client := mcp.NewClient(&mcp.Implementation{
		Name:    "test-client",
		Version: "1.0.0",
	}, nil)

	session, err := client.Connect(context.Background(), clientTransport, nil)
	if err != nil {
		t.Fatalf("client connect: %v", err)
	}
	t.Cleanup(func() { _ = session.Close() })

	return session
}

func TestHTTPRequestTool_GET(t *testing.T) {
	var receivedMethod, receivedPath, receivedAuth string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedMethod = r.Method
		receivedPath = r.URL.Path
		receivedAuth = r.Header.Get("Authorization")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]string{"result": "success"})
	}))
	defer ts.Close()

	session := connectTestClient(t, ts)

	result, err := session.CallTool(context.Background(), &mcp.CallToolParams{
		Name: "http_request",
		Arguments: map[string]any{
			"service": "testapi",
			"method":  "GET",
			"path":    "/api/v1/items",
		},
	})
	if err != nil {
		t.Fatalf("CallTool error: %v", err)
	}

	if receivedMethod != "GET" {
		t.Errorf("method = %q, want GET", receivedMethod)
	}
	if receivedPath != "/api/v1/items" {
		t.Errorf("path = %q, want /api/v1/items", receivedPath)
	}
	if receivedAuth != "Bearer test-bearer-token" {
		t.Errorf("auth = %q, want Bearer test-bearer-token", receivedAuth)
	}
	if result.IsError {
		t.Errorf("unexpected error result")
	}

	if result.StructuredContent != nil {
		scBytes, err := json.Marshal(result.StructuredContent)
		if err != nil {
			t.Fatalf("marshaling structured content: %v", err)
		}
		var resp httpResponseOutput
		if err := json.Unmarshal(scBytes, &resp); err != nil {
			t.Fatalf("unmarshaling structured content: %v", err)
		}
		if resp.StatusCode != 200 {
			t.Errorf("status = %d, want 200", resp.StatusCode)
		}
	}
}

func TestHTTPRequestTool_POST(t *testing.T) {
	var receivedBody string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		buf := make([]byte, 1024)
		n, _ := r.Body.Read(buf)
		receivedBody = string(buf[:n])
		w.WriteHeader(http.StatusCreated)
	}))
	defer ts.Close()

	session := connectTestClient(t, ts)

	body := `{"name":"test-item"}`
	result, err := session.CallTool(context.Background(), &mcp.CallToolParams{
		Name: "http_request",
		Arguments: map[string]any{
			"service": "testapi",
			"method":  "POST",
			"path":    "/api/v1/items",
			"body":    body,
		},
	})
	if err != nil {
		t.Fatalf("CallTool error: %v", err)
	}

	if receivedBody != body {
		t.Errorf("body = %q, want %q", receivedBody, body)
	}
	if result.IsError {
		t.Error("unexpected error result")
	}
}

func TestHTTPRequestTool_UnknownService(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	session := connectTestClient(t, ts)

	result, err := session.CallTool(context.Background(), &mcp.CallToolParams{
		Name: "http_request",
		Arguments: map[string]any{
			"service": "nonexistent",
			"method":  "GET",
			"path":    "/test",
		},
	})
	if err != nil {
		t.Fatalf("CallTool error: %v", err)
	}

	if !result.IsError {
		t.Error("expected error result for unknown service")
	}
}

func TestHTTPRequestTool_NoCredentialsInResponse(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Authorization", "Bearer should-not-appear")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	}))
	defer ts.Close()

	session := connectTestClient(t, ts)

	result, err := session.CallTool(context.Background(), &mcp.CallToolParams{
		Name: "http_request",
		Arguments: map[string]any{
			"service": "testapi",
			"method":  "GET",
			"path":    "/test",
		},
	})
	if err != nil {
		t.Fatalf("CallTool error: %v", err)
	}

	resultJSON, _ := json.Marshal(result)
	resultStr := string(resultJSON)
	if strings.Contains(resultStr, "test-bearer-token") {
		t.Error("response contains credential token")
	}
	if strings.Contains(resultStr, "should-not-appear") {
		t.Error("response contains upstream Authorization header value")
	}
}

func TestListServicesTool(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	session := connectTestClient(t, ts)

	result, err := session.CallTool(context.Background(), &mcp.CallToolParams{
		Name:      "list_services",
		Arguments: map[string]any{},
	})
	if err != nil {
		t.Fatalf("CallTool error: %v", err)
	}

	if result.IsError {
		t.Error("unexpected error result")
	}

	resultJSON, _ := json.Marshal(result)
	resultStr := string(resultJSON)

	if !strings.Contains(resultStr, "testapi") {
		t.Error("list_services should contain service name 'testapi'")
	}
	if !strings.Contains(resultStr, ts.URL) {
		t.Errorf("list_services should contain base URL %q", ts.URL)
	}
	if strings.Contains(resultStr, "test-bearer-token") {
		t.Error("list_services should not contain credentials")
	}
}

func TestToolsList(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	session := connectTestClient(t, ts)

	tools, err := session.ListTools(context.Background(), nil)
	if err != nil {
		t.Fatalf("ListTools error: %v", err)
	}

	toolNames := make(map[string]bool)
	for _, tool := range tools.Tools {
		toolNames[tool.Name] = true
	}

	if !toolNames["http_request"] {
		t.Error("http_request tool not found")
	}
	if !toolNames["list_services"] {
		t.Error("list_services tool not found")
	}
}
