---
title: "Soporte de targets"
description: "Resumen generado de soporte de targets"
canonicalId: "page:reference:target-support"
surface: "reference"
section: "reference"
locale: "es"
generated: true
editLink: false
stability: "historical"
maturity: "historical"
sourceRef: "docs/generated/target_support_matrix.md"
translationRequired: false
historicalVersion: "1.2.4"
sourceSHA: "9beca10448ac50fbe526a52101d1433a12471980"
status: "historical"
---

> Historical plugin-kit-ai v1, baseline **1.2.4** ([exact source](https://github.com/777genius/universal-agent-plugins/tree/9beca10448ac50fbe526a52101d1433a12471980)). Project migration is not available in v2 yet. Maintain legacy projects using the v1 1.2.4 command set.

# Soporte de targets

Use this page when you need the compact lane map across runtime, package, extension, and repo-managed integration outputs.

| Target | Production Class | Runtime Contract | Install Model |
| --- | --- | --- | --- |
| claude | production-ready package+runtime lane | stable runtime subset | marketplace or local |
| codex-package | recommended package lane | official package only | marketplace or local |
| codex-runtime | recommended runtime lane | stable notify runtime | repo-local |
| cursor | repo-managed integration lane | workspace-config lane | marketplace or local |
| cursor-workspace | repo-managed integration lane | workspace-config lane with first-class MCP config and project rules | workspace config |
| gemini | production-ready extension packaging lane | packaging, not runtime | copy install |
| opencode | repo-managed integration lane | workspace-config lane | workspace config |

For full framing, pair this matrix with [Support Boundary](/es/reference/support-boundary) and [Target Model](/es/concepts/target-model).
