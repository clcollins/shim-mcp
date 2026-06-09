# Plan: Add UDM Pro service with tls_skip_verify and env file format

**Plan ID:** 2026-06-08-add-udm-pro
**Status:** Completed
**Created:** 2026-06-08
**Supersedes:** none
**Related plans:** docs/plans/shim-mcp-mvp.md

## Goal

Add UniFi UDM Pro as a shim-mcp service, requiring two new features:
per-service TLS verification skip for self-signed certs, and env-file
credential format support.

## Success criteria

- [x] `format: env` reads KEY=value files with quote stripping
- [x] `tls_skip_verify: true` creates per-service insecure HTTP client
- [x] UDM Pro health endpoint returns HTTP 200 via shim-mcp
- [x] All tests pass

## Scope

**In scope:**

- `tls_skip_verify` boolean in ServiceConfig
- `format: env` in CredentialRef
- UDM Pro example config entry

**Out of scope:**

- INI file support with sections
- Certificate pinning

## Context and background

The UDM Pro uses a self-signed TLS certificate and stores its API key
in a KEY=value env file. Neither capability existed in shim-mcp.

**Predecessor Plans and Lessons Learned:**

Predecessor: docs/plans/shim-mcp-mvp.md. No lessons learned applicable.

## Approach

Added `TLSSkipVerify` to ServiceConfig. When true, proxy creates a
per-service `http.Client` with `InsecureSkipVerify`. Added `env` format
to CredentialRef that parses KEY=value files with optional quote
stripping. Both features are generic and reusable.

## Lessons Learned

Populated after execution. Do not fill in during initial drafting.
