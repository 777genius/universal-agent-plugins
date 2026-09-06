---
title: "Как работает plugin-kit-ai"
description: "Как один repo остаётся source of truth, пока вы генерируете outputs, строго валидируете результат и передаёте чистый handoff."
canonicalId: "page:concepts:managed-project-model"
section: "concepts"
locale: "ru"
generated: false
translationRequired: true
aside: true
outline: [2, 3]
---

[Использовать плагины](/ru/use/) · [Создавать плагины](/ru/build/) — Подготовка, не релиз. Миграция проектов в v2 пока недоступна.
<details><summary>Исторические материалы plugin-kit-ai v1 · 1.2.4; старые инструкции не являются текущим Build.</summary>


# Как работает plugin-kit-ai

plugin-kit-ai держит один repo как source of truth для плагина. Вы редактируете только те файлы, которыми владеете, генерируете нужные outputs, строго валидируете результат и передаёте дальше repo, который остаётся предсказуемым со временем.

## Короткая версия

Базовый цикл очень простой:

```text
source -> generate -> validate --strict -> handoff
```

Этот цикл важен, потому что проект - это не просто starter template. Generated output может меняться вместе с target, а ваш authored source остаётся понятным и поддерживаемым.

## Один repo как source of truth

Repo - это место, где плагин живёт по-настоящему.

- authored files остаются под вашим прямым контролем
- generated outputs пересобираются из этого source
- validation проверяет именно тот результат, который вы собираетесь отдавать
- handoff происходит только после того, как generated результат чистый

Так один проект может аккуратно расти, не расползаясь на несколько копий одного и того же плагина.

## Что вы реально редактируете

Вы продолжаете редактировать исходный проект и тот plugin code, которым владеете. Generated output не должен становиться местом, где живёт настоящая правда проекта.

Именно эта граница делает обновления, смену target и долгую поддержку управляемыми.

## Почему это не просто starter templates

Starter template даёт стартовую форму. plugin-kit-ai продолжает вести проект и после первого дня:

- заново генерирует target-specific output из одного source
- валидирует то, что вы реально собираетесь ship'ить
- чётко разделяет authored files и generated files
- позволяет одному repo позже вырасти в несколько outputs без полной смены модели проекта

## Куда идти дальше

- Читайте [Исходники и generated outputs](/ru/concepts/authoring-architecture), если нужен authored-vs-generated boundary.
- Читайте [Модель target'ов](/ru/concepts/target-model), если нужно понять типы outputs.
- Читайте [Один проект, несколько target'ов](/ru/guide/one-project-multiple-targets), если хотите дальше растить один repo.

</details>
