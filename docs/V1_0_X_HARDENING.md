# `v1.0.x` Hardening Loop

This document defines the allowed scope immediately after `v1.0`.

## Goal

Stabilize the newly promoted `public-stable` contract in production reality before opening `v1.1` scope.

## Allowed Scope

- bug fixes inside approved stable surfaces
- docs clarifications
- diagnostics tightening that does not break stable failure families
- release automation hardening
- Codex smoke reliability improvements that do not widen the contract promise
- install path and checksum hardening inside the existing contract
- beta contract cleanup, change-note hygiene, and documentation follow-through for beta leftovers

## Disallowed Scope

- new stable APIs
- new hooks or new platforms
- unified API experiments in stable surface
- breaking rename or removal in approved stable set
- widening the stable promise without a new reviewed promotion ledger

## First Post-`v1` Backlog

1. collect first-wave user feedback on approved stable surfaces
2. carry `plugin-kit-ai install` from local compatibility matrix coverage to refreshed live tarball/unsupported evidence
3. improve Codex runner story only as operational reliability work
4. keep beta leftovers intentional and documented
5. begin `v1.1` planning only after at least one quiet `v1.0.x` cycle

## Current Patch Candidate Focus

- package-standard authoring is now the only supported authored shape
- Gemini now has a promoted production-ready Go runtime lane for `SessionStart`, `SessionEnd`, `BeforeModel`, `AfterModel`, `BeforeToolSelection`, `BeforeAgent`, `AfterAgent`, `BeforeTool`, and `AfterTool`, with the stable boundary recorded in [GEMINI_RUNTIME_AUDIT.md](./GEMINI_RUNTIME_AUDIT.md)
- the community-first interpreted local-runtime promotion is recorded in [INTERPRETED_STABLE_SUBSET_AUDIT.md](./INTERPRETED_STABLE_SUBSET_AUDIT.md): `python` and `node` are now the stable repo-local subset on `codex-runtime` and `claude`, while `shell` remains `public-beta`
- local exported bundle install for Python/Node is now part of the promoted stable subset and remains intentionally separate from the stable binary-only `install` contract
- remote bundle fetch for Python/Node is now part of the promoted stable subset and is intentionally separate from both stable local `bundle install` and binary-only `install`
- GitHub Releases bundle publish for Python/Node is now part of the promoted stable subset and is intentionally separate from both stable local `bundle install` and binary-only `install`
- the current deterministic patch candidate is `8b3bdbbf400805c410ea05bec8b0c5215dacd131`
