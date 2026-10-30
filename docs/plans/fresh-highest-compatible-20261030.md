# Lock-first, highest-compatible dependency resolution

Owner decision: accepted on 2026-10-30. Initial base: `d2717aea08e8a4d4d7405e6b45755aac4fad76be`.
Implementation status: planned; not a released guarantee.

## Accepted contract

When resolving a Registry node without a usable existing pin, prefer the highest
eligible stable semantic version satisfying the complete dependency graph. For
identical versions, prefer the highest signed release sequence. This is a
deterministic candidate preference with backtracking, not independent latest
selection per package or a global maximum-version optimization.

A compatible existing pin is tried first during ordinary resolution. An unchanged
compatible environment retains its exact lock, source revisions and package bytes.
Locked replay never becomes an update: unavailable, revoked or corrupt pins must
not silently resolve to new content. Exact versions, release sequences, commits
and archive checksums retain their existing meanings. Mutable Git/archive/Skill
sources keep their existing source-resolution and explicit-update contracts.

Existing eligibility, target inheritance, shared constraints, source identity,
ownership and error classification remain authoritative. Candidate ineligibility
and dependency conflicts may try the next candidate; authentication, transport,
integrity and other operational errors must not be hidden as successful fallback.
No prerelease admission, extra version grammar or automatic source aliases.

## Implementation boundary

1. Reuse the existing environment graph solver. Change only Registry candidate
   ranking after its pin-first branch. Do not add a mode flag, second resolver,
   dependency library, lock schema migration or automatic lock rewrite.
2. Integrate the separately reviewed explicit-update work in #563 before changing
   its shared ranking path. Retain selected-update no-downgrade and unselected-root
   preservation; newly introduced nodes use the accepted fresh policy.
3. Adjust only tests whose minimum-version expectations describe the deliberately
   changed contract. Keep conflict, exact-pin and retry assertions meaningful.
4. Update `docs/UAP_DEPENDENCIES.md` and the research decision reference. State the
   development-source/released boundary; do not rewrite historical evidence.
5. Use existing CLI/Directory fixtures for a composed journey: fresh highest
   eligible graph, locked replay after a newer publication, shared constraints,
   conflicting newest candidate/backtracking, target eligibility and refusal
   without durable changes. Add focused core tests for equal-version release
   ordering, 0.x boundaries, exact selectors and source-order independence.
6. Verify final source through applicable repository checks and independent
   available PR reviews. Publish each implementation commit immediately. Merge
   only after concrete findings are resolved and required checks succeed.

## Parallel work, without duplicate implementations

- #575 owns manifest/lock stale-input protection, transitive promotion and the
  portable 36-case E2E runner. Finish its existing verification, including macOS;
  reuse that runner and reconcile overlapping tests after integration.
- #566 owns captured-profile isolation and native SDK lifecycle corrections.
  Diagnose its actual Required failure before changing code or rerunning.
- A separate small author-guide change covers an explicit shared MCP package and
  runtime prerequisites. Do not auto-merge by URL/name, create a broker, parse
  executable dependency graphs from Markdown or auto-run setup during inspection.
- VS Code remains frozen. No release/deployment, unrelated client program or
  broad refactor is included in this implementation.

## Review questions and acceptance

Review round 1: distinguish compatible pin retention, fresh candidate preference
and explicit updates; identify shared-graph/backtracking and security failures.
Review round 2: verify the corrected plan and minimal implementation preserve
those boundaries, without adding speculative abstractions.

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
