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

Evidence entries in review source are pointers, not signed summaries. The
preparer fetches the artifact blob from the exact repository commit and path,
checks its SHA-256 and evidence-artifact schema, and derives every signed
summary field from those verified bytes. A `github_actions` pointer is accepted
only when `/usr/bin/gh attestation verify` proves the blob was attested by a
workflow with a code-owned `trusted_evidence_workflows` policy. That policy
binds the attestation to one protected source ref and requires its source digest
to equal the evidence artifact commit. Self-hosted runner attestations are
denied unless the reviewed policy explicitly permits them. A
`reviewed_external` pointer is accepted only when its complete repository,
revision, path, and digest tuple is present in the code-owned
`trusted_external_evidence` list. Both lists are empty by default. Missing,
malformed, digest-mismatched, unattested, or merely self-asserted evidence fails
before a candidate exists; the signing seed is not present during any fetch or
attestation operation.

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
3. Create eight active repository rulesets. The ledger branch update gate targets only
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
   identity as a bypass actor; do not enable administrator bypass. The
   launch-approval tag creation gate targets only
   `directory-publication-schema-1-launch-approved`, enables **Restrict
   creations**, and names only the installed App as an always-allowed bypass
   actor. Its paired immutability guard enables **Restrict updates** and
   **Restrict deletions** with **no bypass actors**. This fixed tag is absent
   before launch and may be created only by the post-ceremony job; never create,
   move, or delete it manually.
   In addition, split `main` protection into two rulesets before enabling
   publication. The `main` update/review gate must retain required review and
   required status checks for everyone except the installed dedicated
   `uap-directory-publisher` App, which is its only always-allowed bypass actor.
   In a solo-maintainer repository, the Repository administrators role may also
   receive `pull_request`-only bypass so an explicitly approved green PR remains
   mergeable. It must never receive always-allowed bypass or permission to push
   directly to `main`.
   The separate `main` immutability guard must block deletion and force pushes,
   require linear history, and have **no bypass actors**. This narrowly permits
   the App's direct same-tree marker fast-forward while preventing the App from
   rewriting or deleting `main`. This ruleset split is a production prerequisite;
   do not run publication until a maintainer has configured and independently
   reviewed it. No GitHub setting or credential is changed by this repository
   patch, and a disposable-App test is intentionally deferred.
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
   the initialization marker or publication tags. Initialization alone does
   not approve launch and must not create the launch-approval tag.
6. Require CODEOWNER review for the publication scripts, schemas, workflow, and
   this configuration; dismiss stale approvals and require conversation
   resolution and status checks. Configure the split `main` rulesets from step
   3 rather than a single rule that would reject the marker fast-forward.
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
non-cancelling concurrency group. For source head `S`, the no-secret jobs derive
a deterministic marker `P` from `S` and `github.run_id`. `P` has exactly parent
`S`, the same tree, a fixed publisher identity, and a bounded hash-derived UTC
timestamp. It therefore changes no paths and does not recurse through the
workflow's path-filtered push trigger. Candidate `source_commit`, new
in-repository release revisions, generated-site provenance, and downstream
materialization all bind to `P`.

The publication push is a real GitHub receive-pack update of `main` from `S` to
`P`. One atomic command updates `main`, advances the ledger from exact `L` to
signed commit `Q`, and creates the absent immutable sequence tag at `Q`; every
ref has an explicit expected-old lease. Each of at most three attempts reads all
three refs and accepts only the exact pre-state or the exact committed state.
Any mixed state is terminal. An ambiguous or lost response is resolved by the
same exact multi-ref readback. A retry never recreates the marker, regenerates
or rebases `Q`, or reallocates the sequence. `github.run_id` is the publication
ID, so an exact rerun recognizes the already-committed `P`/`Q`/tag state.
Materialization separately advances the ledger with an explicit exact `Q`
lease and accepts only exact idempotent readback; a competing ledger child fails
closed.

The only accepted post-signing state is `(main=P, ledger=M, tag=Q)`, where `M`
is the single-parent site-materialization child of `Q`, has the fixed
materialization commit message, and leaves all `registry/` bytes unchanged. An
exact run retry authenticates that tuple, skips the initial three-ref CAS,
reuses `Q` and its signed bytes, rebuilds the site, and requires the rebuilt tree
to equal `M` before reusing it. Any unrelated descendant, registry change,
moved tag, or different rebuilt tree is terminal.

Pushing `M` is staging, not production promotion. The gate reads `latest.json`
and both versioned artifacts from the immutable raw-commit origin for `M` and
requires the exact run publication ID, sequence, snapshot digest, `Q` tag, and
ledger identity. Sequence 1 additionally requires the complete stable-launch
runtime, OAuth, and external-PR ceremony before Pages may promote `M`. Only
after that ceremony succeeds, a job in the protected `directory-publication`
environment creates the absent, immutable
`directory-publication-schema-1-launch-approved` tag at that exact `M`. It
first reacquires protected `main` and the ledger branch, requires both exact
ceremony heads, and validates the state contract in
`launch-approved-marker.json`. An exact rerun accepts only the existing tag at
the same commit.

Every deployment, including sequence 1, separately reacquires and validates
that protected marker. Its target must be the single-parent materialization
child of the immutable sequence-1 tag, must leave signed `registry/` bytes
unchanged, and must be an ancestor of the current materialized ledger head. The
repository identity, schema, bootstrap seed contract, sequence-tag namespace,
launch signing key, snapshot paths, and initial sequence must match the
code-owned marker contract. Thus a failed sequence-1 ceremony followed by any
number of newly allocated sequences remains blocked: a higher sequence may
skip the launch-only runtime, but it cannot skip the marker gate. Replayed,
moved, rollback, cross-repository, unrelated-lineage, and stale-head markers
fail closed, and pull-request artifacts are not consumed by either marker job.

After approval, weekly expiry refreshes, evidence-only snapshots, suspensions,
and emergency revocations remain independent of the launch-only runtime. They
still require signing, materialization, exact staging, and a marker whose
approved lineage is an ancestor of the current CAS/ledger head. A failed
deployment or gate leaves the prior GitHub Pages production pointer in place
and is retried from the already signed `Q` and authenticated `M`; it never
allocates a new sequence.

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
