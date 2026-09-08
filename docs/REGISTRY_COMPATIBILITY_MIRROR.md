# Registry compatibility mirror

The `agentplugins-registry-mirror` build tool keeps a product deployment
compatible with the optional Universal Agent Plugins Directory and Discovery
feeds. It is a build-time bridge, not a second registry and not an installer.

## What is verified

For each feed the tool fetches the latest pointer, its exact snapshot bytes and
envelope, then:

- checks the schema-1 pointer paths and bounded fetch contract;
- resolves the signing trust document at the snapshot's immutable source
  commit;
- requires the release bootstrap key before accepting that trust document;
- verifies the SHA-256 digest and domain-separated Ed25519 signature using the
  existing Go Directory/Discovery validators;
- rejects sequence rollback and same-sequence digest conflicts when a previous
  `MIRROR_METADATA.json` is supplied.

The output contains the original bytes. No feed is parsed and re-serialized
into the deployed tree. This preserves the signed artifact identity and makes a
deployment reproducible from its metadata marker.

## Local check

From the repository root:

```bash
go run ./cmd/agentplugins-registry-mirror \
  --output "$(mktemp -d /tmp/agentplugins-mirror.XXXXXX)"
```

The command creates a fresh staging directory containing both feeds, their
trust documents, and `MIRROR_METADATA.json`. It refuses to overwrite an
existing output path.

## GitHub Pages workflow

`.github/workflows/registry-compatibility-mirror.yml` runs on a
`registry-published` repository-dispatch event or manually. It verifies the
feeds before the landing site is generated, stages the exact bytes under the
Pages artifact, and deploys only the resulting artifact. A failed fetch,
signature, trust-anchor, rollback, or build leaves the previous deployment
untouched.

The workflow defaults to the renamed catalog repository
`777genius/universal-agent-plugins-registry` and its Pages origin. Event and
manual inputs may override these values for a reviewed compatibility test, but
the source identity is always re-read and verified before deployment.

The mirror has no signing key, no GitHub write token, and no package execution
path. Direct local or Git installs continue to work when the Directory is
unavailable.
