# Lock-first, highest-compatible dependency resolution

Owner decision: accepted on 2026-10-30. Initial base: `d2717aea08e8a4d4d7405e6b45755aac4fad76be`.
Implementation status: implemented in development source by #584; not yet released.
Integration base: `c8c35655d6d1b97839c0e952c6b1491e54f39de9`, including #563.

## Accepted contract

When resolving a Registry node without a usable existing pin, prefer the highest
eligible stable semantic version satisfying the complete dependency graph. For
identical versions, prefer the highest signed release sequence. This is a
deterministic candidate preference with backtracking, not independent latest
selection per package or a global maximum-version/pin-preservation optimization.

A compatible existing pin is tried first during ordinary resolution. An unchanged
compatible environment retains its exact lock, source revisions and package bytes.
Locked replay never becomes an update: unavailable, revoked or corrupt pins must
not silently resolve to new content. A failed retained-pin verification is an
error, not permission to call that pin absent. New or changed requirements may
require a different compatible closure through the existing graph search.

An exact semantic version fixes the version, not the publication: fresh `1.2.3`
may select the highest eligible signed sequence advertising `1.2.3`. A compatible
lock retains its original sequence and bytes. An exact release sequence fixes
that publication through the existing exact-source path. Exact commits and
archive checksums, and mutable Git/archive/Skill sources, retain their existing
source-resolution and explicit-update contracts.

Existing eligibility, target inheritance, shared constraints, source identity,
ownership and error classification remain authoritative. Candidate ineligibility
and dependency conflicts may try the next candidate; authentication, transport,
integrity and other operational errors must not be hidden as successful fallback.
No prerelease admission, extra version grammar or automatic source aliases.

## Implementation boundary

1. Reuse the existing environment graph solver. Change only Registry candidate
   ranking after its pin-first branch. Do not add a mode flag, second resolver,
   dependency library, lock schema migration or automatic lock rewrite.
2. #563 is merged. Retain its candidate-level and acquired-package no-downgrade
   checks, selected-update eligibility recheck and unselected-root preservation.
   Newly introduced nodes use the accepted fresh policy.
3. Adjust minimum-specific expectations AND adversarial fixtures: put conflicting
   dependencies, rejected-only children and injected acquisition failures on the
   highest attempted candidate. Prove genuine backtracking and rejected-node
   removal; transport, integrity, manifest and identity errors stop after the
   first acquisition. Do not let reordered fixtures silently bypass a test.
4. Update `docs/UAP_DEPENDENCIES.md` and the research decision reference. State the
   development-source/released boundary; do not rewrite historical evidence.
5. Use existing CLI/Directory fixtures for a composed journey: fresh highest
   eligible graph, locked replay after a newer publication, shared constraints,
   conflicting newest candidate/backtracking, target eligibility and refusal
   without durable changes. Add focused core tests for equal-version release
   ordering, 0.x boundaries, exact selectors and source-order independence.
6. Test one mixed selected update: a selected root introduces a new Registry node
   with multiple compatible releases, an existing compatible dependency has a
   newer release available, and an unselected root constrains shared state. The
   new node selects highest-compatible; compatible pins and unselected roots stay.
   Separately test fresh exact-version selection, retained same-version publication
   and exact-release selection. Preserve existing cold-restore revocation and
   publication-identity-mismatch refusals.
7. Verify final source through applicable repository checks and independent
   available PR reviews. Publish each implementation commit immediately. Merge
   only after concrete findings are resolved and required checks succeed.

## Parallel work, without duplicate implementations

- #575 owns manifest/lock stale-input protection, transitive promotion and the
  portable E2E runner (now expanded to 67 journeys across four platforms). Reuse
  that runner and reconcile overlapping tests after integration; do not overwrite
  another active worker's branch or repeat its implementation.
- #566 owns captured-profile isolation and native SDK lifecycle corrections.
  Diagnose its actual Required failure before changing code or rerunning.
- A separate small author-guide change covers an explicit shared MCP package and
  runtime prerequisites. Do not auto-merge by URL/name, create a broker, parse
  executable dependency graphs from Markdown or auto-run setup during inspection.
- VS Code remains frozen. No release/deployment, unrelated client program or
  broad refactor is included in this implementation.

## Review and acceptance

Round 1: CodeRabbit source review (comment 6525045960) confirmed the existing
solver boundary and requested meaningful reversed-order failure fixtures, a mixed
selected-update/new-node test, and exact-version/publication distinction. All
three corrections are incorporated above. Strix reviewed the plan document and
found no security issue, explicitly withholding any implementation claim.
Round 2 checks the corrected plan and minimal implementation against these
boundaries. Self-review independently identified the same order-sensitive fixture
risk; it is not counted as another model or independent approval.

Required evidence: the old implementation fails the new fresh-selection
regression; the correction passes it; existing locked/exact/update tests remain
valid; shuffled candidate/declaration ordering yields the same resolved graph;
failed resolution writes no agreement or managed-state changes. Whole-repository
CI and composed fixtures remain distinct from authenticated native/model tests.

Available session tools do not select independent GPT-6.1 Sol/xhigh workers.
Self-review, repository automated review and executed checks must be reported
under their actual provenance, without fabricated model approvals.

## Rationale

The comparison in `docs/research/dependency-runtime-20261029/dependency-policy.md`
separates industry conventions from UAP-specific native-resource constraints.
Cargo and uv document highest-version preference plus retained lock preferences:
https://doc.rust-lang.org/cargo/reference/resolver.html
https://docs.astral.sh/uv/concepts/resolution/
The owner selected this UAP contract; other ecosystems are not normative standards
for Agent Plugins. One version per canonical source and existing exact/caret
syntax remain unchanged.
