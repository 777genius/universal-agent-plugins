---
title: "Выбор стартового репозитория"
description: "Практическая матрица для выбора правильного official starter по target, runtime и модели поставки."
canonicalId: "page:guide:choose-a-starter"
section: "guide"
locale: "ru"
generated: false
translationRequired: true
---

[Использовать плагины](/ru/use/) · [Создавать плагины](/ru/build/) — Подготовка, не релиз. Миграция проектов в v2 пока недоступна.
<details><summary>Исторические материалы plugin-kit-ai v1 · 1.2.4; старые инструкции не являются текущим Build.</summary>


# Выбор стартового репозитория

Используйте эту страницу, когда нужен самый быстрый путь в repo, который потом можно расширять на новые поддерживаемые outputs.

<MermaidDiagram
  :chart="`
flowchart TD
  Start[Нужен starter] --> Product{Основной путь Codex или Claude}
  Product --> Codex[Codex starter family]
  Product --> Claude[Claude starter family]
  Codex --> Runtime{Go, Node or Python}
  Claude --> Runtime2{Go, Node or Python}
`"
/>

Перед выбором держите в голове одно важное правило:

- starter показывает, **как начать**
- он не определяет окончательную границу продукта
- и он не мешает одному repo позже поддерживать больше target’ов

Если эта разница пока неочевидна, сначала прочитайте [Один проект, несколько target’ов](/ru/guide/one-project-multiple-targets).

## Сначала выберите, потом расширяйте

- выбирайте Go, когда нужен самый сильный путь для продакшена
- выбирайте Node/TypeScript, когда нужен основной поддерживаемый путь без Go
- выбирайте Python, когда репозиторий осознанно Python-first и остаётся локальным для репозитория
- выбирайте Claude starters только тогда, когда Claude hooks — это реальное product requirement

Starter нужно выбирать под первый правильный путь, а не под воображаемую окончательную форму продукта.

## Что остаётся верным после выбора

- repo остаётся одним
- основной процесс остаётся тем же
- поддерживаемые target’ы можно добавлять позже
- глубина поддержки зависит от того, какой target вы добавляете

## Матрица starter’ов

| Если вам нужен | Лучший starter | Почему |
| --- | --- | --- |
| Самый сильный путь для Codex в продакшене | `plugin-kit-ai-starter-codex-go` | Go-first production path с самой чистой историей передачи другим людям |
| Repo-local Codex plugin на Python | `plugin-kit-ai-starter-codex-python` | Stable Python subset с проверенным layout репозитория |
| Repo-local Codex plugin на Node/TS | `plugin-kit-ai-starter-codex-node-typescript` | Основной поддерживаемый путь без Go |
| Самый сильный путь для Claude в продакшене | `plugin-kit-ai-starter-claude-go` | Stable Claude subset плюс самый чистый путь для продакшена |
| Repo-local Claude plugin на Python | `plugin-kit-ai-starter-claude-python` | Stable Claude hook subset с Python helpers |
| Repo-local Claude plugin на Node/TS | `plugin-kit-ai-starter-claude-node-typescript` | Stable Claude hook subset для TypeScript-first команд |

## Shared-package варианты

Игнорируйте этот раздел, если заранее не знаете, что команде нужен `plugin-kit-ai-runtime` как reusable dependency вместо vendored helper files.

Используйте shared-package варианты, когда:

- нужна общая dependency across multiple plugin repos
- команда готова явно pin'ить и обновлять runtime package
- вы не хотите копировать helper files в каждый repo

Текущие shared-package starter'ы:

- [`plugin-kit-ai-starter-codex-python-runtime-package`](https://github.com/777genius/plugin-kit-ai-starter-codex-python-runtime-package): Python Codex starter с зафиксированным `plugin-kit-ai-runtime` в `requirements.txt`
- [`plugin-kit-ai-starter-claude-node-typescript-runtime-package`](https://github.com/777genius/plugin-kit-ai-starter-claude-node-typescript-runtime-package): Node/TypeScript Claude starter с зафиксированным `plugin-kit-ai-runtime` в `package.json`

Если выбираете между обычным Python starter и Python starter с runtime-package, сначала прочитайте [Python runtime-плагин](/ru/guide/python-runtime), а затем [Выбор модели поставки](/ru/guide/choose-delivery-model).

## Когда не нужно переоптимизировать выбор

Не тратьте слишком много времени на поиск идеального starter.

Если не уверены:

1. начинайте с Go starter ради самого сильного варианта по умолчанию
2. начинайте с Node/TypeScript starter ради основного поддерживаемого пути без Go
3. переходите к Python или shared-package variant только тогда, когда командный компромисс уже реален

## Хорошая командная политика

Выбор starter’а на уровне команды должен быть достаточно стабильным, чтобы:

- все узнавали layout репозитория
- CI использовал один и тот же readiness flow
- handoff не зависел от объяснений maintainer’а

Но стабильный выбор starter’а не мешает одному репозиторию позже добавить другие target’ы, если этого требует продукт.

Свяжите эту страницу со [Стартовыми шаблонами](/ru/guide/starter-templates), [Выбором модели поставки](/ru/guide/choose-delivery-model), [Передачей bundle](/ru/guide/bundle-handoff) и [Стандартом репозитория](/ru/reference/repository-standard).

</details>
