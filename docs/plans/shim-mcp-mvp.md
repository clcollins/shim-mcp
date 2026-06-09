# Plan: Implement shim-mcp authenticated HTTP proxy MCP server

**Plan ID:** 2026-06-08-shim-mcp-mvp
**Status:** In Progress
**Created:** 2026-06-08
**Supersedes:** none
**Related plans:** none (first plan in repository)

## Goal

Build a lightweight, per-invocation MCP server in Go 1.26 that proxies
authenticated HTTP requests on behalf of AI agents. Agents call MCP tools
with request parameters; the server injects credentials from a config file,
executes the request via stdlib `net/http`, and returns the response. The
server runs locally over stdio, never exposes credentials to agents, and
supports pluggable auth providers and future request/response filters.

## Success criteria

- [ ] `go build ./cmd/shim-mcp` produces a working binary
- [ ] `go test ./...` passes with coverage on auth and proxy packages
- [ ] Binary responds correctly to MCP `initialize` and `tools/list`
- [ ] `http_request` tool proxies a request to a test server with correct auth headers injected
- [ ] `list_services` returns configured services without exposing credentials
- [ ] Credentials never appear in stdout (MCP response) output
- [ ] Config validation rejects missing fields, unknown auth types, and path traversal
- [ ] `make ci-all` passes all linting and checks per CONVENTIONS.md
- [ ] Every package has tests written BEFORE its implementation code

## Development methodology: Test-Driven Development

**TDD is mandatory for all implementation in this project.** Every task
follows the red-green-refactor cycle:

1. **Red**: Write failing tests that define the expected behavior. Tests
   must compile (use stub types/functions that return zero values or
   `t.Fatal("not implemented")`) and fail for the right reason.
2. **Green**: Write the minimum implementation to make the tests pass.
3. **Refactor**: Clean up while keeping tests green.

**Practical enforcement:**

- Each task below lists its test file(s) as the FIRST deliverable. The test
  file is written and committed (or at minimum, fully written) before the
  implementation file.
- Tests define the public API contract: function signatures, expected
  behavior, error conditions, edge cases. Implementation follows from tests.
- Table-driven tests are preferred for Go. Use `httptest.NewServer` for
  HTTP testing. Use temp files and `t.Setenv` for credential resolution
  tests.
- No implementation PR should be opened without corresponding test coverage.
- When a bug is found later, a regression test is written FIRST, then the
  fix is applied.

## Scope

**In scope:**

- Go project scaffolding (go.mod, Makefile, Containerfile, linter configs)
- YAML-based service configuration with Viper
- Auth providers: basic, bearer, token, custom-header
- Credential resolution from files (raw and JSON-path) and env vars
- HTTP proxy with SSRF prevention, timeouts, body size limits
- MCP server with `http_request` and `list_services` tools
- Cobra CLI with `serve` and `version` subcommands
- Filter interfaces (stubs only, no implementations)
- Credential scrubbing from all output
- Security hardening (see Security section)

**Out of scope:**

- Filter implementations (request enrichment, response transformation) — future plan
- Plugin loading system (go-plugin, WASM, etc.) — future plan, interfaces designed for it
- OAuth2 interactive flows — would require persistent state; MVP covers static tokens
- Config hot-reloading — per-invocation server reads config once at startup
- Rate limiting — belongs in a future operational hardening plan
- A2A protocol support — evaluated and deferred (see Context section)
- Remote/network transport — local stdio only per MCP spec

## Context and background

### What this solves

AI agents (Claude Code, etc.) frequently need to call authenticated APIs
(Jira, GitHub, GitLab, PagerDuty). Today, agents either need direct access
to credentials or rely on CLI tools (`gh`, `glab`, `http`) that may not
cover all API operations. shim-mcp centralizes credential management: the
agent knows the service name and request parameters, the server handles auth.

### MCP protocol

MCP uses JSON-RPC 2.0 over stdio. The agent client launches the server as a
subprocess, exchanges messages over stdin/stdout, and the server exits when
the client closes stdin. Tools are registered with JSON Schema input
validation. The official Go SDK is `github.com/modelcontextprotocol/go-sdk`.

### Auth patterns from existing configs

Analysis of supplemental configs reveals four auth patterns in active use:

| Service    | Auth Type      | Credential Source              | Header Format                        |
|------------|----------------|--------------------------------|--------------------------------------|
| Jira       | Basic          | JSON file (email + token)      | `Authorization: Basic base64(e:t)`   |
| GitHub     | Token          | Raw file                       | `Authorization: token <TOKEN>`       |
| GitLab     | Bearer         | CLI-managed / file             | `Authorization: Bearer <TOKEN>`      |
| PagerDuty  | Custom header  | Env var                        | `Authorization: Token token=<TOKEN>` |

Config file: `~/.config/claude/claude.json` (Jira), `~/.config/github/token`
(GitHub). Credentials are never stored in the shim-mcp config — only paths
to credential sources.

### A2A protocol evaluation

Evaluated Google's Agent-to-Agent (A2A) protocol for potential use as an
internal message format between agents and the MCP server. A2A provides rich
structured messaging (Parts, Artifacts, task lifecycle states, contextId)
designed for multi-agent coordination.

**Decision: Defer.** For the MVP use case (single agent, request-response
HTTP proxying), MCP's native tool call model maps 1:1 to HTTP
request/response. A2A's richer format would add a second protocol layer
without external interoperability gain, since the agent client speaks MCP
natively. A2A becomes worth revisiting when adding: multi-step workflows,
streaming responses, agent-to-agent delegation through the proxy, or
long-running async operations. A Go SDK exists
(`github.com/a2aproject/a2a-go/v2`) for future adoption.

**Predecessor Plans and Lessons Learned:**

No predecessor plans exist. This is the first plan in the repository.

## Approach

### Architecture

```
cmd/shim-mcp/
  main.go                 # entry point
internal/
  cli/
    root.go               # root cobra command, --config flag
    serve.go              # "serve" subcommand: load config, start MCP server
    version.go            # "version" subcommand
  config/
    config.go             # structs, Viper loading, validation
    config_test.go
  auth/
    auth.go               # AuthProvider interface, factory
    basic.go              # Basic auth (email:token)
    bearer.go             # Bearer/token auth
    header.go             # Custom header template auth
    credential.go         # Credential source resolution (file, env, json-path)
    *_test.go
  proxy/
    proxy.go              # HTTP request builder + executor
    scrub.go              # credential scrubbing from errors/headers
    *_test.go
  server/
    server.go             # MCP server setup, tool registration
    tools.go              # http_request and list_services handlers
    tools_test.go
  filter/
    filter.go             # RequestFilter/ResponseFilter interfaces (stubs)
```

### Data flow

```
Agent                    shim-mcp                         Target API
  │                         │                                 │
  │── tools/call ──────────→│                                 │
  │   {service, method,     │                                 │
  │    path, headers,       │── lookup service config ──→     │
  │    query_params, body}  │── resolve credentials ────→     │
  │                         │── build http.Request ─────→     │
  │                         │── inject auth headers ────→     │
  │                         │── apply request filters ──→     │
  │                         │                                 │
  │                         │── net/http Do() ──────────────→ │
  │                         │                                 │
  │                         │←─ http.Response ────────────── │
  │                         │── apply response filters ─→     │
  │                         │── scrub credentials ──────→     │
  │                         │                                 │
  │←─ CallToolResult ──────│                                 │
  │   {status_code,         │                                 │
  │    headers, body}       │                                 │
```

### Config format

```yaml
services:
  jira:
    base_url: "https://issues.example.com"
    auth:
      type: "basic"
      username_source: "file"
      username_path: "~/.config/claude/claude.json"
      username_json_path: ".jira.\"user-email\""
      token_source: "file"
      token_path: "~/.config/claude/claude.json"
      token_json_path: ".jira.token"
    default_headers:
      Content-Type: "application/json"
      Accept: "application/json"

  github:
    base_url: "https://api.github.com"
    auth:
      type: "token"
      token_source: "file"
      token_path: "~/.config/github/token"
    default_headers:
      Accept: "application/vnd.github.v3+json"

  pagerduty:
    base_url: "https://api.pagerduty.com"
    auth:
      type: "header"
      header_name: "Authorization"
      header_template: "Token token={{.Token}}"
      token_source: "env"
      token_env: "PAGERDUTY_TOKEN"
```

### MCP tools exposed

**`http_request`** — primary tool, accepts:

- `service` (string, required): configured service name
- `method` (string, required): HTTP method
- `path` (string): appended to base URL
- `headers` (object): additional headers
- `query_params` (object): query parameters
- `body` (string): request body

Returns: `status_code`, `headers` (scrubbed), `body`

**`list_services`** — discovery tool, no input. Returns service names and
base URLs. No credentials.

### Plugin extensibility design

Auth providers implement a Go interface:

```go
type AuthProvider interface {
    Name() string
    Authenticate(req *http.Request) error
}
```

New auth types are added by implementing this interface and registering in
the factory. For the MVP, providers are compiled-in. Future plans can add
dynamic loading (hashicorp/go-plugin, WASM, or config-driven script
execution).

Filter interfaces follow the same pattern:

```go
type RequestFilter interface {
    Name() string
    FilterRequest(req *http.Request) (*http.Request, error)
}

type ResponseFilter interface {
    Name() string
    FilterResponse(resp *http.Response) (*http.Response, error)
}
```

The proxy calls these in order if any are registered. For the MVP, the
slices are empty — the loop is a no-op, not dead code.

### Key dependencies

| Dependency                                     | Purpose              | Justification                |
|------------------------------------------------|----------------------|------------------------------|
| `github.com/modelcontextprotocol/go-sdk`       | MCP protocol         | Official SDK, Google-backed  |
| `github.com/spf13/cobra`                       | CLI framework        | User requirement             |
| `github.com/spf13/viper`                       | Config parsing       | User requirement             |
| `gopkg.in/yaml.v3`                             | YAML (via Viper)     | Transitive                   |
| stdlib `net/http`, `encoding/json`, `text/template`, `log/slog` | Core functionality | Stdlib-preferred per requirement |

### Security considerations

1. **Credential exposure**: Tokens NEVER appear in MCP responses, logs, or
   error messages. All `Authorization` headers scrubbed from returned
   response headers. Error messages run through credential scrubber.

2. **SSRF prevention**: Requests only allowed to configured `base_url`
   prefixes. No arbitrary URL requests. Redirect-following validates each
   hop against the base URL.

3. **Local-only**: Stdio transport only. No network listener.

4. **Credential file permissions**: Warn on stderr if config file
   permissions are more open than 0600.

5. **No credential storage in config**: Config stores PATHS to credentials,
   not credentials themselves. Credentials read at request time, not cached.

6. **Body size limits**: 10 MB max response body via `io.LimitReader`.

7. **Timeouts**: 30-second default HTTP timeout. No timeout = no default in
   `net/http`, so this is critical.

8. **Path traversal**: Credential file paths validated at config load —
   reject `..` components after tilde expansion.

9. **JSON-path injection**: The simplified JSON-path accessor only supports
   dot-notation with optional quoted keys. No expression evaluation.

10. **Logging**: stderr only (per MCP spec). Never includes credentials.

### Alternatives considered

**Alternative: Use A2A message format internally**

A2A's Part/Artifact/Task model provides richer structured messaging than
MCP's tool call model. This was evaluated but deferred because: (a) the MVP
use case is single-agent request-response, which maps cleanly to MCP tool
calls without a second protocol layer; (b) the agent client speaks MCP
natively, so adopting A2A internally would mean translating between two
formats with no external interoperability gain; (c) A2A's value emerges in
multi-agent coordination, streaming, and long-running tasks — none of which
are in MVP scope. A Go SDK exists for future adoption.

**Alternative: Always-on HTTP listener instead of per-invocation stdio**

Rejected because: (a) per-invocation is the standard MCP model and what
Claude Code expects; (b) a persistent listener introduces port management,
process lifecycle, and network exposure concerns; (c) credential handling is
simpler when the process is short-lived.

**Alternative: stdlib flag + yaml.v3 instead of Cobra + Viper**

Would reduce dependencies for a minimal CLI surface. Rejected per user
requirement to use Cobra + Viper. Viper does add value via config search
paths and env var binding.

## Tasks

### Task 1 — Scaffold Go project

- **Depends on:** none
- **Inputs:** CONVENTIONS.md, Go 1.26
- **Deliverables:** `go.mod`, directory tree, `Makefile`, `.golangci.yml`,
  `Containerfile`, `test/Containerfile.ci`, `.yamllint.yaml`,
  `.markdownlint.yaml`
- **Acceptance:** `make build` produces binary (once code exists); directory
  matches architecture; linter configs valid
- **Estimated effort:** S

### Task 2 — Implement config system (TDD)

- **Depends on:** Task 1
- **Inputs:** Config format spec above
- **Deliverables:**
  1. `internal/config/config_test.go` (FIRST — defines API contract)
  2. `internal/config/config.go` (implements to satisfy tests)
- **TDD sequence:**
  1. Write tests: valid YAML loads, missing base_url errors, missing auth
     type errors, unknown auth type errors, tilde expansion, path traversal
     rejection, empty config errors. Use temp files with known YAML content.
  2. Write stub config structs and `LoadConfig()` returning errors.
  3. Implement until all tests pass.
- **Acceptance:** All tests pass; `go test ./internal/config/...` green
- **Estimated effort:** M

### Task 3 — Implement auth providers (TDD)

- **Depends on:** Task 2
- **Inputs:** Auth patterns table, AuthProvider interface
- **Deliverables:**
  1. `internal/auth/credential_test.go` (FIRST)
  2. `internal/auth/credential.go`
  3. `internal/auth/auth_test.go`, `basic_test.go`, `bearer_test.go`,
     `header_test.go` (FIRST — one test file per provider)
  4. `internal/auth/auth.go`, `basic.go`, `bearer.go`, `header.go`
- **TDD sequence:**
  1. Write credential resolution tests: read raw file, read JSON-path from
     file (`.key`, `.nested.key`, `.key."quoted-key"`), read env var, missing
     file error, missing env var error, invalid JSON-path error. Use
     `t.TempDir()` and `t.Setenv()`.
  2. Implement credential resolution.
  3. Write auth provider tests: each provider tested with `httptest.NewServer`
     that asserts correct `Authorization` header. Test factory with valid and
     invalid auth types. Test that credential errors propagate.
  4. Implement auth providers.
- **Acceptance:** All tests pass; each provider verified against real HTTP
  handler expectations
- **Estimated effort:** M

### Task 4 — Implement HTTP proxy (TDD)

- **Depends on:** Task 3
- **Inputs:** Proxy architecture, security requirements
- **Deliverables:**
  1. `internal/proxy/scrub_test.go` (FIRST)
  2. `internal/proxy/scrub.go`
  3. `internal/proxy/proxy_test.go` (FIRST)
  4. `internal/proxy/proxy.go`
- **TDD sequence:**
  1. Write scrub tests: scrub Authorization header from response headers,
     scrub credential strings from error messages, handle nil/empty inputs.
  2. Implement scrubbing.
  3. Write proxy tests using `httptest.NewServer`:
     - Auth header injection (mock AuthProvider)
     - Header merging (default + caller + auth)
     - SSRF prevention (request to non-matching base URL rejected)
     - Redirect validation (redirect to different host rejected)
     - Timeout enforcement (slow server triggers timeout)
     - Body size limit (large response truncated)
     - URL construction (base_url + path joining)
     - Query parameter encoding
     - HTTP method validation
  4. Implement proxy.
- **Acceptance:** All tests pass; security properties verified by test
- **Estimated effort:** M

### Task 5 — Implement MCP server and tools (TDD)

- **Depends on:** Task 4
- **Inputs:** MCP SDK API, tool definitions
- **Deliverables:**
  1. `internal/server/tools_test.go` (FIRST)
  2. `internal/server/tools.go`
  3. `internal/server/server.go`
- **TDD sequence:**
  1. Write tool handler tests using in-memory MCP transport
     (`mcp.NewInMemoryTransports`):
     - `http_request`: valid request returns response, missing service
       errors, invalid method errors, response includes status/headers/body,
       credentials not in response
     - `list_services`: returns service names and base URLs, no credentials
       in output
  2. Implement tool handlers.
  3. Write server integration test: full initialize -> tools/list ->
     tools/call flow over in-memory transport.
  4. Implement server setup.
- **Acceptance:** Full MCP protocol flow tested end-to-end in-memory
- **Estimated effort:** M

### Task 6 — Implement Cobra CLI (TDD)

- **Depends on:** Task 5
- **Inputs:** CLI spec
- **Deliverables:**
  1. `internal/cli/root_test.go` (FIRST — test config flag parsing,
     version output)
  2. `cmd/shim-mcp/main.go`, `internal/cli/root.go`,
     `internal/cli/serve.go`, `internal/cli/version.go`
- **TDD sequence:**
  1. Write tests: version subcommand outputs version string, serve
     subcommand requires valid config, --config flag sets config path,
     invalid config path errors.
  2. Implement CLI commands.
- **Acceptance:** `shim-mcp serve --config path` starts MCP server on stdio;
  `shim-mcp version` prints version; default command is `serve`
- **Estimated effort:** S

### Task 7 — Define filter interfaces

- **Depends on:** none (parallel with other tasks)
- **Inputs:** Filter interface design
- **Deliverables:** `internal/filter/filter.go`
- **Acceptance:** Interfaces compile; proxy has nil-guarded hook points that
  are no-ops when filter slices are empty
- **Estimated effort:** S
- **Note:** No tests needed — these are interface definitions only with no
  implementation logic. Tests will be written when filter implementations
  are added in a future plan.

### Task 8 — Documentation, example config, plan document

- **Depends on:** Tasks 1-7
- **Inputs:** All above
- **Deliverables:** `docs/plans/shim-mcp-mvp.md`, updated `README.md`,
  `examples/config.yaml`
- **Acceptance:** Plan follows TEMPLATE.md format; README covers
  installation, configuration, security; example config demonstrates all
  four auth patterns
- **Estimated effort:** S

## Risks and mitigations

| Risk | Likelihood | Impact | Mitigation |
|------|------------|--------|------------|
| MCP SDK API changes in future versions | M | M | Pin version in go.mod; SDK is stable post-1.0 |
| Simplified JSON-path insufficient for some credential files | L | L | Document supported subset; full jq can be added later |
| Credential scrubbing misses edge cases | M | H | Scrub ALL Authorization headers from responses; use allowlist for returned headers, not denylist |
| Large upstream API responses exhaust memory | M | M | 10 MB LimitReader; return truncation notice |
| Agent sends malformed requests | H | L | SDK auto-validates via JSON Schema; explicit validation in handler as defense-in-depth |
| Config file with credentials paths readable by other users | M | H | Warn on stderr if permissions > 0600; document in README |

## Lessons Learned

<!--
  This section is EMPTY when the Plan is first drafted. It is populated
  AFTER execution.

  Each lesson should state:
  1. What happened
  2. Why it wasn't caught earlier
  3. What should change to prevent recurrence

  Categories:
  - GENUINE ERROR: A real bug, design flaw, or oversight (include)
  - PROCESS GAP: A failure mode that should have been caught earlier (include)
-->

Populated after execution. Do not fill in during initial drafting.
