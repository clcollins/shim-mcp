# Plan: Add Ollama and RamaLama services with auth type none

**Plan ID:** 2026-06-08-local-llm-services
**Status:** Completed
**Created:** 2026-06-08
**Supersedes:** none
**Related plans:** docs/plans/shim-mcp-mvp.md

## Goal

Add support for unauthenticated services by introducing `auth.type: none`,
then add Ollama and RamaLama as example service configurations.

## Success criteria

- [x] `auth.type: none` accepted in config without credential refs
- [x] No-op auth provider injects no headers
- [x] Ollama and RamaLama example configs added
- [x] All tests pass

## Scope

**In scope:**

- `type: none` auth provider
- Ollama and RamaLama example config entries

**Out of scope:**

- Streaming response support (both APIs default to streaming)
- Model lifecycle management (pull/push/create)

## Context and background

Ollama (port 11434) and RamaLama (port 8080) are unauthenticated local
model serving APIs. The value of adding them to shim-mcp is normalizing
access for agents and enabling request validation filters.

**Predecessor Plans and Lessons Learned:**

Predecessor: docs/plans/add-udm-pro.md. Lesson applied: write plan doc
before implementing.

## Approach

Add `none` to the valid auth types. The `noneProvider` implements
`AuthProvider` with a no-op `Authenticate` method. Config validation
skips credential ref checks for `none` type.

## Lessons Learned

Populated after execution. Do not fill in during initial drafting.
