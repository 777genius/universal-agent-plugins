# P1 — Marketing Kit Project/Copilot release qualification

## Context
Continue existing draft PR #775. Five signed Marketing Kit Project client routes have historical E2E evidence; a genuine published `0.1.101` Copilot install fails **before** lifecycle because the signed Directory root lacks Copilot authorization. Latest six-client real run also reported immutable binary/scanner HTTP 504. HTTP 504 is an acquisition failure, not evidence that native lifecycle passed or failed.

## Minimal plan
1. Keep the exact five qualified routes and the sixth Copilot lane in the existing harness; do not invent extra framework/test identities.
2. Separate three outcomes: temporary immutable binary/scanner download failure; **correct signed eligibility refusal**; and genuine accepted Copilot add/list/repeat/update/cold locked restore/remove. Never relabel either of the first two as a lifecycle PASS.
3. Require a separately reviewed, correctly signed immutable Directory v1 root and mandatory child closure authorizing Copilot. **No silent grant/schema v2, revoked or borrowed publisher signature**, and no public release/grant mutation without explicit authorization.
4. On authorized signed release, test real pinned `@github/copilot` CLI Project A/B and retained foreign files after every operation, owner boundaries, lock digests, supported native MCP behavior. Validate six lanes and all original JSON/ZIP, not only workflow color.
5. Exact-head review/CI, merge only after signed grant and actual Copilot lifecycle succeed; post-merge production acceptance. Current npm version/release timing does not block unrelated test improvements.

## Non-goals
No scanner bypass, no forced positive expectation for unsupported Copilot, no new public grants automatically, no duplicate PR replacing #775.

## Cold refusal cache correction (2026-11-10)

Source audit against immutable `agentplugins-v0.1.102` shows that locked declaration
replay visits valid packages before the corrupted Notion pin
(`deps_replay.go:44–70`). Materialization with `publish=false` still scans and
checks the signed Directory (`deps_sync.go:27–63`): the production evaluator
stores assessments synchronously and the Directory client persists authenticated
last-known-good bytes. These own-profile cache additions explain the current
negative-test failure; they do not establish a native installation or a write to
another profile.

Keep peer snapshots and all refusal/digest, inventory, lock-restoration and foreign
guards unchanged. Give only the negative command's own snapshot a finite cache
addition validator: preserve every existing entry, HOME and Project; accept only
new regular Directory cache/lock, 64-hex assessment files and the pinned scanner
for this host, plus their exact parent directories. Reject symlinks, special
files, changed/replaced entries and unknown paths. Permit an initially absent
data root only as a real directory under its unchanged real parent.

Expected implementation is roughly 90–130 Python lines plus 120–180 regression
lines. This adds a bounded path/type check rather than a second cache or security
authority; cache contents remain subject to production signature/checksum checks.
Filesystem snapshots have observation windows, so this does not promise detection
of transient writes undone before inspection. Run the complete focused Marketing
Kit suite and retain genuine released-client CI as the release evidence.

Implemented with the exact policy/key and Linux scanner binary pins already used
by `project_native_windsurf_cycle.py`. Fixture directories start with private
permissions so Directory cache-parent normalization does not alter the baseline.
After validating the refused command's additions, inventory uses a new strict
full snapshot, preserving even those newly created caches during the read.
The focused Marketing Kit suite passes 56 tests, including both scopes' refusal
and lock restoration, valid cold-cache additions with existing/absent data roots,
preexisting cache replacement, state/native/foreign writes, unknown cache paths,
symlinks, a FIFO, incorrect scanner bytes and assessment identity/policy mismatch.
