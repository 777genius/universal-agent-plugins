# Signed Directory publication ledger

Directory publication is a static, Git-backed security boundary. There is no
publication service, database, or transparency-log platform. Reviewed data on
`main` is prepared without secrets; a protected environment signs one bounded
canonical candidate and appends it to the protected publication branch.

## Artifact contract

Schema 1 is served below `registry/schemas/1/`. Historical snapshot and
envelope names are zero-padded sequences and are immutable. `latest.json`
contains only the sequence, two relative same-origin paths, and the client fetch
contract. Clients must independently enforce HTTPS, response limits, at most
two same-origin redirects, no credential forwarding on redirects, detached
SHA-256, the domain-separated Ed25519 signature, supported schema, expiry, and
their effective sequence floor. The pointer is never an authority by itself.

Snapshot bytes use sorted-key, integer-only, NFC UTF-8 JSON with no insignificant
whitespace and one final LF. The signature input is the ASCII domain
`UAP-DIRECTORY-SNAPSHOT-ED25519-V1`, a NUL byte, the eight-byte big-endian
snapshot length, and the exact snapshot bytes. The envelope separately records
the SHA-256 digest of those exact bytes.

Distribution suspension and release policy are separate. A suspended
distribution remains historical but cannot be used for install, new target, or
update. Release revocation is terminal and blocks install, new target, repair,
and rematerialization; safe removal and update to another non-revoked release
remain possible. Evidence pointer or compatibility-policy changes advance only
the snapshot sequence. Weekly refresh advances snapshot sequence and expiry
without allocating a package release. Unchanged releases reuse their original
signed source revision and `published_at` value even if `main` has advanced.
The no-secret preparer reads only canonical `registry/directory.json`, validates
every new in-repository release against the checked-out post-merge tree, and
reacquires every new or newly eligible external release at its reviewed full
SHA. It leaves new release timestamps unset; the privileged signer assigns them
from its own publication clock exactly once. Products, distributions, release
sequences, policies, current evidence pointers, and revocations come from the
canonical Directory model rather than publication configuration. A package-byte
change must allocate a higher distribution release sequence; unchanged signed
releases retain their exact source revision and original `published_at`.

## Required repository configuration

Before enabling `.github/workflows/directory-publication.yml`:

1. Generate an Ed25519 seed in an approved offline/KMS-backed process. Never
   commit it. Add its 32-byte public key (standard base64) and stable key ID to
   `trusted-keys.json` through trusted CODEOWNER review.
2. Create a dedicated GitHub App named `uap-directory-publisher`, install it
   only on this repository, and grant exactly repository **Contents: read and
   write** (plus GitHub's implicit Metadata read). Grant no Actions, Pages,
   Administration, Workflows, Environments, or other permission. Its installation
   token is the only credential allowed to update the ledger branch or create
   publication-floor tags; the workflow's generic `GITHUB_TOKEN` stays read-only.
3. Create four active repository rulesets. The branch update gate targets only
   `directory-publication-ledger`, enables **Restrict updates**, and names only
   the installed `uap-directory-publisher` App as an always-allowed bypass actor.
   A second branch immutability guard targets the same branch, blocks deletion
   and force pushes, requires linear history, and has **no bypass actors**. The
   tag creation gate targets `directory-publication-schema-1-sequence-*`, enables
   **Restrict creations**, and names only that App as an always-allowed bypass
   actor. A second tag immutability guard targets the same pattern, enables
   **Restrict updates** and **Restrict deletions**, and has **no bypass actors**.
   Layering the no-bypass guards means even the publisher cannot reset the branch
   or alter a floor tag. Do not add repository administrators, maintainers,
   teams, users, deploy keys, GitHub Actions, or the repository's generic Actions
   identity as a bypass actor; do not enable administrator bypass.
4. Create the `directory-publication` and
   `directory-publication-materialization` environments. Require trusted
   maintainer approval, prevent administrator bypass/self-review, and restrict
   both to protected `main`. Put `DIRECTORY_PUBLISHER_APP_ID` and
   `DIRECTORY_PUBLISHER_APP_PRIVATE_KEY` in both as environment secrets. Put the
   base64 32-byte `DIRECTORY_ED25519_PRIVATE_KEY` seed only in
   `directory-publication`, and set its environment variable
   `DIRECTORY_SIGNING_KEY_ID` to the reviewed key ID. Never use repository-level
   copies of these credentials.
5. Create `directory-publication-ledger` from the intended Pages seed tree and
   record its exact 40-character head. For the one and only first publication,
   manually dispatch with `initialize_ledger=true` and that exact head as
   `ledger_seed_commit`. Normal push, schedule, and dispatch events cannot
   initialize. The first signed commit persists `ledger-contract.json` and an
   immutable sequence-1 tag; every later signed commit atomically creates its
   own immutable sequence tag. Missing pointers, a non-descendant branch, or a
   sequence below the highest tag then fails closed. Never delete or recreate
   the initialization marker or publication tags.
6. Protect `main`, require CODEOWNER review for the publication scripts,
   schemas, workflow, and this configuration, dismiss stale approvals, require
   conversation resolution and status checks, and forbid bypass/force push.
7. Configure GitHub Pages for GitHub Actions. Grant the workflow its declared
   permissions. After signing, the no-secret site job generates production from
   that exact versioned snapshot, commits the static result without modifying
   `registry/`, and the deployment job archives that exact resulting ledger
   commit. Disable the legacy `Pages` workflow for production when this workflow
   is enabled; it remains suitable for explicitly unsigned pull-request previews.
8. Keep Actions restricted to immutable action SHAs and disallow workflows from
   approving pull requests. Do not add publication secrets to pull-request or
   `pull_request_target` workflows.

The checked-in trusted-key set contains the reviewed launch public key
`uap-directory-2026-01`; its private seed is not in the repository. Test private
seeds and rotation keys exist only under
`tests/fixtures/directory-publication/`. Before launch, independently derive the
public key from the environment seed and confirm it byte-for-byte against this
entry.

The App-token action is pinned to immutable commit
`bcd2ba49218906704ab6c1aa796996da409d3eb1` (`v3.2.0`). Re-verify a proposed
upgrade from a trusted terminal with `gh api` against the action's release tag
and Git tag object before changing that SHA; do not obtain pins from rendered
browser pages.

## Operation and recovery

Pushes to `main`, a weekly schedule, and manual emergency dispatch use one
non-cancelling concurrency group. `github.run_id` is the publication ID, so a
rerun after signing reuses the already committed sequence and Pages tree. A
push is attempted at most three times against the same commit; a competing
branch change fails closed instead of rebasing and allocating another sequence.

The publisher validates the latest ledger signature even after client expiry.
Expired data can supply only the sequence and immutable provenance for recovery,
never client eligibility. If Pages is stale or lost, redeploy the exact protected
branch commit. A Directory rollback is a newly reviewed, higher-sequence
snapshot. Never rewrite, delete, or re-serve an older historical artifact.

Key rotation has one concrete overlap/retirement gate:

1. Add the next reviewed public key and release a stable CLI that embeds both
   current and next keys. Do not switch the signer yet.
2. Switch `DIRECTORY_SIGNING_KEY_ID` only after that dual-key CLI is at or below
   every active release policy's `minimum_installer_version` and its bootstrap,
   current-key, and next-key verification tests pass in release CI.
3. Publish at least one next-key snapshot, then retain both keys in new clients
   for at least one full 30-day maximum snapshot lifetime after the last
   current-key snapshot expires. Retirement is allowed only when no unexpired
   current-key snapshot can satisfy the supported client's floor.
4. A later CLI may remove the retired key. Keep it in this publisher ledger
   trust file permanently so the append process can verify contiguous history.

No committee or separate key service is required, and no test key is permitted
in this file.
