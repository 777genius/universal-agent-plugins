---
name: opencode-basic
description: Use this example skill to verify OpenCode workspace-config generating.
execution_mode: docs_only
supported_agents:
  - claude
  - codex
allowed_tools: []
compatibility: {}
---

## What it does

Keep OpenCode config generating deterministic and validate after every authored change.

## When to use

Use this skill when you need to check whether the authored OpenCode package still renders and validates cleanly.

## How to run

1. Run `plugin-kit-ai generate --check .`
2. Run `plugin-kit-ai validate . --platform opencode --strict`
3. Fix any drift or validation failures before continuing

## Constraints

- Keep the OpenCode config deterministic.
- Do not add unsupported local plugin-code surfaces to this example.
