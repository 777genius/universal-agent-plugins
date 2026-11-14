# plugin-kit-ai (PyPI wrapper)

> **Preserved historical package.** This package belongs to the earlier
> `plugin-kit-ai` authoring workflow. The current public CLI is
> `universal-agent-plugins` (`agentplugins`). Start with the
> [current documentation](https://universal-agent-plugins.com/docs/en/).
> The examples below describe this package's existing API; they do not add
> its authoring commands to the current installer release.

Thin Python launcher for the `plugin-kit-ai` CLI.

It downloads the matching published GitHub Releases binary, verifies
`checksums.txt`, caches the binary locally, and executes it.

Primary user path:

```bash
pipx install plugin-kit-ai==1.2.4
plugin-kit-ai version
```
