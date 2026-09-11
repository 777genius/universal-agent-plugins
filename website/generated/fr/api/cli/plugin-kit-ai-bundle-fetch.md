---
title: "plugin-kit-ai bundle fetch"
description: "Fetch and install a remote exported Python/Node bundle into a destination directory"
canonicalId: "command:plugin-kit-ai:bundle:fetch"
surface: "cli"
section: "api"
locale: "fr"
generated: true
editLink: false
stability: "historical"
maturity: "historical"
sourceRef: "cli:plugin-kit-ai bundle fetch"
translationRequired: false
historicalVersion: "1.2.4"
sourceSHA: "9beca10448ac50fbe526a52101d1433a12471980"
status: "historical"
---

> Historical plugin-kit-ai v1, baseline **1.2.4** ([exact source](https://github.com/777genius/universal-agent-plugins/tree/9beca10448ac50fbe526a52101d1433a12471980)). Project migration is not available in v2 yet. Maintain legacy projects using the v1 1.2.4 command set.

<DocMetaCard surface="cli" stability="historical" maturity="historical" source-ref="cli:plugin-kit-ai bundle fetch" source-href="https://github.com/777genius/universal-agent-plugins/tree/9beca10448ac50fbe526a52101d1433a12471980/cli/plugin-kit-ai" />

# plugin-kit-ai bundle fetch

Généré à partir de l'arbre réel de commandes Cobra.

Fetch and install a remote exported Python/Node bundle into a destination directory

## plugin-kit-ai bundle fetch

Fetch and install a remote exported Python/Node bundle into a destination directory

### Synopsis

Fetch a remote exported Python/Node bundle and install it into a destination directory.

Use either a direct HTTPS bundle URL with --url or a GitHub release reference as owner/repo plus --tag or --latest.
This stable remote handoff surface is intentionally separate from the binary-only plugin-kit-ai install flow.

```
plugin-kit-ai bundle fetch [owner/repo] [flags]
```

### Options

```
      --asset-name string        specific GitHub release bundle asset name to install
      --dest string              destination directory for unpacked bundle contents
  -f, --force                    overwrite an existing destination directory
      --github-api-base string   GitHub API base URL override (for tests or GitHub Enterprise)
      --github-token string      GitHub token (optional; default from GITHUB_TOKEN env)
  -h, --help                     help for fetch
      --latest                   install from the latest GitHub release instead of --tag
      --platform string          bundle platform hint for GitHub mode (codex-runtime or claude)
      --runtime string           bundle runtime hint for GitHub mode (python or node)
      --sha256 string            expected SHA256 for URL mode; overrides .sha256 sidecar lookup
      --tag string               GitHub release tag for bundle selection
      --url string               direct HTTPS URL to an exported .tar.gz bundle
```

### SEE ALSO

* plugin-kit-ai bundle	 - Bundle tooling for exported interpreted-runtime handoff archives
