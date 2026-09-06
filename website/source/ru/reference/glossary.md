---
title: "Словарь терминов"
description: "Короткие определения публичных терминов, которые встречаются в docs plugin-kit-ai."
canonicalId: "page:reference:glossary"
section: "reference"
locale: "ru"
generated: false
translationRequired: true
---

[Использовать плагины](/ru/use/) · [Создавать плагины](/ru/build/) — Подготовка, не релиз. Миграция проектов в v2 пока недоступна.
<p class="locale-historical-identity">Словарь терминов</p>

[Исторический снимок ветки plugin-kit-ai v1. 1.2.4 — базовая версия команд, а не точная версия этого снимка.](/en/legacy/v1/)

<details><summary>Исторический снимок ветки plugin-kit-ai v1. 1.2.4 — базовая версия команд, а не точная версия этого снимка.</summary>


# Словарь терминов

Используйте эту страницу, когда какой-то термин тормозит чтение docs. Цель здесь не идеальная теория, а быстрое общее понимание.

## Авторское состояние

Часть repo, которой команда владеет напрямую. `generate` превращает этот source в target-specific output.

## Сгенерированные target-файлы

Файлы, которые появляются для конкретного target после генерации. Это реальный delivery output, но не долгосрочный source of truth.

## Путь

Практический способ собрать и поставлять plugin. Примеры: default Go runtime path, локальный Node/TypeScript path и repo-owned integration setup.

## Target

Output, в который вы целитесь, например `codex-runtime`, `claude`, `codex-package`, `gemini`, `opencode` или `cursor`.

## Runtime-путь

Path, в котором repo напрямую владеет исполняемым поведением plugin.

## Package- или extension-путь

Path, сфокусированный на правильном package или extension artifact, а не на основной исполняемой runtime-форме.

## Настройка интеграции, которой владеет репозиторий

Path, где repo в основном поставляет checked-in configuration для другого tool или workspace.

## Канал установки

Способ установить CLI, например через Homebrew, npm, PyPI или verified script. Это не public runtime API.

## Общий runtime-пакет

Зависимость `plugin-kit-ai-runtime`, которую используют одобренные Python и Node flows вместо копирования helper files в каждый repo.

## Граница поддержки

Публичная граница между тем, что проект рекомендует по умолчанию, что поддерживает осторожнее и что оставляет experimental.

## Гейт готовности

Проверка, которую стоит считать сигналом, что repo уже достаточно здоров для handoff. Для большинства repo это `validate --strict`.

## Передача

Момент, когда другой человек, другая машина или другой пользователь может использовать repo без скрытых шагов setup.

Связанные страницы: [Модель target'ов](/ru/concepts/target-model), [Граница поддержки](/ru/reference/support-boundary) и [Готовность к продакшену](/ru/guide/production-readiness).

</details>
