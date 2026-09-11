---
title: "plugin-kit-ai capabilities"
description: "Show generated target/package or runtime support metadata"
canonicalId: "command:plugin-kit-ai:capabilities"
surface: "cli"
section: "api"
locale: "zh"
generated: true
editLink: false
stability: "historical"
maturity: "historical"
sourceRef: "cli:plugin-kit-ai capabilities"
translationRequired: false
historicalVersion: "1.2.4"
sourceSHA: "9beca10448ac50fbe526a52101d1433a12471980"
status: "historical"
---

> Historical plugin-kit-ai v1, baseline **1.2.4** ([exact source](https://github.com/777genius/universal-agent-plugins/tree/9beca10448ac50fbe526a52101d1433a12471980)). Project migration is not available in v2 yet. Maintain legacy projects using the v1 1.2.4 command set.

<DocMetaCard surface="cli" stability="historical" maturity="historical" source-ref="cli:plugin-kit-ai capabilities" source-href="https://github.com/777genius/universal-agent-plugins/tree/9beca10448ac50fbe526a52101d1433a12471980/cli/plugin-kit-ai" />

# plugin-kit-ai capabilities

由实际的 Cobra 命令树生成。

Show generated target/package or runtime support metadata

## plugin-kit-ai capabilities

Show generated target/package or runtime support metadata

### Synopsis

Shows generated contract metadata.

Default mode is target/package-oriented because plugin authors usually need to understand target class,
production boundary, import/generate/validate support, and supported component kinds.

Use --mode runtime to inspect runtime-event support for Claude, Codex, and Gemini.

```
plugin-kit-ai capabilities [flags]
```

### Options

```
      --format string     output format: table or json (default "table")
  -h, --help              help for capabilities
      --mode string       capability view: targets or runtime (default "targets")
      --platform string   limit output to a single platform
```

### SEE ALSO

* plugin-kit-ai	 - plugin-kit-ai CLI - scaffold and tooling for AI plugins
