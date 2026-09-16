# Plan: Re-read credential files only when they change on disk

**Plan ID:** 2026-09-16-credential-cache-on-change
**Status:** Proposed
**Created:** 2026-09-16
**Supersedes:** none
**Related plans:** shim-mcp-mvp.md, auth-failure-passthrough.md

## Goal

Stop re-reading and re-parsing credential files on every proxied request
while keeping the property that a credential rotated on disk is picked up
by the next request. A file-backed `CredentialRef` caches the value it
resolved along with the identity and metadata of the file it came from;
the next `Resolve()` stats the file and re-reads only when that stat no
longer matches. No background goroutines, no file watchers, no polling.

## Success criteria

- [ ] A file-backed credential resolved twice with no change on disk is
      read from disk once
- [ ] A credential file rewritten between two requests resolves to the
      new value on the next request, with no process restart
- [ ] A credential file replaced by atomic rename resolves to the new
      value even when the replacement carries the same size and mtime
- [ ] A deleted or unreadable credential file produces an error — never
      a stale cached value
- [ ] Environment-backed credentials are never cached
- [ ] `cache: false` on a credential reference restores per-request reads
- [ ] Concurrent `Resolve()` calls are race-detector clean
- [ ] `make test-all` passes (fmt, vet, lint, test, test-race)

## Scope

**In scope:**

- Caching of resolved file-backed credentials in `internal/config`
- Staleness detection from `os.Stat` metadata (file identity, size, mtime)
- An opt-out `cache` field on `CredentialRef`
- Tests and configuration documentation

**Out of scope:**

- Token *refresh* — shim-mcp does not mint, renew, or exchange
  credentials; it reads what another tool wrote
- File watchers (`fsnotify`) or a background reload goroutine — a process
  that sits idle between requests gains nothing from watching, and a
  watcher is a lifecycle and fd cost per credential file
- Per-service refresh intervals — a timer is strictly worse than a stat:
  it needs tuning and still serves stale values inside the interval
- Any change to `internal/auth` — the providers keep calling
  `ref.Resolve()` per request and are unaware of the cache

## Context and background

Every provider in `internal/auth` stores the `CredentialRef` from the
config rather than a resolved value, and calls `ref.Resolve()` inside
`Authenticate()` — so each proxied request reads the credential file from
disk and re-parses it. That is what makes rotation work today without a
restart, and this Plan does not change it: an expired token in a
long-lived `shim-mcp` process was never a caching problem, and the fix
for it is not in this code path.

What per-request reading does cost is a read plus a YAML or JSON parse of
the whole credential file on every request, for a value that changes
perhaps once a day. Files like `~/.config/glab-cli/config.yml` are
parsed in full to pull one string out of them. Replacing the read with a
single `os.Stat` when nothing changed keeps rotation working and drops
the steady-state cost.

The correctness question this raises is what counts as "nothing changed".
Modification time alone is not enough:

- A file restored from a backup, or copied with `cp -p` / `rsync -t`, can
  carry an *older* mtime than the value already cached.
- A credential rewritten twice inside the filesystem's timestamp
  granularity keeps its mtime.
- An atomic `rename(2)` — how most credential writers publish a new file
  — can install a different file with a preserved mtime.

So the cached entry records the whole `os.FileInfo` and is trusted only
when `os.SameFile` (device and inode), size, and mtime all match the
current stat. Mtime is compared for *equality*, not "is newer than",
so a backwards-moving timestamp invalidates the entry like any other
change.

**Predecessor Plans and Lessons Learned:**

- `shim-mcp-mvp.md` established `CredentialRef` and the resolve-per-request
  auth providers. It recorded no lessons bearing on credential reads.
- `auth-failure-passthrough.md` made upstream 401/403 surface as MCP
  errors so an agent can tell the user which credential needs attention.
  That is the reporting half of credential expiry; this Plan touches the
  reading half, and the two do not interact: a stale credential still
  surfaces as a 401 with the service name.
- Neither predecessor recorded a lesson that this Plan must incorporate.

## Approach

Add an unexported `fileCache *credentialCache` to `CredentialRef`,
holding the last resolved value and the `os.FileInfo` it was read from,
guarded by a mutex.

`Resolve()` keeps its signature and its behavior for environment-backed
refs (always `os.Getenv`, never cached). For a file-backed ref with a
cache attached it stats the file, returns the cached value when the stat
matches, and otherwise re-reads, re-parses, and stores the new value with
the new stat. A stat error is returned as an error; the cached value is
not served as a fallback. A parse error leaves the cache untouched, so
the error is not sticky and a stale value is never returned for a file
whose contents have changed.

The stat happens *before* the read. Should the file change in between,
the value returned is the fresh content and the recorded stat is already
out of date, so the next call re-reads — the conservative direction.

The cache is allocated once per ref during `LoadConfig`, in
`validateCredentialRef`, before the ref is copied into the service map.
Every later copy of the ref — into `ServiceConfig`, into `AuthConfig`,
into an auth provider — carries the same pointer and shares one cache.
A `CredentialRef` built outside `LoadConfig` (tests, direct construction)
has a nil cache and reads on every call, exactly as today.

A `cache: false` field on the credential reference skips the allocation
and restores per-request reads. It is a `*bool` so that unset means
enabled.

### Alternatives considered

- **`sync.RWMutex` as a value field on `CredentialRef`** — the shape the
  first draft of this design proposed. It does not build under this
  repo's CI: `CredentialRef` is copied by value in twenty places
  (`viper.Unmarshal`, the `cfg.Services[name] = svc` write-back in
  `LoadConfig`, `NewAuthProvider(cfg config.AuthConfig)`, and every
  provider constructor), and `go vet`'s copylocks analyzer rejects every
  one of them. Verified: adding the field produces 20 vet errors and
  fails the `vet` CI job. Holding the mutex behind a pointer keeps the
  struct copyable and gives copies a shared cache rather than twenty
  independent ones.
- **Package-level cache keyed by file path** — would also avoid
  copylocks, and would share one entry between services that read the
  same file. Rejected for the global mutable state and the test-ordering
  coupling it creates; the per-ref cache is scoped to the config that
  owns it.
- **Caching mtime only, per the original sketch** — rejected for the
  three staleness cases above. Comparing the full `FileInfo` costs
  nothing extra: the stat has already been made.
- **`fsnotify` watcher per credential file** — rejected as out of scope
  above.
- **Leaving per-request reads in place** — the honest baseline. It is
  already correct; this Plan is an optimization, and the `cache: false`
  escape hatch exists so an operator can choose it back.

## Tasks

### Task 1 — Extract credential parsing from `Resolve()`

- **Depends on:** none
- **Inputs:** `internal/config/config.go`
- **Deliverables:** `parseCredential([]byte)` holding the format
  inference, raw-text, env-file, JSON, and YAML paths; the duplicated
  empty-key branch in `Resolve()` collapsed into one
- **Acceptance:** Existing config tests pass unchanged
- **Estimated effort:** S

### Task 2 — Add the stat-checked cache

- **Depends on:** Task 1
- **Inputs:** `internal/config/config.go`
- **Deliverables:** `credentialCache` type; `fileCache` field;
  `Resolve()` stat-check path; cache allocation in
  `validateCredentialRef`
- **Acceptance:** `go vet` and `golangci-lint` clean; existing tests pass
- **Estimated effort:** M

### Task 3 — Add the `cache` opt-out

- **Depends on:** Task 2
- **Inputs:** `internal/config/config.go`
- **Deliverables:** `Cache *bool` field; validation rejecting `cache`
  alongside `env`
- **Acceptance:** `cache: false` yields a ref that reads every time
- **Estimated effort:** S

### Task 4 — Tests

- **Depends on:** Tasks 2, 3
- **Inputs:** `internal/config/config_test.go`
- **Deliverables:** Tests for cache hit, rewrite, atomic rename with
  preserved metadata, size change, backwards mtime, deletion, parse
  error recovery, env never cached, `cache: false`, `LoadConfig` wiring,
  and concurrent resolution
- **Acceptance:** `make test` and `make test-race` pass; timestamps in
  tests are set explicitly with `os.Chtimes` rather than slept for
- **Estimated effort:** M

### Task 5 — Document the behavior

- **Depends on:** Task 3
- **Inputs:** `docs/configuration.md`, `README.md`
- **Deliverables:** A credential caching section covering what
  invalidates a cached value, the `cache` field, and the known
  same-metadata limitation
- **Acceptance:** `make markdown-lint` passes
- **Estimated effort:** S

## Risks and mitigations

| Risk | Likelihood | Impact | Mitigation |
|------|------------|--------|------------|
| A rewrite in place with identical inode, size, and mtime is not detected | L | M | Documented; `cache: false` opts out. Detecting it needs a content hash, which means reading the file — the cost this Plan removes |
| A writer truncates and rewrites a file non-atomically; a request stats it mid-write | L | M | Pre-existing behavior, unchanged by caching: a short read fails to parse and returns an error, and the cache is not updated, so the next request retries |
| Cached value held after the file becomes unreadable | L | H | Stat errors are returned, never masked by the cache |
| Cache pointer shared by copies is mutated concurrently | M | H | One mutex per cache, covering the whole stat-check-read-store sequence; a concurrency test runs under `make test-race` |
| A `CredentialRef` constructed outside `LoadConfig` silently loses caching | M | L | Behavior is identical to today, only slower; `LoadConfig` is the only production construction path |

## Lessons Learned

Populated after execution. Do not fill in during initial drafting.
