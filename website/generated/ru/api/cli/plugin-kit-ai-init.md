---
title: "plugin-kit-ai init"
description: "Создаёт каркас проекта plugin-kit-ai."
canonicalId: "command:plugin-kit-ai:init"
surface: "cli"
section: "api"
locale: "ru"
generated: true
editLink: false
stability: "historical"
maturity: "historical"
sourceRef: "cli:plugin-kit-ai init"
translationRequired: false
historicalVersion: "1.2.4"
sourceSHA: "9beca10448ac50fbe526a52101d1433a12471980"
status: "historical"
---

> Historical plugin-kit-ai v1, baseline **1.2.4** ([exact source](https://github.com/777genius/universal-agent-plugins/tree/9beca10448ac50fbe526a52101d1433a12471980)). Project migration is not available in v2 yet. Maintain legacy projects using the v1 1.2.4 command set.

<DocMetaCard surface="cli" stability="historical" maturity="historical" source-ref="cli:plugin-kit-ai init" source-href="https://github.com/777genius/universal-agent-plugins/tree/9beca10448ac50fbe526a52101d1433a12471980/cli/plugin-kit-ai" />

# plugin-kit-ai init

Сгенерировано из реального Cobra command tree.

Создаёт каркас проекта plugin-kit-ai.

## plugin-kit-ai init

Создаёт каркас проекта plugin-kit-ai.

### Описание

Создаёт каркас plugin-kit-ai проекта в package-standard формате.

Start with the job you want to solve:

Connect an online service:
  Use --template online-service for hosted integrations like Notion, Stripe, Cloudflare, or Vercel.
  This starter creates an MCP-first repo with shared authored source under plugin/ and no launcher code.

Connect a local tool:
  Use --template local-tool for local MCP-backed tools like Docker Hub, Chrome DevTools, or HubSpot Developer.
  This starter creates an MCP-first repo with local command wiring under plugin/ and no launcher code.

Build custom plugin logic - Advanced:
  Use --template custom-logic when you need launcher-backed code, hooks, or your own runtime behavior.
  This path is more powerful and more engineering-heavy than the first two starters.
  Plain init remains as a legacy compatibility path for the older codex-runtime plus Go starter.

Уже есть нативная конфигурация:
  Используйте `plugin-kit-ai import`, чтобы привести текущие нативные файлы Claude/Codex/Gemini/OpenCode/Cursor к authored layout package-standard проекта.
  `init` нужен для создания нового package-standard проекта, а не для сохранения нативных файлов как основного authored source of truth.

Публичные флаги:
  --template   Recommended start: "online-service", "local-tool", or "custom-logic".
  --platform   Advanced override: "codex-runtime" (default), "codex-package", "claude", "gemini", "opencode", "cursor", or "cursor-workspace".
  --runtime    Поддерживаются: `go` (по умолчанию), `python`, `node`, `shell`; `shell` доступен только для launcher-based targets.
  --typescript Генерирует TypeScript scaffold поверх node runtime lane (требует `--runtime node`).
  --runtime-package
               For --runtime python or --runtime node, import the shared plugin-kit-ai-runtime package instead of vendoring the helper file into plugin/.
  --runtime-package-version
               Фиксирует версию зависимости `plugin-kit-ai-runtime`. Обязательно для development build; выпущенные CLI по умолчанию используют собственный stable tag.
  -o, --output Целевой каталог (по умолчанию: `./&lt;project-name&gt;`).
  -f, --force  Разрешает запись в непустой каталог и перезапись сгенерированных файлов.
  --extras     Дополнительно генерирует optional release helpers, например `Makefile`, `.goreleaser.yml`, переносимые `skills/` и scaffold для stable Python/Node bundle-release workflow там, где это поддерживается.
  --claude-extended-hooks
               Для `--platform claude` генерирует полный runtime-поддерживаемый набор hooks вместо стабильного поднабора по умолчанию.

```
plugin-kit-ai init [project-name] [flags]
```

### Опции

```
      --claude-extended-hooks            для `--platform claude` генерирует полный runtime-поддерживаемый набор hooks вместо стабильного поднабора по умолчанию
      --extras                           включает optional scaffold-файлы (runtime-зависимые extras, а также skills и команды)
  -f, --force                            перезаписывает сгенерированные файлы и разрешает непустой output-каталог
  -h, --help                             справка по init
  -o, --output string                    output directory (default: ./&lt;project-name&gt;)
      --platform string                  target lane ("codex-runtime", "codex-package", "claude", "gemini", "opencode", "cursor", or "cursor-workspace") (default "codex-runtime")
      --runtime string                   runtime (`go`, `python`, `node` или `shell`) (по умолчанию `go`)
      --runtime-package                  для `--runtime python` или `--runtime node` импортирует общий пакет `plugin-kit-ai-runtime` вместо вендоринга helper-файла
      --runtime-package-version string   фиксирует версию сгенерированной зависимости `plugin-kit-ai-runtime`
      --template string                  recommended start ("online-service", "local-tool", or "custom-logic")
      --typescript                       генерирует TypeScript scaffold поверх node runtime lane
```

### См. также

* plugin-kit-ai	 - CLI plugin-kit-ai для создания проектов и служебных операций вокруг AI-плагинов.
