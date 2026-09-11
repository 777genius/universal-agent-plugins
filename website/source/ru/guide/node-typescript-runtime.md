---
title: "Соберите Node/TypeScript runtime-плагин"
description: "Основной поддерживаемый путь без Go для локальных runtime-плагинов."
canonicalId: "page:guide:node-typescript-runtime"
section: "guide"
locale: "ru"
generated: false
translationRequired: true
---

[Использовать плагины](/ru/use/) · [Создавать плагины](/ru/build/) — Подготовка, не релиз. Миграция проектов в v2 пока недоступна.
<p class="locale-historical-identity">Соберите Node/TypeScript runtime-плагин</p>

[Исторический снимок ветки plugin-kit-ai v1. 1.2.4 — базовая версия команд, а не точная версия этого снимка.](/en/legacy/v1/)

<details><summary>Исторический снимок ветки plugin-kit-ai v1. 1.2.4 — базовая версия команд, а не точная версия этого снимка.</summary>


# Соберите Node/TypeScript runtime-плагин

Это основной поддерживаемый путь без Go, когда команде нужен TypeScript, но при этом нужен поддерживаемый локальный runtime-плагин.

## Рекомендуемый сценарий

```bash
plugin-kit-ai init my-plugin --platform codex-runtime --runtime node --typescript
plugin-kit-ai doctor ./my-plugin
plugin-kit-ai bootstrap ./my-plugin
plugin-kit-ai generate ./my-plugin
plugin-kit-ai validate ./my-plugin --platform codex-runtime --strict
```

## Что важно помнить

- это стабильный локальный runtime-путь, а не zero-runtime-dependency Go path
- на машине исполнения всё равно нужен Node.js `20+`
- `doctor` и `bootstrap` здесь важнее, чем в пути Go по умолчанию

## Когда это правильный выбор

- команда уже живёт в TypeScript
- плагин по своей модели остаётся локальным для репозитория
- нужен основной поддерживаемый путь без Go без ухода в beta escape hatch

## Когда Go всё ещё лучше

Предпочитайте Go, когда:

- нужен самый сильный production contract
- важно, чтобы downstream users не ставили Node
- нужен минимум проблем с bootstrap в CI и на других машинах

См. [Выбор runtime](/ru/concepts/choosing-runtime) и [Node Runtime API](/ru/api/runtime-node/) для следующего уровня деталей.

</details>
