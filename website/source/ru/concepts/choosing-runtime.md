---
title: "Выбор runtime"
description: "Как выбирать между Go, Python, Node и shell authoring paths."
canonicalId: "page:concepts:choosing-runtime"
section: "concepts"
locale: "ru"
generated: false
translationRequired: true
---

[Использовать плагины](/ru/use/) · [Создавать плагины](/ru/build/) — Подготовка, не релиз. Миграция проектов в v2 пока недоступна.
<details><summary>Исторические материалы plugin-kit-ai v1 · 1.2.4; старые инструкции не являются текущим Build.</summary>


# Выбор runtime

Выбор runtime - это не только вопрос языка. Он меняет то, как запускается plugin, что должно быть установлено на машине исполнения и насколько простыми будут CI и handoff.

<MermaidDiagram
  :chart="`
flowchart TD
  Start[Нужен runtime lane] --> Prod{Нужен самый сильный runtime lane}
  Prod -->|Да| Go[go]
  Prod -->|Нет| Local{Плагин repo-local по дизайну}
  Local -->|Да| Team{Команда Python-first или Node-first}
  Team --> Python[python]
  Team --> Node[node or node --typescript]
  Local -->|Нет| Escape{Нужен только escape hatch}
  Escape --> Shell[shell]
`"
/>

## Выбирайте Go, когда

- нужен самый сильный runtime lane
- нужны типизированные обработчики и самая чистая release story
- хочется минимальных проблем с bootstrap в CI и на других машинах

## Выбирайте Python или Node, когда

- plugin по дизайну repo-local
- команда уже живёт в этом runtime
- вы готовы сами владеть runtime bootstrap
- вас устраивает, что на машине исполнения должен стоять Python `3.10+` или Node.js `20+`

## Выбирайте Shell только когда

- нужен узкий escape hatch
- вы осознанно принимаете experimental или advanced tradeoff

## Безопасная матрица выбора

| Ситуация | Рекомендуемый выбор |
| --- | --- |
| Самый сильный runtime lane | `go` |
| Основной non-Go runtime lane | `node --typescript` |
| Локальная Python-first команда | `python` |
| Escape hatch | `shell` |

</details>
