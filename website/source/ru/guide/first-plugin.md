---
title: "Соберите первый плагин"
description: "Минимальный пошаговый сценарий от init до строгой проверки готовности."
canonicalId: "page:guide:first-plugin"
section: "guide"
locale: "ru"
generated: false
translationRequired: true
---

[Использовать плагины](/ru/use/) · [Создавать плагины](/ru/build/) — Подготовка, не релиз. Миграция проектов в v2 пока недоступна.
<details><summary>Исторические материалы plugin-kit-ai v1 · 1.2.4; старые инструкции не являются текущим Build.</summary>


# Соберите первый плагин

Этот гайд теперь покрывает узкий legacy-compatible путь для Codex runtime на Go:

- target: `codex-runtime`
- язык: `go`
- readiness gate: `validate --strict`

Если вы ещё выбираете путь для нового repo, сначала откройте [Что именно вы собираете](/ru/guide/choose-what-you-are-building) или [Соберите собственную логику плагина](/ru/guide/build-custom-plugin-logic).

## 1. Установите CLI

```bash
brew install 777genius/homebrew-plugin-kit-ai/plugin-kit-ai
plugin-kit-ai version
```

## 2. Создайте проект

```bash
plugin-kit-ai init my-plugin
cd my-plugin
go mod tidy
```

Этот путь сохраняется ради backward compatibility, но уже не является рекомендуемым первым стартом для новых repo.

Один раз запустите `go mod tidy` после scaffold, чтобы starter записал `go.sum` до первой строгой валидации.

## 3. Сгенерируйте target-файлы

```bash
plugin-kit-ai generate .
```

Не редактируйте сгенерированные target-файлы вручную как главный источник истины. Держите исходное состояние проекта внутри обычного `plugin-kit-ai` workflow.

## 4. Прогоните проверку готовности

```bash
plugin-kit-ai validate . --platform codex-runtime --strict
```

Используйте это как главную проверку готовности для локального проекта.

## Что у вас теперь есть

- один plugin repo
- authored files под `plugin/` для новых репозиториев
- generated output для Codex runtime
- понятная проверка готовности через `validate --strict`

## 5. Когда менять путь

Переходите на другой путь только когда это действительно нужно:

- выбирайте `claude` для плагинов Claude
- выбирайте `--runtime node --typescript` для основного стабильного пути без Go
- выбирайте `--runtime python`, когда проект остаётся локальным для репозитория, а команда осознанно Python-first
- выбирайте `codex-package`, `gemini`, `opencode` или `cursor`, только если вам действительно нужен другой способ поставки

Это не означает, что репозиторий должен навсегда остаться single-target: начинайте с самого важного target'а сегодня и добавляйте остальные только по реальной необходимости.

## Следующие шаги

- Откройте [Соберите собственную логику плагина](/ru/guide/build-custom-plugin-logic), если вам на самом деле нужен продвинутый runtime path, а не узкий legacy-compatible tutorial.
- Прочитайте [Один проект, несколько target’ов](/ru/guide/one-project-multiple-targets), если для вас важна идея одного repo и нескольких outputs как основная идея продукта.
- Используйте [Стартовые шаблоны](/ru/guide/starter-templates), когда нужен проверенный пример репозитория.
- Откройте [Справочник CLI](/ru/api/cli/), когда нужно точное поведение команд.

</details>
