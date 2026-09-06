---
title: "Соберите плагин для Claude"
description: "Сфокусированный guide для стабильного пути Claude в plugin-kit-ai."
canonicalId: "page:guide:claude-plugin"
section: "guide"
locale: "ru"
generated: false
translationRequired: true
---

[Использовать плагины](/ru/use/) · [Создавать плагины](/ru/build/) — Подготовка, не релиз. Миграция проектов в v2 пока недоступна.
<p class="locale-historical-identity">Соберите плагин для Claude</p>

[Исторический снимок ветки plugin-kit-ai v1. 1.2.4 — базовая версия команд, а не точная версия этого снимка.](/en/legacy/v1/)

<details><summary>Исторический снимок ветки plugin-kit-ai v1. 1.2.4 — базовая версия команд, а не точная версия этого снимка.</summary>


# Соберите плагин для Claude

Выбирайте этот путь, когда вам нужны именно Claude hooks, а не путь Codex по умолчанию.

## Рекомендуемый старт

```bash
plugin-kit-ai init my-claude-plugin --platform claude
cd my-claude-plugin
plugin-kit-ai generate .
plugin-kit-ai validate . --platform claude --strict
```

## Что означает этот путь

- проект ориентирован на выполнение Claude hooks
- стабильное подмножество уже, чем полный набор возможностей Claude runtime
- `validate --strict` остаётся главной проверкой готовности

## Осторожно с extended hooks

```bash
plugin-kit-ai init my-claude-plugin --platform claude --claude-extended-hooks
```

Выбирайте extended hooks только если осознанно хотите более широкий набор поддерживаемых возможностей и принимаете менее жёсткую модель стабильности, чем у стабильного подмножества.

## Когда это хороший выбор

- нужен плагин для Claude hooks
- команда хочет управляемую модель проекта вместо ручной правки native Claude artifacts
- нужна более сильная структура, чем у ad-hoc local scripts

## Что дальше

- Прочитайте [Модель target’ов](/ru/concepts/target-model), чтобы понять отличие Claude от packaging и workspace-config target’ов.
- Откройте [Platform Events](/ru/api/platform-events/claude) для event-level reference.

</details>
