---
title: "Документация Agent Plugins"
description: "Установка плагинов и создание переносимых пакетов Skills и MCP."
canonicalId: "page:home"
section: "home"
locale: "ru"
generated: false
translationRequired: true
localeDisposition: "current-translation"
---

<div class="docs-hero docs-hero--feature">
  <p class="docs-kicker">ДОКУМЕНТАЦИЯ</p>
  <h1>Agent Plugins</h1>
  <p class="docs-lead">Устанавливайте Skills и MCP-инструменты для своих агентов или создайте собственный переносимый пакет.</p>
</div>

## Выберите задачу

<div class="docs-grid">
  <a class="docs-card" href="./use/">
    <h2>Использовать плагины</h2>
    <p>Изучите пакет, проверьте план изменений и установите его для своей учётной записи или одного проекта.</p>
  </a>
  <a class="docs-card" href="./build/">
    <h2>Создавать плагины</h2>
    <p>Подготовьте стандартный пакет с plugin.json, Skills и конфигурацией MCP. Попробуйте средства разработки из исходного кода.</p>
  </a>
</div>

## Установите первый плагин

Если установлен Node.js 22 или новее:

```bash
npx universal-agent-plugins add context7
```

CLI предложит совместимые агенты. Проверьте план и выполните указанные действия
по активации или входу в сервис. Для установки нативного исполняемого файла
в macOS, Linux или Windows откройте
[инструкцию](https://github.com/777genius/universal-agent-plugins/blob/main/docs/NATIVE_INSTALL.md).
Самому нативному менеджеру Node.js не нужен.

## Что доступно сейчас

[CLI 0.1.93](https://github.com/777genius/universal-agent-plugins/releases/tag/agentplugins-v0.1.93)
— текущий выпуск установщика. Он поддерживает явную установку в проект и
восстановление по lock-файлу. Указывайте `--scope project` в каждой команде для
проекта: по умолчанию опубликованный CLI использует пользовательское окружение.
Подробнее: [установка](./use/install) и [обслуживание](./use/manage).

В текущем установщике нет `agentplugins author`.
[Руководство по созданию пакетов](./build/) описывает ручную подготовку файлов и
средства разработки, реализованные в текущем исходном коде. Доказательства для
старых выпусков этих средств относятся к указанным версиям и не описывают
нынешний установщик.

## Как понимать результат

Проверка пакета, установка файлов, обнаружение в клиенте, вызов MCP-инструмента
и аутентификация сервиса — отдельные проверки. После записи файлов клиенту
могут потребоваться доверие к проекту, перезапуск или новый сеанс. Нативные тесты
относятся к конкретным версиям клиентов и платформам; они не подтверждают работу
любого плагина во всех агентах.

- [Быстрый старт](./guide/quickstart)
- [Структура пакета](./build/layout)
- [Проверки и их границы](./build/checks)
- [Совместимость клиентов и результаты тестов](./reference/client-compatibility)
- [Спецификация Agent Plugins 1.0](https://agent-plugins.org/specification)

<!-- locale-historical-source:start
locale-historical-frontmatter:start
---
title: "Документация plugin-kit-ai"
description: "Публичная документация по plugin-kit-ai."
canonicalId: "page:home"
section: "home"
locale: "ru"
generated: false
translationRequired: true
---
locale-historical-frontmatter:end

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
locale-historical-source:end -->
