# Agent support synchronization and catalog completion

Date: 2026-11-14. Status: implementation plan; production acceptance pending.

## Owner goal

Make adding an agent predictable and keep the released CLI, signed Directory,
Skills picker, plugin picker and public support pages consistent. Complete the
Hermes, Antigravity and Pi catalog installation paths, review the changes,
merge them, and verify the actual published user journey end to end.

This is one change family across `777genius/universal-agent-plugins` and
`777genius/universal-agent-plugins-registry`. Source synchronization alone is
not completion of the catalog rollout.

## Verified baseline

- UAP main: `36dbc1e7252fd3c6a7bc863a9ccda82899c3a3d3`.
- Registry main: `a12bd3bdffab6991a2541220a16a0a9d6447e47e`.
- Public CLI: `agentplugins-v0.1.108`, source
  `8c3cfa28e332c533b0cc0845739c0275730a4285`.
- Promoted Directory schema 1 sequence 47 contains zero Hermes, Antigravity
  or Pi target policies. Its source is immutable.
- Registry #451 remains Draft at `1900a533c3bb8e763dad6e4b1a3bc9f19ff5bf1b`.
  Reuse its qualified Pi work and preserve its independent acceptance gates.
- UAP #878 owns ongoing native-client changes. Do not duplicate or overwrite
  those adapters or claim its unmerged work as released behavior.
- `domain.ClientDefinitions`, registered adapters and scoped capabilities are
  already the installer authority. `cli/cmd/genclientcapabilities` already
  exports adapter-derived JSON. Several consumers still maintain their own IDs.

## Design and boundaries

### 1. One existing source, generated consumers

Keep client identity and technical capabilities in the existing Go domain and
adapter registry. Extend `genclientcapabilities`; do not create another manual
agent manifest, resolver, installer, runtime registry or capability service.

Generate the landing's runtime ID tuple and TypeScript type with the existing
capability JSON. The Directory decoder and frontend type use that output.
Presentation text, icons, ordering and intentionally hidden public clients stay
in the website layer, with exhaustive identity coverage checked in CI.

Registry consumes an immutable, digest-checked copy of the released capability
artifact. Its pin records the release, exact source commit, source path and
SHA-256. Updates are explicit and reviewable; validation is offline and never
reads mutable upstream `main` or an unpinned `latest` document.
The first pin uses the unchanged 0.1.108 capability artifact; consumers must
not require newly generated fields before a real release provides them.

### 2. Capability, publication and activation remain different facts

Generated metadata says what an installed adapter can prepare. It never grants
permission to install a particular package, invents evidence, or signs data.
Package policies still bind the exact source and dependencies. Prepared files
remain distinct from discovery, activation, OAuth and native runtime proof.

Use the same installation decision in hero, cards and detail pages. Explain
the actual unavailable state briefly; offer an existing verified installation
route when available. Standalone Skills remain independent. Preserve explicit
scope choice, expiry, revocation, signed denials and dependency authorization.
Never silently replace a plugin with Skills only or switch Project to User.

### 3. Stable schema 1, safe future client additions

Preserve the historical feed, signer, cache, ledger, frozen wire client set and
existing immutable snapshots. Its old consumers must keep working.
The historical allowlist applies to both target records and evidence client
identities, including revoked and suspended policies.

Complete the already selected independent modern schema-1 path. Reuse existing
signing, canonical serialization, verification, cache and publication machinery
through a narrow reviewed feed contract. Do not introduce Directory v2, public
feature flags or a second install engine.

Modern authenticated readers must not reject the entire catalog merely because
a future agent appears. Validate the complete signed structure, target and
evidence identities before retaining a well-formed unknown client as
nonselectable. Known-client eligibility remains strict. Unknown targets and
evidence cannot authorize a known adapter or bypass failed known-client
evidence. Historical decoding stays strict. Publisher admission remains bound
to the pinned known-client contract.
The modern reader profile travels with its explicit trust store through
bootstrap, remote verification and every cache re-verification. It must never
fall back to a historical parser after cache persistence. Verify signatures
against the exact complete bytes before accepting the modern structure.

Schema-1 target scopes remain `["user"]`. Explicit Project placement continues
through the existing source and native Project policy checks; generated Project
capabilities never fabricate signed Project grants.

### 4. One onboarding and release-completion workflow

Adding an agent requires its existing domain definition, adapter registration,
native contract tests and website presentation. Generation updates dependent
identities and schemas. A single documented check identifies missing consumers,
release support and catalog coverage rather than relying on manual ID searches.

Publication acceptance binds the actual released CLI contract, signed snapshot,
package/dependency policies and deployed website provenance. It must exercise
the command generated by the production UI with the selected agent and scope.
Technical adapter tests and synthetic signatures are never catalog PASS.

## Implementation sequence and ownership

1. **Plan and independent review.** Inspect overlapping work, publish a Draft PR
   with this plan, incorporate at least two reviews, then implement.
2. **UAP generated identity and UI parity.** Extend the existing generator and
   drift checks; derive the frontend type/decoder; enforce public presentation,
   support table and icon coverage without changing frozen VS Code behavior.
3. **Registry contract consumption.** Add the immutable capability pin and
   deterministic sync/check commands. Derive modern candidate IDs from it and
   explicitly preserve the historical subset. Reuse #451 rather than creating
   a competing Pi candidate.
4. **Modern reader and publisher.** Review and implement future-client handling
   and the minimum shared feed contract; qualify independent trust, cache,
   replay, rollback, ledger and promotion behavior. Integrate real reviewed
   Hermes/Antigravity/Pi source and dependency evidence.
5. **Availability and release checks.** Unify actionable picker explanations;
   verify ordinary Skills and explicit Pi source routes. Add actual catalog
   coverage and released-contract checks to the existing gates.
6. **Review, merge and publish.** Fix independent review findings and actual
   failed checks. Merge only the reviewed heads with required gates satisfied.
   Publish through the existing protected process, update the immutable site
   source pin, and verify the resulting public journeys.

Changes in each repository use one Draft integration PR where feasible. Keep
the existing Pi/native PR authorship and work; never overwrite a moved head.
Push meaningful commits promptly. Waiting workflows remain pending, not PASS.

## Estimates

| Slice | Production/config LOC | Tests LOC | Complexity | Expected reliability | Confidence |
| --- | ---: | ---: | ---: | ---: | ---: |
| UAP generation, consumers and parity | 150-300 | 180-300 | 4/10 | 9/10 | 9/10 |
| Registry pin, schemas and onboarding checks | 160-300 | 120-220 | 5/10 | 9/10 | 8/10 |
| Modern reader and publication integration | 650-1,200 | 500-900 | 8/10 | 9/10 | 7/10 |
| Shared availability and production acceptance | 200-400 | 180-350 | 6/10 | 9/10 | 8/10 |

Ranges exclude reused #451/#878 changes, generated JSON, translations and
documentation. Adjust estimates against the reviewed implementation rather
than adding abstractions to fit them.

## Required verification

- Generator output is byte-reproducible. A newly registered example client
  in a generator test fixture appears in all derived identities; missing
  presentation fails clearly. Do not register an example production adapter.
- Known Hermes/Antigravity/Pi policies decode; unknown identities never become
  install targets. Malformed, duplicate, unsigned, expired, revoked, wrong-key,
  cross-feed and rollback cases still fail at their existing boundaries.
- Modern future-client fixtures preserve known-client resolution; unknown
  client selection refuses. Targets and evidence are both covered. Legacy
  fixtures retain their original refusal behavior and signed bytes.
- Registry contract pin, digest and schema drift fail before signing.
  Capability support alone cannot publish a target. Required dependency gaps
  fail the full install plan, including Marketing Kit/HubSpot.
- Use `make lint`, `make test-core`, affected conformance and scoped lifecycle
  checks. Frontend uses TypeScript 7.0.2 or newer; no TypeScript 5.9 toolchain.
- Test hero/card/detail and Skills on desktop and mobile; inspect visual output
  and actual generated commands. Run applicable Linux/macOS/Windows released
  CLI install/info/update/repair/remove acceptance on disposable profiles.
- Preserve Hermes Project Skills-only and Antigravity Prepared. Unsupported
  mandatory components must remain visible refusals.
- Verify final merged commit, protected publication receipt, remote signed
  snapshot, released binary route and deployed source/tree provenance.

## External publication prerequisites

The independent modern production issuer needs an approved non-test key,
protected environment, origin and ledger/rulesets. Existing source contains
only the historical production trust. Prepare and review all executable work
first; use actual authorized controls to provision/publish. Never promote a
test key, reuse the forbidden historical issuer, invent an approval, change
access controls to bypass a gate, or label an unsigned candidate production.
If a required service permission is unavailable, report the precise remaining
action after completing all possible source and validation work.

## Review amendments

Initial independent reviews agreed on reusing the existing generator and
keeping authorization separate from capability. They identified two required
amendments incorporated above: freeze the historical accepted-client subset,
and make the modern authenticated reader tolerate future client identities as
nonselectable so the same problem does not recur with the next agent.

Round 1: independent architecture and Registry reviews approved the plan with
amendments now incorporated: historical evidence IDs are guarded as well as
targets; the first external pin uses the released unchanged JSON; the modern
reader profile follows all bootstrap/cache paths; new-agent proof uses a test
fixture rather than a production example adapter. Further review checks the
implementation and source-bound acceptance separately.

Plan review outcomes, exact implementation heads, test evidence and production
acceptance will be appended as the work proceeds.

## Implementation review and verification checkpoint

The generated identity and presentation slice is published at `2ee05ac`, and
the authenticated modern reader at `e89e6b7`, in UAP #908. Registry #480 pins
the released contract at `5854c13`. Independent implementation reviews approved
the generator/SDK slice, Registry pin and historical boundary, and modern
reader. Review caught two defects before integration: modern sequence 1 must
require current evidence provenance, and a scope-switch suggestion must have
an actually available User installation route. Both are fixed with negative
regressions. No historical feed bytes or VS Code adapter behavior changed.

Local verification: 536 frontend tests pass without skips; native TypeScript
7.0.2 and Vue template checks pass; lint has zero errors; all nine locale and
locked-advisory checks pass. Registry has 127 builder tests and 56 publication,
contract and historical-boundary tests passing. Full Directory/domain and
focused modern CLI tests pass. Hosted full lifecycle/browser gates are still
required: this local environment refuses the Unix sockets needed by Chromium
and the process containment interface required by ordinary CLI subprocesses.
Those refusals are not counted as passing E2E.

TypeScript uses the official side-by-side arrangement: the native 7.0.2 compiler
checks the project, while Vue/ESLint retain the separate classic API bridge.
The bridge is not the native compiler gate and no TypeScript 5.9 is introduced.

### Exact-source installation qualification

The existing CLI already accepts immutable Git plugin sources independently of
Directory target grants. The current website exposes that route only for Pi
Project. Generalizing its presentation must preserve the exact authenticated
source, explicit agent/scope request, and the released CLI's component and
dependency checks. Marketing Kit's mandatory catalog dependencies still
require their own signed eligibility.

Reuse the existing live-install lifecycle harness and public release verifier
for a bounded source qualification lane. Bind receipts to the actual released
CLI, exact signed snapshot and immutable source, then exercise add, info,
update, repair and remove with scope/foreign-state checks. Preserve real
dependency refusals. Exercise representative real source routes before
publication, including the exact commands generated by the UI. Present the
route as an explicit request for the CLI to validate and install the source,
never as per-package/per-platform qualification. Do not put this alternative
in automatic discovery or mix it with signed-target selections. This lane
does not claim native activation, produce a signed target grant, or complete
the independent modern publisher rollout.

Estimated additional reusable qualification code: 300–450 Python LOC,
150–250 test LOC and about 100 workflow LOC, complexity 7/10, reliability 8/10,
confidence 7/10. Source qualification can proceed through ordinary no-secret
PR CI; modern production signing still needs the protected issuer controls
listed above. The connected tools cannot currently provision those controls.

### Final review corrections and hosted qualification

Registry #480 now contains the shared publisher foundation at `83e98fb`.
The complete 119-test publication/profile/hardening/CAS suite passes, along
with the released-contract and 31-product build checks. Independent review
found a package/direct-import singleton mismatch and missing historical schema
fragment aliases. Both are fixed without relaxing fixed-profile identity or
legacy schema references; the reviewer reproduced the fixes and gave GO.

UAP's external review identified a User-only explicit policy whose broad
Project capabilities obscured the usable User route. Commit `eea082d` fixes
the explanation using actual resolver eligibility. A positive and negative
policy regression, all 12 affected picker tests, and independent review pass.
`make vet` and `make generated-check` also pass. This change does not authorize
an unavailable target or switch the selected scope automatically.

Released-source qualification is isolated in UAP #909 at `e34ab8b`. Its 20
offline regression tests pass independently; the actual released sources
confirm all tested flags, saved identities, command envelopes and refusal
framing. Review tightened exact mandatory-dependency refusal and native MCP
cleanup observation. Seven real hosted lanes are pending: Antigravity and Pi
on Linux/macOS/Windows, and Hermes User on Linux. Positive Notion evidence and
Marketing Kit closure-refusal evidence are not yet production PASS.

Several GitHub-hosted Linux jobs remained unassigned with no test steps.
The existing configured Namespace runner did start Coverage on the same
UAP head. The integration PR therefore returns to the repository's existing
default runner selection for Required/Core/Landing; their test steps and
acceptance requirements are unchanged. Real native qualification retains its
existing GitHub-hosted disposable-runner guard. Neither runner allocation nor
local unit tests replace the final-head browser and lifecycle results.

### Reviewed simplification: explicit source installation

Further independent architecture review corrected an excessive requirement in
the initial source-route plan. A per-source/per-platform receipt index would
be a second UI authorization table for an already released direct-source
contract. It is unnecessary for the truthful promise that the CLI will check
the source, components and mandatory dependencies before installing it.
Representative genuine lifecycle tests establish the feature behavior; they
do not establish that every catalog package activates on every platform.

Generalize the existing Pi Project selector to a single explicitly selected
local agent and User/Project scope, using generated saved-dependency and
component capabilities. Reuse the signed resolver for a currently reviewed,
active, immutable source. Preserve all historical target/denial and canonical
closure vetoes, app-only restrictions, scope limitations, and unsupported or
unknown component refusals. Preserve automatic and signed selection behavior.
Never rewrite mandatory Directory dependencies or add `--no-save` to evade
their eligibility checks.

The UI will say that the source route lets the CLI check components and
dependencies before installation. Shared commands must use the tested public
CLI release, actual saved `update`, and explicit target/scope for maintenance.
Hero/card/detail selection and navigation must agree. This needs no new
installer, feature flag, platform picker, qualification service or manually
maintained agent permission manifest.

Acceptance combines real released-CLI Notion lifecycles, exact Marketing Kit
dependency refusal, the public npm wrapper and generated command contract,
plus desktop/mobile UI and denial/scope regressions. The independent reviewer
approved this narrower implementation boundary; queued or failed runs still
cannot qualify it. Estimated additional UI changes: 180–300 production LOC
and 150–260 test LOC, complexity 5/10, reliability 8/10, confidence 8/10.
