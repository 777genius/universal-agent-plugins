---
title: "Поддержка target’ов"
description: "Сводка по поддержке target’ов"
canonicalId: "page:reference:target-support"
surface: "reference"
section: "reference"
locale: "ru"
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

# Поддержка target’ов

Используйте эту страницу, когда нужен компактный lane map по runtime, package, extension и repo-managed integration outputs.

| Цель | Класс production | Runtime-контракт | Модель установки |
| --- | --- | --- | --- |
| claude | production-ready package+runtime lane | стабильный поднабор runtime | marketplace или локально |
| codex-package | рекомендуемый package lane | только официальный пакет | marketplace или локально |
| codex-runtime | рекомендуемый runtime lane | стабильный notify-runtime | локально в репозитории |
| cursor | repo-managed integration lane | workspace-config вариант | marketplace или локально |
| cursor-workspace | repo-managed integration lane | workspace-config lane with first-class MCP config and project rules | конфигурация workspace |
| gemini | production-ready extension packaging lane | упаковка, не runtime | установка копированием |
| opencode | repo-managed integration lane | workspace-config вариант | конфигурация workspace |

Для полной картины свяжите эту матрицу с [Границей поддержки](/ru/reference/support-boundary) и [Моделью target’ов](/ru/concepts/target-model).
