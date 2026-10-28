# `plugin-kit-ai` npm package

> **Historical YAML CLI reference — baseline 1.2.4.** The instructions below
> preserve the retired `plugin-kit-ai` workflow. For the current public CLI,
> install `universal-agent-plugins` and read the [Use guide](../../website/source/en/use/index.md).
> The [Build guide](../../website/source/en/build/index.md) explains standard
> `plugin.json` authoring and identifies commands that need a qualified/source
> build. Existing `plugin-kit-ai` artifacts remain immutable history; this
> page does not qualify their old commands for the current installer.

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
