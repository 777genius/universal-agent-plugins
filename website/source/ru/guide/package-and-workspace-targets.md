---
title: "Пакеты и настройка интеграций"
description: "Когда packaging или checked-in integration setup нужны вместо исполняемого runtime plugin."
canonicalId: "page:guide:package-and-workspace-targets"
section: "guide"
locale: "ru"
generated: false
translationRequired: true
aside: true
outline: [2, 3]
---

[Использовать плагины](/ru/use/) · [Создавать плагины](/ru/build/) — Подготовка, не релиз. Миграция проектов в v2 пока недоступна.
<p class="locale-historical-identity">Пакеты и настройка интеграций</p>

[Исторический снимок ветки plugin-kit-ai v1. 1.2.4 — базовая версия команд, а не точная версия этого снимка.](/en/legacy/v1/)

<details><summary>Исторический снимок ветки plugin-kit-ai v1. 1.2.4 — базовая версия команд, а не точная версия этого снимка.</summary>


# Пакеты и настройка интеграций

Не каждый проект должен поставляться как исполняемый runtime plugin.

Иногда реальное требование - это package, который загрузит другая система, extension artifact или checked-in integration setup, живущий прямо в repo.

## Короткое правило

Выбирайте packages или integration setup, когда форма поставки важнее, чем прямой запуск plugin.

## Когда нужна именно эта страница

Этот путь подходит, когда:

- packaging - это реальное требование поставки
- host ожидает extension или packaged artifact
- repo в основном должен хранить checked-in integration setup для другого tool
- исполняемый runtime добавил бы лишнюю operational complexity

## Чем это отличается от runtime path

Runtime path обычно остаётся самым понятным default, когда нужен исполняемый plugin.

Packages и integration setup отвечают на другой вопрос: в какой форме этот plugin должен быть доставлен или подключён к другой системе?

## Безопасная модель

Выбирайте runtime, когда хотите запускать plugin напрямую. Выбирайте packages или integration setup, когда форма поставки - это главное требование.

## Граница Codex package

Для официального Codex package lane держите bundle layout узким и явным:

- `.codex-plugin/` содержит только `plugin.json`
- optional `.app.json` и `.mcp.json` лежат в корне plugin

Этот package path нужен для официального Codex plugin bundle surface, а не для смешивания repo-local runtime wiring с package layout.

</details>
