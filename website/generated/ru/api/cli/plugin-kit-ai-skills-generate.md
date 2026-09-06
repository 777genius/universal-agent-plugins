---
title: "plugin-kit-ai skills generate"
description: "Generate Claude/Codex artifacts from canonical SKILL.md files"
canonicalId: "command:plugin-kit-ai:skills:generate"
surface: "cli"
section: "api"
locale: "ru"
generated: true
editLink: false
stability: "historical"
maturity: "historical"
sourceRef: "cli:plugin-kit-ai skills generate"
translationRequired: false
historicalVersion: "1.2.4"
sourceSHA: "9beca10448ac50fbe526a52101d1433a12471980"
status: "historical"
---

> Historical plugin-kit-ai v1, baseline **1.2.4** ([exact source](https://github.com/777genius/universal-agent-plugins/tree/9beca10448ac50fbe526a52101d1433a12471980)). Project migration is not available in v2 yet. Maintain legacy projects using the v1 1.2.4 command set.

<DocMetaCard surface="cli" stability="historical" maturity="historical" source-ref="cli:plugin-kit-ai skills generate" source-href="https://github.com/777genius/universal-agent-plugins/tree/9beca10448ac50fbe526a52101d1433a12471980/cli/plugin-kit-ai" />

# plugin-kit-ai skills generate

Сгенерировано из реального Cobra command tree.

Generate Claude/Codex artifacts from canonical SKILL.md files

## plugin-kit-ai skills generate

Generate Claude/Codex artifacts from canonical SKILL.md files

```
plugin-kit-ai skills generate [path] [flags]
```

### Examples

```
  plugin-kit-ai skills generate . --target all
  plugin-kit-ai skills generate ./examples/skills/cli-wrapper-formatter --target codex
```

### Опции

```
  -h, --help            справка по generate
      --target string   generate target ("all", "claude", "codex") (default "all")
```

### См. также

* plugin-kit-ai skills	 - Экспериментальные инструменты для авторинга skills.
