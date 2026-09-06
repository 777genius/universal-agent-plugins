---
title: "Приложение Codex"
description: "Generated Node runtime reference for CodexApp"
canonicalId: "node-runtime:CodexApp"
surface: "runtime-node"
section: "api"
locale: "ru"
generated: true
editLink: false
stability: "public-stable"
maturity: "stable"
sourceRef: "npm/plugin-kit-ai-runtime"
translationRequired: false
---
<DocMetaCard surface="runtime-node" stability="public-stable" maturity="stable" source-ref="npm/plugin-kit-ai-runtime" source-href="https://github.com/777genius/universal-agent-plugins/tree/main/npm/plugin-kit-ai-runtime" />

# Приложение Codex

Сгенерировано через TypeDoc и typedoc-plugin-markdown.

Определено в: index.d.ts:72

Минимальное Codex-приложение, которое маршрутизирует событие `notify` к зарегистрированному обработчику.

## Конструкторы

### Конструктор

&gt; **new CodexApp**(): `CodexApp`

Определено в: index.d.ts:76

Создаёт Codex runtime-приложение без зарегистрированных обработчиков.

#### Возвращает

`CodexApp`

## Методы

### onNotify()

&gt; **onNotify**(`handler`): `this`

Определено в: index.d.ts:80

Регистрирует обработчик для события Codex `notify`.

#### Параметры

##### handler

`CodexHandler`

#### Возвращает

`this`

***

### run()

&gt; **run**(): `number`

Определено в: index.d.ts:84

Обрабатывает текущий запуск процесса и возвращает код выхода.

#### Возвращает

`number`
