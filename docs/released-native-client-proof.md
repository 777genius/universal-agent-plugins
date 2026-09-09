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

Publication is an operator step, separate from the read-only verifier: create
the non-installer tag at the original harness SHA, attach the exact ZIP and
checksum sidecar without overwrite, and include release/harness identities,
run URL, archive hash and verifier source digest in the notes. Record the later
documentation commit as the verifier source when available. Re-download both
published files and check their bytes before changing the compact record's
publication status. Original logs intentionally retain dummy fixture keys,
loopback ports, runner paths and built-in client prompts. Their disclosure review
must inspect content; safe paths and file extensions do not sanitize logs.

For **offline draft archives**, preserve the existing scope-2
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
