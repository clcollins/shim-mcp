package auth

import (
	"encoding/base64"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/clcollins/shim-mcp/internal/config"
)

func TestBasicAuthProvider_Authenticate(t *testing.T) {
	dir := t.TempDir()
	credsFile := filepath.Join(dir, "creds.json")
	if err := os.WriteFile(credsFile, []byte(`{"user":"test@example.com","token":"api-token-123"}`), 0600); err != nil {
		t.Fatal(err)
	}

	provider, err := NewAuthProvider(config.AuthConfig{
		Type:     "basic",
		Username: config.CredentialRef{File: credsFile, Key: ".user"},
		Token:    config.CredentialRef{File: credsFile, Key: ".token"},
	})
	if err != nil {
		t.Fatalf("creating provider: %v", err)
	}

	var receivedAuth string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedAuth = r.Header.Get("Authorization")
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	req, _ := http.NewRequest(http.MethodGet, ts.URL, nil)
	if err := provider.Authenticate(req); err != nil {
		t.Fatalf("Authenticate: %v", err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	_ = resp.Body.Close()

	expected := "Basic " + base64.StdEncoding.EncodeToString([]byte("test@example.com:api-token-123"))
	if receivedAuth != expected {
		t.Errorf("Authorization = %q, want %q", receivedAuth, expected)
	}
}

func TestBearerAuthProvider_Authenticate(t *testing.T) {
	dir := t.TempDir()
	tokenFile := filepath.Join(dir, "token")
	if err := os.WriteFile(tokenFile, []byte("bearer-token-xyz\n"), 0600); err != nil {
		t.Fatal(err)
	}

	provider, err := NewAuthProvider(config.AuthConfig{
		Type:  "bearer",
		Token: config.CredentialRef{File: tokenFile},
	})
	if err != nil {
		t.Fatalf("creating provider: %v", err)
	}

	var receivedAuth string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedAuth = r.Header.Get("Authorization")
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	req, _ := http.NewRequest(http.MethodGet, ts.URL, nil)
	if err := provider.Authenticate(req); err != nil {
		t.Fatalf("Authenticate: %v", err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	_ = resp.Body.Close()

	if receivedAuth != "Bearer bearer-token-xyz" {
		t.Errorf("Authorization = %q, want Bearer bearer-token-xyz", receivedAuth)
	}
}

func TestTokenAuthProvider_Authenticate(t *testing.T) {
	dir := t.TempDir()
	tokenFile := filepath.Join(dir, "token")
	if err := os.WriteFile(tokenFile, []byte("ghp_test123\n"), 0600); err != nil {
		t.Fatal(err)
	}

	provider, err := NewAuthProvider(config.AuthConfig{
		Type:  "token",
		Token: config.CredentialRef{File: tokenFile},
	})
	if err != nil {
		t.Fatalf("creating provider: %v", err)
	}

	var receivedAuth string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedAuth = r.Header.Get("Authorization")
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	req, _ := http.NewRequest(http.MethodGet, ts.URL, nil)
	if err := provider.Authenticate(req); err != nil {
		t.Fatalf("Authenticate: %v", err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	_ = resp.Body.Close()

	if receivedAuth != "token ghp_test123" {
		t.Errorf("Authorization = %q, want token ghp_test123", receivedAuth)
	}
}

func TestHeaderAuthProvider_Authenticate(t *testing.T) {
	t.Setenv("PD_TEST_TOKEN", "pd-api-key-456")

	provider, err := NewAuthProvider(config.AuthConfig{
		Type:     "header",
		Header:   "Authorization",
		Template: "Token token={{.Token}}",
		Token:    config.CredentialRef{Env: "PD_TEST_TOKEN"},
	})
	if err != nil {
		t.Fatalf("creating provider: %v", err)
	}

	var receivedAuth string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedAuth = r.Header.Get("Authorization")
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	req, _ := http.NewRequest(http.MethodGet, ts.URL, nil)
	if err := provider.Authenticate(req); err != nil {
		t.Fatalf("Authenticate: %v", err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	_ = resp.Body.Close()

	if receivedAuth != "Token token=pd-api-key-456" {
		t.Errorf("Authorization = %q, want Token token=pd-api-key-456", receivedAuth)
	}
}

func TestNewAuthProvider_UnknownType(t *testing.T) {
	_, err := NewAuthProvider(config.AuthConfig{Type: "oauth2"})
	if err == nil {
		t.Fatal("expected error for unknown auth type")
	}
}

func TestAuthProvider_CredentialError(t *testing.T) {
	provider, _ := NewAuthProvider(config.AuthConfig{
		Type:  "bearer",
		Token: config.CredentialRef{File: "/nonexistent/file"},
	})

	req, _ := http.NewRequest(http.MethodGet, "http://example.com", nil)
	if err := provider.Authenticate(req); err == nil {
		t.Fatal("expected error for missing credential file")
	}
}

func TestAuthProvider_Name(t *testing.T) {
	tests := []struct {
		cfg      config.AuthConfig
		wantName string
	}{
		{config.AuthConfig{Type: "basic", Username: config.CredentialRef{Env: "U"}, Token: config.CredentialRef{Env: "T"}}, "basic"},
		{config.AuthConfig{Type: "bearer", Token: config.CredentialRef{Env: "T"}}, "bearer"},
		{config.AuthConfig{Type: "token", Token: config.CredentialRef{Env: "T"}}, "token"},
		{config.AuthConfig{Type: "header", Header: "X-Test", Template: "{{.Token}}", Token: config.CredentialRef{Env: "T"}}, "header"},
	}
	for _, tt := range tests {
		provider, err := NewAuthProvider(tt.cfg)
		if err != nil {
			t.Fatalf("creating %s provider: %v", tt.cfg.Type, err)
		}
		if provider.Name() != tt.wantName {
			t.Errorf("Name() = %q, want %q", provider.Name(), tt.wantName)
		}
	}
}

func TestHeaderAuthProvider_InvalidTemplate(t *testing.T) {
	_, err := NewAuthProvider(config.AuthConfig{
		Type:     "header",
		Header:   "Authorization",
		Template: "{{.Invalid",
		Token:    config.CredentialRef{Env: "DUMMY"},
	})
	if err == nil {
		t.Fatal("expected error for invalid template")
	}
}

func TestBasicAuthProvider_NoCredentialInError(t *testing.T) {
	dir := t.TempDir()
	credsFile := filepath.Join(dir, "creds.json")
	if err := os.WriteFile(credsFile, []byte(`{"user":"test@example.com"}`), 0600); err != nil {
		t.Fatal(err)
	}

	provider, _ := NewAuthProvider(config.AuthConfig{
		Type:     "basic",
		Username: config.CredentialRef{File: credsFile, Key: ".user"},
		Token:    config.CredentialRef{File: "/nonexistent/token/file"},
	})

	req, _ := http.NewRequest(http.MethodGet, "http://example.com", nil)
	err := provider.Authenticate(req)
	if err == nil {
		t.Fatal("expected error")
	}
	if strings.Contains(err.Error(), "test@example.com") {
		t.Error("error message contains username value")
	}
}
