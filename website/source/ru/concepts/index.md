---
title: "Концепции"
description: "Ментальная модель plugin-kit-ai: зачем он нужен, как работает один repo, чем отличаются outputs и где начинает играть роль policy."
canonicalId: "page:concepts:index"
section: "concepts"
locale: "ru"
generated: false
translationRequired: true
aside: false
outline: false
---

[Использовать плагины](/ru/use/) · [Создавать плагины](/ru/build/) — Подготовка, не релиз. Миграция проектов в v2 пока недоступна.
<p class="locale-historical-identity">Концепции</p>

[Исторический снимок ветки plugin-kit-ai v1. 1.2.4 — базовая версия команд, а не точная версия этого снимка.](/en/legacy/v1/)

<details><summary>Исторический снимок ветки plugin-kit-ai v1. 1.2.4 — базовая версия команд, а не точная версия этого снимка.</summary>


<div class="docs-hero docs-hero--compact">
  <p class="docs-kicker">CONCEPTS</p>
  <h1>Ментальная модель для тех, кто хочет понять глубже</h1>
  <p class="docs-lead">
    Читайте этот раздел, когда хотите понять модель продукта за командами: зачем продукт нужен, как один repo остаётся главным, чем отличаются outputs и где уже важна policy.
  </p>
</div>

## Читайте в таком порядке

- начните с [Зачем нужен plugin-kit-ai](/ru/concepts/why-plugin-kit-ai), если нужен product-fit ответ
- затем прочитайте [Как работает plugin-kit-ai](/ru/concepts/managed-project-model) для модели одного repo
- откройте [Выбор runtime](/ru/concepts/choosing-runtime), когда решаете между Go, Node или Python
- откройте [Модель target'ов](/ru/concepts/target-model), когда выбираете тип output
- [Модель стабильности](/ru/concepts/stability-model) оставьте напоследок, когда понадобится точный policy language

<div class="docs-grid">
  <a class="docs-card" href="./why-plugin-kit-ai">
    <h2>Зачем нужен plugin-kit-ai</h2>
    <p>Поймите, какую проблему решает продукт, кому он подходит и когда это не тот инструмент.</p>
  </a>
  <a class="docs-card" href="./managed-project-model">
    <h2>Как работает plugin-kit-ai</h2>
    <p>Посмотрите, как один repo остаётся главным, пока вы генерируете output, строго валидируете результат и готовите handoff.</p>
  </a>
  <a class="docs-card" href="./choosing-runtime">
    <h2>Выбор runtime</h2>
    <p>Выберите лучший стартовый язык, не смешивая это решение с packaging или integration setup.</p>
  </a>
  <a class="docs-card" href="./target-model">
    <h2>Модель target'ов</h2>
    <p>Разберитесь в разнице между runtime, package, extension и repo-owned integration outputs.</p>
  </a>
  <a class="docs-card" href="./stability-model">
    <h2>Модель стабильности</h2>
    <p>Используйте точный vocabulary поддержки, когда нужен формальный compatibility contract.</p>
  </a>
</div>

</details>
