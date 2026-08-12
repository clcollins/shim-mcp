# Plan: Surface auth failures as MCP errors with HTTP status codes

**Plan ID:** 2026-08-12-auth-failure-passthrough
**Status:** Proposed
**Created:** 2026-08-12
**Supersedes:** none
**Related plans:** shim-mcp-mvp.md, filter-pipeline.md

## Goal

When a remote API returns HTTP 401 or 403, return it as an MCP-level
error (`IsError: true`) with the HTTP status code, service name, and
upstream response body so the calling agent can identify expired or
invalid credentials and inform the user instead of attempting
workarounds.

## Success criteria

- [ ] HTTP 401 responses produce `IsError: true` with status code,
      service name, and response body in the error message
- [ ] HTTP 403 responses produce `IsError: true` with status code,
      service name, and response body in the error message
- [ ] Non-auth HTTP errors (404, 500, etc.) continue to return as
      normal responses (`IsError: false`)
- [ ] Successful responses (2xx) are unaffected
- [ ] Upstream response headers (scrubbed) are still available in the
      error context
- [ ] All existing tests pass, new tests cover 401/403 behavior
- [ ] Lint passes clean

## Scope

**In scope:**

- Classify HTTP 401 and 403 as MCP errors in `httpRequestHandler`
- Include HTTP status code, service name, and response body in error
- Tests for 401, 403, and non-auth error codes

**Out of scope:**

- Token refresh or re-authentication logic
- Response filter changes (auth classification happens in the handler,
  after filters run)
- Changes to the proxy layer (stays transport-agnostic)
- Handling other 4xx codes as errors (the agent can interpret those)

## Context and background

shim-mcp proxies authenticated HTTP requests on behalf of MCP tool
callers (Claude Code agents). When a credential expires or is invalid,
the remote API returns 401/403. Currently this comes back as a
successful MCP response (`IsError: false`) with the HTTP status code
buried in the structured `httpResponseOutput`. The calling agent often
fails to inspect the status code and either tries to work around the
error body or attempts to implement its own HTTP client.

By surfacing 401/403 as MCP-level errors, the agent immediately sees
the failure and can tell the user which service credential needs
attention.

**Predecessor Plans and Lessons Learned:**

- `shim-mcp-mvp.md` established the proxy and tool handler patterns.
  The handler currently returns all HTTP responses uniformly regardless
  of status code.
- `filter-pipeline.md` added request/response filters. Auth failure
  classification is intentionally placed after filters run (in the
  handler, not the proxy) to keep the proxy transport-agnostic and
  filters composable.

## Approach

Add auth failure detection in `httpRequestHandler` in
`internal/server/tools.go`, after `p.Do()` returns successfully but
before returning the `httpResponseOutput`. When the status code is
401 or 403, return an error with a structured message containing:

- The HTTP status code (e.g., 401, 403)
- The service name (so the user knows which credential to fix)
- The upstream response body (e.g., "Bad credentials", "Token expired")

This keeps the proxy layer clean and transport-agnostic — it still
returns all responses uniformly. The MCP-level policy decision lives
in the tool handler where it belongs.

### Alternatives considered

- **Response filter**: Could add a filter that converts 401/403 to
  errors. Rejected because filters modify `*http.Response` objects
  and return them — they don't produce MCP errors. This is a
  presentation concern for the MCP layer, not a response transformation.
- **Proxy-level error**: Could make `proxy.Do()` return an error on
  401/403. Rejected because the proxy should stay HTTP-transport-agnostic.
  Different MCP tools might want different policies on status codes.

## Tasks

### Task 1 — Write tests for auth failure detection

- **Depends on:** none
- **Inputs:** `internal/server/tools_test.go`
- **Deliverables:** Tests for 401, 403 returning `IsError: true` with
  status code and service name in message; test for 404 returning
  `IsError: false`
- **Acceptance:** Tests compile and fail (red)
- **Estimated effort:** S

### Task 2 — Implement auth failure detection in handler

- **Depends on:** Task 1
- **Inputs:** `internal/server/tools.go`
- **Deliverables:** Updated `httpRequestHandler` that returns MCP error
  on 401/403
- **Acceptance:** All tests pass (green), lint clean
- **Estimated effort:** S

## Risks and mitigations

| Risk | Likelihood | Impact | Mitigation |
|------|------------|--------|------------|
| Some APIs return 401 for non-auth reasons | L | L | Body is included so agent can distinguish |
| Breaking change for agents expecting IsError: false on 401 | L | M | Agents should handle IsError: true; this is the correct behavior |

## Lessons Learned

Populated after execution. Do not fill in during initial drafting.
