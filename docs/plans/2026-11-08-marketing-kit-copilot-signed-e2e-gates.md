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

## Project qualification and Copilot User policy boundary

The release goal is six positive Project A/B lifecycles. Retain the five existing
positive User lifecycles. Copilot User placement shares a physical backend and
must still refuse this signed closure's missing VS Code authorization; Project
placement uses independent qualified native backends. After completing Copilot's
positive Project lifecycle, run a real User add and require the exact signed
`missing vscode` refusal, a nonzero exit without timeout, empty User inventory,
and unchanged own/peer/foreign profiles and portable/native/installed state.
Report this as a separate negative User policy contract, never a positive User
lifecycle or completion of the full two-scope program. No VS Code grant or
implementation change is authorized by this boundary.

Immutable CLI `0.1.102` incorrectly recomputes User backend siblings while
selecting a v1 source for explicit Project placement. The source correction is
separate PR #836 and requires a genuinely published executable release before
Copilot Project qualification can pass. Keep the existing `0.1.102` version and
artifact pins until corrected immutable release metadata actually exists; do not
guess a new version/hash or count source-built evidence as published qualification.

Preparation validation: 65 focused Marketing Kit regressions pass. Refusal checks
require an exactly empty raw stdout tail and the exact raw stderr diagnostic
with at most its normal LF/CRLF; real-capture padding/truncation regressions
reject hidden mixed output. A cleanup exception after successful lifecycle
steps resets the overall report to failed and returns nonzero. The unchanged
published `0.1.102` binary also passes the new exact User-negative helper against
the immutable signed sequence-45 feed after a read-only `info` warms its own
authenticated cache: exit 1, exact missing-VS-Code refusal, empty User inventory,
and strict own/foreign snapshots unchanged. This is negative User evidence only;
it does not qualify a positive Copilot Project lifecycle on `0.1.102`.
