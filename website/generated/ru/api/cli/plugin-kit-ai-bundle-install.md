---
title: "plugin-kit-ai bundle install"
description: "Устанавливает локальный экспортированный Python/Node bundle в целевой каталог."
canonicalId: "command:plugin-kit-ai:bundle:install"
surface: "cli"
section: "api"
locale: "ru"
generated: true
editLink: false
stability: "historical"
maturity: "historical"
sourceRef: "cli:plugin-kit-ai bundle install"
translationRequired: false
historicalVersion: "1.2.4"
sourceSHA: "9beca10448ac50fbe526a52101d1433a12471980"
status: "historical"
---

> Historical plugin-kit-ai v1, baseline **1.2.4** ([exact source](https://github.com/777genius/universal-agent-plugins/tree/9beca10448ac50fbe526a52101d1433a12471980)). Project migration is not available in v2 yet. Maintain legacy projects using the v1 1.2.4 command set.

<DocMetaCard surface="cli" stability="historical" maturity="historical" source-ref="cli:plugin-kit-ai bundle install" source-href="https://github.com/777genius/universal-agent-plugins/tree/9beca10448ac50fbe526a52101d1433a12471980/cli/plugin-kit-ai" />

# plugin-kit-ai bundle install

Сгенерировано из реального Cobra command tree.

Устанавливает локальный экспортированный Python/Node bundle в целевой каталог.

## plugin-kit-ai bundle install

Устанавливает локальный экспортированный Python/Node bundle в целевой каталог.

### Описание

Устанавливает локальный `.tar.gz` bundle, созданный через `plugin-kit-ai export`, в целевой каталог.

Эта стабильная handoff-поверхность поддерживает только локальные экспортированные Python/Node bundle для `codex-runtime` или `claude`.
Команда безопасно распаковывает содержимое bundle, печатает следующие шаги и не расширяет binary-only сценарий установки `plugin-kit-ai install`.

```
plugin-kit-ai bundle install &lt;bundle.tar.gz&gt; [flags]
```

### Опции

```
      --dest string   целевой каталог для распакованного содержимого bundle
  -f, --force         перезаписывает существующий целевой каталог
  -h, --help          справка по install
```

### См. также

* plugin-kit-ai bundle	 - Инструменты bundle-экспорта для переносимых архивов интерпретируемого runtime.
