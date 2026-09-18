# Configuration Reference

shim-mcp reads its configuration from a YAML file. Pass the path with
`--config` or place it at `~/.config/shim-mcp/config.yaml`.

The config file should be `chmod 600` since it contains paths to credential
files.

## Structure

```yaml
log_level: <string>          # optional — debug, info, warn, error (default: error)

services:
  <service-name>:
    base_url: <string>        # required — API base URL (https)
    auth:                     # required — authentication configuration
      type: <string>          # required — basic, bearer, token, or header
      username: <credential>  # required for basic auth
      token: <credential>     # required for all auth types
      header: <string>        # required for header auth — HTTP header name
      template: <string>      # required for header auth — Go template
    headers:                  # optional — default headers sent with every request
      <Header-Name>: <value>
```

## Log Level

Controls logging verbosity. Logs are written to both stderr and the systemd
journal (when available). If journald is unavailable, stderr is the sole
output.

Precedence (highest to lowest):

1. `--log-level` CLI flag
2. `SHIM_MCP_LOG_LEVEL` environment variable
3. `log_level` config file key
4. Default: `error`

Valid values: `debug`, `info`, `warn`, `error` (case-insensitive).

## Auth Types

### basic

HTTP Basic Authentication. Resolves a username and token, encodes as
`base64(username:token)`, sets `Authorization: Basic <encoded>`.

```yaml
auth:
  type: basic
  username: {file: "~/.config/app/creds.json", key: ".email"}
  token: {file: "~/.config/app/creds.json", key: ".token"}
```

### bearer

Bearer token authentication. Sets `Authorization: Bearer <token>`.

```yaml
auth:
  type: bearer
  token: {file: "~/.config/app/token.yml", key: ".api.token"}
```

### token

GitHub-style token authentication. Sets `Authorization: token <token>`.

```yaml
auth:
  type: token
  token: {file: "~/.config/github/token"}
```

### header

Custom header with Go template. The template receives `{{.Token}}` as the
resolved token value.

```yaml
auth:
  type: header
  header: "Authorization"
  template: "Token token={{.Token}}"
  token: {env: "API_TOKEN"}
```

## Credential References

A credential reference (`<credential>`) tells shim-mcp where to read a
secret value. It is an inline YAML object with one source mode.

### From a file

```yaml
token: {file: "~/.config/app/token"}
```

Reads the file and returns the trimmed content. Tilde (`~`) is expanded to
the user's home directory. Paths containing `..` are rejected.

### From a structured file (JSON or YAML)

```yaml
token:
  file: "~/.config/app/config.json"
  key: ".nested.key"
```

The `format` is inferred from the file extension (`.json` -> JSON,
`.yaml`/`.yml` -> YAML). Use `format` to override inference when the
extension does not match the content:

```yaml
token:
  file: "~/.config/app/credentials"
  format: json
  key: ".api_key"
```

### From an environment variable

```yaml
token: {env: "API_TOKEN"}
```

Returns the value of the environment variable. Errors if unset or empty.

### Key path syntax

The `key` field uses a dot-path syntax to navigate structured files:

| Pattern | Meaning |
| ------- | ------- |
| `.token` | Top-level key `token` |
| `.api.token` | Nested key `api` -> `token` |
| `."user-email"` | Quoted key containing special characters |
| `.items[0].token` | Array index access |
| `.data[2]."my-key"` | Array index then quoted key |

The same syntax works for both JSON and YAML files.

### Caching

A credential read from a file is resolved on every proxied request, so a
rotated token is picked up without restarting shim-mcp. To avoid re-reading
and re-parsing the file when nothing has changed, the resolved value is
cached and the file is stat-ed instead. The cached value is discarded, and
the file read again, when any of the following differ from the read that
produced it:

- the file identity (device and inode — so a replacement installed by
  atomic rename is detected)
- the file size
- the modification time (compared for equality, so a timestamp moved
  backwards by a restore or a `cp -p` also invalidates the cache)

There are no background watchers and no polling: the check happens on the
next request that needs the credential.

A file rewritten in place with the same inode, the same length, and a
modification time the filesystem records as identical is not detected.
Detecting that case would require hashing the contents, which means
reading the file — the cost the cache exists to avoid. Set `cache: false`
to read the file on every request instead:

```yaml
token:
  file: "~/.config/app/token"
  cache: false
```

Credentials read from the environment are never cached — `env` references
are re-read from the process environment on every request, and `cache` is
not a valid field alongside `env`.

A stat failure — a deleted, renamed, or unreadable file — is returned as
an error. A cached value is never served in place of a file that can no
longer be read.

### Validation rules

- Exactly one of `file` or `env` must be set (not both, not neither)
- `key` and `format` are only valid with `file` (not `env`)
- `cache` is only valid with `file` (not `env`); it defaults to `true`
- `format` must be `json`, `yaml`, or `text` when set
- File paths must not contain `..` (path traversal prevention)
- The value at the resolved path must be a string

## Full Example

```yaml
services:
  jira:
    base_url: "https://jira.example.com"
    auth:
      type: basic
      username:
        file: "~/.config/app/jira.json"
        key: '.credentials."user-email"'
      token:
        file: "~/.config/app/jira.json"
        key: ".credentials.token"
    headers:
      Content-Type: application/json
      Accept: application/json

  github:
    base_url: "https://api.github.com"
    auth:
      type: token
      token: {file: "~/.config/github/token"}
    headers:
      Accept: application/vnd.github.v3+json

  gitlab:
    base_url: "https://gitlab.example.com/api/v4"
    auth:
      type: bearer
      token:
        file: "~/.config/glab-cli/config.yml"
        key: '.hosts."gitlab.example.com".token'

  pagerduty:
    base_url: "https://api.pagerduty.com"
    auth:
      type: header
      header: "Authorization"
      template: "Token token={{.Token}}"
      token: {env: "PAGERDUTY_TOKEN"}
    headers:
      Content-Type: application/json
```
