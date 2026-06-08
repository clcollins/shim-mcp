# shim-mcp Agent

Agent instructions for the shim-mcp repository.

## Role

Assist with development, documentation, and maintenance of this project.

## Scope

This repository only. All file paths are relative to the repository root.

## Capabilities

- Read, create, and modify source code and configuration files
- Run tests, linters, and build commands via `make` targets
- Create branches, commits, and pull requests (with user approval)
- Create and update plan documents in `docs/plans/`

## Boundaries

- **Create-only posture**: agents may only create — new files, new commits,
  new PRs, new comments. Agents must NOT delete files, force-push, rewrite
  history, or modify infrastructure without explicit human approval.
- Do not commit secrets, tokens, keys, credentials, or PII in any artifact.
- Do not merge PRs — the user merges.
- Do not expand scope beyond the assigned task. If a task seems to require
  out-of-scope changes, report back rather than improvising.

## Conventions

Follow `CONVENTIONS.md` in this repository for all CI, linting, container,
Makefile, documentation, and version control standards.

### Plan documents

Every meaningful change must have an associated plan document in
`docs/plans/`. Use `docs/plans/TEMPLATE.md` as the starting point.

- Plan documents use descriptive filenames (e.g., `add-mcp-server.md`),
  not numeric prefixes.
- Plans must reference predecessor plans in the same directory and
  incorporate their Lessons Learned.
- Superseded plans are preserved with a note at the top pointing to
  the replacement.
- Every PR should include or reference a `docs/plans/<slug>.md` explaining
  what it accomplishes, why, and in what context.

### Lessons Learned

Lessons Learned are appended to the plan document that caused them, not
filed separately. They are living annotations that inform future plans.

A Lesson Learned is recorded when:

- A genuine error is discovered (bug, design flaw, oversight)
- A process gap allowed a failure to reach a later stage than it should have

A Lesson Learned is NOT:

- A best-practice suggestion without a concrete failure
- A style preference

Each lesson states: (1) what happened, (2) why it wasn't caught earlier,
(3) what should change to prevent recurrence.

### Context isolation

When multiple agents review or work on this repository, each operates
from its own perspective. Agents do not inherit assumptions from other
agents' sessions. Each agent forms its own assessment from the code,
the plan documents, and the conventions — not from another agent's
internal reasoning.

This isolation prevents shared blind spots. A bug that one agent misses
has additional chances to be caught by an independent reviewer.
