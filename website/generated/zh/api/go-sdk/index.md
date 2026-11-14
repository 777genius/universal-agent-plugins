---
title: "Go SDK"
description: "生成的 Go SDK package 参考"
canonicalId: "page:api:go-sdk:index"
surface: "go-sdk"
section: "api"
locale: "zh"
generated: true
editLink: false
stability: "public-stable"
maturity: "stable"
sourceRef: "sdk"
translationRequired: false
---
# Go SDK

The Go SDK is the recommended default path when you want the strongest production contract.

- Open this area when you are building a production-oriented Go plugin.
- This is the best starting point when you want the least downstream runtime friction.
- If you are still choosing between Go, Python, and Node, start with `/guide/what-you-can-build` and `/concepts/choosing-runtime`.

| Package | Summary |
| --- | --- |
| [`sdk`](/zh/api/go-sdk/sdk) | Root composition and runtime entry package. |
| [`claude`](/zh/api/go-sdk/claude) | Public Claude-oriented handlers and event wiring. |
| [`codex`](/zh/api/go-sdk/codex) | Public Codex-oriented handlers and runtime integration. |
| [`gemini`](/zh/api/go-sdk/gemini) | Public Gemini-oriented handlers and runtime integration. |
| [`platformmeta`](/zh/api/go-sdk/platformmeta) | Platform metadata and support-oriented helpers. |
