# Release Notes Template

Use this template for an owner-approved `agentplugins` release.

Pair this with [REHEARSAL_TEMPLATE.md](./REHEARSAL_TEMPLATE.md) when collecting the actual evidence that feeds the final decision.

## Summary

- release tag:
- candidate commit SHA:
- release type:
- one-sentence user summary:

## Public-Stable In This Release

- list promoted stable surfaces
- list any post-`v1` promotion ledger reviewed, for example `INTERPRETED_STABLE_SUBSET_AUDIT.md`

## Why This Release Matters

- explain the main user-facing outcome in plain language
- say who benefits first
- avoid release-process wording in the first paragraph

## What Changed For Users

- list the main user-facing changes
- prefer concrete outcomes over internal implementation details

## What To Do Now

- list the default recommendation after this release
- list any migration or upgrade action a user should actually take

## Release evidence

- required:
- polyglot-smoke:
- generated-config/runtime-contract drift:
- version-sync-check:
- extended:
- live:
- agentplugins draft qualification:
- agentplugins native release and attestations:
- universal-agent-plugins npm publish and public verification:
- npm runtime-package publish:
- npm runtime-package postpublish registry smoke:
- npm runtime-package live install:
- PyPI runtime-package publish:
- PyPI runtime-package postpublish registry smoke:
- PyPI runtime-package live install:
- waivers:

## Known Limitations

- list documented limitations
- distinguish historical tests from current product claims

## Decision Record

- final audit outcome:
- any `stays-beta` decision:
- maintainer sign-off:
