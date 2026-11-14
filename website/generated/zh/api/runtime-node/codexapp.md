---
title: "Codex App"
description: "生成的 Node runtime 参考 for CodexApp"
canonicalId: "node-runtime:CodexApp"
surface: "runtime-node"
section: "api"
locale: "zh"
generated: true
editLink: false
stability: "public-stable"
maturity: "stable"
sourceRef: "npm/plugin-kit-ai-runtime"
translationRequired: false
---
<DocMetaCard surface="runtime-node" stability="public-stable" maturity="stable" source-ref="npm/plugin-kit-ai-runtime" source-href="https://github.com/777genius/universal-agent-plugins/tree/main/npm/plugin-kit-ai-runtime" />

# Codex App

通过 TypeDoc 和 typedoc-plugin-markdown 生成。

Defined in: index.d.ts:72

Minimal Codex app that dispatches the `notify` event to a registered handler.

## Constructors

### Constructor

&gt; **new CodexApp**(): `CodexApp`

Defined in: index.d.ts:76

Creates a Codex runtime app with no registered handlers.

#### Returns

`CodexApp`

## Methods

### onNotify()

&gt; **onNotify**(`handler`): `this`

Defined in: index.d.ts:80

Registers a handler for the Codex `notify` event.

#### Parameters

##### handler

`CodexHandler`

#### Returns

`this`

***

### run()

&gt; **run**(): `number`

Defined in: index.d.ts:84

Dispatches the current process invocation and returns the exit code.

#### Returns

`number`
