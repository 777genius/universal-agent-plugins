# Changelog

All notable changes to `universal-agent-plugins-management-sdk` are documented
here. Versions follow SemVer. While the package is 0.x, a breaking change ships
as a minor release with migration notes in this file. Each release is built
from an immutable `management-sdk-vX.Y.Z` tag by the
[Management SDK Release workflow](../../.github/workflows/management-sdk-release.yml).

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
