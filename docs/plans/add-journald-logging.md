# Plan: Add journald logging with configurable log level

**Plan ID:** 2026-06-10-add-journald-logging
**Status:** Implemented
**Created:** 2026-06-10
**Supersedes:** none
**Related plans:** shim-mcp-mvp.md

## Goal

Add persistent logging to the systemd journal so errors can be reviewed
after the MCP session ends. Change the default log level from Info to Error
and make it configurable via CLI flag, environment variable, or config file.

## Success criteria

- [x] Logs written to both journald and stderr when journald is available
- [x] Falls back to stderr-only when journald is unavailable (macOS, containers)
- [x] Default log level is Error (was Info)
- [x] `--log-level` flag, `SHIM_MCP_LOG_LEVEL` env var, and `log_level` config key work
- [x] All existing tests pass, new tests added for logging and config
- [x] Lint passes clean

## Scope

**In scope:**

- journald integration via `github.com/systemd/slog-journal`
- Configurable log level (CLI flag, env var, config file)
- Logger centralization (one logger created in serve.go, injected into proxy)
- Documentation updates

**Out of scope:**

- Log rotation (journald handles this)
- Structured log output format changes beyond journald key uppercasing
- Log file output (journald replaces the need for file-based logging)

## Context and background

shim-mcp runs as an MCP stdio subprocess launched by Claude Code. Stdout is
the MCP protocol channel; stderr is captured by Claude Code but disappears
when the session ends. Errors during credential resolution, HTTP requests,
or filter execution are invisible after the session closes.

The systemd journal persists across sessions and supports structured
queries via `journalctl`.

**Predecessor Plans and Lessons Learned:**

- `shim-mcp-mvp.md` established the initial logging pattern using `log/slog`
  with `TextHandler` on stderr at Info level. This plan builds on that by
  adding journald as a second output and making the level configurable.

## Approach

### Multi-handler architecture

Created `internal/logging` package with a `multiHandler` that fans out
`slog.Handler.Handle()` calls to both a journald handler and a stderr
handler. The journald handler uses `ReplaceAttr` to uppercase attribute
keys (journald requires `^[A-Z_][A-Z0-9_]*$`).

### Two-phase level initialization

Logger is created before config loads (to log config errors), using
`slog.LevelVar` for dynamic level adjustment. After config loads, the
level can be updated from the config file if no CLI/env override was given.

### Alternatives considered

- **stderr-only with systemd service capture**: Rejected — shim-mcp is a
  subprocess, not a systemd service. stderr goes to Claude Code, not journald.
- **File-based logging**: Rejected — user specifically wanted `journalctl`.
- **coreos/go-systemd**: Rejected — not an slog handler, known socket leak.

## Tasks

### Task 1 — Add slog-journal dependency

- **Depends on:** none
- **Deliverables:** Updated go.mod and go.sum
- **Acceptance:** `go build ./...` succeeds
- **Estimated effort:** S

### Task 2 — Add LogLevel to Config struct

- **Depends on:** none
- **Deliverables:** `LogLevel` field with validation, tests
- **Acceptance:** Tests pass for valid/invalid/empty levels
- **Estimated effort:** S

### Task 3 — Create internal/logging package

- **Depends on:** Task 1
- **Deliverables:** `ParseLevel`, `NewLogger`, `multiHandler`, tests
- **Acceptance:** All tests pass
- **Estimated effort:** M

### Task 4 — Inject logger into Proxy

- **Depends on:** Task 3
- **Deliverables:** Updated `proxy.New()` signature, updated callers and tests
- **Acceptance:** All tests pass
- **Estimated effort:** S

### Task 5 — Wire --log-level flag and centralized logger

- **Depends on:** Task 4
- **Deliverables:** Updated serve.go with flag, env, config precedence
- **Acceptance:** Build succeeds, all tests pass, lint clean
- **Estimated effort:** M

### Task 6 — Update documentation

- **Depends on:** Task 5
- **Deliverables:** Updated docs/configuration.md, examples/config.yaml, this plan
- **Acceptance:** Documentation reflects new config key and behavior
- **Estimated effort:** S

## Risks and mitigations

| Risk | Likelihood | Impact | Mitigation |
|------|------------|--------|------------|
| slog-journal pre-1.0 API changes | M | L | Pinned version; wrapper isolates dep |
| Journald unavailable in CI | L | L | Graceful fallback is the design |
| Default error level hides startup messages | L | M | Intentional; --log-level info restores |

## Lessons Learned

Populated after execution. Do not fill in during initial drafting.
