# `plugin-kit-ai` npm package

> **Historical v1 reference — baseline 1.2.4.** The commands and capabilities
> below describe the preserved YAML v1 workflow. Prepared Milestone A static
> authoring targets npm `universal-agent-plugins@0.1.62` or `plugin-kit-ai@2.0.2`;
> see the [current Build guide](../../website/source/en/build/index.md). Runtime/dev/bootstrap, migration,
> export and publication remain deferred from v2. Use `plugin-kit-ai@1.2.4`
> explicitly for the historical instructions below. Candidate availability is
> unverified pending npm/PyPI/Homebrew/GitHub readbacks and public-channel E2E.

Official `public-beta` npm wrapper for the `plugin-kit-ai` CLI itself.

Install paths:

```bash
npm i -g plugin-kit-ai
plugin-kit-ai version
```

```bash
npx plugin-kit-ai@latest version
```

This package downloads the matching published GitHub Release binary from `777genius/plugin-kit-ai`, verifies `checksums.txt`, and then runs the installed binary. It does not build Go from source and it does not widen `plugin-kit-ai install`.
