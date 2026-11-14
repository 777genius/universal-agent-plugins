# format-changed

> Claude generate for `skills/format-changed/SKILL.md`. Edit the canonical source, then re-run `plugin-kit-ai skills generate`.

## Summary

Format changed files through an existing external formatter command.
## Command

- Runtime: `node`
- Invocation: `npx prettier@3.4.2 --write .`
## Compatibility
- Requires: node >=20
- Supported OS: darwin, linux
- Requires a repository checkout
- May require network access
- The first run may download the pinned package through npm.
## Allowed tools
- `bash`
- `node`

## Canonical instructions

# Format Changed Files

## What it does

Runs a repository formatter through an existing external CLI instead of custom Go code.

## When to use

Use this when the repository already standardizes on a formatter and you want a reusable skill wrapper around it.

## How to run

Run `npx prettier@3.4.2 --write .` from the repository root or adapt the command to the subset of files you want to format.

## Constraints

- This is a non-interactive command.
- It may download dependencies on the first run.
- It writes files in place, so review the diff after execution.
