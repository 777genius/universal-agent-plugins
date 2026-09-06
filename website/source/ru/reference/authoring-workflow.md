---
title: "Процесс авторинга"
description: "Основной workflow от init до generate, validate, test и handoff."
canonicalId: "page:reference:authoring-workflow"
section: "reference"
locale: "ru"
generated: false
translationRequired: true
---

[Использовать плагины](/ru/use/) · [Создавать плагины](/ru/build/) — Подготовка, не релиз. Миграция проектов в v2 пока недоступна.
<details><summary>Исторические материалы plugin-kit-ai v1 · 1.2.4; старые инструкции не являются текущим Build.</summary>


# Процесс авторинга

Рекомендуемый workflow намеренно простой:

```text
init -> generate -> validate --strict -> test -> handoff
```

<MermaidDiagram
  :chart="`
flowchart LR
  Init[init] --> Generate[generate]
  Generate --> Validate[validate --strict]
  Validate --> Test[test or smoke checks]
  Test --> Handoff[handoff]
  Bootstrap[doctor or bootstrap when needed] -. supports .-> Generate
  Bootstrap -. supports .-> Validate
`"
/>

## Что означает каждый шаг

| Шаг | Назначение |
| --- | --- |
| `init` | Создать package-standard layout проекта |
| `generate` | Сгенерировать target artifacts из исходного состояния проекта |
| `validate --strict` | Запустить главную проверку готовности |
| `test` | Запустить стабильные smoke-тесты там, где это применимо |
| `export` / bundle flow | Выпустить handoff artifacts для поддерживаемых Python и Node сценариев |

## Правила, которые держат repo здоровым

- исходное состояние проекта живёт в package-standard layout
- generated target files — это outputs, а не долгосрочный source of truth
- strict validation — это обязательная проверка, а не необязательная опция

Этот workflow одинаково важен и для single-target, и для multi-target repo.

Разница только в том, что в multi-target проекте цикл `generate` и `validate` повторяется для каждого target'а, который repo действительно обещает поддерживать.

## Когда workflow меняется

Workflow может расширяться в специальных случаях:

- `doctor` и `bootstrap` важны для Python и Node runtime-путей
- `import` и `normalize` важны, когда нужно собрать вручную поддерживаемые target files обратно в управляемую модель проекта
- bundle commands важны для portable Python и Node handoff flows

Начинайте с [Быстрого старта](/ru/guide/quickstart), если нужен самый короткий путь.

</details>
