# Plan: Expand ~ and $HOME in credential file paths

**Plan ID:** 2026-09-03-expand-credential-file-paths
**Status:** Proposed
**Created:** 2026-09-03
**Supersedes:** none
**Related plans:** docs/plans/shim-mcp-mvp.md, docs/plans/add-ocm-service.md

## Goal

Allow the on-disk path *values* inside `config.yaml` — specifically the
`auth.*.file` credential references — to use `~/`, `$HOME`, and `${HOME}`
so users can write portable, home-relative credential paths instead of
absolute ones. `~/` is already supported; this Plan adds `$HOME` and
`${HOME}` and keeps them all resolving to the same home directory.

## Success criteria

- [ ] `auth.*.file` values `~/x`, `$HOME/x`, and `${HOME}/x` all resolve
      to `<home>/x`
- [ ] Other environment variables (e.g. `$FOO`) are left literal, NOT
      expanded
- [ ] A `$HOME`/`${HOME}` value that expands to a path containing `..`
      is still rejected by the traversal guard
- [ ] Existing `~/` and path-traversal behavior is unchanged
- [ ] `make test` and `make lint` pass

## Scope

**In scope:**

- Expansion of `~`, `$HOME`, and `${HOME}` in `CredentialRef.File`
  (`auth.*.file`) — the only on-disk path value inside the config file
- Documentation of the supported forms in `README.md` and
  `examples/config.yaml`

**Out of scope:**

- Expansion of arbitrary environment variables (only `HOME` is honored,
  per explicit decision)
- The `--config` path to the config file itself (only the config file
  *contents* are in scope)
- `base_url`, `headers`, `auth.template`, `auth.header` — these are not
  on-disk paths and are excluded

## Context and background

`CredentialRef.expandAndValidatePath()` in `internal/config/config.go`
is the single choke-point every credential path passes through before
`os.ReadFile`. It already performs `~/` expansion, a `..` traversal
guard, and `filepath.Clean`. This Plan adds `$HOME`/`${HOME}` handling
in the same function. See CONVENTIONS.md for TDD and Go standards.

**Predecessor Plans and Lessons Learned:**

- docs/plans/shim-mcp-mvp.md — introduced the CredentialRef design and
  the `~/`/traversal-guard logic. No lessons applicable.
- docs/plans/add-ocm-service.md — relies on home-relative credential
  paths (`~/.config/ocm/ocm.json`); this Plan makes `$HOME` equivalent.
  No lessons applicable.

## Approach

In `expandAndValidatePath()`, before the existing traversal guard,
expand only the `HOME` variable using `os.Expand` with a custom mapping
that returns the home directory for `HOME` and leaves any other `$VAR`
literal. Resolve the home directory once and reuse it for both the
`$HOME`/`${HOME}` expansion and the existing `~/` prefix expansion.

```go
func (ref *CredentialRef) expandAndValidatePath() error {
    if ref.File == "" {
        return nil
    }

    // Resolve home lazily, at most once, and only when a form that
    // actually needs it is present.
    var home string
    var homeResolved bool
    var homeErr error
    getHome := func() string {
        if !homeResolved {
            home, homeErr = os.UserHomeDir()
            homeResolved = true
        }
        return home
    }

    ref.File = os.Expand(ref.File, func(name string) string {
        if name == "HOME" {
            return getHome()
        }
        return "$" + name // leave other vars literal
    })

    // Concatenate (not filepath.Join) so a "~/" whose home contains ".."
    // is not Clean'd before the traversal guard runs.
    if strings.HasPrefix(ref.File, "~/") {
        ref.File = getHome() + ref.File[1:]
    }

    if homeErr != nil {
        return fmt.Errorf("expanding home directory: %w", homeErr)
    }

    if strings.Contains(ref.File, "..") {
        return fmt.Errorf("path traversal not allowed: %q", ref.File)
    }

    ref.File = filepath.Clean(ref.File)
    return nil
}
```

Order is deliberate: both `$HOME` and `~/` are expanded **before** the
`..` guard so a home directory value containing `..` cannot bypass the
check. Home is resolved lazily via `getHome()` — called only when a
`HOME` token or a `~/` prefix is actually present — so a path like
`$HOMEDIR/token` (parsed by `os.Expand` as the variable `HOMEDIR`, not
`HOME`) never triggers a home lookup or its error.

### Alternatives considered

**Use `os.ExpandEnv` (expand all env vars):** Rejected by explicit
decision — only `HOME` should be honored to keep the blast radius small
and avoid surprising expansion of unrelated variables.

**$HOME-only via `os.ExpandEnv` then re-guard:** Rejected — `ExpandEnv`
would still expand every variable; `os.Expand` with a custom mapping is
the correct primitive for a single-variable allowlist.

## Tasks

### Task 1 — Write failing tests

- **Depends on:** none
- **Inputs:** `internal/config/config_test.go`
- **Deliverables:** New test cases for `$HOME`, `${HOME}`, other-var
  literal, and home-with-`..` rejection
- **Acceptance:** Tests compile and fail against current code
- **Estimated effort:** S

### Task 2 — Implement expansion

- **Depends on:** Task 1
- **Inputs:** `internal/config/config.go`
- **Deliverables:** Updated `expandAndValidatePath()`
- **Acceptance:** All tests in Task 1 pass; existing tests unchanged
- **Estimated effort:** S

### Task 3 — Documentation

- **Depends on:** Task 2
- **Inputs:** `README.md`, `examples/config.yaml`
- **Deliverables:** Note that `auth.*.file` supports `~/`, `$HOME`,
  `${HOME}`
- **Acceptance:** Docs mention all three forms; example remains valid
- **Estimated effort:** S

### Task 4 — Verify

- **Depends on:** Task 2, Task 3
- **Deliverables:** Passing `make test` and `make lint`
- **Acceptance:** Both targets green
- **Estimated effort:** S

## Risks and mitigations

| Risk | Likelihood | Impact | Mitigation |
| ---- | ---------- | ------ | ---------- |
| Home value with `..` bypasses guard | L | H | Expand HOME before the traversal guard; explicit security test |
| Other env vars unexpectedly expanded | L | M | Custom `os.Expand` mapping honors only `HOME`; test asserts other vars stay literal |
| `os.UserHomeDir()` fails on exotic hosts | L | L | Error only surfaced when the path actually uses home |

## Lessons Learned

### Substring precheck for `$HOME` caused a false-positive error path

1. **What happened:** The first implementation gated home resolution on a
   `usesHome` precheck using `strings.Contains(ref.File, "$HOME")`. This
   matches `$HOMEDIR` as a substring, but `os.Expand` parses that token as
   the variable `HOMEDIR` and leaves it literal. On a host where
   `os.UserHomeDir()` fails (no `HOME` set), a path like `$HOMEDIR/token`
   would have errored even though it never references `$HOME`.
2. **Why it wasn't caught earlier:** The initial tests only covered the
   exact `$HOME`/`${HOME}` tokens and the happy path where home resolves.
   No test exercised a longer variable name that shares the `$HOME`
   prefix, and no test ran with `HOME` unset. Caught in code review.
3. **What should change:** Resolve `home` lazily inside the `os.Expand`
   mapping closure, keyed on the exact parsed variable name
   (`name == "HOME"`), rather than a substring precheck. Added
   `TestCredentialRef_HomeSubstringVarNotTreatedAsHome` to lock this in.

### `~/` expansion via `filepath.Join` silently Clean'd away `..`

1. **What happened:** The `~/` branch used `filepath.Join(home, ...)`,
   which calls `filepath.Clean` internally. If home resolved to a value
   containing `..` (e.g. `/etc/../etc`), the traversal guard that runs
   afterward would never see the `..` — it had already been collapsed.
   The `$HOME` traversal test passed, masking the gap for the `~/` form.
2. **Why it wasn't caught earlier:** The traversal-rejection test used the
   `$HOME` form only; there was no equivalent test for `~/`.
3. **What should change:** Expand `~/` by string concatenation so the
   guard sees the raw `..`, then `filepath.Clean` at the very end. Added
   `TestCredentialRef_TildeExpansionTraversalRejected`.

### Misread CI coverage from an incomplete grep

1. **What happened:** During review I claimed `.github/workflows/ci.yml`
   "runs only `go test`" and that markdownlint/golangci-lint were not in
   CI. That was wrong: `ci.yml` has dedicated jobs for Lint (`make lint`),
   Markdown Lint (`make markdown-lint`), YAML Lint, Format Check, Vet,
   Race, Coverage, and Plan Docs Check. Relying on CI for markdown lint is
   in fact valid.
2. **Why it wasn't caught earlier:** I grepped `ci.yml` for a narrow set
   of terms that happened to match only the coverage job's `go test` line
   and drew a conclusion from that single hit instead of reading the
   whole workflow.
3. **What should change:** When asserting what CI does or does not do,
   read the full workflow file rather than inferring from a partial grep.
