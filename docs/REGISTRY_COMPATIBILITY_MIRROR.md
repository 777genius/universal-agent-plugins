# Registry compatibility mirror

The `agentplugins-registry-mirror` build tool keeps a product deployment
compatible with the optional Universal Agent Plugins Directory and Discovery
feeds. It is a build-time bridge, not a second registry and not an installer.

The public catalog still reads Discovery and Security at runtime. It tries the
baked same-origin copy first, then the live registry Pages origin used by the
CLI. A stale baked snapshot therefore does not hide community results while a
current signed feed is available.

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

`.github/workflows/registry-compatibility-mirror.yml` keeps the baked copy
fresh without a product-repo code change. It runs:

- every 12 hours (`27 */12 * * *`), inside the Discovery snapshot lifetime;
- on a `registry-published` repository-dispatch event;
- or manually.

The Pages deploy workflow uses the same `github-pages` concurrency group, so a
docs push and a feed refresh queue as complete artifact deploys instead of
cancelling each other. Both workflows take the previous marker from the deployed
product `MIRROR_METADATA.json`, then verify the feeds before generating the
landing site, stage the exact bytes under the Pages artifact, and deploy only
the resulting artifact. A failed fetch, signature, trust-anchor, rollback, or
build leaves the previous deployment untouched.

The workflow defaults to the renamed catalog repository
`777genius/universal-agent-plugins-registry` and its Pages origin. Event and
manual inputs may override these values for a reviewed compatibility test, but
the source identity is always re-read and verified before deployment.

The mirror has no signing key, no GitHub write token, and no package execution
path. Direct local or Git installs continue to work when the Directory is
unavailable.
