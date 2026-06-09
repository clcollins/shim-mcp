# Plan: Add OCM service to example config

**Plan ID:** 2026-06-08-add-ocm-service
**Status:** Completed
**Created:** 2026-06-08
**Supersedes:** none
**Related plans:** docs/plans/shim-mcp-mvp.md

## Goal

Add the OpenShift Cluster Manager (OCM) API as an example service
configuration, demonstrating Bearer token auth from a JSON credential file.

## Success criteria

- [x] OCM service entry added to `examples/config.yaml`
- [x] Verified against live OCM API (HTTP 200 with fresh token)

## Scope

**In scope:**

- Example config entry for OCM

**Out of scope:**

- Token refresh automation (tracked in issue #6)
- OCM-specific request/response filters (future work)

## Context and background

OCM is the control plane API for managed OpenShift (ROSA, OSD). The OCM
CLI stores its OAuth2 Bearer token at `~/.config/ocm/ocm.json`. The
access token expires every ~15 minutes and is refreshed by srepd or
ocm-container.

**Predecessor Plans and Lessons Learned:**

Predecessor: docs/plans/shim-mcp-mvp.md. No lessons learned applicable
to this change.

## Approach

Add a standard Bearer token service entry pointing to the OCM config
file with key `.access_token`. No code changes required — the existing
CredentialRef with JSON format inference handles this.

### Alternatives considered

**Implement token refresh first:** Rejected — the static file approach
works when srepd or ocm-container has recently refreshed the token.
Token refresh is tracked separately in issue #6.

## Tasks

### Task 1 — Add OCM to examples/config.yaml

- **Depends on:** none
- **Inputs:** OCM API base URL, credential file location
- **Deliverables:** Updated `examples/config.yaml`
- **Acceptance:** Valid YAML, service entry follows existing patterns
- **Estimated effort:** S

## Risks and mitigations

| Risk | Likelihood | Impact | Mitigation |
| ---- | ---------- | ------ | ---------- |
| Token expired when user tries example | H | L | Clear note in docs about token staleness |

## Lessons Learned

Populated after execution. Do not fill in during initial drafting.
