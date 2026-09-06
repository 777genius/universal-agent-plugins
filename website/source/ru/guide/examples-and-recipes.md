---
title: "Примеры и рецепты"
description: "Путеводитель по публичным example repos, starter repos, локальным runtime references и skill examples в plugin-kit-ai."
canonicalId: "page:guide:examples-and-recipes"
section: "guide"
locale: "ru"
generated: false
translationRequired: true
---

[Использовать плагины](/ru/use/) · [Создавать плагины](/ru/build/) — Подготовка, не релиз. Миграция проектов в v2 пока недоступна.
<details><summary>Исторические материалы plugin-kit-ai v1 · 1.2.4; старые инструкции не являются текущим Build.</summary>


# Примеры и рецепты

Используйте эту страницу, когда хотите увидеть, как `plugin-kit-ai` выглядит в реальных репозиториях, а не только в абстрактных объяснениях.

## 1. Продакшен-примеры плагинов

Это самые наглядные примеры законченных публичных форм:

- [`codex-basic-prod`](https://github.com/777genius/plugin-kit-ai/tree/main/examples/plugins/codex-basic-prod): Go плюс продакшен-репозиторий для `codex-runtime`
- [`claude-basic-prod`](https://github.com/777genius/plugin-kit-ai/tree/main/examples/plugins/claude-basic-prod): Go плюс продакшен-репозиторий для `claude`
- [`codex-package-prod`](https://github.com/777genius/plugin-kit-ai/tree/main/examples/plugins/codex-package-prod): target `codex-package`
- [`gemini-extension-package`](https://github.com/777genius/plugin-kit-ai/tree/main/examples/plugins/gemini-extension-package): packaging target `gemini`
- [`cursor-basic`](https://github.com/777genius/plugin-kit-ai/tree/main/examples/plugins/cursor-basic): workspace-config target `cursor`
- [`opencode-basic`](https://github.com/777genius/plugin-kit-ai/tree/main/examples/plugins/opencode-basic): workspace-config target `opencode`

Читайте их, когда нужен:

- конкретную структуру репозитория
- реальный пример generated outputs
- честный публичный пример того, как выглядит здоровый проект

Важно: эти примеры показывают отдельные публичные формы продукта, а не требуют делить реальную систему на отдельный репозиторий под каждый target.

## 2. Стартовые репозитории

Используйте стартовые репозитории, когда хотите начинать не с пустой директории, а с проверенного baseline.

Они лучше всего подходят для:

- первого старта
- онбординга команды
- выбора между Go, Python, Node, Claude и Codex

Самые прямые code-first starter-ссылки:

- [`plugin-kit-ai-starter-codex-go`](https://github.com/777genius/plugin-kit-ai-starter-codex-go)
- [`plugin-kit-ai-starter-codex-python`](https://github.com/777genius/plugin-kit-ai-starter-codex-python)
- [`plugin-kit-ai-starter-codex-node-typescript`](https://github.com/777genius/plugin-kit-ai-starter-codex-node-typescript)
- [`plugin-kit-ai-starter-claude-go`](https://github.com/777genius/plugin-kit-ai-starter-claude-go)
- [`plugin-kit-ai-starter-claude-python`](https://github.com/777genius/plugin-kit-ai-starter-claude-python)
- [`plugin-kit-ai-starter-claude-node-typescript`](https://github.com/777genius/plugin-kit-ai-starter-claude-node-typescript)

Если вы ещё выбираете стартовую точку, свяжите это с [Выбором стартового репозитория](/ru/guide/choose-a-starter).

## 3. Локальные runtime references

Каталог `examples/local` показывает локальные Python и Node runtime references.

Он полезен, когда:

- нужно глубже понять историю interpreted runtime
- вы хотите сравнить JavaScript, TypeScript и Python local-runtime setups
- нужен конкретный reference сверх стартовых репозиториев

Начните с:

- [`codex-node-typescript-local`](https://github.com/777genius/plugin-kit-ai/tree/main/examples/local/codex-node-typescript-local)
- [`codex-python-local`](https://github.com/777genius/plugin-kit-ai/tree/main/examples/local/codex-python-local)

## 4. Skill examples

Каталог `examples/skills` показывает примеры skills и вспомогательных интеграций.

Это не главная точка входа для большинства авторов плагинов, но раздел полезен, когда:

- вы хотите встроить docs, review или formatting helpers в более широкий workflow
- нужно понять, как соседние skills могут жить рядом с plugin repos

## Что читать по цели

- Нужен самый сильный runtime example: начните с production example для Codex или Claude, потом прочитайте [Плагин для команды](/ru/guide/team-ready-plugin).
- Нужен code-first пример по языку и target: начните с Go, Python или Node starter repo выше, потом откройте [Соберите собственную логику плагина](/ru/guide/build-custom-plugin-logic).
- Нужны packaging или workspace-config examples: начните с примеров для Codex package, Gemini, Cursor или OpenCode, потом прочитайте [Package и workspace targets](/ru/guide/package-and-workspace-targets).
- Нужна чистая стартовая точка, а не finished example: идите в [Стартовые шаблоны](/ru/guide/starter-templates).
- Сначала нужно выбрать сам target: прочитайте [Выбор target](/ru/guide/choose-a-target).
- Сначала нужен общий обзор продукта: прочитайте [Что можно построить](/ru/guide/what-you-can-build).

## Главное правило

Примеры должны прояснять публичный контракт, а не заменять его.

Используйте example repos, чтобы увидеть форму и корректные outputs. Для multi-target mental model переходите к [Одному проекту, нескольким target'ам](/ru/guide/one-project-multiple-targets).

</details>
