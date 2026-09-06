---
title: "Модель target'ов"
description: "Чем отличаются runtime, package, extension и repo-owned integration outputs, и как выбрать правильный путь."
canonicalId: "page:concepts:target-model"
section: "concepts"
locale: "ru"
generated: false
translationRequired: true
aside: true
outline: [2, 3]
---

[Использовать плагины](/ru/use/) · [Создавать плагины](/ru/build/) — Подготовка, не релиз. Миграция проектов в v2 пока недоступна.
<details><summary>Исторические материалы plugin-kit-ai v1 · 1.2.4; старые инструкции не являются текущим Build.</summary>


# Модель target'ов

Target - это тип output, который должен собирать ваш repo.

Здесь важна не абстрактная taxonomy, а то, что именно вы хотите ship'ить.

## Быстрое правило

- Выбирайте runtime path, когда нужен исполняемый plugin.
- Выбирайте package path, когда другая система будет загружать package output.
- Выбирайте extension path, когда host ожидает extension artifact.
- Выбирайте repo-owned integration setup, когда repo в основном должен хранить checked-in configuration для другого инструмента.

## Runtime paths

Runtime targets дают исполняемый результат. Это default start для большинства команд, потому что так проще всего владеть поведением, валидировать output и позже растить repo.

## Package paths

Package targets дают package output вместо основной исполняемой runtime-формы. Используйте их, когда packaging - это реальное требование поставки, а не просто дополнительный export на будущее.

## Extension paths

Extension targets подходят для host'ов, которые ожидают конкретный extension artifact или installable package shape.

## Repo-owned integration setup

Некоторые outputs - это в основном checked-in configuration, которое помогает другому tool или workspace использовать plugin. Это тоже полезные supported paths, но они отвечают на другой delivery question, чем исполняемый runtime.

## Безопасная модель

Сначала выбирайте тот output, который реально нужен. Если repo позже вырастет, вы сможете добавить ещё один supported output, не меняя тот факт, что главный источник всё равно один.

</details>
