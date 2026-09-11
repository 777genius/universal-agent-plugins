# Reproduce released installer native-client evidence

This proof acquires an exact `777genius/universal-agent-plugins` installer in
one of two explicit modes: `public` downloads a published stable release;
`draft` authenticates an unpublished draft and its frozen producer npm bundle.
Both run genuine pinned Codex, Claude Code, and OpenCode clients in scripted
fixtures. Select `target_scope=historical-nine` for nine jobs on disposable
GitHub-hosted Linux arm64 (`ubuntu-24.04-arm`), macOS arm64 and Windows amd64
runners, or `target_scope=linux-amd64` for three jobs on `ubuntu-24.04`.
Linux amd64 is supplemental coverage, not a replacement for the historical nine.
Draft 0.1.55 has two separately qualified scopes: [historical-nine run 34309083566](https://github.com/777genius/universal-agent-plugins/actions/runs/34309083566) and [Linux amd64 run 34307075390](https://github.com/777genius/universal-agent-plugins/actions/runs/34307075390). Their distinct harness identities and final draft recheck times are recorded in [the compact evidence metadata](https://github.com/777genius/universal-agent-plugins/blob/main/docs/evidence/client-compatibility-draft-0.1.55.json); 0.1.55 remains unpublished.
Never run these clients against your existing projects or profiles.

For public reproduction, use an immutable UAP harness tag containing
`.github/workflows/agentplugins-released-native-clients.yml`.
The public workflow and checkout execute at that tag's exact commit.
Draft dispatch is restricted to this workflow in the canonical repository on
`main`; each run checks out its captured immutable `github.sha`
and binds that harness commit/tree into its receipts.
The installer producer tag and SHA are separate inputs; the proof
checks the producer tag's commit against GitHub and records both identities.
Both sources are in the canonical UAP repository; the old `plugin-kit-ai`
name redirects there. Producer and harness commits may differ.

For public acquisition, omit all draft provenance inputs:

```sh
gh workflow run agentplugins-released-native-clients.yml \
  --repo 777genius/universal-agent-plugins \
  --ref <immutable-UAP-harness-tag> \
  -f release_state=public \
  -f target_scope=historical-nine \
  -f release_tag=agentplugins-vX.Y.Z \
  -f release_commit=<40-character-installer-producer-SHA>
gh run list --repo 777genius/universal-agent-plugins \
  --workflow agentplugins-released-native-clients.yml
# Use the resulting run ID:
gh run watch <run-id> --repo 777genius/universal-agent-plugins --exit-status
gh run download <run-id> --repo 777genius/universal-agent-plugins \
  --dir ./released-native-evidence
```

Dispatch requires repository Actions permissions. To reproduce public mode independently,
fork the repository and dispatch the same checked-in workflow at the same
harness commit in your fork; change `--repo` above to your fork. The installer
producer remains `777genius/universal-agent-plugins`. No client account or model
credential is used.

Public mode rejects draft/prerelease assets and all draft provenance inputs.
Both modes reject incorrect producer commits, modified
checksums, mismatched manifest metadata, missing attestations and unexpected
binary versions. Attestations must identify the producer release workflow and
exact producer source SHA. It verifies the complete six-binary release asset set
and the selected binary's provenance before execution. The current notices-bearing
asset set is **nine**: six binaries, `release-manifest.json`, `checksums.txt`,
and `THIRD_PARTY_NOTICES.txt`, as enforced by `release-assets.js` and the draft
archive verifier. Historical 0.1.53 has eight assets (no companion notices).
Harness and probe are
source-built helpers with separate hashes; the installer is never rebuilt in
released mode. Client, scanner and ripgrep archives retain their checked-in pins.

Download all `released-native-client-*` artifacts for the selected scope
(nine for `historical-nine`, three for `linux-amd64`). Each must have
`runner-evidence.json` with `status: passed`, all required tests passed and no
skipped tests; transcripts and fixture evidence accompany it. Identity includes
the released installer digest, producer tag/commit/tree, verified attestations,
harness commit/tree and helper hashes. Artifacts expire after 14 days, so archive
them privately before expiry as described below; publication is optional.
A configured lane is not successful
runtime evidence until its corresponding job passes.

These suites prove fixture installation, discovery and scripted native runtime
behavior. They do not prove real-model quality, OAuth or live external-service
availability. The separate platform proof covers additional installer targets.
The original nine-lane 0.1.53 run covered Linux arm64; [supplemental released-0.1.53 run 34195481283](https://github.com/777genius/universal-agent-plugins/actions/runs/34195481283) passed all three Linux amd64 jobs and six required tests without skips, as recorded in [its separate evidence metadata](https://github.com/777genius/universal-agent-plugins/blob/main/docs/evidence/client-compatibility-linux-amd64-0.1.53.json). This supplements the original nine lanes without rewriting their evidence.

## Draft dispatch and acquisition

Dispatch only after the exact producer attempt has passed all six platform
proofs, their aggregate and `verified-draft`. Draft dispatch requires the
canonical repository's workflow on `refs/heads/main`; public-mode fork and
immutable-tag instructions do not apply to draft acquisition.
Freeze the release ID, producer run/attempt, `agentplugins-npm-<version>` artifact
ID and original ZIP digest, and the digest of `checksums.txt`. IDs/attempts are
positive integers; both digests are 64 lowercase hex characters (no `sha256:`
prefix), and the producer SHA is 40 lowercase hex characters.
An in-progress producer attempt does not satisfy this prerequisite.

```sh
gh workflow run agentplugins-released-native-clients.yml \
  --repo 777genius/universal-agent-plugins \
  --ref main \
  -f release_state=draft \
  -f target_scope=historical-nine \
  -f release_tag=agentplugins-vX.Y.Z \
  -f release_commit=<40-character-installer-producer-SHA> \
  -f producer_run_id=<successful-producer-run-ID> \
  -f producer_run_attempt=<successful-producer-attempt> \
  -f release_id=<frozen-draft-release-ID> \
  -f expected_asset_set_digest=<checksums.txt-SHA256> \
  -f producer_artifact_id=<producer-npm-artifact-ID> \
  -f producer_artifact_digest=<original-producer-artifact-ZIP-SHA256>
```

Dispatch separately with `target_scope=linux-amd64` for the three supplemental
lanes, again using `--ref main` and retaining the same frozen producer and
release identities. Capture each scope's resolved harness SHA from its run,
then resolve its tree:

```sh
gh run view <scope-run-id> --repo 777genius/universal-agent-plugins \
  --json headSha,url,attempt
gh api repos/777genius/universal-agent-plugins/git/commits/<resolved-harness-SHA> \
  --jq '.tree.sha'
```

Record these identities for both the nine-lane and three-lane dispatches and
require each scope's receipts to match its recorded harness SHA/tree and helper
hashes. Two independently qualified scopes may use distinct recorded harness
identities. Each scope must have passed its own exact required lanes,
`draft-complete` aggregation and final `draft-recheck`. Both scopes must bind
the same frozen producer invocation (commit/tree and run/attempt), release
identity and asset set, original producer bundle bytes and package bytes.
Archive and verify each scope separately using its recorded identities.
Do not substitute lanes across scopes, mix lanes from different dispatches or
harness identities within one scope, or rewrite receipts. A partial or failed
historical-nine run, including its successful Linux arm64 lanes, cannot replace
any lane of a new historical-nine qualification.
Combined coverage must record both scopes' harness identities and their
qualification times, including each final draft recheck time. A retained,
fully qualified scope may be reused without a current recheck; its original
qualification remains tied to its recorded time and harness identity.
Use the watch and download commands above for each run, with separate fresh
output directories.

Draft mode verifies the exact successful producer attempt and artifact ID/name,
the still-unpublished, non-prerelease draft and all nine attested assets. It uses
the original npm tarball from that producer artifact, never a local rebuild,
repack or registry substitute. Package pins and notices must match the frozen
release. In a disposable project it installs offline with scripts disabled,
checks installed bytes, proves cold bootstrap from the frozen binary and a warm
launch without that proof source, then runs the cached binary in native fixtures.
Acquisition authentication is not inherited by npm or client child environments.
This proof assumes trusted pinned clients, the canonical package and
repository-owned fixtures. The child environment provides no OS process
isolation from the runner or the subsequent artifact-upload step. Network
access remains enabled, no real-model or OAuth credentials are supplied, and
collected logs are not guaranteed to be secret-free.

Require every selected lane to pass without skips and the `draft-complete` job
to pass its read-only aggregation of lane/package/helper identities, fixture
evidence and snapshot binding, producing the `draft-lanes` receipt.
Then require `draft-recheck` to pass: this metadata-only job authenticates the
snapshot and lanes receipts, reauthenticates the producer and reverifies the
live draft before uploading `draft-native-qualification-<scope>` with
`qualification.json` and `final-draft.json`.
The metadata-only `draft-snapshot` and `draft-recheck` jobs require
`contents: write` for draft visibility, even though they do not publish releases.
Retain the qualification files alongside the lanes and original producer ZIP
using the private offline archive instructions below. Qualification is scoped
to that dispatch; publication of evidence is not required. This installer proof
does not qualify the standard authoring executable release (D5).

## Optional historical public archive procedure

This is the historical nine-lane public evidence procedure. Any new publication
requires explicit owner permission after disclosure review; draft qualification
uses unpublished/private archives and does not depend on publication.

For a durable release record, the release operator downloads all nine artifacts
from the successful run, verifies each identity and test verdict above, and
packs their original contents with the run metadata (`gh run view <run-id>
--repo 777genius/universal-agent-plugins --json headSha,url,conclusion,createdAt`).
Publish the archive and its SHA-256 sidecar on a separate evidence release tied
to the exact harness SHA. Its tag must not begin with `agentplugins-v`, for example
`native-evidence-run-<run-id>`. Never add evidence assets to the installer release:
the historical 0.1.53 closed set of eight assets is checked by installation and
proof tooling (current notices-bearing releases have nine).
Use a uniquely named archive, for example
`released-native-evidence-run-<run-id>.tar.gz`, and `gh release upload
<native-evidence-tag> <archive> <sha256-sidecar> --repo
777genius/universal-agent-plugins` without `--clobber`. Record the installer tag
and SHA, harness SHA, Actions run URL, published asset URLs and archive digest
in the separate evidence release's notes, and link that release from the durable
release documentation. Download the published assets again and verify the digest
before calling the evidence durable; public-mode workflow jobs have no
release-write permission.

The Linux runner label is a native arm64 GitHub-hosted VM, as documented in
[GitHub's runner reference](https://docs.github.com/en/actions/reference/runners/github-hosted-runners).
All three Linux clients use the same checked-in versions as the other platforms:
Codex `0.153.4`, Claude Code `2.1.263`, OpenCode `1.18.29`. Linux archive digests
come from the exact GitHub release assets or npm version metadata and are
verified before extraction; the scanner is lintai `0.1.3` and ripgrep is `15.2.0`.

## Verified 0.1.53 run and archive reproduction

Run [34166817934](https://github.com/777genius/universal-agent-plugins/actions/runs/34166817934)
passed all nine jobs using public installer commit
`28cf05af0a1e4fea642825dd34b78f9c99094ab5` and harness commit
`9c33cfac81fc00e61205591321c89c5675fedb60`, tree
`d17e72793961e6798d09eb44256c832465f2d8c7`. Its 529 original files plus summary
were frozen in `uap-0.1.53-native-34166817934.zip` (3,165,833 bytes), SHA-256
`0d315f3a24bb6c41d184f0632d76a7228e18a6c1c41be2774e23c8066dfd3af6`.

The [public evidence release](https://github.com/777genius/universal-agent-plugins/releases/tag/native-evidence-0.1.53-34166817934)
points to that exact harness commit. Its ZIP and `.sha256` sidecar were
re-downloaded after publication and checked against the digest above on
2026-09-08. The installer release's eight assets remain untouched.

The verifier is added by a later documentation commit. It was not part of the
runtime run and is not source for the released binary. To reproduce verification,
use a clean checkout at the documentation commit recorded with the evidence
publication, retaining its sibling `run-native-client-matrix.py` pins. Compare
`scripts/verify-released-native-evidence.py` with the SHA-256 in
[the compact record](evidence/client-compatibility-released-0.1.53.json).
From that exact source checkout, use the durable public archive as the primary
input. Choose fresh paths; the extraction below refuses overwritten files and
keeps the archive summary outside the required nine-directory input:

```sh
gh release download native-evidence-0.1.53-34166817934 \
  --repo 777genius/universal-agent-plugins --dir ./native-053-download
python - <<'PYTHON'
from pathlib import Path
import hashlib, stat, zipfile
root = Path("native-053-download")
archive = root / "uap-0.1.53-native-34166817934.zip"
expected = "0d315f3a24bb6c41d184f0632d76a7228e18a6c1c41be2774e23c8066dfd3af6"
if hashlib.sha256(archive.read_bytes()).hexdigest() != expected:
    raise SystemExit("archive digest mismatch")
if (root / (archive.name + ".sha256")).read_text().split() != [expected, archive.name]:
    raise SystemExit("checksum sidecar mismatch")
output = root / "native-053-input"
output.mkdir()
with zipfile.ZipFile(archive) as zipped:
    for item in zipped.infolist():
        parts = item.filename.split("/")
        if (any(p in ("", ".", "..") for p in parts)
                or any(ord(c) < 32 or c in "\\:" for c in item.filename)
                or not stat.S_ISREG(item.external_attr >> 16)):
            raise SystemExit("unsafe archive entry")
        if item.filename == "summary.json":
            destination = root / "original-summary.json"
        else:
            if not parts[0].startswith("released-native-client-"):
                raise SystemExit("unexpected archive entry")
            destination = output.joinpath(*parts)
        destination.parent.mkdir(parents=True, exist_ok=True)
        with destination.open("xb") as stream:
            stream.write(zipped.read(item))
PYTHON
python scripts/verify-released-native-evidence.py ./native-053-download/native-053-input \
  --release-version 0.1.53 \
  --release-commit 28cf05af0a1e4fea642825dd34b78f9c99094ab5 \
  --harness-commit 9c33cfac81fc00e61205591321c89c5675fedb60 \
  --harness-tree d17e72793961e6798d09eb44256c832465f2d8c7 \
  --archive ./reproduced-native-053.zip > ./reproduced-native-053-summary.json
```

The tool verifies exact job/fixture identities, required stage results, pinned
client/tool records and every indexed file digest. Embedded release provenance
was verified by the runner; this offline step checks recorded consistency, not
fresh signatures. ZIP bytes are deterministic on the same Python/compression
runtime; across implementations, compare member bytes and hashes rather than
assuming identical compression output. It does not execute clients.

While Actions artifacts remain available, `gh run download 34166817934 --repo
777genius/universal-agent-plugins --pattern 'released-native-client-*' --dir
./native-053-input` is an alternative acquisition route. It expires; the public
release ZIP above is the durable route. Never execute log or fixture contents.

For this historical public procedure, any new publication requires explicit
owner permission after disclosure review. Publication is optional and separate
from the read-only verifier: create
the non-installer tag at the original harness SHA, attach the exact ZIP and
checksum sidecar without overwrite, and include release/harness identities,
run URL, archive hash and verifier source digest in the notes. Record the later
documentation commit as the verifier source when available. Re-download both
published files and check their bytes before changing the compact record's
publication status. Original logs intentionally retain dummy fixture keys,
loopback ports, runner paths and built-in client prompts. Their disclosure review
must inspect content; safe paths and file extensions do not sanitize logs.

## Private offline draft archives

Keep draft evidence unpublished; any later publication requires explicit owner
permission after disclosure review. Preserve the existing scope-2
`draft-native-qualification-<scope>` artifact's original `qualification.json`
and `final-draft.json` in a separate directory, and all expected
`released-native-client-<client>-<target>` directories with every original indexed
log, fixture and initial receipt under the lane input directory. Also preserve the
**original Actions ZIP bytes**, selected by producer run/attempt and artifact ID,
for `agentplugins-npm-<version>` before its **7-day retention** expires (native
lanes and qualification expire after 14 days); re-zipping its extracted files
changes the artifact digest. That ZIP retains the exact tested tarball, verification
record and complete original release assets including notices. Run
`python3 scripts/verify-released-native-evidence.py <lane-directory> --release-state draft --qualification <qualification-directory> --producer-bundle <original-producer.zip> --release-version <version> --release-commit <producer-SHA> --harness-commit <harness-SHA> --harness-tree <harness-tree> --scope <historical-nine-or-linux-amd64> --producer-run-id <producer-run> --producer-run-attempt <attempt> --producer-artifact-id <artifact-ID> --producer-artifact-digest <original-ZIP-SHA256> --release-id <release-ID> --expected-asset-set-digest <checksums.txt-SHA256> --archive <new-private-archive.zip>`.
Use identities from the original run records, not latest/name-only selection.
The archive retains input bytes plus an offline summary; create an exclusive
SHA-256 sidecar (for example, `(set -C; sha256sum <new-private-archive.zip> > <new-private-archive.zip.sha256>)`),
retain both privately with the harness run URL/attempt and original artifact
IDs/digests, and verify the sidecar after copying. To reverify, first check that
sidecar, safely unpack into fresh paths, keep `summary.json` outside the lane
input, and pass the archived `draft-qualification/` and `producer-artifact.zip`
to the same command. This checks recorded consistency, **not fresh attestations
or current draft state**; it executes no installer or clients. Recorded helper
hashes bind the original harness, not the later offline verifier checkout.
Unsupported schemas fail for review. This adds no compatibility rows, public
evidence publication, real-model/OAuth claim or authoring D5 qualification;
only successful real runs can support later matrix changes. Never overwrite
original evidence, repack the frozen npm tarball, or attach evidence to the
installer release. Historical 0.1.53 reproduction above remains unchanged.
