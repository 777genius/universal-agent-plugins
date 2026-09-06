---
title: "Зачем нужен plugin-kit-ai"
description: "Какую проблему решает plugin-kit-ai, кому он подходит и когда это не тот инструмент."
canonicalId: "page:concepts:why-plugin-kit-ai"
section: "concepts"
locale: "ru"
generated: false
translationRequired: true
aside: true
outline: [2, 3]
---

[Использовать плагины](/ru/use/) · [Создавать плагины](/ru/build/) — Подготовка, не релиз. Миграция проектов в v2 пока недоступна.
<p class="locale-historical-identity">Зачем нужен plugin-kit-ai</p>

[Исторический снимок ветки plugin-kit-ai v1. 1.2.4 — базовая версия команд, а не точная версия этого снимка.](/en/legacy/v1/)

<details><summary>Исторический снимок ветки plugin-kit-ai v1. 1.2.4 — базовая версия команд, а не точная версия этого снимка.</summary>


# Зачем нужен plugin-kit-ai

plugin-kit-ai нужен командам, которые хотят один поддерживаемый plugin project, а не набор несвязанных repo, target-specific copies и starter templates.

## Какую проблему он решает

Большинство agent integrations легко начать и трудно поддерживать.

Сначала появляются одноразовые шаблоны, отдельные папки под разные target'ы и повторяющийся setup. Как только нужно поддержать ещё один runtime, package или integration path, проект начинает фрагментироваться.

plugin-kit-ai даёт один repo, который остаётся главным, пока вы генерируете нужные outputs.

## Кому он подходит

Этот продукт подходит, если вы хотите:

- держать один source project для плагина
- регенерировать supported outputs вместо ручной поддержки копий
- валидировать именно тот результат, который собираетесь ship'ить
- аккуратно расширять проект на дополнительные outputs со временем

## Когда это не тот инструмент

Скорее всего это не ваш случай, если вам нужен только:

- одноразовый single-target starter
- быстрый copy-paste prototype без плана долгой поддержки
- repo, в котором generated output становится главным source of truth

## Как устроена модель продукта

Если product fit уже понятен и нужен operating model, читайте [Как работает plugin-kit-ai](/ru/concepts/managed-project-model).

</details>
