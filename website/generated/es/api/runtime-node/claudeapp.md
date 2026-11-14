---
title: "Claude App"
description: "Referencia generada de Node runtime for ClaudeApp"
canonicalId: "node-runtime:ClaudeApp"
surface: "runtime-node"
section: "api"
locale: "es"
generated: true
editLink: false
stability: "public-stable"
maturity: "stable"
sourceRef: "npm/plugin-kit-ai-runtime"
translationRequired: false
---
<DocMetaCard surface="runtime-node" stability="public-stable" maturity="stable" source-ref="npm/plugin-kit-ai-runtime" source-href="https://github.com/777genius/universal-agent-plugins/tree/main/npm/plugin-kit-ai-runtime" />

# Claude App

Generado mediante TypeDoc y typedoc-plugin-markdown.

Defined in: index.d.ts:39

Minimal Claude hook app that dispatches supported hook names to registered handlers.

## Constructors

### Constructor

&gt; **new ClaudeApp**(`options`): `ClaudeApp`

Defined in: index.d.ts:46

Creates a Claude runtime app.

#### Parameters

##### options

###### allowedHooks

readonly `string`[] \| `string`[]

Hook names that this binary accepts on argv.

###### usage

`string`

Usage string printed when the invocation is invalid.

#### Returns

`ClaudeApp`

## Methods

### on()

&gt; **on**(`hookName`, `handler`): `this`

Defined in: index.d.ts:50

Registers a handler for an arbitrary Claude hook name.

#### Parameters

##### hookName

`string`

##### handler

`ClaudeHandler`

#### Returns

`this`

***

### onPreToolUse()

&gt; **onPreToolUse**(`handler`): `this`

Defined in: index.d.ts:58

Registers a handler for the `PreToolUse` hook.

#### Parameters

##### handler

`ClaudeHandler`

#### Returns

`this`

***

### onStop()

&gt; **onStop**(`handler`): `this`

Defined in: index.d.ts:54

Registers a handler for the `Stop` hook.

#### Parameters

##### handler

`ClaudeHandler`

#### Returns

`this`

***

### onUserPromptSubmit()

&gt; **onUserPromptSubmit**(`handler`): `this`

Defined in: index.d.ts:62

Registers a handler for the `UserPromptSubmit` hook.

#### Parameters

##### handler

`ClaudeHandler`

#### Returns

`this`

***

### run()

&gt; **run**(): `number`

Defined in: index.d.ts:66

Dispatches the current process invocation and returns the exit code.

#### Returns

`number`
