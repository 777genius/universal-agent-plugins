---
title: "Соберите собственную логику плагина"
description: "Продвинутый путь для плагинов, в которых ценность живёт в runtime-коде, hooks и orchestration."
canonicalId: "page:guide:build-custom-plugin-logic"
section: "guide"
locale: "ru"
generated: false
translationRequired: true
---

[Использовать плагины](/ru/use/) · [Создавать плагины](/ru/build/) — Подготовка, не релиз. Миграция проектов в v2 пока недоступна.
<p class="locale-historical-identity">Соберите собственную логику плагина</p>

[Исторический снимок ветки plugin-kit-ai v1. 1.2.4 — базовая версия команд, а не точная версия этого снимка.](/en/legacy/v1/)

<details><summary>Исторический снимок ветки plugin-kit-ai v1. 1.2.4 — базовая версия команд, а не точная версия этого снимка.</summary>


# Соберите собственную логику плагина

Выбирайте этот путь, когда плагин не просто подключает существующий сервис или локальный инструмент.

Это продвинутый путь для repo, в которых ценность живёт в:

- runtime-коде, которым владеете вы
- hooks и логике оркестрации
- policy, transformation или guardrail behavior
- custom behavior, которого не было бы без вашего кода

Если вы подключаете hosted service вроде Notion или Stripe, откройте [Что именно вы собираете](/ru/guide/choose-what-you-are-building) и начните с `online-service`.
Если вы подключаете local tool вроде Docker Hub или HubSpot Developer, начните с `local-tool`.

## Стартуйте отсюда

```bash
plugin-kit-ai init my-plugin --template custom-logic
cd my-plugin
plugin-kit-ai inspect . --authoring
go mod tidy
go build -o bin/my-plugin ./cmd/my-plugin
plugin-kit-ai validate . --platform codex-runtime --strict
plugin-kit-ai test . --platform codex-runtime --event Notify
```

Для стартового Go scaffolding один раз запустите `go mod tidy`, чтобы проект записал `go.sum` перед первым циклом validate или test, а перед первым `test` или `dev` один раз соберите `bin/my-plugin`.

## Reference repos

Используйте эти ссылки, когда нужны именно видимые code-first примеры, а не только абстрактное объяснение:

- [`codex-basic-prod`](https://github.com/777genius/plugin-kit-ai/tree/main/examples/plugins/codex-basic-prod): production reference для Go плюс `codex-runtime`
- [`claude-basic-prod`](https://github.com/777genius/plugin-kit-ai/tree/main/examples/plugins/claude-basic-prod): production reference для Go плюс `claude`
- [`plugin-kit-ai-starter-codex-go`](https://github.com/777genius/plugin-kit-ai-starter-codex-go): самый прямой Go-first starter для `codex-runtime`
- [`plugin-kit-ai-starter-codex-python`](https://github.com/777genius/plugin-kit-ai-starter-codex-python): starter для Python плюс `codex-runtime`
- [`plugin-kit-ai-starter-codex-node-typescript`](https://github.com/777genius/plugin-kit-ai-starter-codex-node-typescript): starter для Node или TypeScript плюс `codex-runtime`
- [`plugin-kit-ai-starter-claude-go`](https://github.com/777genius/plugin-kit-ai-starter-claude-go): starter для Go плюс `claude`
- [`plugin-kit-ai-starter-claude-python`](https://github.com/777genius/plugin-kit-ai-starter-claude-python): starter для Python плюс `claude`
- [`plugin-kit-ai-starter-claude-node-typescript`](https://github.com/777genius/plugin-kit-ai-starter-claude-node-typescript): starter для Node или TypeScript плюс `claude`

Если нужны packaging-only или workspace-config примеры, переходите в [Примеры и рецепты](/ru/guide/examples-and-recipes).

## Что вы редактируете

Authored source of truth живёт под `plugin/`.

Обычно важны такие файлы:

- `plugin/plugin.yaml`
- `plugin/launcher.yaml`
- `plugin/targets/...`
- ваш runtime entrypoint вроде `cmd/<name>/main.go` или `plugin/main.*`

Используйте `plugin-kit-ai inspect . --authoring`, когда нужно точно увидеть границу между editable source, managed guidance files и generated target outputs.

## Что генерируется

`plugin-kit-ai generate` по-прежнему владеет generated output files в корне repo.

Обычно это включает:

- root guidance files вроде `README.md`, `CLAUDE.md`, `AGENTS.md` и `GENERATED.md`
- native output для target'а, который вы ship'ите, например `.codex/config.toml`, `hooks/hooks.json` или `gemini-extension.json`

Редактируйте source под `plugin/`.
Root outputs воспринимайте как managed outputs.

## Почему этот путь более advanced

По сравнению с `online-service` и `local-tool` этот путь даёт:

- больше контроля над поведением
- больше ответственности за runtime contract
- больше пространства для tests, hooks и policy logic

Именно поэтому он остаётся на первом экране, но помечается как продвинутый путь.

## Первый запуск по runtime shape

### Go runtime

```bash
go mod tidy
go test ./...
go build -o bin/my-plugin ./cmd/my-plugin
plugin-kit-ai validate . --platform codex-runtime --strict
plugin-kit-ai test . --platform codex-runtime --event Notify
```

### Node или Python runtime

```bash
plugin-kit-ai doctor .
plugin-kit-ai bootstrap .
plugin-kit-ai validate . --platform codex-runtime --strict
plugin-kit-ai test . --platform codex-runtime --event Notify
```

## Куда идти глубже

- Откройте [Быстрый старт](/ru/guide/quickstart), если хотите сравнить этот путь с более простыми job-first starter'ами.
- Откройте [Создайте первый plugin](/ru/guide/first-plugin), если вам нужен узкий legacy-compatible tutorial для Codex runtime.
- Откройте [Примеры и рецепты](/ru/guide/examples-and-recipes), если хотите прямые ссылки на repo вместо только conceptual path.
- Откройте [Выбор target](/ru/guide/choose-a-target), когда понадобятся конкретные решения по способу поставки.
- Откройте [Один проект, несколько target'ов](/ru/guide/one-project-multiple-targets), когда repo будет готов расти в несколько outputs.

</details>
