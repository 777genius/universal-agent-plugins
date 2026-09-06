---
title: "plugin-kit-ai publication remove"
description: "Remove a materialized local marketplace package root and catalog entry"
canonicalId: "command:plugin-kit-ai:publication:remove"
surface: "cli"
section: "api"
locale: "en"
generated: true
editLink: false
stability: "historical"
maturity: "historical"
sourceRef: "cli:plugin-kit-ai publication remove"
translationRequired: false
historicalVersion: "1.2.4"
sourceSHA: "9beca10448ac50fbe526a52101d1433a12471980"
status: "historical"
---

> Historical plugin-kit-ai v1, baseline **1.2.4** ([exact source](https://github.com/777genius/universal-agent-plugins/tree/9beca10448ac50fbe526a52101d1433a12471980)). Project migration is not available in v2 yet. Maintain legacy projects using the v1 1.2.4 command set.

<DocMetaCard surface="cli" stability="historical" maturity="historical" source-ref="cli:plugin-kit-ai publication remove" source-href="https://github.com/777genius/universal-agent-plugins/tree/9beca10448ac50fbe526a52101d1433a12471980/cli/plugin-kit-ai" />

# plugin-kit-ai publication remove

Generated from the live Cobra command tree.

Remove a materialized local marketplace package root and catalog entry

## plugin-kit-ai publication remove

Remove a materialized local marketplace package root and catalog entry

### Synopsis

Remove a single plugin from a local Codex or Claude marketplace root.

This workflow is intentionally scoped to documented local/catalog flows and is safe to rerun.
It removes the selected package root and prunes the matching plugin entry from the marketplace catalog while preserving the marketplace root itself.

```
plugin-kit-ai publication remove [path] [flags]
```

### Options

```
      --dest string           destination marketplace root directory
      --dry-run               preview the package root and catalog pruning without writing changes
  -h, --help                  help for remove
      --package-root string   relative package root inside the destination marketplace root (default: plugins/&lt;name&gt;)
      --target string         removal target ("claude" or "codex-package")
```

### SEE ALSO

* plugin-kit-ai publication	 - Show the publication-oriented package and channel view
