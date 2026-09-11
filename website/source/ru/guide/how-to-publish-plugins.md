---
title: "Как публиковать плагины"
description: "Практический гайд по публикации plugin-kit-ai проектов в Codex, Claude и Gemini без путаницы между local apply и publication planning."
canonicalId: "page:guide:how-to-publish-plugins"
section: "guide"
locale: "ru"
generated: false
translationRequired: true
---

[Использовать плагины](/ru/use/) · [Создавать плагины](/ru/build/) — Подготовка, не релиз. Миграция проектов в v2 пока недоступна.
<p class="locale-historical-identity">Как публиковать плагины</p>

[Исторический снимок ветки plugin-kit-ai v1. 1.2.4 — базовая версия команд, а не точная версия этого снимка.](/en/legacy/v1/)

<details><summary>Исторический снимок ветки plugin-kit-ai v1. 1.2.4 — базовая версия команд, а не точная версия этого снимка.</summary>


# Как публиковать плагины

Откройте этот гайд, когда repo уже авторится через `plugin-kit-ai`, и вам нужен самый понятный следующий шаг для публикации в Codex, Claude или Gemini.

Начинайте с repo, где уже проходят `plugin-kit-ai generate .` и `plugin-kit-ai validate . --strict`, чтобы publication-команды читали актуальные managed artifacts, а не устаревшие.

## Что покрывает этот гайд

- какие платформы уже поддерживают реальный local apply
- где вместо этого используется plan-and-readiness publication
- какую команду запускать первой
- какого результата ждать после команды

## Короткое сравнение

| Платформа | Модель публикации | Реальный apply в `plugin-kit-ai` | Главная команда | Что получается |
|---|---|---:|---|---|
| Codex | локальный marketplace root | да | `publish --channel codex-marketplace` | `.agents/plugins/marketplace.json` и `plugins/<name>/...` |
| Claude | локальный marketplace root | да | `publish --channel claude-marketplace` | `.claude-plugin/marketplace.json` и `plugins/<name>/...` |
| Gemini | readiness для repository/release | нет | `publish --channel gemini-gallery --dry-run` | bounded publication plan и readiness diagnostics |

## Короткое правило

- используйте `publish`, когда нужен publication workflow
- используйте `publication`, когда сначала нужен inspect или doctor
- Codex и Claude уже поддерживают реальный local apply
- Gemini в v1 использует plan-and-readiness publication, а не local apply

Базовая модель repo остаётся простой:

- `plugin.yaml` это core plugin manifest
- `targets/...` содержит target-specific authored inputs
- `publish/...` содержит publication intent
- `publication` это inspect и doctor surface
- `publish` это publication workflow surface

## Публикация в Codex

Для Codex публикация означает materialize локального marketplace root.

Сначала запустите:

```bash
plugin-kit-ai publish ./my-plugin --channel codex-marketplace --dest ./local-codex-marketplace --dry-run
```

Когда план вас устраивает, примените его:

```bash
plugin-kit-ai publish ./my-plugin --channel codex-marketplace --dest ./local-codex-marketplace
```

Ожидаемый результат:

- `.agents/plugins/marketplace.json`
- `plugins/<name>/...`

Такой локальный root уже может работать как source плагинов для Codex.

## Публикация в Claude

Для Claude публикация тоже означает materialize локального marketplace root.

Сначала запустите:

```bash
plugin-kit-ai publish ./my-plugin --channel claude-marketplace --dest ./local-claude-marketplace --dry-run
```

Когда план вас устраивает, примените его:

```bash
plugin-kit-ai publish ./my-plugin --channel claude-marketplace --dest ./local-claude-marketplace
```

Ожидаемый результат:

- `.claude-plugin/marketplace.json`
- `plugins/<name>/...`

## Публикация в Gemini

Для Gemini публикация **не** означает сборку локального marketplace root.

В v1 `plugin-kit-ai` делает три bounded шага:

- валидирует publication intent
- проверяет readiness репозитория
- строит publication plan

Начните с readiness:

```bash
plugin-kit-ai publication doctor ./my-plugin --target gemini
```

Потом посмотрите publication plan:

```bash
plugin-kit-ai publish ./my-plugin --channel gemini-gallery --dry-run
```

Нужные prerequisites:

- публичный GitHub repository
- корректный `origin`, указывающий на GitHub
- GitHub topic `gemini-cli-extension`
- `gemini-extension.json` в правильном root

Gemini в v1 использует plan-and-readiness publication, а не local apply.

## План по всем authored channels

Используйте это, когда один repo авторит больше одного publication channel:

```bash
plugin-kit-ai publish ./my-plugin --all --dry-run --dest ./local-marketplaces --format json
```

Важные правила:

- используются только authored `publish/...` channels
- команда не выводит channels из `targets`
- это только planning-mode в v1
- `--dest` нужен только если среди authored channels есть local marketplace flow для Codex или Claude
- для Gemini-only orchestration `--dest` не нужен

Если repo авторит только `gemini-gallery`, подойдёт и такой вариант:

```bash
plugin-kit-ai publish ./my-plugin --all --dry-run --format json
```

## Какую команду запускать?

- Хочу локальный Codex marketplace root: `plugin-kit-ai publish --channel codex-marketplace --dest <marketplace-root>`
- Хочу локальный Claude marketplace root: `plugin-kit-ai publish --channel claude-marketplace --dest <marketplace-root>`
- Хочу проверить Gemini publication readiness: `plugin-kit-ai publication doctor --target gemini`
- Хочу увидеть Gemini publication plan: `plugin-kit-ai publish --channel gemini-gallery --dry-run`
- Хочу увидеть один общий publication plan: `plugin-kit-ai publish --all --dry-run`, а если среди authored channels есть Codex или Claude, добавьте `--dest <marketplace-root>`

## Что почитать дальше

- [Publication section в CLI README](https://github.com/777genius/plugin-kit-ai/tree/main/cli/plugin-kit-ai)
- [`plugin-kit-ai publish`](/ru/api/cli/plugin-kit-ai-publish)
- [`plugin-kit-ai publication`](/ru/api/cli/plugin-kit-ai-publication)
- [`plugin-kit-ai publication doctor`](/ru/api/cli/plugin-kit-ai-publication-doctor)

</details>
