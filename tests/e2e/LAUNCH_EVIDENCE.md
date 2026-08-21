# Stable launch evidence

The stable gate is intentionally split into two modes. Pull requests run only
fixture and contract checks and emit `runtime_claims: false`. A protected release
caller invokes `.github/workflows/live-e2e.yml`, which requires the reusable
`launch-evidence-e2e.yml` gate; a manual dispatch is diagnostic, not a substitute
for that release dependency.

## Official release and Directory identity

Live runs take neither repository nor release-tag identity from the caller.
`tests/e2e/production-launch.json` fixes the catalog repository to
`777genius/universal-agent-plugins` and the binary release repository/tag to
`777genius/plugin-kit-ai` / `agentplugins-v0.1.8`. This makes the immutable
upstream CLI manifest an explicit prerequisite for evidence at the exact catalog
commit (`GITHUB_SHA`); the challenge binds both sides of that release sequence.
`scripts/prepare_launch_evidence.py`
resolves that published GitHub release without using the catalog repository's
token,
dereferences the tag to a commit, downloads `release-manifest.json`,
`checksums.txt`, and a manifest-listed asset through the GitHub API, then
requires GitHub's `immutable: true`, the exact eight-file release set, and the
fixed release-workflow artifact attestation for every native asset. The
trusted signer workflow is exactly
`777genius/plugin-kit-ai/.github/workflows/agentplugins-release.yml`; similarly
named or legacy workflow paths are rejected. The
attestation verifier pins repository, workflow, tag ref, tag commit, subject
name, and SHA-256 in addition to verifying repository, tag, version, size, and
SHA-256 against both authenticated release metadata files. There are no production URL/checksum/version
inputs. The manifest must contain macOS arm64/amd64, Linux arm64/amd64, and
Windows arm64/amd64 assets. The separately published exact
`universal-agent-plugins@0.1.8` npm facade is resolved from the npm registry;
its exact registry tarball URL and bytes are verified against `dist.integrity`,
its npm provenance/signatures are cryptographically audited, and it is installed
on Node 22. The resolved installed executable must be byte-for-byte identical to
the attested native GitHub asset; a postinstall shim that only reports the right
version fails. There is no GitHub `.tgz` input or fallback. Aggregation rejects
observations bound to different GitHub release manifests.

The same preparation step fetches `latest.json`, the exact snapshot, and its
envelope from `production_origin` in `tests/e2e/production-launch.json`. Real
Ed25519 verification and complete Directory semantics use the checked-in
`registry/publication/trusted-keys.json`. The trust root is never downloaded
beside the signature. Direct unit tests may inject local publication fixtures;
enforced mode may not.

The reusable workflow requires the publication caller's exact `publication_id`,
sequence, snapshot digest, and source commit. Preparation accepts these values
only as expectations: the canonical public pointer paths, signed snapshot,
envelope sequence/digest, and signed snapshot fields must independently match.
A stale public deployment therefore cannot be used as evidence for a newer
publication. The Directory publication owner must pass these four outputs when
calling this contract; this evidence patch does not change publication code.

## Challenge and observers

Each live execution creates a cryptographically random challenge bound to the
GitHub SHA, run ID/attempt, release-manifest digest, Directory digest, and fresh
disposable root. `scripts/observe_launch_scenario.py` is the only scenario
executor: it has an immutable scenario allowlist, records timestamped argv/exit
and output-digest traces, and independently hashes manager and native client
state before and after. An omitted postcondition is a failure, never a boolean
claim supplied by another executable.

The schema-3 release gate covers every relevant acceptance 26.1 family: the
26-package and hero matrices, grouped Context7 acquisition, shared Copilot/VS
Code backend, native release slots, runtime/OAuth rows, all immutable
postconditions, fault injection/recovery, source selection/switch/promotion,
and direct/contributor journeys. `fault_matrix()` and `journeys()` are enforced
runtime paths. Fixture-only mode emits only harness rows and cannot be escalated
to a runtime or stable-gate claim.

The no-hidden-`--yes` row runs representative add, update, and remove commands
with `--yes`, requires each parser to report an unknown option, and compares
manager/native state before and after. Help text alone is not evidence.

Runtime/OAuth passes arrive in one canonical, fresh, challenge-bound bundle
signed with Ed25519. The protected environment fixes
`STABLE_LAUNCH_OBSERVER_ED25519_PUBLIC_KEY` and
`STABLE_LAUNCH_OBSERVER_KEY_ID`; both the request client and final harness verify
the signature over the complete artifact objects. HTTPS endpoint selection or a
self-asserted GitHub object is therefore insufficient to create a pass.
The individual artifacts must conform to
`tests/e2e/schemas/runtime-attestations.schema.json`: current challenge,
fresh start/end timestamps, command traces, exact client/application IDs and
HTTPS endpoint, isolated consent identity, complete release/Directory tuple,
and a GitHub attestation for this repository/SHA/run/attempt/workflow/job.
Projection or fixture output cannot become runtime evidence.

Consent and every runtime record are independently bound to the same challenge,
GitHub run/attempt, and scenario contract. They carry only pseudonymous
dedicated identity/workspace IDs and signed fields for disposable-project state,
read-only or synthetic operation, authentication origin, cleanup, and proof that
no real project, copied auth, credential material, or absolute local path entered
the export. The top-level privacy result is derived from those verified consent
fields; it is not a set of unconditional booleans.

The first stable launch additionally requires
`external_pr_evidence` inside the signed primary runtime artifact. Its schema
records the catalog and genuinely external fork identities, canonical PR
number/URL, exact head/base SHAs, an explicit null merge SHA, contributor-flow
paths, successful head-bound check runs, final validated closure without merge,
fresh observation time, and an immutable digest/reference. Its binding repeats
the current challenge, exact catalog base SHA, signed Directory publication
identity, and exact CLI release identity. Missing, local-only, catalog-owner,
stale, wrong-head, failed-check, unexpectedly merged, or mismatched evidence
emits a failed required gate. The repository's local fork-clone
accepted/rejected journeys remain useful supplemental contract coverage and
explicitly cannot satisfy this external PR gate.

Lifecycle and runtime expectations are resolved through
`build_registry.resolve_directory` with the complete target set for that row.
The evidence preserves the selected single distribution, release sequence,
resolved targets, and the authoritative resolver's exact fallback reason.

## Reproduction

PR contract run:

```bash
run_root="$(mktemp -u /tmp/uap-fixture-XXXXXXXX)"
python3 scripts/run_launch_evidence_e2e.py --mode fixture-only \
  --consent tests/e2e/fixtures/fixture-only-consent.json \
  --run-root "$run_root" --output "$run_root/evidence/launch-evidence.json"
```

Live reproduction is the protected reusable workflow after the fixed upstream
CLI release exists.
It uses Node 22 for the npm facade and native GitHub runners for every required
OS/architecture slot. It must end with all immutable scenario IDs/counts and all
rows passed; missing clients, OAuth consent, observations, runtimes, or release
assets keep the gate red.

The obsolete host launch artifact was moved unchanged to `tests/e2e/legacy/`
after exact-head schema validation proved it was not canonical client evidence.
Other committed files under `tests/e2e/results/` retain their exact historical
scope. None are rewritten or treated as schema-3 stable-launch passes.
