---
title: "plugin-kit-ai publication doctor"
description: "Inspect publication readiness without mutating files"
canonicalId: "command:plugin-kit-ai:publication:doctor"
surface: "cli"
section: "api"
locale: "fr"
generated: true
editLink: false
stability: "historical"
maturity: "historical"
sourceRef: "cli:plugin-kit-ai publication doctor"
translationRequired: false
historicalVersion: "1.2.4"
sourceSHA: "9beca10448ac50fbe526a52101d1433a12471980"
status: "historical"
---

> Historical plugin-kit-ai v1, baseline **1.2.4** ([exact source](https://github.com/777genius/universal-agent-plugins/tree/9beca10448ac50fbe526a52101d1433a12471980)). Project migration is not available in v2 yet. Maintain legacy projects using the v1 1.2.4 command set.

<DocMetaCard surface="cli" stability="historical" maturity="historical" source-ref="cli:plugin-kit-ai publication doctor" source-href="https://github.com/777genius/universal-agent-plugins/tree/9beca10448ac50fbe526a52101d1433a12471980/cli/plugin-kit-ai" />

# plugin-kit-ai publication doctor

Généré à partir de l'arbre réel de commandes Cobra.

Inspect publication readiness without mutating files

## plugin-kit-ai publication doctor

Inspect publication readiness without mutating files

### Synopsis

Read-only publication readiness check for package-capable targets and authored publish/... channels.

```
plugin-kit-ai publication doctor [path] [flags]
```

### Options

```
      --dest string           optional materialized marketplace root to verify for local codex-package or claude publication flows
      --format string         output format: text or json (default "text")
  -h, --help                  help for doctor
      --package-root string   relative package root inside the destination marketplace root (default: plugins/&lt;name&gt;)
      --target string         publication target ("all", "claude", "codex-package", or "gemini") (default "all")
```

### SEE ALSO

* plugin-kit-ai publication	 - Show the publication-oriented package and channel view
