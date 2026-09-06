---
title: "Частые вопросы"
description: "Короткие ответы на вопросы, которые команды чаще всего задают при старте и росте plugin-kit-ai repo."
canonicalId: "page:reference:faq"
section: "reference"
locale: "ru"
generated: false
translationRequired: true
---

[Использовать плагины](/ru/use/) · [Создавать плагины](/ru/build/) — Подготовка, не релиз. Миграция проектов в v2 пока недоступна.
<details><summary>Исторические материалы plugin-kit-ai v1 · 1.2.4; старые инструкции не являются текущим Build.</summary>


# Частые вопросы

## С чего начинать: Go, Python или Node?

Начинайте с Go, если нет реальной причины выбрать иначе.

Node/TypeScript - основной поддерживаемый non-Go path. Python подходит, когда plugin остаётся локальным для repo и команда уже Python-first.

## Какой самый простой Python-сценарий?

Сначала используйте обычный Python scaffold:

```bash
plugin-kit-ai init my-plugin --platform codex-runtime --runtime python
plugin-kit-ai doctor ./my-plugin
plugin-kit-ai bootstrap ./my-plugin
plugin-kit-ai generate ./my-plugin
plugin-kit-ai validate ./my-plugin --platform codex-runtime --strict
```

Дальше редактируйте plugin, заново делайте generate и снова валидируйте.

См. [Python runtime-плагин](/ru/guide/python-runtime).

## Когда нужен `--runtime-package`?

Используйте `--runtime-package` только тогда, когда осознанно хотите один общий helper dependency для нескольких repo.

Большинству команд лучше сначала пройти обычный путь с локальным helper.

## npm и PyPI пакеты `plugin-kit-ai` - это runtime API?

Нет. Они устанавливают CLI. Это не runtime API и не SDK.

## Когда использовать bundle-команды?

Используйте bundle-команды, когда другой машине нужны переносимые Python или Node artifacts для скачивания или установки.

Не путайте bundle delivery с основным способом установки CLI.

## Можно ли держать native target files как source of truth?

Нет. Рекомендуемая долгосрочная модель - держать source of truth в package-standard layout, а target files считать generated output.

## `generate` - это опционально?

Нет, если вы хотите управляемый project flow. `generate` - часть workflow.

## `validate --strict` - это опционально?

Воспринимайте его как главную проверку готовности, особенно для локальных Python и Node runtime repo.

## Один repo может вести несколько target'ов?

Да.

Практическое правило такое:

- держите authored state в одном managed repo
- начинайте с главного target, который нужен сегодня
- добавляйте другие target'ы только когда появляется реальная product, delivery или integration задача

См. [Один проект, несколько target'ов](/ru/guide/one-project-multiple-targets) и [Модель target'ов](/ru/concepts/target-model).

## Все target'ы одинаково стабильны?

Нет.

Разные paths несут разное обещание поддержки. Используйте [Границу поддержки](/ru/reference/support-boundary) для короткого ответа и [Поддержку target'ов](/ru/reference/target-support) для точной матрицы.

</details>
