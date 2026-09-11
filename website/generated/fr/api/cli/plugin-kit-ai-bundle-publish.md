---
title: "plugin-kit-ai bundle publish"
description: "Publish an exported Python/Node bundle to GitHub Releases"
canonicalId: "command:plugin-kit-ai:bundle:publish"
surface: "cli"
section: "api"
locale: "fr"
generated: true
editLink: false
stability: "historical"
maturity: "historical"
sourceRef: "cli:plugin-kit-ai bundle publish"
translationRequired: false
historicalVersion: "1.2.4"
sourceSHA: "9beca10448ac50fbe526a52101d1433a12471980"
status: "historical"
---

> Historical plugin-kit-ai v1, baseline **1.2.4** ([exact source](https://github.com/777genius/universal-agent-plugins/tree/9beca10448ac50fbe526a52101d1433a12471980)). Project migration is not available in v2 yet. Maintain legacy projects using the v1 1.2.4 command set.

<DocMetaCard surface="cli" stability="historical" maturity="historical" source-ref="cli:plugin-kit-ai bundle publish" source-href="https://github.com/777genius/universal-agent-plugins/tree/9beca10448ac50fbe526a52101d1433a12471980/cli/plugin-kit-ai" />

# plugin-kit-ai bundle publish

Généré à partir de l'arbre réel de commandes Cobra.

Publish an exported Python/Node bundle to GitHub Releases

## plugin-kit-ai bundle publish

Publish an exported Python/Node bundle to GitHub Releases

### Synopsis

Publish an exported Python/Node bundle to GitHub Releases.

This stable producer-side handoff surface exports a bundle, creates a published release by default,
uses --draft to keep the release as draft, uploads the bundle plus a sibling .sha256 asset,
and remains separate from the binary-only plugin-kit-ai install flow.

```
plugin-kit-ai bundle publish [path] [flags]
```

### Options

```
      --draft                 keep the target release as draft instead of published
  -f, --force                 replace existing bundle assets with the same name
      --github-token string   GitHub token (optional; default from GITHUB_TOKEN env)
  -h, --help                  help for publish
      --platform string       target platform to export and publish (codex-runtime or claude)
      --repo string           GitHub owner/repo that will receive the bundle assets
      --tag string            GitHub release tag to reuse or create
```

### SEE ALSO

* plugin-kit-ai bundle	 - Bundle tooling for exported interpreted-runtime handoff archives
