# review-checklist

> Claude generate for `skills/review-checklist/SKILL.md`. Edit the canonical source, then re-run `plugin-kit-ai skills generate`.

## Summary

Apply a short, consistent review checklist before merging changes.
## Compatibility
- Requires a repository checkout
- Works best when the agent can inspect diffs and the touched files locally.

## Canonical instructions

# Review Checklist

## What it does

Provides a repeatable review checklist for code, docs, and generated artifact changes before handoff or merge.

## When to use

Use this when you want a quick review pass before merging or handing work off to another maintainer.

## How to run

Read the checklist, inspect the changed files, and record concrete findings or the absence of findings.

Recommended checklist:

1. Check whether authored `SKILL.md` and generated artifacts drifted.
2. Check whether validation or generate behavior changed unexpectedly.
3. Check whether docs still describe the real authored contract.
4. Check whether new failures are tied to exact files or behaviors.

## Constraints

- This skill is instructional only and has no command to execute.
- Keep findings concrete and tied to changed files or behaviors.
