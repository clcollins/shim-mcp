![shim-mcp](img/header.png)

# shim-mcp

[![Go Version](https://img.shields.io/github/go-mod/go-version/clcollins/shim-mcp)](https://golang.org)
[![Build Status](https://github.com/clcollins/shim-mcp/actions/workflows/ci.yml/badge.svg)](https://github.com/clcollins/shim-mcp/actions/workflows/ci.yml)
[![codecov](https://codecov.io/gh/clcollins/shim-mcp/graph/badge.svg)](https://codecov.io/gh/clcollins/shim-mcp)
[![Go Report Card](https://goreportcard.com/badge/github.com/clcollins/shim-mcp)](https://goreportcard.com/report/github.com/clcollins/shim-mcp)
[![Go Reference](https://pkg.go.dev/badge/github.com/clcollins/shim-mcp.svg)](https://pkg.go.dev/github.com/clcollins/shim-mcp)
[![GitHub Release](https://img.shields.io/github/v/release/clcollins/shim-mcp)](https://github.com/clcollins/shim-mcp/releases)
[![License](https://img.shields.io/github/license/clcollins/shim-mcp)](https://github.com/clcollins/shim-mcp/blob/main/LICENSE)

A lightweight MCP server that lets AI agents make authenticated HTTP
requests without access to your credentials.

## Quick Start

### 1. Install

```bash
go install github.com/clcollins/shim-mcp/cmd/shim-mcp@latest
```

Verify it installed:

```bash
shim-mcp version
```

### 2. Create a config file

Create the config directory and file with secure permissions — set
permissions **before** writing any content:

```bash
mkdir -p ~/.config/shim-mcp
touch ~/.config/shim-mcp/config.yaml
chmod 600 ~/.config/shim-mcp/config.yaml
```

Edit `~/.config/shim-mcp/config.yaml` and add at least one service.
This GitHub example assumes your token is in `~/.config/github/token`:

```yaml
services:
  github:
    base_url: "https://api.github.com"
    auth:
      type: token
      token: {file: "~/.config/github/token"}
    headers:
      Accept: application/vnd.github.v3+json
```

See [Configuration](#configuration) below for all auth types and
credential source options.

### 3. Configure Claude Code

Add shim-mcp as an MCP server in Claude Code. You can configure it at
the user level (available in all projects) or the project level.

**User-level** (recommended — add to `~/.claude/settings.json`):

```json
{
  "mcpServers": {
    "shim-mcp": {
      "command": "shim-mcp",
      "args": ["serve", "--config", "/home/YOUR_USER/.config/shim-mcp/config.yaml"]
    }
  }
}
```

**Project-level** (add to `.claude/settings.json` in any repo):

```json
{
  "mcpServers": {
    "shim-mcp": {
      "command": "shim-mcp",
      "args": ["serve", "--config", "/home/YOUR_USER/.config/shim-mcp/config.yaml"]
    }
  }
}
```

Replace `/home/YOUR_USER/` with your actual home directory path. The
`--config` flag requires an absolute path — tilde expansion (`~`) is
handled inside the config file for credential paths, but the config
file path itself must be absolute.

### 4. Verify it works

Start a new Claude Code session. The agent should now have access to
two new tools:

- **`list_services`** — call this first to confirm your services are
  loaded. It returns service names and base URLs (no credentials).
- **`http_request`** — make an authenticated request. Try:

  ```text
  Use http_request to GET /user from the github service
  ```

  The agent will call shim-mcp, which injects your token and returns
  the GitHub API response.

If the tools don't appear, check that:

- `shim-mcp` is on your `$PATH` (run `which shim-mcp` in a terminal)
- The config file path in settings.json is absolute and correct
- The config file parses without errors (`shim-mcp serve --config
  /path/to/config.yaml` in a terminal should start without error
  messages on stderr)

---

## How It Works

```text
Agent ──[MCP stdio]──> shim-mcp ──[authenticated HTTP]──> API
                          │
                    reads credentials
                    from local files
                    or env vars
```

1. Claude Code launches shim-mcp as a subprocess via MCP stdio transport
2. The agent calls the `http_request` tool with a service name and
   request parameters
3. shim-mcp looks up the service, resolves credentials from a local
   file or environment variable, and injects the appropriate auth header
4. The HTTP request is made with Go's `net/http` stdlib
5. The response is returned with all credential headers scrubbed

Credentials never cross the MCP protocol boundary — they exist only
inside the server process.

## MCP Tools

### `http_request`

Make an authenticated HTTP request to a configured service.

| Parameter | Type | Required | Description |
| --------- | ---- | -------- | ----------- |
| `service` | string | yes | Configured service name |
| `method` | string | yes | HTTP method (GET, POST, PUT, DELETE, PATCH, HEAD) |
| `path` | string | no | Path appended to the service base URL |
| `headers` | object | no | Additional request headers |
| `query_params` | object | no | Query parameters |
| `body` | string | no | Request body |

Returns `status_code`, `headers` (scrubbed), and `body`.

### `list_services`

Lists configured services and their base URLs. No credentials are
exposed.

## Configuration

Services are defined in a YAML config file. Each service specifies a
base URL, an auth method, and where to find credentials.

```yaml
services:
  jira:
    base_url: "https://jira.example.com"
    auth:
      type: basic
      username: {file: "~/.config/app/creds.json", key: ".email"}
      token: {file: "~/.config/app/creds.json", key: ".token"}
    headers:
      Content-Type: application/json
```

### Auth types

| Type | Header format | Fields |
| ---- | ------------- | ------ |
| `basic` | `Authorization: Basic base64(user:token)` | `username` + `token` |
| `bearer` | `Authorization: Bearer <token>` | `token` |
| `token` | `Authorization: token <token>` | `token` |
| `header` | Custom via Go template | `token` + `header` + `template` |

### Credential references

Each credential (`username`, `token`) is an inline object pointing to
a source:

```yaml
# Raw text file
token: {file: "~/.config/github/token"}

# JSON file with key extraction (format inferred from .json extension)
token: {file: "~/.config/app/config.json", key: ".api.token"}

# YAML file with key extraction (format inferred from .yml extension)
token: {file: "~/.config/app/config.yml", key: ".credentials.token"}

# Explicit format override (when extension doesn't match content)
token: {file: "~/.config/app/credentials", format: json, key: ".token"}

# Environment variable
token: {env: "API_TOKEN"}
```

See [docs/configuration.md](docs/configuration.md) for the full
reference including key path syntax and validation rules.

## Security

- Credentials are resolved at request time, never cached or sent
  over MCP
- All `Authorization` headers are scrubbed from responses
- Requests are restricted to configured `base_url` prefixes (SSRF
  prevention)
- Stdio transport only — no network listener
- Config file should be `chmod 600`

See [docs/security.md](docs/security.md) for the full security model.

## Building from Source

```bash
# Clone
git clone https://github.com/clcollins/shim-mcp.git
cd shim-mcp

# Build
go build -o bin/shim-mcp ./cmd/shim-mcp

# Run tests
go test ./...

# Run tests with race detector
CGO_ENABLED=1 go test -race ./...

# Install to $GOPATH/bin
go install ./cmd/shim-mcp
```

## Project Conventions

This repository follows the conventions defined in
[CONVENTIONS.md](CONVENTIONS.md), covering CI, linting, container,
Makefile, and documentation standards.

## Plan Documents

Every meaningful change includes a plan document in
[docs/plans/](docs/plans/). See
[docs/plans/TEMPLATE.md](docs/plans/TEMPLATE.md) for the template.
