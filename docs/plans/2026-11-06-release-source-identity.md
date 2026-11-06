# Release workflow and source identity separation

The operator must be able to select any reachable repository commit, including a
commit which predates the release workflow changes. A release must continue to
bind immutable assets to both the selected source and a trusted workflow.

## Identity contract

- `source_commit` is the exact commit dereferenced from the immutable stable tag.
  Every build and source qualification checks out that SHA, and the manifest
  records it. The tag is checked again before draft promotion and npm publish.
- `workflow_commit` is the exact reviewed release harness revision on main.
  It is fixed at dispatch, retained in the qualification receipt, and used by
  all later phases. No later phase may substitute a newer main or execute
  release scripts from the selected source as trusted verification tools.
- GitHub attestations identify the workflow run's source (`workflow_commit`),
  not the independently checked-out source. Verify the attestation against the
  frozen workflow identity and independently verify `source_commit` in the
  signed manifest and qualification receipt. Do not relabel one as the other.
- The draft receipt binds tag, source commit, workflow commit, run ID and
  attempt, assets and the exact npm tarball. Promotion and npm reuse must reject
  any mismatch, changed tag, stale attempt or expired/missing receipt.
- A historical standalone audit remains bound to its historical workflow; it
  cannot qualify a new release via a modern harness without the new receipt.

## Changes needed across the pipeline

1. Dispatch the reviewed workflow from an immutable harness ref, separately
   select the tag, and pin both SHAs. Source checks use the selected commit;
   verification tools use the harness. Remove the merged-PR prerequisite while
   retaining applicable release qualification and environment protections.
2. Pass both identities through platform proof, asset attestations, verified
   draft and promotion. The six platform jobs must exercise the exact built
   assets and tarball. Fetch source separately from proof tooling.
3. Propagate the frozen harness identity into npm prepare and publication;
   verify the same tarball and manifest before and after publication. The npm
   dispatch must not resolve a moving branch or execute old source as authority.
4. Update the runbook and adversarial release rehearsal for distinct SHAs,
   absent PR, non-main source, tag movement, changed workflow, mismatched receipt,
   substituted assets, retry and npm handoff. Keep existing same-SHA and
   historical contracts covered.

This document describes work to implement, not evidence that the new path is
available or that any candidate has qualified.
