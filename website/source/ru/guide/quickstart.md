---
title: "Быстрый старт"
description: "Установите доступные плагины или подготовьте переносимый пакет Agent Plugins 1.0."
canonicalId: "page:guide:quickstart"
section: "guide"
locale: "ru"
generated: false
translationRequired: true
---

# Используйте плагины / Создавайте плагины

Установите доступные плагины или подготовьте переносимый пакет Agent Plugins 1.0.

## Используйте плагины {#use-plugins}

Доступно сейчас: установка через Universal Agent Plugins. С Node.js 22+ выполните:

```bash
npx universal-agent-plugins add context7
```

Для нативной установки на macOS, Linux или Windows откройте инструкцию. CLI предложит совместимые агенты; отдельным плагинам может требоваться своя среда исполнения.

[Инструкция по установке](https://github.com/777genius/universal-agent-plugins#quick-start)

Версии проверены 2026-09-07: universal-agent-plugins 0.1.51 (npm), plugin-kit-ai 1.2.4 (npm/PyPI), стабильный релиз GitHub agentplugins-v0.1.51.

Совместимость зависит от пакета. Проверка схемы не доказывает работу, OAuth или активацию. Codex не поддерживает заявленный MCP SSE; поддержка stdio и Streamable HTTP сохраняется в существующих адаптерах.

[Совместимость клиентов (на английском)](https://github.com/777genius/universal-agent-plugins#supported-clients)

## Создавайте плагины {#build-plugins}

Создавайте переносимый пакет Agent Plugins 1.0 на основе plugin.json с необязательными skills/ и mcp.json. Поддержка клиентов зависит от пакета и его компонентов.

**Подготовка — ещё не выпущено**

CLI для разработки с приоритетом стандарта находится в подготовке и ещё не выпущен. Опубликованный plugin-kit-ai 1.2.4 в npm и PyPI — исторический инструмент v1, а не standard-first v2. Установка plugin-kit-ai@latest не предоставляет будущий процесс разработки.

[Спецификация Agent Plugins 1.0](https://agent-plugins.org/specification)

## Сопровождение исторических проектов v1 {#historical-v1}

Используйте инструкции v1 ниже для существующих проектов plugin.yaml. Эти шаблоны и генерируемые результаты относятся к историческому процессу v1, а не к новой разработке с приоритетом стандарта.

```bash
brew install 777genius/homebrew-plugin-kit-ai/plugin-kit-ai
plugin-kit-ai version
plugin-kit-ai init my-plugin
cd my-plugin
go mod tidy
plugin-kit-ai generate .
plugin-kit-ai validate . --platform codex-runtime --strict
```

```bash
plugin-kit-ai init my-plugin --template online-service
plugin-kit-ai init my-plugin --template local-tool
plugin-kit-ai init my-plugin --template custom-logic
```

### Что вы получите

- один plugin repo с первого дня
- authored files под `plugin/`
- generated output для Codex runtime из того же repo
- понятную проверку готовности через `validate --strict`

### Поддерживаемые пути для Node и Python

Если команда уже живёт в Node/TypeScript или Python, эти пути поддерживаются и видны с самого начала:

- `codex-runtime --runtime node --typescript`
- `codex-runtime --runtime python`
- оба варианта являются локальными interpreted runtime paths, поэтому на машине исполнения всё равно нужен Node.js `20+` или Python `3.10+`
- Go всё равно остаётся путём по умолчанию, когда нужен самый сильный общий сценарий для продакшна

### Если вы осознанно начинаете с Node или Python

Используйте этот альтернативный flow только тогда, когда выбор языка уже является частью продуктового требования:

```bash
plugin-kit-ai init my-plugin --platform codex-runtime --runtime node --typescript
plugin-kit-ai doctor ./my-plugin
plugin-kit-ai bootstrap ./my-plugin
plugin-kit-ai generate ./my-plugin
plugin-kit-ai validate ./my-plugin --platform codex-runtime --strict
```

Или стартуйте с Python:

```bash
plugin-kit-ai init my-plugin --platform codex-runtime --runtime python
plugin-kit-ai doctor ./my-plugin
plugin-kit-ai bootstrap ./my-plugin
plugin-kit-ai generate ./my-plugin
plugin-kit-ai validate ./my-plugin --platform codex-runtime --strict
```

### Что делать дальше

- правьте плагин под `plugin/`
- после изменений снова запускайте `plugin-kit-ai generate ./my-plugin`
- потом снова запускайте `plugin-kit-ai validate ./my-plugin --platform codex-runtime --strict`
- и только после этого добавляйте другие способы поставки, если продукту это действительно нужно

### Что добавлять потом

| Цель | Что добавлять позже |
| --- | --- |
| Claude hooks как реальный продукт | `claude` |
| Официальный пакет Codex | `codex-package` |
| Пакет расширения Gemini | `gemini` |
| Настройка интеграции в самом repo | `opencode` или `cursor` |

`claude` выбирайте первым только тогда, когда hooks Claude уже являются реальным требованием продукта.

### Что расширяется потом

- repo остаётся единым, когда вы добавляете новые lanes
- package и extension lanes идут из того же authored source
- OpenCode и Cursor нужны тогда, когда repo должен хранить и вести настройку интеграции
- точная support boundary живёт в reference docs, а не в вашем первом стартовом flow

### Что читать дальше

- [Что именно вы собираете](/ru/guide/choose-what-you-are-building)
- [Соберите собственную логику плагина](/ru/guide/build-custom-plugin-logic)
- [Создайте первый plugin](/ru/guide/first-plugin)
- [Что можно собрать](/ru/guide/what-you-can-build)
- [Выбор target](/ru/guide/choose-a-target)
