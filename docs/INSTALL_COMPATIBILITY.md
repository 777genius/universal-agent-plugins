# Install Compatibility Contract

This document records the real-world-inspired compatibility boundary for `plugin-kit-ai install`.

## Scope

`plugin-kit-ai install` is a stable surface for **verified installation of third-party plugin binaries from GitHub Releases**.

It does **not** cover:

- self-update of the `plugin-kit-ai` CLI
- auto-update behavior
- arbitrary GitHub release layouts
- zip extraction support
- exported interpreted-runtime bundles; those use the stable local `plugin-kit-ai bundle install` surface, the stable remote `plugin-kit-ai bundle fetch` surface, or the stable GitHub Releases producer companion `plugin-kit-ai bundle publish`, not `plugin-kit-ai install`

For the `plugin-kit-ai` CLI itself, the recommended local install path is `brew install 777genius/homebrew-plugin-kit-ai/plugin-kit-ai`. The official JavaScript ecosystem path is `npm i -g plugin-kit-ai` or `npx plugin-kit-ai@latest ...`; this wrapper stays `public-beta`, downloads the matching published GitHub Releases binary, and verifies `checksums.txt`. When a release is published to PyPI, the Python ecosystem path is `pipx install plugin-kit-ai` or `pipx run plugin-kit-ai version`; this wrapper stays `public-beta`, downloads the matching published GitHub Releases binary, and verifies `checksums.txt`. The verified fallback bootstrap path is `scripts/install.sh`, and the official CI setup path is `777genius/universal-agent-plugins/setup-plugin-kit-ai@v1`. `scripts/install.sh` also accepts one-shot pass-through commands such as `sh -s -- add notion --dry-run`. Those surfaces install the CLI itself and stay separate from `plugin-kit-ai install`.

## Supported Release Layouts

The current stable install contract supports these release shapes:

1. GoReleaser-style tarball:
   - one matching `*_GOOS_GOARCH.tar.gz` asset for the requested target
   - `checksums.txt` present on the same release
   - the tarball contains one installable root binary

2. Raw binary release:
   - one matching `*-GOOS-GOARCH` asset, or `*.exe` on Windows
   - `checksums.txt` present on the same release

Selection precedence:

- if exactly one matching tarball exists, it wins
- raw binaries are used only when no single matching tarball is available

Known companion raw utilities are ignored during selection:

- `sound-preview-*`
- `list-devices-*`
- `list-sounds-*`

## Required Files

Stable verified install requires:

- release metadata resolvable by `--tag` or `--latest`
- `checksums.txt`
- a checksum line for the exact installed asset

Without `checksums.txt`, the install contract fails with the checksum error family by design.

## Unsupported Or Refused Layouts

These patterns are currently outside the stable contract:

- zip-only releases
- multiple matching `*_GOOS_GOARCH.tar.gz` archives
- multiple matching raw binaries for the same target
- asset names that do not encode target GOOS/GOARCH in the supported forms
- tarballs without a single installable binary in the archive root
- custom checksum filenames instead of `checksums.txt`

When these appear, `plugin-kit-ai install` is expected to fail cleanly with the documented release, checksum, filesystem, or ambiguous exit-family.

## Repo-Owned Compatibility Evidence

Compatibility evidence lives in two layers:

- local fixture matrix:
  - [repotests/testdata/install_compatibility/matrix.json](../repotests/testdata/install_compatibility/matrix.json)
  - exercised by [repotests/plugin-kit-ai_install_compatibility_test.go](../repotests/plugin-kit-ai_install_compatibility_test.go)
- live optional smoke:
  - [repotests/plugin-kit-ai_live_install_e2e_test.go](../repotests/plugin-kit-ai_live_install_e2e_test.go)

The local matrix is the default compatibility proof.
The live lane is only for release evidence and manual confidence refresh.

## Optional Live Compatibility Inputs

Current live inputs:

- built-in raw-binary smoke for `777genius/claude-notifications-go`
- optional tarball smoke via:
  - `PLUGIN_KIT_AI_E2E_TARBALL_OWNER_REPO`
  - `PLUGIN_KIT_AI_E2E_TARBALL_TAG`
  - `PLUGIN_KIT_AI_E2E_TARBALL_BINARY`
- optional unsupported-layout smoke via:
  - `PLUGIN_KIT_AI_E2E_UNSUPPORTED_OWNER_REPO`
  - `PLUGIN_KIT_AI_E2E_UNSUPPORTED_TAG`
  - `PLUGIN_KIT_AI_E2E_UNSUPPORTED_EXPECT_EXIT`
  - `PLUGIN_KIT_AI_E2E_UNSUPPORTED_SUBSTRING`

These live checks are opt-in and are not part of the default `go test ./...` path.
