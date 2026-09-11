---
title: "plugin-kit-ai import"
description: "Импортирует текущие нативные артефакты в package-standard структуру."
canonicalId: "command:plugin-kit-ai:import"
surface: "cli"
section: "api"
locale: "ru"
generated: true
editLink: false
stability: "historical"
maturity: "historical"
sourceRef: "cli:plugin-kit-ai import"
translationRequired: false
historicalVersion: "1.2.4"
sourceSHA: "9beca10448ac50fbe526a52101d1433a12471980"
status: "historical"
---

> Historical plugin-kit-ai v1, baseline **1.2.4** ([exact source](https://github.com/777genius/universal-agent-plugins/tree/9beca10448ac50fbe526a52101d1433a12471980)). Project migration is not available in v2 yet. Maintain legacy projects using the v1 1.2.4 command set.

<DocMetaCard surface="cli" stability="historical" maturity="historical" source-ref="cli:plugin-kit-ai import" source-href="https://github.com/777genius/universal-agent-plugins/tree/9beca10448ac50fbe526a52101d1433a12471980/cli/plugin-kit-ai" />

# plugin-kit-ai import

Сгенерировано из реального Cobra command tree.

Импортирует текущие нативные артефакты в package-standard структуру.

## plugin-kit-ai import

Импортирует текущие нативные артефакты в package-standard структуру.

### Описание

Import an existing native plugin into the package standard layout.

Claude import maps native plugin artifacts into the package-standard layout under plugin/.
Codex import materializes either the official package lane or the local runtime lane from current native artifacts. Use codex-package or codex-runtime explicitly for the lane you want to preserve.
Gemini import backfills the extension package layout and may preserve an optional launcher-based Go runtime lane when that authored project already uses one. That runtime lane now exposes a production-ready 9-hook surface, but it still does not imply blanket Gemini runtime parity for future hooks beyond the promoted contract.
OpenCode import is workspace-config-only in the current contract: it normalizes project-native JSON/JSONC config, commands, agents, themes, local plugin code, plugin-local package metadata, compatible skill roots, and optional user-scope OpenCode sources into the canonical package-standard layout.
Cursor import defaults to the packaged plugin lane through .cursor-plugin/plugin.json, root skills/, and optional .mcp.json. Use --from cursor-workspace when you intentionally want the repo-local .cursor workspace subset instead.

Use --source to import from a remote or external source reference such as github:owner/repo@ref//subdir into the destination path.

```
plugin-kit-ai import [path] [flags]
```

### Опции

```
  -f, --force                overwrite plugin/plugin.yaml if it already exists
      --from string          source platform ("claude", "codex-package", "codex-runtime", "gemini", "opencode", "cursor", or "cursor-workspace"; omit to auto-detect current native layouts)
  -h, --help                 справка по import
      --include-user-scope   include explicit user-scope native sources when supported by the import target
      --source string        native source reference to import from (local path, github:owner/repo@ref//subdir, or git URL with optional #ref)
```

### См. также

* plugin-kit-ai	 - CLI plugin-kit-ai для создания проектов и служебных операций вокруг AI-плагинов.
