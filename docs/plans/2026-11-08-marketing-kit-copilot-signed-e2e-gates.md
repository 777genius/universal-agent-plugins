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

## Six positive User and Project lifecycles (owner clarification, 2026-11-10)

The final release boundary is **six positive User and six positive Project
lifecycles**, as clarified in Registry PR #461 comments 6779721254 and 6781163730.
The interim Copilot User expected refusal is superseded as an acceptance target.
Copilot must execute the same complete User A/C lifecycle as the other five
clients: signed add, list, repeat add, unchanged update, intact repair, independent
cold locked restore, corrupt-lock refusal, native removal, and retained foreign,
Project, peer-profile, ownership and installation identity checks.

Retain a separate signed-policy negative in the Copilot lane after both positive
lifecycles finish: an **explicit VS Code User** add through its documented `--no-save` route must receive the exact
missing-VS-Code grant refusal. It does not qualify a positive lifecycle. Require
nonzero integer exit without timeout, exactly empty raw stdout and the exact raw
two-line stderr diagnostic (including the normal resolving/validating prefix)
with only its normal LF/CRLF permitted. The capture tail bound
is longer than the complete expected diagnostic, so strict equality cannot accept
truncated output or conceal mixed output. Require empty Copilot and VS Code User
inventory, unchanged existing own state and strict peer/foreign snapshots. The
refused command alone may add only the authenticated Directory cache and its empty
lock with restrictive regular-file types/modes; scanner/assessment additions are
not allowed before this eligibility refusal. Inventory receives a fresh strict
snapshot and cannot change even those additions.

PR #836 fixes explicit Project source selection and is merged on main at
`3abe2e5745c18fbdf6d67c31b8a6224d724be6ea`. Separately reviewed PR #838 permits
Copilot's own signed User grant while preserving requirements for explicit VS Code,
mixed target requests and existing VS-Code-owned bindings. Neither change grants
VS Code authorization or changes its frozen implementation or dedicated fixtures.
The immutable signed sequence-45 root and three mandatory children authorize
Copilot; the root does not authorize VS Code. Production still uses sequence 44.

Keep genuine published CLI `0.1.102` version and artifact pins until a corrected
immutable executable release actually exists. Source-built evidence cannot qualify
the final released lifecycle. Set `two_scope_lifecycle_qualified=true` only after
both positive lifecycles, the required separate negative, all native/foreign/peer
checks and disposable-directory cleanup succeed. A negative or cleanup failure
keeps the report failed and qualification false even after positive phase statuses
have passed.

Preparation validation: 68 focused Marketing Kit regressions and all 153 tests in
`scripts/test_live_install_audit_env.py` pass. Genuine published `0.1.102` (binary
SHA-256 `3ccfead44b8d539fd577bfc5b142a75dbec3f33b018123ac59ca40cb89522983`)
passes the new explicit VS Code helper against authenticated immutable sequence 45
(snapshot digest `sha256:2aba9b0d97556ac0756dff5f974aa7377914e0d8d1d9b735b46c5bc804b81f50`):
exit 1, exact complete two-line stderr, empty stdout, empty Copilot/VS Code User
inventories, and an exactly unchanged warmed HOME/data/Project snapshot. This is
negative policy evidence only. It does not qualify either positive Copilot
lifecycle on the unchanged published `0.1.102` executable.
