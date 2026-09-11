---
title: "plugin-kit-ai bundle fetch"
description: "Загружает и устанавливает удалённый экспортированный Python/Node bundle в целевой каталог."
canonicalId: "command:plugin-kit-ai:bundle:fetch"
surface: "cli"
section: "api"
locale: "ru"
generated: true
editLink: false
stability: "historical"
maturity: "historical"
sourceRef: "cli:plugin-kit-ai bundle fetch"
translationRequired: false
historicalVersion: "1.2.4"
sourceSHA: "9beca10448ac50fbe526a52101d1433a12471980"
status: "historical"
---

> Historical plugin-kit-ai v1, baseline **1.2.4** ([exact source](https://github.com/777genius/universal-agent-plugins/tree/9beca10448ac50fbe526a52101d1433a12471980)). Project migration is not available in v2 yet. Maintain legacy projects using the v1 1.2.4 command set.

<DocMetaCard surface="cli" stability="historical" maturity="historical" source-ref="cli:plugin-kit-ai bundle fetch" source-href="https://github.com/777genius/universal-agent-plugins/tree/9beca10448ac50fbe526a52101d1433a12471980/cli/plugin-kit-ai" />

# plugin-kit-ai bundle fetch

Сгенерировано из реального Cobra command tree.

Загружает и устанавливает удалённый экспортированный Python/Node bundle в целевой каталог.

## plugin-kit-ai bundle fetch

Загружает и устанавливает удалённый экспортированный Python/Node bundle в целевой каталог.

### Описание

Загружает удалённый экспортированный Python/Node bundle и устанавливает его в целевой каталог.

Используйте либо прямой HTTPS URL bundle через `--url`, либо ссылку на GitHub release в формате `owner/repo` вместе с `--tag` или `--latest`.
Эта стабильная remote handoff-поверхность намеренно отделена от binary-only сценария `plugin-kit-ai install`.

```
plugin-kit-ai bundle fetch [owner/repo] [flags]
```

### Опции

```
      --asset-name string        конкретное имя bundle-asset в GitHub release для установки
      --dest string              целевой каталог для распакованного содержимого bundle
  -f, --force                    перезаписывает существующий целевой каталог
      --github-api-base string   переопределение базового URL GitHub API (для тестов или GitHub Enterprise)
      --github-token string      GitHub token (необязательно; по умолчанию берётся из `GITHUB_TOKEN`)
  -h, --help                     справка по fetch
      --latest                   устанавливает bundle из последнего GitHub release вместо `--tag`
      --platform string          подсказка по платформе bundle для GitHub-режима (`codex-runtime` или `claude`)
      --runtime string           подсказка по runtime bundle для GitHub-режима (`python` или `node`)
      --sha256 string            ожидаемый SHA256 для URL-режима; переопределяет поиск соседнего `.sha256` файла
      --tag string               GitHub release tag для выбора bundle
      --url string               прямой HTTPS URL к экспортированному `.tar.gz` bundle
```

### См. также

* plugin-kit-ai bundle	 - Инструменты bundle-экспорта для переносимых архивов интерпретируемого runtime.
