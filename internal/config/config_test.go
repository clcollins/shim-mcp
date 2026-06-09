package config

import (
	"os"
	"path/filepath"
	"testing"
)

func writeTestConfig(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(path, []byte(content), 0600); err != nil {
		t.Fatalf("writing test config: %v", err)
	}
	return path
}

func writeFile(t *testing.T, path string, content []byte) {
	t.Helper()
	if err := os.WriteFile(path, content, 0600); err != nil {
		t.Fatalf("writing test file: %v", err)
	}
}

// --- CredentialRef.Resolve tests ---

func TestCredentialRef_ResolveEnvVar(t *testing.T) {
	t.Setenv("TEST_CRED", "env-token-value")
	ref := CredentialRef{Env: "TEST_CRED"}
	val, err := ref.Resolve()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if val != "env-token-value" {
		t.Errorf("got %q, want %q", val, "env-token-value")
	}
}

func TestCredentialRef_ResolveMissingEnv(t *testing.T) {
	ref := CredentialRef{Env: "DEFINITELY_NOT_SET_XYZ"}
	_, err := ref.Resolve()
	if err == nil {
		t.Fatal("expected error for missing env var")
	}
}

func TestCredentialRef_ResolveRawTextFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "token")
	writeFile(t, path, []byte("my-secret-token\n"))

	ref := CredentialRef{File: path}
	val, err := ref.Resolve()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if val != "my-secret-token" {
		t.Errorf("got %q, want %q", val, "my-secret-token")
	}
}

func TestCredentialRef_ResolveEmptyFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "empty")
	writeFile(t, path, []byte(""))

	ref := CredentialRef{File: path}
	_, err := ref.Resolve()
	if err == nil {
		t.Fatal("expected error for empty file")
	}
}

func TestCredentialRef_ResolveMissingFile(t *testing.T) {
	ref := CredentialRef{File: "/nonexistent/file"}
	_, err := ref.Resolve()
	if err == nil {
		t.Fatal("expected error for missing file")
	}
}

func TestCredentialRef_ResolveJSONInferred(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")
	writeFile(t, path, []byte(`{"jira":{"token":"secret123"}}`))

	ref := CredentialRef{File: path, Key: ".jira.token"}
	val, err := ref.Resolve()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if val != "secret123" {
		t.Errorf("got %q, want %q", val, "secret123")
	}
}

func TestCredentialRef_ResolveYAMLInferred(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yml")
	writeFile(t, path, []byte("hosts:\n  gitlab.example.com:\n    token: yaml-secret\n"))

	ref := CredentialRef{File: path, Key: `.hosts."gitlab.example.com".token`}
	val, err := ref.Resolve()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if val != "yaml-secret" {
		t.Errorf("got %q, want %q", val, "yaml-secret")
	}
}

func TestCredentialRef_ResolveYAMLExplicitFormat(t *testing.T) {
	dir := t.TempDir()
	// File has .json extension but content is YAML — explicit format overrides
	path := filepath.Join(dir, "config.json")
	writeFile(t, path, []byte("token: yaml-value\n"))

	ref := CredentialRef{File: path, Format: "yaml", Key: ".token"}
	val, err := ref.Resolve()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if val != "yaml-value" {
		t.Errorf("got %q, want %q", val, "yaml-value")
	}
}

func TestCredentialRef_ResolveArrayIndex(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")
	writeFile(t, path, []byte(`{"subdomains":[{"token":"pd-token"}]}`))

	ref := CredentialRef{File: path, Key: ".subdomains[0].token"}
	val, err := ref.Resolve()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if val != "pd-token" {
		t.Errorf("got %q, want %q", val, "pd-token")
	}
}

func TestCredentialRef_ResolveQuotedKey(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")
	writeFile(t, path, []byte(`{"jira":{"user-email":"test@example.com"}}`))

	ref := CredentialRef{File: path, Key: `.jira."user-email"`}
	val, err := ref.Resolve()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if val != "test@example.com" {
		t.Errorf("got %q, want %q", val, "test@example.com")
	}
}

func TestCredentialRef_ResolveInvalidJSON(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "bad.json")
	writeFile(t, path, []byte(`{invalid`))

	ref := CredentialRef{File: path, Key: ".token"}
	_, err := ref.Resolve()
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

func TestCredentialRef_ResolveKeyNotFound(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")
	writeFile(t, path, []byte(`{"token":"secret"}`))

	ref := CredentialRef{File: path, Key: ".nonexistent"}
	_, err := ref.Resolve()
	if err == nil {
		t.Fatal("expected error for nonexistent key")
	}
}

// --- CredentialRef.Validate tests ---

func TestCredentialRef_ValidateBothSet(t *testing.T) {
	ref := CredentialRef{File: "/some/path", Env: "SOME_VAR"}
	if err := ref.Validate(); err == nil {
		t.Fatal("expected error when both file and env set")
	}
}

func TestCredentialRef_ValidateNeitherSet(t *testing.T) {
	ref := CredentialRef{}
	if err := ref.Validate(); err == nil {
		t.Fatal("expected error when neither file nor env set")
	}
}

func TestCredentialRef_ValidateKeyWithoutFile(t *testing.T) {
	ref := CredentialRef{Env: "TOKEN", Key: ".token"}
	if err := ref.Validate(); err == nil {
		t.Fatal("expected error for key without file")
	}
}

func TestCredentialRef_ValidateFormatWithoutFile(t *testing.T) {
	ref := CredentialRef{Env: "TOKEN", Format: "json"}
	if err := ref.Validate(); err == nil {
		t.Fatal("expected error for format without file")
	}
}

func TestCredentialRef_ValidateValidFile(t *testing.T) {
	ref := CredentialRef{File: "/some/path"}
	if err := ref.Validate(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCredentialRef_ValidateValidEnv(t *testing.T) {
	ref := CredentialRef{Env: "TOKEN"}
	if err := ref.Validate(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// --- Path expansion tests ---

func TestCredentialRef_TildeExpansion(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Skip("cannot determine home dir")
	}

	ref := CredentialRef{File: "~/some/token"}
	if err := ref.expandAndValidatePath(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	expected := filepath.Join(home, "some/token")
	if ref.File != expected {
		t.Errorf("got %q, want %q", ref.File, expected)
	}
}

func TestCredentialRef_PathTraversalRejected(t *testing.T) {
	ref := CredentialRef{File: "/etc/../etc/shadow"}
	if err := ref.expandAndValidatePath(); err == nil {
		t.Fatal("expected error for path traversal")
	}
}

// --- inferFormat tests ---

func TestInferFormat(t *testing.T) {
	tests := []struct {
		file     string
		explicit string
		want     string
	}{
		{"config.json", "", "json"},
		{"config.yaml", "", "yaml"},
		{"config.yml", "", "yaml"},
		{"token", "", "text"},
		{"config.json", "yaml", "yaml"},
		{".env", "", "env"},
	}
	for _, tt := range tests {
		got := inferFormat(tt.file, tt.explicit)
		if got != tt.want {
			t.Errorf("inferFormat(%q, %q) = %q, want %q", tt.file, tt.explicit, got, tt.want)
		}
	}
}

// --- parsePath tests ---

func TestParsePath(t *testing.T) {
	tests := []struct {
		path    string
		wantLen int
		wantErr bool
	}{
		{".token", 1, false},
		{".jira.token", 2, false},
		{`.jira."user-email"`, 2, false},
		{".subdomains[0].token", 3, false},
		{".a[0][1].b", 4, false},
		{"token", 0, true},
		{".", 0, true},
		{`.jira."unclosed`, 0, true},
		{".arr[-1].x", 0, true},
		{".arr[abc].x", 0, true},
	}
	for _, tt := range tests {
		segs, err := parsePath(tt.path)
		if tt.wantErr {
			if err == nil {
				t.Errorf("parsePath(%q): want error, got %v", tt.path, segs)
			}
			continue
		}
		if err != nil {
			t.Errorf("parsePath(%q): unexpected error: %v", tt.path, err)
			continue
		}
		if len(segs) != tt.wantLen {
			t.Errorf("parsePath(%q): got %d segments, want %d", tt.path, len(segs), tt.wantLen)
		}
	}
}

// --- LoadConfig tests ---

func TestLoadConfig_BasicAuth(t *testing.T) {
	credsFile := filepath.Join(t.TempDir(), "creds.json")
	writeFile(t, credsFile, []byte(`{"user":"test@example.com","token":"secret"}`))

	yaml := `
services:
  jira:
    base_url: "https://jira.example.com"
    auth:
      type: basic
      username:
        file: "` + credsFile + `"
        key: ".user"
      token:
        file: "` + credsFile + `"
        key: ".token"
    headers:
      content-type: "application/json"
`
	cfg, err := LoadConfig(writeTestConfig(t, yaml))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	svc := cfg.Services["jira"]
	if svc.Auth.Type != "basic" {
		t.Errorf("type = %q, want basic", svc.Auth.Type)
	}
	if svc.Auth.Username.File != credsFile {
		t.Errorf("username.file = %q, want %q", svc.Auth.Username.File, credsFile)
	}
	if svc.Auth.Username.Key != ".user" {
		t.Errorf("username.key = %q, want .user", svc.Auth.Username.Key)
	}
	if svc.Headers["content-type"] != "application/json" {
		t.Errorf("headers[content-type] = %q, want application/json", svc.Headers["content-type"])
	}
}

func TestLoadConfig_TokenAuth(t *testing.T) {
	tokenFile := filepath.Join(t.TempDir(), "token")
	writeFile(t, tokenFile, []byte("ghp_test\n"))

	yaml := `
services:
  github:
    base_url: "https://api.github.com"
    auth:
      type: token
      token:
        file: "` + tokenFile + `"
`
	cfg, err := LoadConfig(writeTestConfig(t, yaml))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Services["github"].Auth.Token.File != tokenFile {
		t.Error("token.file mismatch")
	}
}

func TestLoadConfig_BearerAuth(t *testing.T) {
	yaml := `
services:
  svc:
    base_url: "https://example.com"
    auth:
      type: bearer
      token:
        env: "BEARER_TOKEN"
`
	cfg, err := LoadConfig(writeTestConfig(t, yaml))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Services["svc"].Auth.Token.Env != "BEARER_TOKEN" {
		t.Error("token.env mismatch")
	}
}

func TestLoadConfig_HeaderAuth(t *testing.T) {
	yaml := `
services:
  pd:
    base_url: "https://api.pagerduty.com"
    auth:
      type: header
      header: "Authorization"
      template: "Token token={{.Token}}"
      token:
        env: "PD_TOKEN"
`
	cfg, err := LoadConfig(writeTestConfig(t, yaml))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	svc := cfg.Services["pd"]
	if svc.Auth.Header != "Authorization" {
		t.Errorf("header = %q", svc.Auth.Header)
	}
	if svc.Auth.Template != "Token token={{.Token}}" {
		t.Errorf("template = %q", svc.Auth.Template)
	}
}

func TestLoadConfig_MissingBaseURL(t *testing.T) {
	yaml := `
services:
  broken:
    auth:
      type: bearer
      token: {env: "TOKEN"}
`
	_, err := LoadConfig(writeTestConfig(t, yaml))
	if err == nil {
		t.Fatal("expected error for missing base_url")
	}
}

func TestLoadConfig_MissingAuthType(t *testing.T) {
	yaml := `
services:
  broken:
    base_url: "https://example.com"
    auth:
      token: {env: "TOKEN"}
`
	_, err := LoadConfig(writeTestConfig(t, yaml))
	if err == nil {
		t.Fatal("expected error for missing auth type")
	}
}

func TestLoadConfig_UnknownAuthType(t *testing.T) {
	yaml := `
services:
  broken:
    base_url: "https://example.com"
    auth:
      type: oauth2
      token: {env: "TOKEN"}
`
	_, err := LoadConfig(writeTestConfig(t, yaml))
	if err == nil {
		t.Fatal("expected error for unknown auth type")
	}
}

func TestLoadConfig_EmptyServices(t *testing.T) {
	_, err := LoadConfig(writeTestConfig(t, "services: {}"))
	if err == nil {
		t.Fatal("expected error for empty services")
	}
}

func TestLoadConfig_BasicMissingUsername(t *testing.T) {
	yaml := `
services:
  broken:
    base_url: "https://example.com"
    auth:
      type: basic
      token: {env: "TOKEN"}
`
	_, err := LoadConfig(writeTestConfig(t, yaml))
	if err == nil {
		t.Fatal("expected error for basic auth without username")
	}
}

func TestLoadConfig_BasicMissingToken(t *testing.T) {
	yaml := `
services:
  broken:
    base_url: "https://example.com"
    auth:
      type: basic
      username: {env: "USER"}
`
	_, err := LoadConfig(writeTestConfig(t, yaml))
	if err == nil {
		t.Fatal("expected error for basic auth without token")
	}
}

func TestLoadConfig_HeaderMissingTemplate(t *testing.T) {
	yaml := `
services:
  broken:
    base_url: "https://example.com"
    auth:
      type: header
      header: "Authorization"
      token: {env: "TOKEN"}
`
	_, err := LoadConfig(writeTestConfig(t, yaml))
	if err == nil {
		t.Fatal("expected error for header auth without template")
	}
}

func TestLoadConfig_HeaderMissingHeaderName(t *testing.T) {
	yaml := `
services:
  broken:
    base_url: "https://example.com"
    auth:
      type: header
      template: "Bearer {{.Token}}"
      token: {env: "TOKEN"}
`
	_, err := LoadConfig(writeTestConfig(t, yaml))
	if err == nil {
		t.Fatal("expected error for header auth without header name")
	}
}

func TestLoadConfig_InvalidBaseURL(t *testing.T) {
	yaml := `
services:
  broken:
    base_url: "not-a-url"
    auth:
      type: bearer
      token: {env: "TOKEN"}
`
	_, err := LoadConfig(writeTestConfig(t, yaml))
	if err == nil {
		t.Fatal("expected error for invalid base_url")
	}
}

func TestLoadConfig_BaseURLTrailingSlash(t *testing.T) {
	yaml := `
services:
  svc:
    base_url: "https://example.com/"
    auth:
      type: bearer
      token: {env: "TOKEN"}
`
	cfg, err := LoadConfig(writeTestConfig(t, yaml))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Services["svc"].BaseURL != "https://example.com" {
		t.Errorf("trailing slash not stripped: %q", cfg.Services["svc"].BaseURL)
	}
}

func TestLoadConfig_TildeExpansion(t *testing.T) {
	home, _ := os.UserHomeDir()
	yaml := `
services:
  svc:
    base_url: "https://example.com"
    auth:
      type: token
      token:
        file: "~/some/token"
`
	cfg, err := LoadConfig(writeTestConfig(t, yaml))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	expected := filepath.Join(home, "some/token")
	if cfg.Services["svc"].Auth.Token.File != expected {
		t.Errorf("got %q, want %q", cfg.Services["svc"].Auth.Token.File, expected)
	}
}

func TestLoadConfig_PathTraversal(t *testing.T) {
	yaml := `
services:
  broken:
    base_url: "https://example.com"
    auth:
      type: token
      token:
        file: "/etc/../etc/shadow"
`
	_, err := LoadConfig(writeTestConfig(t, yaml))
	if err == nil {
		t.Fatal("expected error for path traversal")
	}
}

func TestLoadConfig_FileNotFound(t *testing.T) {
	_, err := LoadConfig("/nonexistent/path/config.yaml")
	if err == nil {
		t.Fatal("expected error for missing config file")
	}
}

func TestLoadConfig_MultipleServices(t *testing.T) {
	yaml := `
services:
  svc1:
    base_url: "https://one.example.com"
    auth:
      type: bearer
      token: {env: "TOKEN1"}
  svc2:
    base_url: "https://two.example.com"
    auth:
      type: bearer
      token: {env: "TOKEN2"}
`
	cfg, err := LoadConfig(writeTestConfig(t, yaml))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(cfg.Services) != 2 {
		t.Errorf("got %d services, want 2", len(cfg.Services))
	}
}

func TestLoadConfig_CredentialRefBothFileAndEnv(t *testing.T) {
	yaml := `
services:
  broken:
    base_url: "https://example.com"
    auth:
      type: bearer
      token:
        file: "/some/path"
        env: "SOME_VAR"
`
	_, err := LoadConfig(writeTestConfig(t, yaml))
	if err == nil {
		t.Fatal("expected error when both file and env set on credential ref")
	}
}

func TestLoadConfig_WithFilters(t *testing.T) {
	yaml := `
services:
  svc:
    base_url: "https://example.com"
    auth:
      type: bearer
      token: {env: "TOKEN"}
    filters:
      request:
        validate_json_body: true
        auto_content_type: true
        reject_empty_body: true
      response:
        strip_fields:
          - "expand"
          - "self"
          - "schema"
`
	cfg, err := LoadConfig(writeTestConfig(t, yaml))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	svc := cfg.Services["svc"]
	if !svc.Filters.Request.ValidateJSONBody {
		t.Error("validate_json_body should be true")
	}
	if !svc.Filters.Request.AutoContentType {
		t.Error("auto_content_type should be true")
	}
	if !svc.Filters.Request.RejectEmptyBody {
		t.Error("reject_empty_body should be true")
	}
	if len(svc.Filters.Response.StripFields) != 3 {
		t.Errorf("strip_fields length = %d, want 3", len(svc.Filters.Response.StripFields))
	}
}

func TestLoadConfig_WithoutFilters(t *testing.T) {
	yaml := `
services:
  svc:
    base_url: "https://example.com"
    auth:
      type: bearer
      token: {env: "TOKEN"}
`
	cfg, err := LoadConfig(writeTestConfig(t, yaml))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	svc := cfg.Services["svc"]
	if svc.Filters.Request.ValidateJSONBody {
		t.Error("validate_json_body should default to false")
	}
	if len(svc.Filters.Response.StripFields) != 0 {
		t.Error("strip_fields should default to empty")
	}
}

func TestCredentialRef_ResolveEnvFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.env")
	writeFile(t, path, []byte("# comment\nAPI_HOST=https://192.168.1.1\nAPI_KEY=secret-key-123\nEMPTY_VAL=\n"))

	ref := CredentialRef{File: path, Format: "env", Key: "API_KEY"}
	val, err := ref.Resolve()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if val != "secret-key-123" {
		t.Errorf("got %q, want %q", val, "secret-key-123")
	}
}

func TestCredentialRef_ResolveEnvFileQuoted(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.env")
	writeFile(t, path, []byte("TOKEN=\"quoted-value\"\nSINGLE='single-quoted'\n"))

	tests := []struct {
		key  string
		want string
	}{
		{"TOKEN", "quoted-value"},
		{"SINGLE", "single-quoted"},
	}
	for _, tt := range tests {
		ref := CredentialRef{File: path, Format: "env", Key: tt.key}
		val, err := ref.Resolve()
		if err != nil {
			t.Fatalf("key %q: unexpected error: %v", tt.key, err)
		}
		if val != tt.want {
			t.Errorf("key %q: got %q, want %q", tt.key, val, tt.want)
		}
	}
}

func TestCredentialRef_ResolveEnvFileMissingKey(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.env")
	writeFile(t, path, []byte("OTHER_KEY=value\n"))

	ref := CredentialRef{File: path, Format: "env", Key: "MISSING"}
	_, err := ref.Resolve()
	if err == nil {
		t.Fatal("expected error for missing key")
	}
}

func TestCredentialRef_ResolveEnvFileEmptyValue(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.env")
	writeFile(t, path, []byte("EMPTY=\n"))

	ref := CredentialRef{File: path, Format: "env", Key: "EMPTY"}
	_, err := ref.Resolve()
	if err == nil {
		t.Fatal("expected error for empty value")
	}
}

func TestCredentialRef_ResolveEnvFileInferred(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "credentials.env")
	writeFile(t, path, []byte("MY_TOKEN=inferred-from-ext\n"))

	ref := CredentialRef{File: path, Key: "MY_TOKEN"}
	val, err := ref.Resolve()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if val != "inferred-from-ext" {
		t.Errorf("got %q, want %q", val, "inferred-from-ext")
	}
}

func TestLoadConfig_TLSSkipVerify(t *testing.T) {
	yaml := `
services:
  svc:
    base_url: "https://192.168.1.1"
    tls_skip_verify: true
    auth:
      type: bearer
      token: {env: "TOKEN"}
`
	cfg, err := LoadConfig(writeTestConfig(t, yaml))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !cfg.Services["svc"].TLSSkipVerify {
		t.Error("tls_skip_verify should be true")
	}
}

func TestLoadConfig_TLSSkipVerifyDefault(t *testing.T) {
	yaml := `
services:
  svc:
    base_url: "https://example.com"
    auth:
      type: bearer
      token: {env: "TOKEN"}
`
	cfg, err := LoadConfig(writeTestConfig(t, yaml))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Services["svc"].TLSSkipVerify {
		t.Error("tls_skip_verify should default to false")
	}
}

func TestLoadConfig_AuthTypeNone(t *testing.T) {
	yaml := `
services:
  ollama:
    base_url: "http://localhost:11434"
    auth:
      type: none
`
	cfg, err := LoadConfig(writeTestConfig(t, yaml))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Services["ollama"].Auth.Type != "none" {
		t.Errorf("auth.type = %q, want none", cfg.Services["ollama"].Auth.Type)
	}
}

func TestLoadConfig_AuthTypeNoneNoCredentials(t *testing.T) {
	yaml := `
services:
  svc:
    base_url: "http://localhost:8080"
    auth:
      type: none
    filters:
      request:
        auto_content_type: true
`
	cfg, err := LoadConfig(writeTestConfig(t, yaml))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Services["svc"].Auth.Token.File != "" && cfg.Services["svc"].Auth.Token.Env != "" {
		t.Error("none auth should not require credentials")
	}
}
