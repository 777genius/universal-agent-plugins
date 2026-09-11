---
title: "plugin-kit-ai publication materialize"
description: "Materialize a safe local marketplace root for Codex or Claude"
canonicalId: "command:plugin-kit-ai:publication:materialize"
surface: "cli"
section: "api"
locale: "ru"
generated: true
editLink: false
stability: "historical"
maturity: "historical"
sourceRef: "cli:plugin-kit-ai publication materialize"
translationRequired: false
historicalVersion: "1.2.4"
sourceSHA: "9beca10448ac50fbe526a52101d1433a12471980"
status: "historical"
---

> Historical plugin-kit-ai v1, baseline **1.2.4** ([exact source](https://github.com/777genius/universal-agent-plugins/tree/9beca10448ac50fbe526a52101d1433a12471980)). Project migration is not available in v2 yet. Maintain legacy projects using the v1 1.2.4 command set.

<DocMetaCard surface="cli" stability="historical" maturity="historical" source-ref="cli:plugin-kit-ai publication materialize" source-href="https://github.com/777genius/universal-agent-plugins/tree/9beca10448ac50fbe526a52101d1433a12471980/cli/plugin-kit-ai" />

# plugin-kit-ai publication materialize

Сгенерировано из реального Cobra command tree.

Materialize a safe local marketplace root for Codex or Claude

## plugin-kit-ai publication materialize

Materialize a safe local marketplace root for Codex or Claude

### Описание

Create or update a local marketplace root for a single publication-capable package target.

This workflow is intentionally limited to documented local/catalog flows:
- Codex marketplace roots with .agents/plugins/marketplace.json
- Claude marketplace roots with .claude-plugin/marketplace.json

It copies the materialized package bundle under a managed package root, then merges or creates the marketplace catalog artifact.

```
plugin-kit-ai publication materialize [path] [flags]
```

### Опции

```
      --dest string           destination marketplace root directory
      --dry-run               preview the materialized package root and catalog changes without writing them
  -h, --help                  справка по materialize
      --package-root string   relative package root inside the destination marketplace root (default: plugins/&lt;name&gt;)
      --target string         materialization target ("claude" or "codex-package")
```

### См. также

* plugin-kit-ai publication	 - Show the publication-oriented package and channel view
