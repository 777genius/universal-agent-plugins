---
title: "Что именно вы собираете"
description: "Выберите правильный стартовый путь в plugin-kit-ai до того, как уйдёте в target taxonomy."
canonicalId: "page:guide:choose-what-you-are-building"
section: "guide"
locale: "ru"
generated: false
translationRequired: true
---

[Использовать плагины](/ru/use/) · [Создавать плагины](/ru/build/) — Подготовка, не релиз. Миграция проектов в v2 пока недоступна.
<p class="locale-historical-identity">Что именно вы собираете</p>

[Исторический снимок ветки plugin-kit-ai v1. 1.2.4 — базовая версия команд, а не точная версия этого снимка.](/en/legacy/v1/)

<details><summary>Исторический снимок ветки plugin-kit-ai v1. 1.2.4 — базовая версия команд, а не точная версия этого снимка.</summary>


# Что именно вы собираете

Начинайте с задачи. Вам не нужно понимать target IDs, runtime lanes или детали local MCP transport до того, как вы создадите repo.

## Подключить онлайн-сервис

Используйте это, когда плагин должен подключаться к hosted-сервису вроде Notion, Stripe, Cloudflare или Vercel.

```bash
plugin-kit-ai init my-plugin --template online-service
```

Это создаёт:

- один редактируемый source под `plugin/`
- общий hosted-service wiring под `plugin/mcp/servers.yaml`
- generated app-specific output files для поддерживаемых package и workspace targets
- без runtime-кода и launcher-контракта по умолчанию

## Подключить локальный инструмент

Используйте это, когда плагин должен обращаться к repo-owned CLI, container или локальному executable tool вроде Docker Hub, Chrome DevTools или HubSpot Developer.

```bash
plugin-kit-ai init my-plugin --template local-tool
```

Это создаёт:

- один редактируемый source под `plugin/`
- wiring для локальной команды, контейнера или инструмента под `plugin/mcp/servers.yaml`
- generated app-specific output files для поддерживаемых package и workspace targets
- без runtime-кода и launcher-контракта по умолчанию

## Сделать свой plugin с логикой - Advanced

Используйте это, когда ценность плагина живёт в вашем коде, hooks, runtime behavior или orchestration logic.

```bash
plugin-kit-ai init my-plugin --template custom-logic
```

Этот путь даёт больше контроля и больше ответственности, чем первые два starter'а:

- вы редактируете runtime-facing files под `plugin/`
- вы сохраняете один repo, даже когда generated target outputs появляются в корне
- вы сами владеете runtime entrypoint, test flow и поведением, которые определяют плагин

Откройте [Соберите собственную логику плагина](/ru/guide/build-custom-plugin-logic), если вам нужен отдельный продвинутый гайд для этого пути.

## Что делать дальше

После любого из этих стартов:

```bash
cd my-plugin
plugin-kit-ai inspect . --authoring
plugin-kit-ai generate .
plugin-kit-ai generate --check .
```

Потом валидируйте тот supported output, который реально собираетесь поставлять первым.

## Когда открывать advanced pages

- Открывайте [Быстрый старт](/ru/guide/quickstart), когда нужен самый короткий first-run flow.
- Открывайте [Соберите собственную логику плагина](/ru/guide/build-custom-plugin-logic), когда вы осознанно выбираете продвинутый runtime path.
- Открывайте [Выбор target](/ru/guide/choose-a-target), когда нужны конкретные решения по способу поставки.
- Открывайте [Что можно собрать](/ru/guide/what-you-can-build), когда нужна полная product map.

</details>
