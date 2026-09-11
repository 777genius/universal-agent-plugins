---
title: "Выбор модели поставки"
description: "Как выбрать между локальными helper-файлами и shared runtime package для Python и Node плагинов."
canonicalId: "page:guide:choose-delivery-model"
section: "guide"
locale: "ru"
generated: false
translationRequired: true
---

[Использовать плагины](/ru/use/) · [Создавать плагины](/ru/build/) — Подготовка, не релиз. Миграция проектов в v2 пока недоступна.
<p class="locale-historical-identity">Выбор модели поставки</p>

[Исторический снимок ветки plugin-kit-ai v1. 1.2.4 — базовая версия команд, а не точная версия этого снимка.](/en/legacy/v1/)

<details><summary>Исторический снимок ветки plugin-kit-ai v1. 1.2.4 — базовая версия команд, а не точная версия этого снимка.</summary>


# Выбор модели поставки

У Python и Node плагинов есть два поддерживаемых способа доставки helper-логики. Они решают разные практические задачи.

<MermaidDiagram
  :chart="`
flowchart TD
  Start[Python or Node plugin] --> Shared{Нужна одна reusable dependency across repos}
  Shared -->|Да| Package[shared runtime package]
  Shared -->|Нет| Smooth{Нужен самый гладкий self contained start}
  Smooth -->|Да| Vendored[vendored helper]
  Smooth -->|Нет| Package
`"
/>

## Быстрое практическое правило

Если вам сегодня нужен просто самый простой рабочий Python или Node репозиторий, начните с пути по умолчанию.

Если вы уже точно знаете, что нескольким репозиториям нужен один общий helper dependency, начинайте сразу с `--runtime-package`.

## Два режима

- `vendored helper`: scaffold записывает helper-файлы прямо в репозиторий
- `shared runtime package`: `--runtime-package` подключает `plugin-kit-ai-runtime` как dependency вместо записи helper в `plugin/`

## Один и тот же проект в двух режимах

Путь по умолчанию с локальным helper:

```bash
plugin-kit-ai init my-plugin --platform codex-runtime --runtime python
```

Путь с общим пакетом:

```bash
plugin-kit-ai init my-plugin --platform codex-runtime --runtime python --runtime-package
```

## Когда выбирать vendored helper

- нужен самый гладкий первый старт
- вы хотите, чтобы репозиторий оставался самодостаточным
- хотите видеть helper implementation прямо в репозитории
- команда ещё не стандартизировалась на одной версии helper-пакета в PyPI или npm

Это путь по умолчанию, потому что он проще всего для первого старта на Python и Node.

## Когда выбирать shared runtime package

- нужна одна reusable helper dependency на несколько plugin repos
- удобнее обновлять helper behavior через обычные package version bumps
- команда готова pin'ить версии в `requirements.txt` или `package.json`
- вы уже знаете, что репозиторий должен идти по shared dependency path с первого дня

## Что это обычно значит на практике

- выбирайте vendored helper, когда главная цель: "быстро запустить один рабочий репозиторий"
- выбирайте shared runtime package, когда главная цель: "использовать один и тот же helper package в нескольких репозиториях"
- не выбирайте shared package только потому, что он звучит более production-like; он не убирает требование иметь Python или Node на машине исполнения

## Что при этом не меняется

- Go всё ещё остаётся рекомендуемым путём по умолчанию, когда нужен самый сильный путь для продакшена
- Python всё ещё требует Python `3.10+` на машине исполнения
- Node всё ещё требует Node.js `20+` на машине исполнения
- `validate --strict` остаётся главной проверкой готовности
- CLI install packages не превращаются в runtime API

## Рекомендуемая политика для команды

- выбирайте Go, когда нужен самый сильный долгосрочно поддерживаемый путь
- выбирайте vendored helpers, когда нужен самый гладкий Python или Node старт
- выбирайте shared runtime package, когда вы уже знаете, что нужна reusable dependency strategy across repos

Свяжите эту страницу с [Python runtime-плагином](/ru/guide/python-runtime), [Выбором стартового репозитория](/ru/guide/choose-a-starter), [Передачей bundle](/ru/guide/bundle-handoff), [Стартовыми шаблонами](/ru/guide/starter-templates) и [Готовностью к продакшену](/ru/guide/production-readiness).

</details>
