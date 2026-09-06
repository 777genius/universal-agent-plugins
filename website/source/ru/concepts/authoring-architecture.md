---
title: "Исходники и generated outputs"
description: "Как authored files, generated outputs, strict validation и handoff складываются в рабочую модель plugin-kit-ai."
canonicalId: "page:concepts:authoring-architecture"
section: "concepts"
locale: "ru"
generated: false
translationRequired: true
aside: true
outline: [2, 3]
---

[Использовать плагины](/ru/use/) · [Создавать плагины](/ru/build/) — Подготовка, не релиз. Миграция проектов в v2 пока недоступна.
<details><summary>Исторические материалы plugin-kit-ai v1 · 1.2.4; старые инструкции не являются текущим Build.</summary>


# Исходники и generated outputs

Эта страница уже уже, чем основная модель продукта. Она объясняет рабочую границу внутри repo: что вы author'ите, что генерируется и почему это разделение делает проект поддерживаемым.

## Базовая форма

```text
project source -> generate -> target outputs -> validate --strict -> handoff
```

Source остаётся стабильным. Outputs могут отличаться в зависимости от target. Validation проверяет, что generated результат всё ещё безопасно передавать дальше.

## Authored files и generated files

Authored files - это часть repo, которую команда поддерживает напрямую.

Generated files - это build artifacts для выбранных target'ов. Они реальны и нужны для поставки, но именно они не должны становиться местом, где начинает дрейфовать правда проекта.

Это разделение делает regen повторяемым и keeps the repo readable.

## Почему это важно

Без этой границы команды начинают редактировать generated output вручную, теряют повторяемость и усложняют обновления сильнее, чем нужно.

С этой границей можно:

- ревьюить изменения в source напрямую
- спокойно пересобирать output
- валидировать один и тот же delivery shape каждый раз
- позже добавлять ещё один supported output без пересборки repo с нуля

## Как это связано с общей моделью

Если нужен уровень выше, начинайте с [Как работает plugin-kit-ai](/ru/concepts/managed-project-model).

</details>
