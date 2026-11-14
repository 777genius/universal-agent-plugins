---
title: "Version And Compatibility Policy"
description: "How to think about releases, compatibility promises, wrappers, SDKs, and support vocabulary in plugin-kit-ai."
canonicalId: "page:reference:version-and-compatibility"
section: "reference"
locale: "en"
generated: false
translationRequired: true
---

# Version And Compatibility Policy

This page is for one practical team decision: what are we standardizing, and how strong is that promise?

## Choose In 60 Seconds

- read this page when your team needs one compact policy for releases, wrappers, SDKs, runtimes, and compatibility promises
- read [Support Boundary](/en/reference/support-boundary) when you want the shortest practical support answer
- read [Releases](/en/releases/) when you want the story of a specific release

## The Public Baseline

Think about standardization in three layers:

- the release line you choose across repos
- the support level of the path you choose inside that release line
- the install or delivery mechanism around that path

These layers are related, but they are not interchangeable.

## Recommended Lanes And Formal Tiers

Use one simple translation across docs and policy:

- `Recommended` usually means a promoted `public-stable` production path
- `Advanced` means a supported surface with a narrower or more specialized contract
- `Experimental` means opt-in churn outside the normal compatibility expectation

The main recommended paths today are:

- `Codex runtime Go`
- `Codex package`
- `Gemini packaging`
- `Gemini Go runtime`
- `Claude default stable lane`
- `Python` and `Node` local runtime paths as the supported and recommended non-Go authoring choice on supported targets

## What Compatibility Really Covers Here

The strongest public promise is around:

- the declared public CLI contract
- the recommended Go SDK path and the recommended production paths listed above
- the recommended local Python and Node runtime paths on supported targets
- the documented behavior of `public-stable` generated outputs

Compatibility does not mean every wrapper, convenience path, or specialized surface moves with the same promise.

## Public Language Versus Formal Terms

Use this translation when talking to a team:

- `Recommended` usually means the path is inside the strongest current `public-stable` contract
- `Advanced` means the surface is supported, but more specialized or narrower than the first default
- `Experimental` means opt-in churn with no normal compatibility expectation

When the team needs exact policy, use the formal terms `public-stable`, `public-beta`, and `public-experimental`.

## Wrappers, SDKs, And Runtime APIs

Do not standardize these as if they were the same thing.

- Homebrew, npm, PyPI, and the verified script are install channels for the CLI
- the Go SDK is a public SDK surface
- runtime APIs are tied to their declared runtime paths

If you treat install wrappers as if they carry the same promise as an SDK or runtime path, you will standardize the wrong layer.

## What Teams Should Standardize

Healthy teams usually standardize:

- one declared release baseline
- one primary path with a clear support story
- one validation gate before handoff and rollout
- one shared interpretation of the formal compatibility terms

## Final Rule

Standardize only the release line and path whose public promise your team is actually willing to defend in CI, handoff, and rollout.
