# Plan: Implement request/response filter pipeline

**Plan ID:** 2026-06-08-filter-pipeline
**Status:** Completed
**Created:** 2026-06-08
**Supersedes:** none
**Related plans:** docs/plans/shim-mcp-mvp.md

## Goal

Implement the filter framework and four generic filters that work for
any service, providing JSON validation, content-type detection, empty
body rejection, and configurable response field stripping.

## Success criteria

- [x] Filter interfaces updated with service context
- [x] Four filters implemented with TDD
- [x] Per-service filter configuration via YAML
- [x] Request logging to stderr
- [x] Overall test coverage >70%
- [x] `make ci-all` passes

## Scope

**In scope:**

- Filter interface with `Context` (service name, method, path)
- ValidateJSONBody, AutoContentType, RejectEmptyBody request filters
- StripFields response filter
- Per-service filter config in YAML
- Structured request logging via slog

**Out of scope:**

- Jira-specific filters (ADF validation, field format enforcement) — Phase 2/3
- Response field flattening — Phase 2
- ADF-to-markdown conversion — Phase 2

## Context and background

The shim-mcp proxy had empty filter interfaces wired into the pipeline
but no implementations. Issue #5 defined three phases; this implements
Phase 1.

**Predecessor Plans and Lessons Learned:**

Predecessor: docs/plans/shim-mcp-mvp.md. Lesson applied: the filter
interfaces needed service context to be useful — the original
context-free interfaces couldn't distinguish which service a request
targeted.

## Approach

Four filter implementations, each as a separate file with its own test
file. Filters are configured per-service via YAML and built at proxy
initialization. The proxy stores filters as `map[string][]filter.X`
keyed by service name.

## Lessons Learned

Populated after execution. Do not fill in during initial drafting.
