---
title: "Выбор target"
description: "Практический гид по выбору target под то, как вы хотите поставлять плагин."
canonicalId: "page:guide:choose-a-target"
section: "guide"
locale: "ru"
generated: false
translationRequired: true
---

[Использовать плагины](/ru/use/) · [Создавать плагины](/ru/build/) — Подготовка, не релиз. Миграция проектов в v2 пока недоступна.
<details><summary>Исторические материалы plugin-kit-ai v1 · 1.2.4; старые инструкции не являются текущим Build.</summary>


# Выбор target

Это страница для более точного выбора пути поставки.
Если вы ещё только выбираете, какой repo создавать, сначала откройте [Что именно вы собираете](/ru/guide/choose-what-you-are-building).

Используйте эту страницу, когда вы уже понимаете, что хотите работать с `plugin-kit-ai`, но ещё сопоставляете repo с тем, как именно хотите поставлять плагин.

Выбор target означает выбор главного пути на сегодня, а не вечный lock-in на один output.

<MermaidDiagram
  :chart="`
flowchart TD
  Need[Что нужно продукту сейчас] --> Exec{Исполняемое поведение}
  Need --> Artifact{Package или extension}
  Need --> Config{Repo managed integration}
  Exec --> Codex[codex-runtime]
  Exec --> Claude[claude]
  Artifact --> CodexPackage[codex-package]
  Artifact --> Gemini[gemini]
  Config --> OpenCode[opencode]
  Config --> Cursor[cursor]
`"
/>

## Короткое правило

- выбирайте `codex-runtime`, когда нужен самый сильный runtime путь по умолчанию
- выбирайте `claude`, когда Claude hooks и есть реальное требование продукта
- выбирайте `codex-package`, когда продуктом является официальный пакет Codex
- выбирайте `gemini`, когда продуктом является пакет расширения Gemini
- выбирайте `opencode` или `cursor`, когда repo должен хранить integration/config setup

## Краткий справочник по target'ам

| Target | Когда выбирать | Lane |
| --- | --- | --- |
| `codex-runtime` | Нужен основной путь для исполняемого плагина | Рекомендуемый runtime path |
| `claude` | Нужны именно Claude hooks | Рекомендуемый Claude path |
| `codex-package` | Нужен package output для Codex | Рекомендуемый package path |
| `gemini` | Вы выпускаете пакет расширения Gemini | Рекомендуемый extension path |
| `opencode` | Нужна настройка OpenCode в самом repo | Настройка интеграции в самом repo |
| `cursor` | Нужна настройка Cursor в самом repo | Настройка интеграции в самом repo |

## Безопасный выбор по умолчанию

Если вы не уверены, начинайте с `codex-runtime` и стандартного пути на Go.

Это даёт самую чистую production starting point перед тем, как идти в более узкий или специализированный путь.

Когда позже вы переходите на `codex-package`, этот путь использует официальный bundle layout с `.codex-plugin/plugin.json`.

Если вы осознанно стартуете на поддерживаемом Node/TypeScript или Python, это меняет выбор языка, а не заставляет в первый день решать все вопросы с упаковкой и интеграциями.

## Что делать, если target'ов нужно несколько

- сначала выберите основной путь, который определяет продукт сегодня
- держите repo единым
- добавляйте новые target'ы только тогда, когда появляется реальная delivery или integration задача

</details>
