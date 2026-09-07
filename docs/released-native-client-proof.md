# Reproduce released installer native-client evidence

This proof downloads an exact public `777genius/universal-agent-plugins` binary release
and runs the real pinned Codex, Claude Code, and OpenCode clients against it.
The nine jobs use disposable GitHub-hosted Linux arm64 (`ubuntu-24.04-arm`),
macOS arm64 and Windows amd64 runners.
Never run these clients against your existing projects or profiles.

Use a UAP tag containing `.github/workflows/agentplugins-released-native-clients.yml`.
The workflow and checkout execute at that tag's exact commit. The installer
producer tag and SHA are separate inputs; the proof checks their equality
against GitHub and records both producer and harness identities. Both sources
are in the same canonical UAP repository; the old `plugin-kit-ai` repository
name redirects there. Their release and harness commits can still differ.

```sh
gh workflow run agentplugins-released-native-clients.yml \
  --repo 777genius/universal-agent-plugins \
  --ref <immutable-UAP-harness-tag> \
  -f release_tag=agentplugins-vX.Y.Z \
  -f release_commit=<40-character-installer-producer-SHA>
gh run list --repo 777genius/universal-agent-plugins \
  --workflow agentplugins-released-native-clients.yml
# Use the resulting run ID:
gh run watch <run-id> --repo 777genius/universal-agent-plugins --exit-status
gh run download <run-id> --repo 777genius/universal-agent-plugins \
  --dir ./released-native-evidence
```

Dispatch requires repository Actions permissions. To reproduce independently,
fork the repository and dispatch the same checked-in workflow at the same
harness commit in your fork; change `--repo` above to your fork. The installer
producer remains `777genius/universal-agent-plugins`. No client account or model
credential is used.

The runner rejects draft/prerelease assets, incorrect producer commits, modified
checksums, mismatched manifest metadata, missing attestations and unexpected
binary versions. Attestations must identify the producer release workflow and
exact producer source SHA. It verifies the complete six-binary release asset set
and the selected binary's provenance before execution. Harness and probe are
source-built helpers with separate hashes; the installer is never rebuilt in
released mode. Client, scanner and ripgrep archives retain their checked-in pins.

Download all nine `released-native-client-*` artifacts. Each must have
`runner-evidence.json` with `status: passed`, all required tests passed and no
skipped tests; transcripts and fixture evidence accompany it. Identity includes
the released installer digest, producer tag/commit/tree, verified attestations,
harness commit/tree and helper hashes. Artifacts expire after 14 days, so archive
them before publication as described below. A configured lane is not successful
runtime evidence until its corresponding job passes.

These suites prove fixture installation, discovery and scripted native runtime
behavior. They do not prove real-model quality, OAuth or live external-service
availability. The separate platform proof covers additional installer targets.
Linux native client coverage is arm64 only; it does not imply Linux amd64 client
proof.

For a durable release record, the release operator downloads all nine artifacts
from the successful run, verifies each identity and test verdict above, and
packs their original contents with the run metadata (`gh run view <run-id>
--repo 777genius/universal-agent-plugins --json headSha,url,conclusion,createdAt`).
Publish the archive and its SHA-256 sidecar on a separate evidence release tied
to the exact harness SHA. Its tag must not begin with `agentplugins-v`, for example
`native-evidence-run-<run-id>`. Never add evidence assets to the installer release:
its closed set of eight assets is checked by installation and proof tooling.
Use a uniquely named archive, for example
`released-native-evidence-run-<run-id>.tar.gz`, and `gh release upload
<native-evidence-tag> <archive> <sha256-sidecar> --repo
777genius/universal-agent-plugins` without `--clobber`. Record the installer tag
and SHA, harness SHA, Actions run URL, published asset URLs and archive digest
in the separate evidence release's notes, and link that release from the durable
release documentation. Download the published assets again and verify the digest
before calling the evidence durable; this workflow has no release-write permission.

The Linux runner label is a native arm64 GitHub-hosted VM, as documented in
[GitHub's runner reference](https://docs.github.com/en/actions/reference/runners/github-hosted-runners).
All three Linux clients use the same checked-in versions as the other platforms:
Codex `0.153.4`, Claude Code `2.1.263`, OpenCode `1.18.29`. Linux archive digests
come from the exact GitHub release assets or npm version metadata and are
verified before extraction; the scanner is lintai `0.1.3` and ripgrep is `15.2.0`.
