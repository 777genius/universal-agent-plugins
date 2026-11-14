---
title: "Support Boundary"
description: "The shortest practical answer to what plugin-kit-ai recommends, supports with care, and keeps experimental."
canonicalId: "page:reference:support-boundary"
section: "reference"
locale: "en"
generated: false
translationRequired: true
---

# Support Boundary

Use this page when you need the shortest honest answer about support.

It answers three team questions:

- what is safe to recommend by default
- what is supported, but should be chosen on purpose
- what is still experimental and should not quietly become team policy

## Safe Defaults

These are the safest defaults today:

- Go is the recommended default runtime path.
- `validate --strict` is the main readiness gate for local Python and Node runtime repos.
- `Codex runtime Go`, `Codex package`, `Gemini packaging`, `Gemini Go runtime`, and the Claude default stable lane are the main Recommended production lanes.
- `Python` and `Node` are supported non-Go paths and the recommended non-Go choice when the local interpreted runtime tradeoff is intentional.

## How This Maps To The Formal Contract

The public docs use three simple words first:

- `Recommended` usually maps to the strongest current `public-stable` production lanes.
- `Advanced` means a supported surface with a narrower, more specialized, or more careful contract.
- `Experimental` means opt-in churn outside the normal compatibility expectation.

When a team needs exact policy language, the formal terms win: `public-stable`, `public-beta`, and `public-experimental`.

## Recommended Today

If you need the practical answer, start here:

- Claude is recommended on the default stable hook path.
- Codex is recommended both for the `Notify` runtime path and the official `codex-package` path.
- Gemini packaging is recommended, and the promoted Gemini Go runtime is also production-ready.
- OpenCode and Cursor are repo-owned integration setup paths. They are useful, but they are not the default executable runtime start.

## Advanced Surfaces

Choose advanced surfaces only when the tradeoff is explicit and worth it.

Typical examples:

- OpenCode and Cursor when the repo should own integration config instead of shipping a runtime path
- narrower or specialized runtime expansions beyond the main recommended paths
- install wrappers when the real concern is CLI delivery, not runtime APIs or SDKs
- specialized configuration surfaces that are useful, but not the first default for most teams

## Experimental Surfaces

Treat experimental areas as opt-in and high-churn.

They can be useful for early adopters, but they should not quietly become a long-term standard for the team.

## Practical Rule

If you are choosing for a team, standardize the narrowest path whose promise you are actually willing to defend in CI, rollout, and handoff.
