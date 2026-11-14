# OpenAI Codex CLI plugins — справочник

Консолидированные заметки по официальной документации OpenAI Codex, сабагентному ресёрчу и проверке репозитория **plugin-kit-ai** (`plugin-kit-ai`, пример `codex-basic-prod`). Где спецификация тонкая или в статусе «в пути» — отмечено.

**Дата сборки:** 2026-03-28. Версии Codex CLI и сайта developers.openai.com меняются — перепроверяйте первоисточники.

---

## Официальные URL (Codex)

| Ресурс | URL |
|--------|-----|
| Plugins (обзор, каталог, CLI) | https://developers.openai.com/codex/plugins/ |
| Build plugins (структура, marketplace, манифест) | https://developers.openai.com/codex/plugins/build |
| Agent Skills | https://developers.openai.com/codex/skills |
| MCP | https://developers.openai.com/codex/mcp |
| Hooks | https://developers.openai.com/codex/hooks |
| Configuration reference | https://developers.openai.com/codex/config-reference |
| Config JSON Schema | https://developers.openai.com/codex/config-schema.json |
| CLI reference | https://developers.openai.com/codex/cli/reference |
| Agent approvals & security | https://developers.openai.com/codex/agent-approvals-security |
| Репозиторий (ориентир, не замена build-docs) | https://github.com/openai/codex |
| Каталог skills (openai/skills) | https://github.com/openai/skills |
| Стандарт Agent Skills | https://agentskills.io/specification |

---

## 1. Codex CLI plugins — что это

**Модель:** устанавливаемый **пакет** (каталог), который Codex копирует в кэш и подхватывает как источник **skills**, опционально **MCP** и **Apps** (ChatGPT-коннекторы).

| Элемент | Факт |
|--------|------|
| **Манифест** | Обязателен: `.codex-plugin/plugin.json` ([Build plugins](https://developers.openai.com/codex/plugins/build)). |
| **Что лежит в `.codex-plugin/`** | По докам — **только** `plugin.json`; остальное — **корень плагина**. |
| **Типичное содержимое корня** | `skills/`, `assets/`, `.mcp.json`, `.app.json` — пути в манифесте **относительно корня**, с префиксом `./` ([там же](https://developers.openai.com/codex/plugins/build)). |
| **Поля манифеста (пример «полного»)** | `name`, `version`, `description`, `author`, `homepage`, `repository`, `license`, `keywords`, **`skills`**, **`mcpServers`**, **`apps`**, блок **`interface`** (иконки, тексты, `defaultPrompt`, юр. ссылки и т.д.) ([Manifest fields](https://developers.openai.com/codex/plugins/build#manifest-fields)). |
| **Отдельного JSON Schema для `plugin.json` в паблике** | В нарративе + примеры; отдельная схема как артефакт — **не выделена** так же явно, как для `config.toml`. |
| **Каталог / CLI** | Обзор: [Plugins](https://developers.openai.com/codex/plugins/); в CLI — **`/plugins`**. |
| **Публикация в официальный каталог** | В доках — **«coming soon»** (self-serve / directory) ([Build plugins](https://developers.openai.com/codex/plugins/build)). |

### Skills в плагине

Каталоги под `./skills/.../SKILL.md` с front matter (`name`, `description`) — как в [ручном создании плагина](https://developers.openai.com/codex/plugins/build).

### MCP в плагине

В манифесте **`mcpServers`** → путь к файлу, обычно **`./.mcp.json`**. Рантайм MCP для Codex детально описан в **`~/.codex/config.toml`**, проектном **`.codex/config.toml`**, странице [MCP](https://developers.openai.com/codex/mcp) и команде **`codex mcp`** (в CLI reference может быть помечена как experimental).

Формат **внутреннего** `.mcp.json` плагина в публичных доках **не расписан** с той же полнотой, что отдельные «Copilot-style» mcp.json workflow — ориентир: семантика близка к **`[mcp_servers.*]`** в TOML. Unifying с VS Code mcp.json для Codex явно **не планируется** (см. обсуждения в [openai/codex](https://github.com/openai/codex)).

### Hooks

У Codex есть lifecycle hooks через **`hooks.json`** и флаг **`features.hooks`**; старый `features.codex_hooks` остаётся deprecated alias. Источник: [Hooks](https://developers.openai.com/codex/hooks), [config reference](https://developers.openai.com/codex/config-reference).

Свежая официальная структура плагина допускает lifecycle config через plugin manifest или default **`hooks/hooks.json`**. В `plugin-kit-ai` это теперь моделируется как target-native Codex package surface: `plugin/targets/codex-package/hooks/**` зеркалится в bundled `hooks/**`.

### Marketplace (свой список плагинов)

- Файлы: **`$REPO_ROOT/.agents/plugins/marketplace.json`** или **`~/.agents/plugins/marketplace.json`**
- **`source.path`**: путь с префиксом **`./`**, относительно **корня marketplace** (не обязательно относительно `.agents/plugins/`)
- Кэш установки: **`~/.codex/plugins/cache/$MARKETPLACE_NAME/$PLUGIN_NAME/$VERSION/`**; для локальных плагинов **`$VERSION`** = литерал **`local`**
- Детали: [How Codex uses marketplaces](https://developers.openai.com/codex/plugins/build#how-codex-uses-marketplaces)

Поля записи marketplace могут включать **`policy.installation`** (`AVAILABLE`, `INSTALLED_BY_DEFAULT`, `NOT_AVAILABLE`), **`policy.authentication`**, **`interface.displayName`**, категории и т.д. — см. раздел Marketplace metadata на странице Build plugins.

### Включение / отключение плагина

В **`~/.codex/config.toml`**, например:

`[plugins."plugin-id@marketplace-name"] enabled = false`

После изменений — перезапуск Codex ([Plugins](https://developers.openai.com/codex/plugins/), [Build plugins](https://developers.openai.com/codex/plugins/build)).

### Live CLI observations in this repo (`2026-04-03`)

- Real `codex exec` smoke against the repository-owned notify harness **passes** when the hook path is supplied via explicit CLI config override (`-c notify=...`).
- The checked-in production runtime example `examples/plugins/codex-basic-prod` also passes a real `codex exec` smoke when rebuilt and projected through the same explicit `-c notify=...` override path.
- The same checked-in production runtime example now also passes real `codex mcp get --json` and `codex mcp list --json` when its generated runtime MCP config is projected back into documented `-c mcp_servers...` overrides.
- The same checked-in production runtime example still does **not** prove project-local `.codex/config.toml` behavior on the current live CLI build: a separate no-override probe kept reporting model `gpt-5.4` instead of the generated `gpt-5.4-mini`, so that evidence remains vendor-gap only.
- Real `codex mcp get --json` and `codex mcp list --json` preflights **pass** when the MCP server is supplied via explicit CLI config overrides (`-c mcp_servers.release-checks...`).
- Real `codex mcp add|get|list|remove` also work noninteractively in an isolated temporary home for both documented stdio servers (`codex mcp add name -- /bin/echo ...`) and streamable HTTP servers (`codex mcp add name --url https://...`).
- The same mutable CLI config-management path also works when seeded from this repo's checked-in reference examples: `codex-basic-prod` runtime MCP config and `codex-package-prod` `.mcp.json` can both be replayed into isolated `codex mcp add|get|list|remove` checks.
- The same checked-in reference examples now also pass that mutable path inside an auth-seeded temporary `CODEX_HOME`, not only in a blank isolated home.
- A follow-up `codex exec` probe against that same isolated `CODEX_HOME` did not become a clean positive signal in the current build: the persisted MCP server remained configurable, but `exec` lost live auth and therefore stays evidence-only.
- A stronger auth-seeded temp-home probe now copies the current live Codex auth artifacts into a temporary `CODEX_HOME`; with that seed, `codex login status` plus documented stdio and HTTP `mcp add|get|list|remove` flows still work, but `codex exec` can still finish without exposing the persisted `release_checks` tool in-session. This is a stronger vendor fact than the older auth-loss probe: auth survives, tool materialization still remains session-variable.
- The same auth-seeded temporary-home evidence now also covers the documented `codex mcp login` and `codex mcp logout` subcommands on a real stdio server configuration: on the live CLI they deterministically reject stdio transports with OAuth-only diagnostics, while `get`, `list`, and `remove` continue to work on that same persisted server entry.
- The same auth-seeded temporary-home evidence now also covers missing-server behavior after removal: on the live CLI `codex mcp get <name> --json` fails with `No MCP server named '<name>' found.`, while `codex mcp remove <name>` stays a successful no-op with the same message.
- Real `codex-package` MCP sidecar coverage can also be exercised without inventing an install flow: generate `.mcp.json`, project it back into documented `-c mcp_servers...` overrides, and verify `codex mcp get --json`, `codex mcp list --json`, plus `codex exec` MCP-tool use against the live CLI.
- The same generated stdio `.mcp.json` sidecar can also be replayed through documented mutable CLI config management: `codex mcp add|get|list|remove` now passes for that sidecar in both blank isolated homes and auth-seeded temporary homes.
- A synthetic generated HTTP `.mcp.json` sidecar now also passes that same mutable CLI config-management replay: the live CLI accepts it through `codex mcp add|get|list|remove` in both blank isolated homes and auth-seeded temporary homes.
- The checked-in production example `examples/plugins/codex-package-prod` now also has live evidence through the same documented override path: real `codex mcp get --json` and `codex mcp list --json` both surface its generated remote MCP server metadata from `.mcp.json`.
- Real probes for **project-local** `.codex/config.toml` in the current Codex CLI build (`v0.117.0`) did **not** show the same behavior:
  - `codex exec` continued to report model `gpt-5.4` instead of the generated project-local `model = "gpt-5.4-mini"`, and did not invoke the generated `notify` hook path.
  - `codex -C <dir> mcp get release-checks --json` did not expose a server generated into project-local `.codex/config.toml`.
  - `codex -C <dir> mcp list --json` did not expose that same generated project-local MCP server either.
- Practical conclusion for `plugin-kit-ai`:
  - treat generated project-local `.codex/config.toml` as a **repo-owned authored/generate/validate contract**
  - treat current real-CLI project-config probes as **evidence-only**, not as a shipped vendor guarantee
  - keep positive live Codex smoke on the explicit override path until OpenAI documents and ships stronger project-config behavior for `exec` / `mcp`

### Skills вне плагина

Обнаружение в **`.agents/skills`**, **`~/.agents/skills`**, родительских каталогах, **`/etc/codex/skills`**, системных скилах — [Agent Skills](https://developers.openai.com/codex/skills).

**Нюанс:** публичные доки в основном описывают **`.agents/skills`**; bundled **`skill-installer`** документирует установку в **`$CODEX_HOME/skills`** (часто **`~/.codex/skills`**). Возможен **разрыв нарратива** между доками и установщиком — на конкретной сборке проверяйте появление скила в **`/skills`** после установки. См. также issues в [openai/codex](https://github.com/openai/codex) (symlinks, `AGENT_SKILLS_PATH`).

### Slash-команды и промпты в плагине

Отдельного ключа **`commands`** в официальном примере `plugin.json` **нет**. Включённые skills могут проявляться как slash-команды (см. раздел про команды в доках Codex app/CLI). Стартовые строки промпта для витрины — **`interface.defaultPrompt`** (массив строк).

### Enterprise / безопасность

Админские ограничения, allowlist MCP и т.д. — **`requirements.toml`**, managed configuration ([Managed configuration](https://developers.openai.com/codex/enterprise/managed-configuration)). Поведение согласуется с [Agent approvals & security](https://developers.openai.com/codex/agent-approvals-security).

### skill-installer и skill-creator

- **`$skill-installer`** — установка скиллов из каталога [openai/skills](https://github.com/openai/skills); для **шаринга** команды предпочитают **plugins**
- **`$skill-creator`** — авторинг скилла; опционально **`agents/openai.yaml`** на скип ([Agent Skills](https://developers.openai.com/codex/skills))
- Отключение скила без удаления: **`[[skills.config]]`** в `config.toml` с `path` к `SKILL.md` и `enabled = false`

---

## 2. Сравнение с другими стеками (по вендорским докам)

Детали релизов могут меняться — сверяйте первоисточники.

| Измерение | **Codex** | **Claude Code** | **Gemini CLI** | **OpenCode** | **Cursor** |
|-----------|-----------|-----------------|----------------|--------------|------------|
| **Формат пакета** | `.codex-plugin/plugin.json` + корневые `skills/`, `hooks/`, `.mcp.json`, `.app.json` | `.claude-plugin/plugin.json` + `skills/`, `agents/`, `commands/`, `hooks/`, `.mcp.json`, `.lsp.json`, `settings.json` | **`gemini-extension.json`** в корне расширения | **JS/TS plugins** + `opencode.json`; **custom tools** | **`.cursor-plugin/plugin.json`** (IDE) |
| **Skills** | `SKILL.md` в плагине и в `.agents/skills` | `SKILL.md` + `commands/`; неймспейс `/plugin:skill` | `skills/**/SKILL.md` в расширении | `SKILL.md` в `.opencode/skills/` + совместимость `.claude`/`.agents` | Skills в **плагине IDE** + Rules |
| **MCP** | Плагин → `.mcp.json`; рантайм в `config.toml`, `codex mcp` | `.mcp.json` + `${CLAUDE_PLUGIN_ROOT}` | `mcpServers` **внутри** `gemini-extension.json` | Блок `mcp` + `opencode mcp` | `.cursor/mcp.json`, маркетплейс |
| **Хуки в пакете** | **Да**, default `hooks/hooks.json` или lifecycle config через manifest | **Да**, `hooks/hooks.json` / inline | `hooks/hooks.json` в расширении (другая модель событий) | In-process хуки в JS-плагине | Hooks в плагине IDE |
| **Субагенты в пакете** | Не как у Claude в публичной спеке плагина | **`agents/`** в плагине | `agents/` (preview) + experimental в ядре | Агенты в конфиге | Agents в плагине IDE |
| **Уникально у Codex** | **Apps** (`.app.json`), Plugin Directory, **`@`**, кэш `~/.codex/plugins/cache/...` | **LSP**, **`settings.json`** + default agent, marketplaces | Один манифест, **GEMINI.md**, TOML commands, галерея | npm/Bun plugins, custom tools | Team rules, **CLI без полных plugin bundles** ([Plugins](https://cursor.com/docs/plugins)) |
| **CLI и полный пакет** | **Да** — `/plugins` | `--plugin-dir`, marketplaces | `gemini extensions install` / link | Тот же стек | Headless CLI + MCP; плагины IDE отдельно |

**Коротко:**

- **Codex** — плагин как **дистрибутив skills + MCP + apps**, маркетплейс через **`marketplace.json`**, упор на **CLI + каталог**.
- **Claude** — широкий пакет: hooks, agents, commands, LSP, MCP, marketplaces.
- **Gemini** — один JSON-манифест, MCP часто inline, команды/контекст/настройки расширения.
- **OpenCode** — программируемые плагины и custom tools.
- **Cursor** — плагины IDE; headless CLI не загружает те же бандлы целиком.

Ссылки для сравнения: [Claude Create plugins](https://docs.anthropic.com/en/docs/claude-code/plugins), [Plugins reference](https://docs.anthropic.com/en/docs/claude-code/plugins-reference), [Gemini Extensions](https://google-gemini.github.io/gemini-cli/docs/extensions/), [OpenCode plugins](https://opencode.ai/docs/plugins), [Cursor Plugins](https://cursor.com/docs/plugins).

---

## 3. Примеры репозиториев (Codex-ориентированные)

Проверяемые примеры (не исчерпывающий список):

- [useorgx/orgx-codex-plugin](https://github.com/useorgx/orgx-codex-plugin) — `.codex-plugin`, `.mcp.json`, `skills/`
- [jankrom/expo-plugin](https://github.com/jankrom/expo-plugin) — плагин под `plugins/expo/`, marketplace, много skills
- [sickn33/antigravity-awesome-skills](https://github.com/sickn33/antigravity-awesome-skills) — монорепо с `.codex-plugin`
- [openai/codex](https://github.com/openai/codex) — семпл/скилл **plugin-creator** для авторинга плагинов

---

## 4. Связь с plugin-kit-ai

Пример в этом репозитории: **`examples/plugins/codex-basic-prod/`** — `.codex-plugin/plugin.json`, `.codex/config.toml`, targets под Codex.

Рендер цели **codex** вызывает генерацию управляемого манифеста и конфига в адаптере:

- `cli/internal/platformexec/codex.go` — `Generate` пишет `.codex-plugin/plugin.json`, `.codex/config.toml`, копирует target-артефакты (`commands/`, `contexts/` и т.д. по состоянию пакета)

Общая логика полей **`skills`** → `"./skills/"` и **`mcpServers`** → `"./.mcp.json"` с эмитом **`.mcp.json`** при наличии portable MCP:

- `cli/internal/pluginmanifest/manifest.go` — функция **`renderManagedPluginArtifacts`** (около строк 1359–1388 в текущем дереве)

Импорт существующего Codex-плагина нормализует пути skills/MCP к управляемым `./skills/` и `./.mcp.json` (см. предупреждения в `codex.go` при импорте).

---

## 5. Дополнительная ось ландшафта

Для сравнения «все CLI-агенты», не только вендоры Big Tech, полезны:

- **Continue** — config-first, Hub, MCP в agent mode ([continue.dev docs](https://docs.continue.dev/))
- **Aider** — нет единого manifest bundle как у Codex/Claude; MCP через отдельные интеграции (см. релизы/PR проекта)

---

## 6. Практические рекомендации

1. **Якориться на первоисточниках и датировать снимок** — закрепить URL (Codex Build/Plugins/MCP/Hooks, сравниваемые продукты) и перепроверять при мажорных обновлениях.  
   - **Уверенность:** 9/10  
   - **Надёжность:** 9/10  

2. **Фикстурный smoke-репозиторий** — минимальный плагин на каждую цель + CI на «файлы на месте / JSON валиден» (без полной семантики рантайма).  
   - **Уверенность:** 7/10  
   - **Надёжность:** 8/10  

---

## 7. См. также в этом репозитории

- [Claude Code plugins — справочник](../claude-code-plugins/README.md)
- [Gemini CLI extensions — справочник](../gemini-cli-extensions/README.md)

---

## Лицензия заметок

Внутренний research-документ репозитория plugin-kit-ai. Описание продуктов OpenAI и третьих сторон основано на публичных источниках на дату сборки.
