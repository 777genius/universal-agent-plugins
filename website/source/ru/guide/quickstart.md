---
title: "Быстрый старт"
description: "Установите доступные плагины или подготовьте переносимый пакет Agent Plugins 1.0."
canonicalId: "page:guide:quickstart"
section: "guide"
locale: "ru"
generated: false
translationRequired: true
---

<a id="быстрыи-старт"></a>

# Используйте плагины / Создавайте плагины

Установите доступные плагины или подготовьте переносимый пакет Agent Plugins 1.0.

## Используйте плагины {#use-plugins}

<a id="опциональная-быстрая-проверка"></a>

Доступно сейчас: установка через Universal Agent Plugins. С Node.js 22+ выполните:

```bash
npx universal-agent-plugins add context7
```

Для нативной установки на macOS, Linux или Windows откройте инструкцию. CLI предложит совместимые агенты; отдельным плагинам может требоваться своя среда исполнения.

[Инструкция по установке](https://github.com/777genius/universal-agent-plugins#quick-start)

[Проверенные версии клиентов и ограничения платформ](/ru/reference/client-compatibility)

Авторинг Agent Plugins доступен в `universal-agent-plugins@0.1.65`; GitHub-релиз: `agentplugins-v0.1.65`.

Совместимость зависит от пакета. Проверка схемы не доказывает работу, OAuth или активацию. Codex не поддерживает заявленный MCP SSE; поддержка stdio и Streamable HTTP сохраняется в существующих адаптерах.

[Совместимость клиентов (на английском)](https://github.com/777genius/universal-agent-plugins#supported-clients)

## Создавайте плагины {#build-plugins}

<a id="если-читать-только-одно"></a>

Создавайте переносимый пакет Agent Plugins 1.0 на основе plugin.json с необязательными skills/ и mcp.json. Поддержка клиентов зависит от пакета и его компонентов.

**Авторинг Agent Plugins доступен**

Статический авторинг доступен через `agentplugins author` в `universal-agent-plugins@0.1.65`; GitHub-релиз: `agentplugins-v0.1.65`. npm, Homebrew, нативные архивы и E2E публичных каналов проверены. Этот выпуск не запускает код пакета или серверы разработки, не устанавливает зависимости, не экспортирует пакеты и не публикует их.

[Спецификация Agent Plugins 1.0](https://agent-plugins.org/specification)
