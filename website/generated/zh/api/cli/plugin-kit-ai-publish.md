---
title: "plugin-kit-ai publish"
description: "Publish a package target through a bounded channel workflow"
canonicalId: "command:plugin-kit-ai:publish"
surface: "cli"
section: "api"
locale: "zh"
generated: true
editLink: false
stability: "historical"
maturity: "historical"
sourceRef: "cli:plugin-kit-ai publish"
translationRequired: false
historicalVersion: "1.2.4"
sourceSHA: "9beca10448ac50fbe526a52101d1433a12471980"
status: "historical"
---

> Historical plugin-kit-ai v1, baseline **1.2.4** ([exact source](https://github.com/777genius/universal-agent-plugins/tree/9beca10448ac50fbe526a52101d1433a12471980)). Project migration is not available in v2 yet. Maintain legacy projects using the v1 1.2.4 command set.

<DocMetaCard surface="cli" stability="historical" maturity="historical" source-ref="cli:plugin-kit-ai publish" source-href="https://github.com/777genius/universal-agent-plugins/tree/9beca10448ac50fbe526a52101d1433a12471980/cli/plugin-kit-ai" />

# plugin-kit-ai publish

由实际的 Cobra 命令树生成。

Publish a package target through a bounded channel workflow

## plugin-kit-ai publish

Publish a package target through a bounded channel workflow

### Synopsis

Publish a package target through a bounded channel-family workflow.

This first-class publish entrypoint is intentionally bounded to documented channel flows:
- codex-marketplace
- claude-marketplace
- gemini-gallery (dry-run plan only)
- all authored channels (dry-run plan only)

Codex and Claude materialize a safe local marketplace root.
		Gemini stays repository/release rooted, so publish only supports --dry-run planning there instead of a local marketplace materialization path.

```
plugin-kit-ai publish [path] [flags]
```

### Options

```
      --all                   plan across all authored publication channels (dry-run only)
      --channel string        publish channel ("codex-marketplace", "claude-marketplace", or "gemini-gallery")
      --dest string           destination marketplace root directory for local Codex/Claude marketplace flows
      --dry-run               preview the materialized publish result without writing changes
      --format string         output format ("text" or "json") (default "text")
  -h, --help                  help for publish
      --package-root string   relative package root inside the destination marketplace root (default: plugins/&lt;name&gt;)
```

### SEE ALSO

* plugin-kit-ai	 - plugin-kit-ai CLI - scaffold and tooling for AI plugins
