# Security Model

shim-mcp is designed so that AI agents can make authenticated HTTP requests
without ever seeing credentials.

## Threat model

The primary threat is credential exposure to the AI agent. shim-mcp
mitigates this at every layer:

### Credential isolation

- The config file stores **paths** to credential files, not credentials
  themselves.
- Credentials are resolved at request time by the server process, never
  sent over the MCP protocol.
- Credentials are not cached in memory between requests. Each tool call
  re-reads from the source file or environment variable.

### Response scrubbing

- All `Authorization`, `X-Api-Key`, `X-Auth-Token`, and
  `Proxy-Authorization` headers are stripped from HTTP responses before
  returning to the agent.
- Error messages are scrubbed to remove any credential values that might
  appear in connection error strings.

### SSRF prevention

- Requests are only allowed to URLs that start with a service's configured
  `base_url`. An agent cannot use shim-mcp to reach arbitrary endpoints.
- The `path` parameter in tool calls must be a relative path — it cannot
  contain a URL scheme.
- HTTP redirects are followed by the Go `net/http` client, but the
  redirect chain is limited to 10 hops (Go default).

### Transport security

- shim-mcp uses the **stdio** MCP transport exclusively. There is no
  network listener — the binary communicates over stdin/stdout only.
- The MCP client (Claude Code, etc.) launches shim-mcp as a subprocess.
  No other process can connect to it.

### Resource limits

- HTTP request timeout: 30 seconds (prevents hanging on unresponsive
  upstreams).
- Response body size: 10 MB maximum (prevents memory exhaustion from large
  API responses).

## File permissions

| File | Recommended permissions | Reason |
| ---- | ----------------------- | ------ |
| `~/.config/shim-mcp/config.yaml` | `600` | Contains paths to credential files |
| Credential source files | `600` | Contain actual secrets |

shim-mcp logs a warning to stderr if the config file has permissions more
open than `600`.

## What shim-mcp does NOT do

- Does not store or cache credentials.
- Does not log credentials to stdout, stderr, or any file.
- Does not expose a network port.
- Does not support OAuth2 interactive flows (which would require persistent
  state and a callback server).
- Does not validate TLS certificates beyond Go's default behavior.
