# Launch evidence harness

`scripts/run_launch_evidence_e2e.py` is the Phase 6 evidence gate. It always
uses newly created homes and emits the redacted schema in
`tests/e2e/schemas/launch-evidence.schema.json`.

Run the host audit without a lifecycle binary:

```bash
python3 scripts/run_launch_evidence_e2e.py \
  --output /tmp/launch-evidence.json
```

Run the disposable CLI matrix against an immutable catalog:

```bash
catalog_digest="sha256:$(sha256sum catalog/v1/catalog.json | cut -d ' ' -f 1)"
python3 scripts/run_launch_evidence_e2e.py \
  --binary /absolute/path/to/agentplugins \
  --catalog-url https://raw.githubusercontent.com/OWNER/REPO/FULL_SHA/catalog/v1/catalog.json \
  --catalog-digest "$catalog_digest" \
  --output /tmp/launch-evidence.json
```

Add `--require-gates` only for a release gate. It exits `2` when any required
row is `failed`, `inconclusive`, or `not_tested`. Missing tools, client
versions, identities, and consent never become passes.

## Runtime and OAuth attestations

Runtime observations are separate from package projection. Supply a reviewed
JSON file conforming to
`tests/e2e/schemas/runtime-attestations.schema.json` with `--attestations`.
Every passed row must identify the package/dependency, installer/adapter,
client version, OS, architecture, and observation time. Passed Notion and
ChatGPT rows additionally require both `consent_attested: true` and
`isolated_identity: true`.

No credentials, authorization URLs, account identifiers, raw transcripts, or
absolute client-home paths belong in either attestation input or exported
evidence.

## Fault and contribution driver

State migration, crash recovery, Directory failure modes, adapter repair,
promotion, persistent data, and fork submission use an optional executable
passed through `--scenario-driver`. The harness calls it as:

```text
DRIVER SCENARIO_ID /absolute/path/to/agentplugins
```

The driver runs inside the disposable environment and returns one JSON object:

```json
{
  "outcome": "passed",
  "reason": "observable invariant was satisfied",
  "tuple": {
    "package_digest": "sha256:...",
    "dependency_identity": "fixture-name@revision",
    "installer_version": "...",
    "adapter_version": "...",
    "client_version": "isolated-fixture/v1",
    "os": "Linux",
    "architecture": "x86_64",
    "observed_at": "2026-08-20T00:00:00Z"
  }
}
```

The driver returns zero with an honest `inconclusive` outcome when the scenario
was attempted but could not reach a deterministic result. Non-zero means the
driver itself failed. A missing driver is `not_tested`, not a fixture pass.
