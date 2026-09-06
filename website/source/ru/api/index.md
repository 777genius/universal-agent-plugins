---
title: "API"
description: "Сгенерированный API-справочник для plugin-kit-ai."
canonicalId: "page:api:index"
section: "api"
locale: "ru"
generated: false
translationRequired: true
aside: false
outline: false
---

[Использовать плагины](/ru/use/) · [Создавать плагины](/ru/build/) — Подготовка, не релиз. Миграция проектов в v2 пока недоступна.
<details><summary>Исторические материалы plugin-kit-ai v1 · 1.2.4; старые инструкции не являются текущим Build.</summary>


<div class="docs-hero docs-hero--compact">
  <p class="docs-kicker">СГЕНЕРИРОВАННЫЙ СПРАВОЧНИК</p>
  <h1>Поверхности API</h1>
  <p class="docs-lead">
    Этот раздел собирает публичные API plugin-kit-ai: CLI, Go SDK, runtime-хелперы, события платформ и возможности API.
  </p>
</div>

<div class="docs-grid">
  <a class="docs-card" href="./cli/">
    <h2>CLI</h2>
    <p>Команды, экспортированные из живого дерева Cobra.</p>
  </a>
  <a class="docs-card" href="./go-sdk/">
    <h2>Go SDK</h2>
    <p>Публичные Go-пакеты для надёжных runtime-плагинов.</p>
  </a>
  <a class="docs-card" href="./runtime-node/">
    <h2>Node Runtime</h2>
    <p>Типизированные runtime-хелперы для Node, JavaScript и TypeScript.</p>
  </a>
  <a class="docs-card" href="./runtime-python/">
    <h2>Python Runtime</h2>
    <p>Только публичные Python runtime-хелперы, без install-wrapper пакетов.</p>
  </a>
  <a class="docs-card" href="./platform-events/">
    <h2>События платформ</h2>
    <p>События и точки входа, сгруппированные по целевым платформам.</p>
  </a>
  <a class="docs-card" href="./capabilities/">
    <h2>Capabilities</h2>
    <p>Возможности API, сгруппированные поперёк платформ и событий.</p>
  </a>
</div>

## Как выбрать нужную поверхность

- Открывайте `CLI`, когда нужны команды, флаги и шаги авторинга.
- Открывайте `Go SDK`, когда собираете надёжный runtime-плагин на Go.
- Открывайте `Node Runtime` или `Python Runtime`, когда нужен общий API хелперов для локального runtime в репозитории.
- Открывайте `Platform Events`, когда выбираете конкретные события целевой платформы.
- Открывайте `Capabilities`, когда нужно понять, какие действия и точки расширения доступны поперёк платформ.

## Что покрывает эта API-зона

- живое дерево команд Cobra
- публичные Go-пакеты
- общие runtime-хелперы для Node и Python
- события конкретных платформ
- сводку по возможностям API поперёк платформ

</details>
