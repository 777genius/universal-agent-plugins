# /lint-repo

Generated command reference for `lint-repo`.

## Purpose

Check that this skill package is internally consistent and report actionable failures.

## Invocation

`go run ./cmd/lint-repo`
## Runtime

`go`
## Allowed tools
- `bash`
- `go`
## Compatibility
- Requires: go >=1.22
- Supported OS: darwin, linux
- Requires a repository checkout
- Run from the example root so the command can inspect the authored and generated skill files together.

## Notes
- Safe to retry: yes
- Writes files: no
- Produces JSON: no
- This file is generated from `skills/lint-repo/SKILL.md`.
- Regenerate with `plugin-kit-ai skills generate`.
