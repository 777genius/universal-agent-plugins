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
The no-secret preparer leaves new release timestamps unset; the privileged
signer assigns them from its own publication clock exactly once.
`release_sequences` and `distribution_status` in `config.json` use a
distribution ID such as `777genius/context7`. Release policy and current
evidence overrides use the complete immutable identity, for example
`777genius/context7@2`. A package-byte change must allocate exactly the next
distribution release sequence; unchanged bytes are rejected as a new release.

## Required repository configuration

Before enabling `.github/workflows/directory-publication.yml`:

1. Generate an Ed25519 seed in an approved offline/KMS-backed process. Never
   commit it. Add its 32-byte public key (standard base64) and stable key ID to
   `trusted-keys.json` through trusted CODEOWNER review.
2. Create the `directory-publication-ledger` branch by seeding it with the exact
   current Pages tree. Protect it against deletion and force pushes, require
   linear history, restrict pushes to GitHub Actions for this repository, and
   require the publication status check. Human and deploy-key pushes stay
   disabled.
3. Create the `directory-publication` environment. Require trusted maintainer
   approval and restrict it to the protected `main` branch. Add the 32-byte seed
   as base64 secret `DIRECTORY_ED25519_PRIVATE_KEY` and add repository variable
   `DIRECTORY_SIGNING_KEY_ID` with the reviewed key ID.
4. Protect `main`, require CODEOWNER review for the publication scripts,
   schemas, workflow, and this configuration, dismiss stale approvals, require
   conversation resolution and status checks, and forbid bypass/force push.
5. Configure GitHub Pages for GitHub Actions. Grant the workflow its declared
   `pages: write` and `id-token: write` permissions. The deployment job archives
   the exact ledger commit emitted by the signing job; it never rebuilds a
   different tree. Disable the legacy `Pages` workflow when this workflow is
   enabled so two workflows cannot race to deploy different source trees.
6. Keep Actions restricted to immutable action SHAs and disallow workflows from
   approving pull requests. Do not add publication secrets to pull-request or
   `pull_request_target` workflows.

The checked-in trusted-key set is intentionally empty until a production public
key completes that review. Test private seeds and rotation keys exist only under
`tests/fixtures/directory-publication/`.

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

For key rotation, first release clients trusting both current and next public
keys, then add the next key here and change `DIRECTORY_SIGNING_KEY_ID`. Remove
the retired public key from client trust only after the documented overlap
window. Keep historical public keys in this publisher ledger-trust file so each
append can verify the complete contiguous snapshot history. No test key is
permitted in this file.
