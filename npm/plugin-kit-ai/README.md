# `plugin-kit-ai` npm package

> **Historical v1 reference — baseline 1.2.4.** The commands and capabilities
> below describe the preserved YAML v1 workflow. Released Milestone A static
> authoring is available from npm as `universal-agent-plugins` or `plugin-kit-ai@2.0.5`;
> see the [current Build guide](../../website/source/en/build/index.md). Runtime/dev/bootstrap, migration,
> export and publication remain deferred from v2. Use `plugin-kit-ai@1.2.4`
> explicitly for the historical instructions below. npm, PyPI, Homebrew,
> GitHub Releases and public-channel E2E have been verified for Milestone A.

Official `public-beta` npm wrapper for the `plugin-kit-ai` CLI itself.

Install paths:

```bash
npm install --global plugin-kit-ai@1.2.4
plugin-kit-ai version
```

```bash
npx plugin-kit-ai@1.2.4 version
```

This package downloads the matching published GitHub Release binary from `777genius/plugin-kit-ai`, verifies `checksums.txt`, and then runs the installed binary. It does not build Go from source and it does not widen `plugin-kit-ai install`.
