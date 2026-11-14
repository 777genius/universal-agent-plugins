# lint-repo

> Claude generate for `skills/lint-repo/SKILL.md`. Edit the canonical source, then re-run `plugin-kit-ai skills generate`.

## Summary

Check that this skill package is internally consistent and report actionable failures.
## Command

- Runtime: `go`
- Invocation: `go run ./cmd/lint-repo`
## Compatibility
- Requires: go >=1.22
- Supported OS: darwin, linux
- Requires a repository checkout
- Run from the example root so the command can inspect the authored and generated skill files together.
## Allowed tools
- `bash`
- `go`

## Canonical instructions

# Lint Repository

## What it does

Checks that the example skill package keeps its canonical `SKILL.md`, generated artifacts, and command doc in sync.

## When to use

Use this when you want a small but real Go-backed skill example instead of a placeholder command stub.

## How to run

Run `go run ./cmd/lint-repo` from the example root.

## Constraints

- This is a non-interactive command.
- It assumes the current directory is the example checkout.
- Diagnostics go to stdout and stderr; fix the reported issues before rerunning.
