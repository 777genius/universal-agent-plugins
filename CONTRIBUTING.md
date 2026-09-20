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

## Lint Gate and Size Limits

The repository is linted by [golangci-lint](https://golangci-lint.run) **v2.13.2**
or newer. A single `.golangci.yml` at the root covers the modules that hold
Go code in the install core: `.`, `cli`, `install/integrationctl`,
and `install/integrationctl/agentplugins`.

```bash
make lint                       # both passes, compared against origin/main
make lint LINT_BASE=origin/some-branch
make lint-fix                   # gofmt + goimports only
make test-core                  # agentplugins core and CLI tests, fast preflight
```

`make lint` runs golangci-lint twice, on purpose:

| Pass | Scope | What it catches |
|------|-------|-----------------|
| size and architecture gate | whole files, always | file length, function length, complexity, duplication, forbidden imports |
| changed lines | `--new-from-merge-base` | correctness and style on the lines you actually touched |

One pass is not enough: `--new-from-*` filters by changed line, but `funlen`,
`gocyclo` and `file-length-limit` report on the declaration line, so adding forty
lines to the middle of an old function would slip through a changed-lines-only
run.

Limits for new code:

| Limit | Value | Linter |
|-------|-------|--------|
| lines of code per file | 500 (comments and blanks not counted) | `revive` `file-length-limit` |
| lines per function | 60 | `funlen` |
| statements per function | 40 | `funlen` |
| cyclomatic complexity | 20 | `gocyclo` |
| cognitive complexity | 25 | `gocognit` |
| duplicated tokens | 120 | `dupl` |

Test files are exempt from the size and complexity limits.

### The legacy baseline

Files that already exceeded these limits when the gate landed are listed between
the `BEGIN LEGACY SIZE BASELINE` and `END LEGACY SIZE BASELINE` markers in
`.golangci.yml`. Each entry names only the linters and message shapes that file
actually trips, so a file exempt from `gocognit` is still checked by `funlen`,
`gocyclo`, `dupl` and `revive`.

The block covers three operating systems. golangci-lint only analyses files whose
build tags match the target, so regenerating it means running the size gate under
`GOOS=linux` and `GOOS=windows` as well as natively - otherwise every `*_linux.go`
and `*_windows.go` file drops out of the list without a word.

`scripts/check-lint-baseline.sh` keeps every lint exclusion **shrink-only**: an
entry may be removed or narrowed, never added or widened, and that applies to
exclusions outside the markers too. A file leaves the block when it is split or
simplified; it is never added to buy silence for new code.

Moving an exempt file is the one case the rule cannot tell apart from buying
silence for new code: the old path disappears and an unknown one appears. Declare
the move in `scripts/lint-baseline-renames.txt` as
`<old pattern><TAB><new pattern>`, copying both patterns from the `- path:` keys
and nothing else - no surrounding quotes. The check strips the `^`/`$` anchors
and the `\.` escapes to get the literal paths it compares against git, so a
pattern that is not one literal file (a `.*`, a character class) is rejected
rather than guessed at.

A declaration grants nothing on its own. The move has to be one git can confirm:
the old path was exempt in the base revision and is gone from the working tree,
the new path exists and is new, no two lines name the same file, and `git diff -M`
sees the pair as a rename. A move that also rewrites the file past the similarity
threshold is a rewrite, and a rewritten file does not keep its amnesty. The
entry itself still has to carry the same linters and the same message patterns,
or the comparison fails as it would for any widening.

Once the move has landed in the base branch the line is reported as removable and
skipped, so a stale entry never blocks the next change.

The script is a speed bump, not a proof. It reads the flat three-line entry shape
the generator emits and compares linters and message patterns as literal strings,
so restructuring the YAML or rewording a pattern will read as a change even when
the effect is identical. That is deliberate: such an edit should be looked at.

### Suppressions

`//nolint` is allowed only with an explanation (`nolintlint` enforces it):

```go
//nolint:gosec // path is validated by pathpolicy.RequireExactPath above
```

Layering rules for the `agentplugins` install core are enforced by `depguard` and
documented in [docs/ARCHITECTURE.md](./docs/ARCHITECTURE.md). Adding a production
client is three edits plus the harness: a row in `domain.ClientDefinitions`, a
package `clients/<id>`, a `New()` line in `clients/all`, then
`clients/contracttest`. Generic packages are not in that list. See
[ADR 0007](./docs/adr/0007-client-adapter-contract-and-registry.md).

## Golden Files and the Architecture Ratchet

`make test-core` also runs two guardrails that do not need the linter.

Golden files record what the install core produces today for every client. If a
change is meant to alter that output, rewrite them in the same commit and say
why in the message:

```bash
UPDATE_GOLDEN=1 go test -count=1 ./install/integrationctl/agentplugins/...
cd cli && UPDATE_GOLDEN=1 go test -count=1 ./internal/agentpluginscli/...
```

`internal/archtest` counts how often each core package names a client identity
and fails when a package grows. Regenerate the baseline only when the numbers
went down:

```bash
cd install/integrationctl/agentplugins && go run ./internal/archtest -update
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
