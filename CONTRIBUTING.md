# Contributing

Thanks for contributing to `plugin-kit-ai`.

## Before You Start

- read [docs/SUPPORT.md](./docs/SUPPORT.md) to understand stable vs beta boundaries
- read [CODE_OF_CONDUCT.md](./CODE_OF_CONDUCT.md)
- keep user-facing claims aligned with generated contracts and release docs

## Local Setup

```bash
go test ./...
make vet
make generated-check
make version-sync-check
```

If your change touches install flows, runtime contracts, launcher behavior, or
generated artifacts, also run:

```bash
make test-install-compat
make test-polyglot-smoke
```

If your change touches docs or the docs toolchain:

```bash
cd website
pnpm install --frozen-lockfile
pnpm run docs:check
```

## Pull Requests

- keep PRs scoped to one change family
- explain contract impact, risk, and verification
- add or update tests with the behavior change
- update docs when stable or beta claims move
- use Conventional Commits for commit messages

## Contribution License

Unless you explicitly state otherwise, any contribution intentionally submitted
for inclusion in this project is provided under the Apache License 2.0, as
described in section 5 of the license.

## Release-Sensitive Changes

Treat these as release-sensitive and verify them explicitly:

- generated config or manifest shape
- runtime decode or encode behavior
- install, bundle, bootstrap, or registry flows
- support matrix, support policy, or release docs
- npm, PyPI, Homebrew, or release asset workflows

The canonical maintainer playbook lives in [docs/RELEASE.md](./docs/RELEASE.md)
and [docs/RELEASE_CHECKLIST.md](./docs/RELEASE_CHECKLIST.md).
