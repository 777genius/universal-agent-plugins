# Hermes and Antigravity: public release and end-to-end completion

## Owner goal and starting point

The owner asked to finish the integrations end to end, including the released
CLI, README, landing, public documentation and actual Antigravity IDE discovery.
The 2026-11-10 instruction authorizes the next suitable unused `agentplugins`
stable version and normal publication through the existing release gates.
It does not waive protected environment review, immutable assets, signed
Directory authorization or native-client authentication.

Initial source: `307862c8a847438378d9295ea6c687c3db517de6`.
Public CLI at this audit: `agentplugins-v0.1.102`, source
`1717381d88cea9eef2cbf3ecf481dbe37188cde2`. That release does not contain
Hermes or Antigravity. Source presence must not be presented as public
availability. The next version will be selected from the live tag inventory
after the release candidate is approved; it is not reserved by this plan.

The core integrations are merged in #814, #815, #816 and #827. Their combined
commit `cc29c1ac0674d61e3ad66b3a8b97ce1e65e1d97f` passed the applicable
post-merge checks. This does not qualify later changes or published artifacts.

## Existing authorities and scope

- Client adapters remain in `clients/hermes` and `clients/antigravity`; the
  existing registry, planner, ownership receipts and transactional filesystem
  code remain the authorities. No second installer or activation engine.
- Reuse the real-client lifecycle harnesses and their evidence files.
  Release qualification must execute the frozen release binary, without a
  source build substituting for the artifact under test.
- Reuse `agentplugins-release.yml`, its complete asset verifier, six-platform
  proof, immutable draft receipt, protected promotion, npm OIDC publication and
  post-publication verification. No new publisher, token fallback or bypass.
- README and the landing support matrix share the existing row contract.
  Preserve the published-capability guard and the frozen VS Code exclusion.
- Public docs are maintained in all nine existing locales. Generated capability,
  API and locale inventories are produced by their existing generators.
- Production Pages uses `website/production-source.json`. A successful main
  build alone is not proof that the new public surfaces were deployed.

### Overlapping work observed before implementation

PRs #834 and #835 qualify and activate a compatible production source for
#831. They overlap README, two browser tests and the production-source pin.
PRs #836, #837 and #838 change signed Directory source/target behavior.
Re-read their state before merging or selecting release/site sources. Do not
overwrite their changes or manufacture target grants. #832 is separate Codex
MCP security work. Frozen VS Code code and dedicated fixtures are out of scope.

## Delivery sequence

### 1. Readiness pull request

1. Publish this plan before implementation and retain concrete review changes.
2. Update the native guides and common compatibility references with exact
   source-qualified evidence and honest remaining release/activation boundaries.
3. Extend the existing native workflows only at their artifact-acquisition seam:
   source runs keep their existing source build; release runs consume the exact
   verified same-run bundle and supply its binary to the same lifecycle driver.
   Record binary digest/version and the candidate source/run identity.
4. Make applicable Hermes and Antigravity native proofs prerequisites of the
   existing verified-draft gate. Failed, skipped or incomplete proofs cannot
   qualify a draft or reach publication. Preserve the existing asset authority.
5. Exercise a disposable official Antigravity IDE/profile, with an exact version
   and backend identity. Observe its native Customizations surface for User and
   Project discovery and enablement, including isolation and owned removal.
   A CLI list, strict manifest validation or prepared file is not an IDE oracle.
6. Review the diff independently, run the affected tests and native workflows on
   the final commit, merge through the existing checks, and qualify that exact
   main commit before tagging.

Public install targets remain hidden until step 2 below has completed.

### 2. Immutable release qualification and publication

1. Select and report the next unused stable version; freeze an approved main
   commit in an annotated tag. Never move or reuse a stable tag.
2. Run the existing draft qualification from that tag. Require its release-bound
   Directory bootstrap, Go/npm checks, six platform builds and cold/warm launcher
   lifecycles, plus the artifact-bound new-client native tests.
3. Retain the immutable verified-draft receipt, all nine asset identities,
   attestations, exact npm tarball integrity and native evidence. Finalize notes
   before qualification; do not mutate qualified notes/assets in place.
4. Promote the same bytes through the protected release gate, then complete the
   automatically dispatched trusted npm publisher and its public verification.
5. Install the exact public npm/native version into disposable profiles. Verify
   Hermes and Antigravity supported add/repeat/update/repair/remove behavior and
   unchanged foreign data. Verify actual signed Directory eligibility separately
   from generic client support. Wait for and verify the tap-owned Homebrew sync.

### 3. Public integration and deployment pull request

1. Enable Hermes and Antigravity in the existing public target catalog only once
   the published-capability check accepts the actual stable version.
2. Add matching README header icons/support rows, authentic client vector assets,
   client support rows, setup actions and nine-locale translations. Preserve
   honest `Prepared` labels and client-owned trust/authentication steps.
3. Update all nine compatibility pages and installation journeys where needed
   with the exact released version and evidence; preserve historical records.
4. Run locale generation, README parity, registry/i18n tests, lint, public
   capability verification, full Docs checks and browser smoke tests. Inspect
   desktop/mobile, light/dark and RTL layouts and the actual copied commands.
5. Merge the reviewed public source and activate its immutable production source
   through the existing Pages contract, preserving #831/#835 work. Wait for
   deployment and verify production home, both agent routes, Directory target
   choices, copied commands and all nine documentation routes.

## Contract and evidence matrix

| Client | Supported delivery | Native evidence already available | Additional completion evidence |
| --- | --- | --- | --- |
| Hermes | User Skills and stdio/Streamable HTTP MCP; Project Skills at the selected Git root | Hermes 0.23.0, Linux x64/macOS arm64/Windows x64; run `42813277862`, UAP head `a50c8a245f1c273efb3d171f4c54ed540475336a` | Same supported lifecycle against the exact release binary and public npm facade |
| Antigravity | Owned User/Project native plugin with Skills and stdio/Streamable HTTP MCP, prepared for the host | `agy` 1.5.3 strict validation and separate disposable CLI import/list/uninstall on three OSes; run `42804680737` | Exact release artifact lifecycle plus actual IDE discovery/enablement observations with versions and scope |

Hermes Project MCP, SSE and hooks remain unsupported. Portable commands follow
the existing Skills projection rules; explicit-only commands are withheld when
the host cannot enforce explicit invocation. The Hermes native MCP fixture
proves initialize/tools-list over loopback Streamable HTTP, not tools/call,
provider login or a model turn. Project discovery does not grant project trust.

Antigravity strict validation and CLI import are distinct from IDE activation.
If IDE authentication requires an owner session, retain the reproducible
candidate, safe profile and observed sign-in boundary; do not impersonate the
owner or mark activation as passed. An IDE observation on one OS must not be
promoted to an untested cross-platform claim.

## Plan review and accepted revisions

These are documented critiques, not claims of independent approval or executed
tests. Code/release review and actual checks remain separate gates.

### Round 1: scope and truthful support

The initial shorthand "add both agents everywhere" mixed delivery, activation
and authentication. The documentation reviewer required a component/scope
matrix, explicit Hermes Project MCP refusal, conditional command projection,
and a distinct IDE oracle. Those requirements are incorporated above.

### Round 2: publication, locales and signed eligibility

The reviewer identified that `docs:check` and a merged source do not prove the
deployed production pin. Acceptance now includes all nine live routes and the
exact production source. A CLI target capability does not authorize a signed
Directory package for that target; actual grants are a separate check.
Historical 0.1.22/0.1.53/0.1.55 evidence stays unchanged. Generated files are not
edited manually.

### Round 3: artifact identity and release reachability

The release reviewer found that the historical released-native-client workflow
has a different closed client/schema contract. Reuse the two new-client
workflows at their acquisition seam instead of widening that unrelated engine.
The release path must authenticate and verify the complete frozen bundle before
runtime, omit source compilation, record the binary identity and fail closed
before verified-draft/promotion. Credentials belong only in acquisition; the
released binary and client processes must not receive them.

## Size, risk and validation budget

Expected changes are concentrated in existing workflow/harness seams and
public data/docs, approximately 400–800 executable lines plus localized prose.
This is a planning estimate, not a target. Keep each new helper cohesive and
small; reuse the existing release asset and evidence contracts. No Go production
behavior change is planned. If the IDE spike proves a missing adapter behavior,
revise this plan with the observed defect before implementing it.

Highest risks are false native-activation claims, an artifact/source mismatch,
unsafe host registration, release-gate reachability and conflicting production
pins. Ownership refusals, signatures, attestations, environment reviews and the
public capability guard cannot be weakened to obtain a PASS.

Completion requires public readback and exact-version evidence, not a queued
workflow, an uploaded plan or a locally generated site. Waiting for CI or
publication propagation is normal execution. A genuine missing owner session
or protected approval is recorded only after the reviewable candidate and all
other useful authorized work are complete.
