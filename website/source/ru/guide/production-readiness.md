---
title: "Готовность к продакшену"
description: "Публичный checklist для оценки готовности plugin-kit-ai проекта к CI, handoff и широкому показу."
canonicalId: "page:guide:production-readiness"
section: "guide"
locale: "ru"
generated: false
translationRequired: true
---

[Использовать плагины](/ru/use/) · [Создавать плагины](/ru/build/) — Подготовка, не релиз. Миграция проектов в v2 пока недоступна.
<p class="locale-historical-identity">Готовность к продакшену</p>

[Исторический снимок ветки plugin-kit-ai v1. 1.2.4 — базовая версия команд, а не точная версия этого снимка.](/en/legacy/v1/)

<details><summary>Исторический снимок ветки plugin-kit-ai v1. 1.2.4 — базовая версия команд, а не точная версия этого снимка.</summary>


# Готовность к продакшену

Используйте этот checklist перед тем, как называть проект production-ready, handoff-ready или готовым к широкому показу.

<MermaidDiagram
  :chart="`
flowchart LR
  path[Путь выбран осознанно] --> source[Один исходный repo]
  source --> checks[Generate и validate gates]
  checks --> boundary[Граница поддержки подтверждена]
  boundary --> handoff[Документация и handoff явные]
  handoff --> ready[Проект готов к продакшену]
`"
/>

## 1. Осознанно выберите правильный путь

- по умолчанию выбирайте Go, когда нужен самый сильный runtime lane
- выбирайте Node/TypeScript или Python, когда non-Go local-runtime tradeoff действительно нужен
- выбирайте package, extension или integration lanes только тогда, когда именно они являются реальными outputs продукта

## 2. Держите один repo честным

- исходное состояние проекта должно жить в package-standard layout
- generated target files - это outputs, а не главное место для ручного редактирования
- не патчите generated files руками, если ожидаете, что `generate` сохранит эти правки

## 3. Прогоняйте contract gates

Как минимум, repo должен чисто проходить такой flow:

```bash
plugin-kit-ai doctor .
plugin-kit-ai generate .
plugin-kit-ai validate . --platform <target> --strict
```

Для Go launcher lanes сначала соберите `bin/<name>`, чтобы launcher entrypoint уже существовал на диске:

```bash
go build -o bin/my-plugin ./cmd/my-plugin
plugin-kit-ai doctor .
plugin-kit-ai generate .
plugin-kit-ai validate . --platform codex-runtime --strict
```

Для Python и Node runtime lanes `doctor` и `bootstrap` - часть готовности.

## 4. Проверяйте точную support boundary

- убедитесь, что основной lane и каждый дополнительный lane в scope действительно входят в публичную support boundary
- используйте reference pages, когда нужны точные термины `public-stable`, `public-beta` или `public-experimental`
- смотрите generated target support matrix до того, как обещать совместимость downstream-пользователям

## 5. Не смешивайте install story и API story

- Homebrew, npm и PyPI пакеты - это способы установить CLI
- это не runtime API и не SDK surface
- публичный API живёт в generated API section и в задокументированных workflows

## 6. Документируйте handoff

Для публичного repo должны быть очевидны такие вещи:

- какой lane основной
- какие дополнительные lanes действительно поддерживаются
- какой runtime используется и меняется ли он по target'ам
- какой набор команд является canonical validation gate
- зависит ли проект от shared runtime package или от Go SDK path

## Финальное правило

Если коллега не может клонировать repo, пройти задокументированный flow, успешно выполнить `validate --strict` и понять выбранный lane без tribal knowledge, значит проект ещё не готов к продакшену.

</details>
