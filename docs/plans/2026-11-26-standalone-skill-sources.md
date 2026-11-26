# Standalone Skill sources and pending installation qualification

## Owner scope

On 2026-11-26 the owner authorized parallel completion of the pending
installation work, generic/private Git Skills, local Skill snapshots and the
extension-policy documentation correction, including review, E2E and merging
the resulting pull requests. Keep the existing UX and avoid a second installer,
resolver, store or background service. Existing ownership, immutable identity,
containment, rollback and recovery rules remain authoritative.

Baseline: `a19856c9b3236496427006a7091433d8dbc79a79`. PR #1071 is already
merged; do not repeat its Cursor work. PR #1114 remains deferred and VS Code
remains frozen. The separate #1078 audit work is not taken over.

## Parallel work and integration order

| Lane | Bounded result | Shared authority |
| --- | --- | --- |
| Generic/private Git Skills | Standalone Skills from native HTTPS and SSH Git sources, including authenticated GitHub URLs | Existing `sourcespec`, native Git transport, `skillsource.Normalize`, environment pins and installer |
| Local Skill folders | `skills add ./skills/review` saves an immutable snapshot; explicit update adopts changed source bytes | Existing local snapshot, Skill parser, normalized package, environment store and lifecycle |
| Existing PRs | Resolve actual failures/conflicts and finish #1118, #1120, #1023, #1016 and #1093 at their final source | Existing review, required checks and affected native E2E; no inferred success from queues |
| Extension documentation | Record the merged upstream 1.1 proposal and closed issue, retaining published 1.0 behavior | Pinned conformance schema, corpus and loading policy |

Git and local work use isolated branches. Git owns the first shared source-model
and batch-selection changes; local integrates that commit before editing the
same seams. Merge existing PRs serially after current-head review and actual
qualification. Review their combined source rather than borrowing incompatible
evidence from older heads. Do not overwrite a branch that another worker moved.

## Git source contract

- Reuse existing native Git syntax: HTTPS clone URLs, `git+https://`, `ssh://`,
  SCP syntax and `#ref//path`. Keep GitHub shorthand, folder/blob/raw URLs and
  their historical source and binding identities compatible.
- Ordinary standalone GitHub URLs use already configured native Git credentials.
  Signed Directory and hosted anonymous consumers retain their existing transport
  contracts. Do not add a credential store or a new authentication flag.
- Explicit Git syntax routes to Git; arbitrary HTTPS publisher URLs retain the
  existing website Skill flow. Ambiguous HTTPS inputs can use `git+https://`.
- Saved Git intent selects exactly one repository or URL. Generic Skill keys
  hash a length-unambiguous canonical endpoint/name/requested-path tuple in a
  separate namespace. Equal Skill names on different hosts remain distinct.
- A thin raw-Skill acquisition wrapper reuses native Git transport while rejecting
  root and subtree gitlinks. Immutable discovery fetches all required objects with
  native credentials; later object reads must not silently become anonymous fetches.
- Re-add retains a saved pin. Explicit update resolves a newer revision or the
  existing exact-name relocation rule; ambiguous relocation refuses before apply.

## Local snapshot contract

- Accept one explicit local Skill directory with its bounded `SKILL.md` and
  complete resources. Reuse the shared conformance parser and directory/name
  rule. Do not execute scripts, watch files or install live symlinks.
- Extend the existing environment source union with a local Skill arm. Local
  dependencies are root-only; published packages cannot require machine paths.
  Reject mixed source arms and cover every source-selection branch explicitly.
- Persist a canonical absolute path in a host-neutral spelling, the Skill name,
  raw tree digest and existing normalizer version. Reuse existing source facts
  and durable receipts instead of adding a second receipt/state format.
- Re-add and restore retain the pinned bytes. Explicit update snapshots the
  selected source again. Prefer the verified store on replay; if it is missing,
  reopen only a valid native path and require the old raw digest exactly.
  A missing, changed or foreign-OS source refuses clearly rather than selecting
  a replacement. Warm verified-store replay does not depend on source availability.
- Export/import keeps its current manifest-and-lock contract, without a new
  byte-bundle format. It preserves the pin; a cold destination still needs the
  source or an already verified store. Host-based policies keep their current
  fail-closed treatment of an unsupported source kind.
- Existing local plugin `add` behavior is unchanged. The new saved behavior is
  specific to explicit standalone Skill installation.

## Review and verification

Plan review round 1: Git and local owners independently reviewed each other's
source, identity, acquisition and replay contracts. Required corrections included
native credentials for canonical GitHub, strict raw gitlinks, explicit source-union
branches, stable-key precedence, root-only local dependencies and honest export
semantics. Root round 2 accepted those corrections before implementation.

Implementation review must inspect the resulting shared source-kind handling and
unchanged historical keys. Run focused parser, identity, transport and lifecycle
tests, `make lint` and `make test-core`; installer changes also run the applicable
compatibility/polyglot checks and `python scripts/package-manager-e2e.py`.
Use existing real TLS/credential-helper and SSH fixtures. Verify full resources,
executable inventory, contained links, escaping-link/gitlink refusal, authentication
failure before durable mutation, branch movement, cold locked restore, explicit
update, repair and removal. Local tests additionally cover changed/missing source,
wrong-OS reopening, warm-store replay and manifest/lock roundtrips.

Run affected actual native and cross-platform cases on the final source. Queued,
cancelled, skipped or evidence-only jobs are not passing product evidence. A real
failure is investigated and fixed; tests and ownership refusals are not weakened.
Push logical commits promptly. Finish independent review and required checks
before merging; release qualification remains separate from source availability.

## Extension policy correction

Upstream issue #77 closed and PR #82 merged on 2026-11-06. The proposal changes
the 1.1 working draft and explicitly leaves published 1.0 unchanged. Add this
current status after the historical 2026-09-06 review record. Do not change the
pinned schema/corpus, add 1.1 support, or silently relax malformed member handling.

## Public documentation

Update CLI help and the existing Skill installer documentation with practical
examples and the snapshot/update boundary. State source availability truthfully;
new main-branch behavior is not a released feature until the release qualifies.
Any changed public journey page needs matching availability and routes in all nine
maintained locales. No new command family, mode, watcher or runtime probe is needed.
