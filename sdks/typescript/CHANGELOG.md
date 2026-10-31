# Changelog

All notable changes to `universal-agent-plugins-management-sdk` are documented
here. Versions follow SemVer. While the package is 0.x, a breaking change ships
as a minor release with migration notes in this file. Each release is built
from an immutable `management-sdk-vX.Y.Z` tag by the
[Management SDK Release workflow](../../.github/workflows/management-sdk-release.yml).

## [0.1.1]

First version that the release workflow publishes through npm trusted
publishing, with npm provenance for the `management-sdk-v0.1.1` tag.

### Added

- `client.checkUpdates({ all: true })` and
  `client.checkUpdates({ aliases: [...] })` call
  `POST /api/v1/environment/check-updates`. This explicit read-only check
  returns `complete`, `changed` and per-entry source and dependency graph
  facts. It does not save freshness or change installed files; an available
  update still needs a fresh `prepare({ action: 'update', ... })` review.
- Exported types `UpdateCheckRequest`, `UpdateCheck`, `UpdateCheckEntry` and
  `UpdateIdentity`.

### Compatibility

- No runtime or wire changes for existing methods, the `/node` entry point or
  the existing management API v1 operations. The `ManagementClient` type gains
  `checkUpdates`, so code that implements that type itself (for example, a
  test double) must add it.
- `checkUpdates` needs an engine that serves the operation. CLI 0.1.95 and
  older do not and answer `404`, which the SDK reports as a `ManagementError`
  with `code` `HTTP_ERROR` and `status` 404; no capability entry announces the
  operation. Use a CLI release whose release notes list it, or build the
  engine from the `management-sdk-v0.1.1` tag (see the README). Every other
  method still works with CLI 0.1.95 or newer.

### Upgrade

Run `npm install --save-exact universal-agent-plugins-management-sdk@0.1.1`.
Code that only calls the client needs no changes.

## [0.1.0]

Initial npm package.

### Added

- Package `universal-agent-plugins-management-sdk`. The first publication uses
  a tarball built and attested by the release workflow from the
  `management-sdk-v0.1.0` tag. npm trusted
  publishing with npm provenance applies from the next version, because npm
  can configure it only for an existing package.
- Pure entry point: `createClient`, `ManagementError`, `isTerminal` and the
  generated management API v1 types. It has no filesystem, process, Electron
  or UI dependency.
- Node entry point `universal-agent-plugins-management-sdk/node`: `startUap`
  owns one `agentplugins manage serve` process; `discoverTargets` runs
  `agentplugins manage discover`.

### Compatibility

- Requires an `agentplugins` engine that provides `manage serve` and
  `manage discover`: CLI 0.1.95 or newer. CLI 0.1.94 and older do not have
  `manage`.
- Speaks management API v1 (`apiVersion: 1`). The API version is independent
  of the engine and package versions; the SDK refuses any other API version
  during startup and discovery.
- Node.js 22.18 or newer for the `/node` entry point and development scripts.

### Migration from the development tarball

Earlier builds were consumed only as integrity-pinned tarballs named
`@uap/management-sdk` with version `0.0.0-development`. The name changed because
the `@uap` npm scope is not available to this project.

1. After the version is available on npm, replace the vendored tarball dependency
   with that exact registry version:
   `npm install --save-exact universal-agent-plugins-management-sdk@0.1.0`.
2. Rename imports: `@uap/management-sdk` becomes
   `universal-agent-plugins-management-sdk`, and `@uap/management-sdk/node`
   becomes `universal-agent-plugins-management-sdk/node`.
3. Provision the released CLI 0.1.95 or newer instead of a source-built engine,
   unless you need unreleased engine changes. Keep recording the engine digest
   in your build evidence.

Compared with a development tarball built from the same source commit, the
API and wire protocol are the same; the package name, version and npm metadata
changed. A source version or tag alone does not establish npm publication.
