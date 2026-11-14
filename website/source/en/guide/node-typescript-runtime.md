---
title: "Build A Node/TypeScript Runtime Plugin"
description: "The main supported non-Go path for local runtime plugins."
canonicalId: "page:guide:node-typescript-runtime"
section: "guide"
locale: "en"
generated: false
translationRequired: true
---

# Build A Node/TypeScript Runtime Plugin

This is the main supported non-Go path when your team wants TypeScript but still needs a supported local runtime plugin.

## Recommended Flow

```bash
plugin-kit-ai init my-plugin --platform codex-runtime --runtime node --typescript
plugin-kit-ai doctor ./my-plugin
plugin-kit-ai bootstrap ./my-plugin
plugin-kit-ai generate ./my-plugin
plugin-kit-ai validate ./my-plugin --platform codex-runtime --strict
```

## What To Remember

- this is a stable local-runtime path, not the zero-runtime-dependency Go path
- the execution machine still needs Node.js `20+`
- `doctor` and `bootstrap` matter more here than in the default Go path

## When This Is The Right Choice

- your team already works in TypeScript
- the plugin stays local to the repo by design
- you want the main supported non-Go path without dropping into a beta escape hatch

## When Go Is Still Better

Prefer Go instead when:

- you want the strongest production contract
- you want downstream users to avoid installing Node
- you want the least bootstrap friction in CI and on other machines

See [Choosing Runtime](/en/concepts/choosing-runtime) and [Node Runtime API](/en/api/runtime-node/) for the next layer of detail.
