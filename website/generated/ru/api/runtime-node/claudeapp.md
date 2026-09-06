---
title: "Приложение Claude"
description: "Generated Node runtime reference for ClaudeApp"
canonicalId: "node-runtime:ClaudeApp"
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

# Приложение Claude

Сгенерировано через TypeDoc и typedoc-plugin-markdown.

Определено в: index.d.ts:39

Минимальное Claude-приложение, которое маршрутизирует поддерживаемые имена hooks к зарегистрированным обработчикам.

## Конструкторы

### Конструктор

&gt; **new ClaudeApp**(`options`): `ClaudeApp`

Определено в: index.d.ts:46

Создаёт Claude runtime-приложение.

#### Параметры

##### options

###### allowedHooks

readonly `string`[] \| `string`[]

Имена hooks, которые этот бинарник принимает через argv.

###### usage

`string`

Строка помощи, которая печатается при некорректном вызове.

#### Возвращает

`ClaudeApp`

## Методы

### on()

&gt; **on**(`hookName`, `handler`): `this`

Определено в: index.d.ts:50

Регистрирует обработчик для произвольного имени Claude hook.

#### Параметры

##### hookName

`string`

##### handler

`ClaudeHandler`

#### Возвращает

`this`

***

### onPreToolUse()

&gt; **onPreToolUse**(`handler`): `this`

Определено в: index.d.ts:58

Регистрирует обработчик для hook `PreToolUse`.

#### Параметры

##### handler

`ClaudeHandler`

#### Возвращает

`this`

***

### onStop()

&gt; **onStop**(`handler`): `this`

Определено в: index.d.ts:54

Регистрирует обработчик для hook `Stop`.

#### Параметры

##### handler

`ClaudeHandler`

#### Возвращает

`this`

***

### onUserPromptSubmit()

&gt; **onUserPromptSubmit**(`handler`): `this`

Определено в: index.d.ts:62

Регистрирует обработчик для hook `UserPromptSubmit`.

#### Параметры

##### handler

`ClaudeHandler`

#### Возвращает

`this`

***

### run()

&gt; **run**(): `number`

Определено в: index.d.ts:66

Обрабатывает текущий запуск процесса и возвращает код выхода.

#### Возвращает

`number`
