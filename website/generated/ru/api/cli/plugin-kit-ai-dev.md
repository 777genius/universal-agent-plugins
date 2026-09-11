---
title: "plugin-kit-ai dev"
description: "Следит за проектом, повторно рендерит, валидирует, пересобирает и перезапускает фикстуры при изменениях."
canonicalId: "command:plugin-kit-ai:dev"
surface: "cli"
section: "api"
locale: "ru"
generated: true
editLink: false
stability: "historical"
maturity: "historical"
sourceRef: "cli:plugin-kit-ai dev"
translationRequired: false
historicalVersion: "1.2.4"
sourceSHA: "9beca10448ac50fbe526a52101d1433a12471980"
status: "historical"
---

> Historical plugin-kit-ai v1, baseline **1.2.4** ([exact source](https://github.com/777genius/universal-agent-plugins/tree/9beca10448ac50fbe526a52101d1433a12471980)). Project migration is not available in v2 yet. Maintain legacy projects using the v1 1.2.4 command set.

<DocMetaCard surface="cli" stability="historical" maturity="historical" source-ref="cli:plugin-kit-ai dev" source-href="https://github.com/777genius/universal-agent-plugins/tree/9beca10448ac50fbe526a52101d1433a12471980/cli/plugin-kit-ai" />

# plugin-kit-ai dev

Сгенерировано из реального Cobra command tree.

Следит за проектом, повторно рендерит, валидирует, пересобирает и перезапускает фикстуры при изменениях.

## plugin-kit-ai dev

Следит за проектом, повторно рендерит, валидирует, пересобирает и перезапускает фикстуры при изменениях.

### Описание

Watch launcher-based runtime targets in a fast inner loop.

Each cycle re-generates the selected target, performs runtime-aware rebuilds when needed,
runs strict validation, and reruns the configured stable Claude or Codex fixture smoke tests.

Gemini has a production-ready 9-hook Go runtime with dedicated runtime gates and stays outside this stable watch loop.
For Gemini use generate, generate --check, validate --strict, inspect, capabilities --mode runtime,
make test-gemini-runtime, then gemini extensions link . and optionally rerun
make test-gemini-runtime-live after changes.

```
plugin-kit-ai dev [path] [flags]
```

### Опции

```
      --all                 run every stable event for the selected platform on each cycle
      --event string        stable event to execute (for example Stop, PreToolUse, UserPromptSubmit, or Notify)
      --fixture string      fixture JSON path for single-event runs (default: fixtures/&lt;platform&gt;/&lt;event&gt;.json)
      --golden-dir string   golden output directory (default: goldens/&lt;platform&gt;)
  -h, --help                справка по dev
      --interval duration   poll interval for watch mode (default 750ms)
      --once                run a single generate/validate/test cycle and exit
      --platform string     target override ("claude" or "codex-runtime")
```

### См. также

* plugin-kit-ai	 - CLI plugin-kit-ai для создания проектов и служебных операций вокруг AI-плагинов.
