---
title: "Документация plugin-kit-ai"
description: "Публичная документация по plugin-kit-ai."
canonicalId: "page:home"
section: "home"
locale: "ru"
generated: false
translationRequired: true
---

[Использовать плагины](/ru/use/) · [Создавать плагины](/ru/build/) — Подготовка, не релиз. Миграция проектов в v2 пока недоступна.
<p class="locale-historical-identity">Документация plugin-kit-ai</p>

[Исторический снимок ветки plugin-kit-ai v1. 1.2.4 — базовая версия команд, а не точная версия этого снимка.](/en/legacy/v1/)

<details><summary>Исторический снимок ветки plugin-kit-ai v1. 1.2.4 — базовая версия команд, а не точная версия этого снимка.</summary>


<div class="docs-hero docs-hero--feature">
  <p class="docs-kicker">ПУБЛИЧНАЯ ДОКУМЕНТАЦИЯ</p>
  <h1>plugin-kit-ai</h1>
  <p class="docs-lead">
    Собирайте один plugin repo и отправляйте его в несколько AI agents, не погружаясь в полную target-модель в первый же день.
  </p>
</div>

## Начните с задачи

- [Подключить онлайн-сервис](/ru/guide/choose-what-you-are-building#подключить-онлайн-сервис)
- [Подключить локальный инструмент](/ru/guide/choose-what-you-are-building#подключить-локальный-инструмент)
- [Сделать свой plugin с логикой - Advanced](/ru/guide/build-custom-plugin-logic)

## Что важно понять сразу

- один репозиторий остаётся source of truth по мере добавления новых lanes
- выбирайте стартовый путь под задачу, которая нужна прямо сейчас
- расширяйтесь позже из того же repo, когда продукту понадобятся новые outputs
- используйте `generate` и `validate --strict` как общий readiness workflow

<div class="docs-grid">
  <a class="docs-card" href="./guide/choose-what-you-are-building">
    <h2>Что именно вы делаете</h2>
    <p>Сначала выберите задачу, а уже потом уходите в детали target'ов и packaging.</p>
  </a>
  <a class="docs-card" href="./guide/quickstart">
    <h2>Быстрый старт</h2>
    <p>Быстро поднимите рабочий repo через новый job-first вход.</p>
  </a>
  <a class="docs-card" href="./guide/build-custom-plugin-logic">
    <h2>Advanced custom logic</h2>
    <p>Откройте guided path для runtime code, hooks и orchestration, когда одного wiring уже недостаточно.</p>
  </a>
  <a class="docs-card" href="./guide/what-you-can-build">
    <h2>Что можно построить</h2>
    <p>Посмотрите на общую форму продукта: runtime, package, extension и настройка интеграций в самом repo.</p>
  </a>
  <a class="docs-card" href="./guide/choose-a-target">
    <h2>Выбор target</h2>
    <p>Сопоставьте target с тем, как вы хотите поставлять плагин, а не пытайтесь считать все outputs одним и тем же.</p>
  </a>
  <a class="docs-card" href="./reference/support-boundary">
    <h2>Точный контракт</h2>
    <p>Переходите в reference, когда нужен точный язык совместимости и границ поддержки.</p>
  </a>
</div>

## Читайте в таком порядке

<div class="docs-grid">
  <a class="docs-card" href="./guide/choose-what-you-are-building">
    <h2>1. Что вы собираете</h2>
    <p>Выберите online service, local tool или custom logic до target-деталей.</p>
  </a>
  <a class="docs-card" href="./guide/quickstart">
    <h2>2. Быстрый старт</h2>
    <p>Превратите этот выбор в рабочий repo и первый validation loop.</p>
  </a>
  <a class="docs-card" href="./guide/build-custom-plugin-logic">
    <h2>3. Advanced custom logic</h2>
    <p>Используйте этот путь, когда ценность плагина живёт в вашем коде, hooks и runtime behavior.</p>
  </a>
  <a class="docs-card" href="./guide/what-you-can-build">
    <h2>4. Что можно построить</h2>
    <p>Посмотрите на общую product shape по runtime, package, extension и integration lanes.</p>
  </a>
  <a class="docs-card" href="./guide/choose-a-target">
    <h2>5. Выбор target</h2>
    <p>Открывайте это позже, когда уже нужны конкретные решения по способу поставки.</p>
  </a>
  <a class="docs-card" href="./reference/support-boundary">
    <h2>6. Граница поддержки</h2>
    <p>Открывайте reference cluster, когда нужен точный compatibility language и support details.</p>
  </a>
</div>

Если вы новый пользователь, на стартовых страницах уже можно остановиться.

## Текущий базовый релиз репозитория

- Текущая публичная опорная версия в этом наборе docs - [`v1.1.2`](/ru/releases/v1-1-2).
- Эта patch-линейка вернула совместимость first-party installs между legacy и current authored layouts, а затем починила Gemini full multi-target installs для GitHub repo-path sources.
- Начинайте с него, если нужен актуальный baseline.

</details>
