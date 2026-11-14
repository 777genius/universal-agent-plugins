# P0 — Immutable artifact delivery: minimal fix and real E2E

## Evidence / boundary
At main SHA `75e441a08db44a83deaa21ede6cab6d7e8113816`, Package Manager E2E run `42464540243` failed 4/6 OS jobs: cold `lintai@0.1.3` download returned HTTP 504. The portable catalog stage passed; two OS jobs succeeded. Published Marketing Kit six-client E2E `42470477054` also reported immutable CLI asset HTTP 504, and one scanner HTTP 504. Treat these as observed external delivery failures, **not** proven code corruption or permission to bypass scanning.

## Minimal implementation
1. Inspect existing `install/integrationctl/agentplugins/adapters/securityscan/release.go` and npm published launcher transport. Reuse one established download implementation per subsystem; no new package manager/alternate feed.
2. Where necessary, add a small **bounded** retry for explicitly transient GET failures (HTTP 502/503/504 and transient network errors), obeying caller context and an overall deadline. Fail promptly for 4xx, invalid URLs, checksum mismatch, invalid archives, size limits, cancellation, and unsupported platforms.
3. Preserve immutable version/asset names and the existing pinned SHA-256; publish a cache entry **only after** full verified download and safe extraction. Retry must never turn a failed scanner into PASS or suppress an install security check. Do not embed token URLs in logs.
4. Separate pre-existing npm launcher handling from production lintai acquisition; avoid a cross-layer retry framework and do not modify signed Directory policy.

## End-to-end acceptance
- Focused HTTP test: transient 504 then exact-byte 200 succeeds once; persistent 504 exhausts and leaves no executable/cache state; 403, bad digest, oversize and cancelled request fail closed, without further retry.
- Run Go securityscan tests, published npm launcher installer tests where touched, `go vet` / applicable targeted suites; review timeouts, races and foreign-state preservation.
- Real cold scanner install on six supported architectures: immutable `lintai@0.1.3` scanned, no skip; full Package Manager E2E + original evidence (including stdout/stderr digests) on exact PR HEAD. On actual network outage retain a failing red E2E and diagnostic artifact; do not relabel it green.
- Independent review, all applicable CI green, SHA-gated merge, post-merge main qualification. **No release dispatch**; public npm version changes are not needed to work on these independent source fixes.

## Non-goals
No fake offline PASS, no scanner disabling, no unpinned mirrors, no security fallback, no unbounded retries or changes to user packages.
