# Claude Code plugins — справочник

**Installer slot (2026-09-18):** Agent Plugins Claude uses the official in-place `@skills-dir` plugin slot, not marketplace cache. Spike notes (not public qualification): [SPIKE-2026-09-18-install-slots.md](SPIKE-2026-09-18-install-slots.md). Decision: [ADR 0007](../../adr/0007-agentplugins-claude-skills-dir-plugin-slot.md).

Консолидированные заметки по официальной документации Anthropic (Claude Code), **10 параллельным сабагентам** (generalPurpose) и контексту репозитория **plugin-kit-ai** (`plugin-kit-ai`, пример `claude-basic-prod`). Где в доках есть пробелы или известные баги релиза — отмечено.

**Дата сборки:** 2026-03-28. Доки дублируются на **docs.anthropic.com** и **code.claude.com** — при расхождении ориентируйтесь на актуальную версию страницы и `claude --version`.

---

## Официальные URL

| Ресурс | URL (Anthropic) | Зеркало |
|--------|-----------------|--------|
| Create plugins | https://docs.anthropic.com/en/docs/claude-code/plugins | https://code.claude.com/docs/en/plugins |
| Plugins reference | https://docs.anthropic.com/en/docs/claude-code/plugins-reference | https://code.claude.com/docs/en/plugins-reference |
| Discover and install plugins | https://docs.anthropic.com/en/docs/claude-code/discover-plugins | https://code.claude.com/docs/en/discover-plugins |
| Plugin marketplaces | https://docs.anthropic.com/en/docs/claude-code/plugin-marketplaces | https://code.claude.com/docs/en/plugin-marketplaces |
| Agent Skills | https://docs.anthropic.com/en/docs/claude-code/skills | https://code.claude.com/docs/en/skills |
| Subagents | https://docs.anthropic.com/en/docs/claude-code/sub-agents | (см. code.claude.com/en/sub-agents) |
| Hooks | https://docs.anthropic.com/en/docs/claude-code/hooks | https://code.claude.com/docs/en/hooks |
| MCP | https://docs.anthropic.com/en/docs/claude-code/mcp | https://code.claude.com/docs/en/mcp |
| Settings | https://docs.anthropic.com/en/docs/claude-code/settings | https://code.claude.com/docs/en/settings |
| Claude Code — Security | https://docs.anthropic.com/en/docs/claude-code/security | — |
| Индекс для инструментов | https://code.claude.com/docs/llms.txt | — |
| Веб-каталог плагинов | https://claude.com/plugins | — |
| Submit plugin (Claude.ai) | https://claude.ai/settings/plugins/submit | — |
| Submit plugin (Console) | https://platform.claude.com/plugins/submit | — |
| Официальные плагины (GitHub) | https://github.com/anthropics/claude-plugins-official | — |

Короткие алиасы в доках (например `/en/plugins-reference`) ведут на тот же контент, что и полные пути `.../claude-code/...`.

---

## 1. Что такое плагин Claude Code

**Пакет** с манифестом **`.claude-plugin/plugin.json`** и компонентами в **корне каталога плагина** (не внутри `.claude-plugin/`). Предназначен для команд, переиспользования, маркетплейсов и версионирования — в отличие от «голого» дерева **`.claude/`** в одном репозитории ([When to use plugins](https://docs.anthropic.com/en/docs/claude-code/plugins)).

### Манифест

| Факт | Источник |
|------|----------|
| Путь: **`{plugin-root}/.claude-plugin/plugin.json`** | [Create plugins](https://docs.anthropic.com/en/docs/claude-code/plugins) |
| В **`.claude-plugin/`** допустим **только** `plugin.json`; класть туда `skills/`, `commands/` и т.д. — типичная ошибка | [Plugin structure overview](https://docs.anthropic.com/en/docs/claude-code/plugins#plugin-structure-overview) |
| Манифест **может отсутствовать**: компоненты ищутся по стандартным путям, имя плагина берётся из **имени каталога** | [Plugin manifest schema](https://docs.anthropic.com/en/docs/claude-code/plugins-reference#plugin-manifest-schema) |
| Если `plugin.json` есть, **обязательно поле `name`** (kebab-case, уникальный id, неймспейс для slash-команд) | [Required fields](https://docs.anthropic.com/en/docs/claude-code/plugins-reference#required-fields) |
| Для публикации обычно добавляют `description`, `version`, опционально `author` | [Create your first plugin](https://docs.anthropic.com/en/docs/claude-code/plugins#create-your-first-plugin) |

### Структура корня плагина (типично)

См. таблицы [Create plugins](https://docs.anthropic.com/en/docs/claude-code/plugins) и [Plugin directory structure](https://docs.anthropic.com/en/docs/claude-code/plugins-reference#plugin-directory-structure):

- **`skills/`** — каталоги с `SKILL.md`
- **`commands/`** — по одному markdown-файлу на slash-команду
- **`agents/`** — markdown-файлы субагентов
- **`hooks/`** — в т.ч. `hooks/hooks.json`
- **`.mcp.json`**, **`.lsp.json`**
- **`settings.json`** — дефолты при включении плагина (см. раздел ниже)
- **`output-styles/`** и др. по reference

### Path behavior rules

У **hooks / MCP / LSP** семантика нескольких источников (merge vs replace) **отличается** от `commands` / `agents` / `skills`, где кастомные пути в манифесте могут **заменять** сканирование по умолчанию. Детали: [Component path fields](https://docs.anthropic.com/en/docs/claude-code/plugins-reference#component-path-fields), [Path behavior rules](https://docs.anthropic.com/en/docs/claude-code/plugins-reference#path-behavior-rules).

### Переменные окружения для путей

- **`${CLAUDE_PLUGIN_ROOT}`** — абсолютный путь к **установленному** каталогу плагина; подстановка в skills, агентах, хуках, MCP/LSP; также env для дочерних процессов
- **`${CLAUDE_PLUGIN_DATA}`** — персистентные данные под `~/.claude/plugins/data/{id}/` (обновления плагина)

Источник: [Environment variables](https://docs.anthropic.com/en/docs/claude-code/plugins-reference#environment-variables).

### Локальная разработка

- **`claude --plugin-dir ./plugin-root`** — загрузка плагина на сессию
- После правок: **`/reload-plugins`**

Источник: [Test your plugins locally](https://docs.anthropic.com/en/docs/claude-code/plugins#test-your-plugins-locally).

### Отладка

В reference: **`claude plugin validate`**, **`/plugin validate`**, **`claude --debug`** для ошибок MCP — [Debugging and development tools](https://docs.anthropic.com/en/docs/claude-code/plugins-reference#debugging-and-development-tools).

---

## 2. Skills и commands

| Тема | Факт |
|------|------|
| **Skills** | Каталог `skills/<id>/SKILL.md`, YAML front matter + тело; опционально `scripts/`, ссылки и т.д. | [Plugins reference — Skills](https://docs.anthropic.com/en/docs/claude-code/plugins-reference), [Skills](https://docs.anthropic.com/en/docs/claude-code/skills) |
| **Commands** | Файлы **`.md`** в `commands/` — «простые» slash-команды без обязательной структуры папки skill | Там же |
| **Неймспейс** | **`/<plugin-name>:<skill-or-command>`**, где `plugin-name` = поле **`name` в `plugin.json`** (не литерал `plugin:`) | [Create plugins](https://docs.anthropic.com/en/docs/claude-code/plugins) |
| **Проект vs плагин** | В проекте: короткие имена `/hello`; в плагине — с префиксом плагина, чтобы не конфликтовать | [When to use plugins](https://docs.anthropic.com/en/docs/claude-code/plugins) |
| **Коллизия skill vs command** | Если имя совпадает, **побеждает skill** | [Skills](https://docs.anthropic.com/en/docs/claude-code/skills) |

---

## 3. Agents (`agents/`)

- Один агент = один **`.md`** с YAML front matter + системный промпт в теле
- Задокументированные поля включают: `name`, `description`, `model`, `effort`, `maxTurns`, `tools`, `disallowedTools`, `skills`, `memory`, `background`, `isolation` (допустимо **`"worktree"`**)
- **Запрещено** в агентах **внутри плагина** (безопасность): **`hooks`**, **`mcpServers`**, **`permissionMode`**

Источник: [Plugins reference — Agents](https://docs.anthropic.com/en/docs/claude-code/plugins-reference#agents).

Поведение в UI: **`/agents`**, авто- или ручной вызов; детали модели — [Subagents](https://docs.anthropic.com/en/docs/claude-code/sub-agents).

Переопределение путей: поле **`agents`** в `plugin.json` может указывать кастомные файлы/каталоги — см. раздел манифеста в reference.

---

## 4. MCP и LSP

### MCP

- Файл **`.mcp.json`** в **корне плагина** **или** поле **`mcpServers`** в `plugin.json`: путь(и), массив путей или **inline**-объект (схема MCP)
- При включении плагина серверы стартуют как отдельный слой от user/project MCP
- Пути к бинарям в бандле — через **`${CLAUDE_PLUGIN_ROOT}`**

Источники: [Plugins reference — MCP servers](https://docs.anthropic.com/en/docs/claude-code/plugins-reference#mcp-servers), [MCP — Plugin-provided MCP servers](https://docs.anthropic.com/en/docs/claude-code/mcp#plugin-provided-mcp-servers).

### LSP

- **`.lsp.json`** в корне **или** **`lspServers`** в `plugin.json` (path / array / inline)
- Плагин задаёт **подключение**; бинарь LSP должен быть доступен (например в `PATH`)

Источники: [Plugins reference — LSP servers](https://docs.anthropic.com/en/docs/claude-code/plugins-reference#lsp-servers), [Create plugins — Add LSP servers](https://docs.anthropic.com/en/docs/claude-code/plugins#add-lsp-servers-to-your-plugin).

---

## 5. Hooks

- По умолчанию: **`hooks/hooks.json`** в корне плагина
- В **`plugin.json`**: поле **`hooks`** — путь, массив путей или **inline**-конфиг (та же форма, что у пользовательских hooks)
- События те же, что у [Hooks reference](https://docs.anthropic.com/en/docs/claude-code/hooks): `SessionStart`, `UserPromptSubmit`, `PreToolUse`, `PostToolUse`, `SubagentStart` / `SubagentStop`, `PreCompact` / `PostCompact`, `WorktreeCreate` / `WorktreeRemove`, `Elicitation`, …
- Типы обработчиков: **`command`**, **`http`**, **`prompt`**, **`agent`** — [Hook handler fields](https://docs.anthropic.com/en/docs/claude-code/hooks#hook-handler-fields)

Связь с «глобальными» hooks: одна система, плагин — отдельная область видимости; расположения перечислены в [Hook locations](https://docs.anthropic.com/en/docs/claude-code/hooks#hook-locations).

**Сообщество:** возможны регрессии загрузки command-hooks из `hooks/hooks.json` для отдельных событий — см. [anthropics/claude-code#34573](https://github.com/anthropics/claude-code/issues/34573); при сбое проверяйте версию CLI и обходные пути (inline в манифесте, другой тип handler, временно project `settings`).

---

## 6. `settings.json` в корне плагина

| Факт | Источник |
|------|----------|
| Расположение: **корень плагина**, не `.claude-plugin/` | [Create plugins](https://docs.anthropic.com/en/docs/claude-code/plugins) |
| Назначение: дефолты **при включении** плагина | Там же |
| Явно задокументированный ключ: **`agent`** — имя агента из `agents/` как **основной поток** | [Ship default settings](https://docs.anthropic.com/en/docs/claude-code/plugins#ship-default-settings-with-your-plugin) |
| Неизвестные ключи **игнорируются** | Там же |
| Приоритет выше, чем у **`settings` внутри `plugin.json`**, если заданы оба | Там же |

### `userConfig` в манифесте (отдельно от `settings.json`)

Значения, которые пользователь вводит при включении; подстановка **`${user_config.KEY}`** в MCP/LSP/hooks/skills/agents; несекретные опции попадают в пользовательский settings, секреты — keychain / credentials. Подробно: [User configuration](https://docs.anthropic.com/en/docs/claude-code/plugins-reference#user-configuration).

### Несостыковка в доках

В блоке **Complete schema** для `plugin.json` пример может не перечислять поле `settings`, тогда как Create plugins описывает `settings` в манифесте и приоритет над ним у файла `settings.json` — сверяйте обе страницы и поведение на вашей версии.

---

## 7. Маркетплейсы и установка

### Файл каталога

- В git-репозитории маркетплейса: **`.claude-plugin/marketplace.json`** в **корне репо**
- Можно добавить маркетплейс по **локальному пути** или **HTTPS URL** прямо на JSON

Источник: [Plugin marketplaces](https://docs.anthropic.com/en/docs/claude-code/plugin-marketplaces), [Discover plugins](https://docs.anthropic.com/en/docs/claude-code/discover-plugins).

### Команды (типично)

| Действие | Команда |
|----------|---------|
| Добавить маркетплейс | `/plugin marketplace add …` (сокр. `market add`) |
| Список | `/plugin marketplace list` |
| Обновить каталог | `/plugin marketplace update <name>` |
| Удалить | `/plugin marketplace remove <name>` |
| Установить плагин | `/plugin install <plugin>@<marketplace>` |
| Перезагрузить | `/reload-plugins` |

Источник: [Discover plugins](https://docs.anthropic.com/en/docs/claude-code/discover-plugins).

Источники git: `owner/repo`, полный HTTPS/SSH URL с **`#ref`**, локальная папка с `.claude-plugin/marketplace.json`. Демо: добавление **`anthropics/claude-code`** — в доке discover-plugins.

### Официальный каталог

- Репозиторий **[anthropics/claude-plugins-official](https://github.com/anthropics/claude-plugins-official)**: каталоги **`plugins/`** (внутренние) и **`external_plugins/`** (партнёры/комьюнити)
- Установка: **`/plugin install <name>@claude-plugins-official`** или Discover; каталог на [claude.com/plugins](https://claude.com/plugins)
- Внешние плагины: форма в README репо (в т.ч. [clau.de/plugin-directory-submission](https://clau.de/plugin-directory-submission))

### Scopes установки

**user** (по умолчанию), **project**, **local**; managed-плагины могут быть заданы организацией. См. [Discover plugins](https://docs.anthropic.com/en/docs/claude-code/discover-plugins), [Settings — scopes](https://docs.anthropic.com/en/docs/claude-code/settings).

### Кэш и разрешение путей

Плагины из маркетплейса **копируются** в **`~/.claude/plugins/cache`** (безопасность и верификация); после установки запрещены пути «наружу» через `../`; symlinks учитываются на этапе копирования. См. [Plugin caching and file resolution](https://docs.anthropic.com/en/docs/claude-code/plugins-reference#plugin-caching-and-file-resolution).

**Сессия без установки:** `claude --plugin-dir` — только на текущую сессию.

### Ограничения для организаций

Админы могут ограничить список маркетплейсов (**`strictKnownMarketplaces`**, **`extraKnownMarketplaces`**). См. [Managed marketplace restrictions](https://docs.anthropic.com/en/docs/claude-code/plugin-marketplaces#managed-marketplace-restrictions).

---

## 8. Безопасность и доверие

- Плагины и маркетплейсы — **высокий уровень доверия**; код выполняется с привилегиями пользователя; добавляйте только **проверенные** источники — [Discover plugins — Security](https://docs.anthropic.com/en/docs/claude-code/discover-plugins#security)
- Для сторонних плагинов Anthropic **не контролирует** содержимое MCP, файлов и ПО внутри пакета — проверяйте homepage автора — тот же раздел discover-plugins
- **Официальный** маркетплейс поддерживается Anthropic; публично детализированный «security review pipeline» для каждого внешнего плагина в выжимке доков не фигурирует
- Общая модель безопасности Claude Code: [Security](https://docs.anthropic.com/en/docs/claude-code/security); Trust Center — организационный уровень, не отдельная глава «только про плагины»
- MCP: доверие к серверу или self-hosted — [MCP security](https://docs.anthropic.com/en/docs/claude-code/security#mcp-security)

---

## 9. Сравнение с Codex и Gemini CLI (по вендорским докам)

| | **Claude Code plugins** | **OpenAI Codex plugins** | **Gemini CLI extensions** |
|---|-------------------------|---------------------------|----------------------------|
| **Манифест** | `.claude-plugin/plugin.json` (опционален при автообнаружении) | `.codex-plugin/plugin.json` обязателен | `gemini-extension.json` в корне |
| **Hooks в пакете** | Да (`hooks/hooks.json` или inline) | Нет в структуре плагина; отдельный слой `hooks.json` + флаг | `hooks/hooks.json` у расширения |
| **Agents** | `agents/*.md` в плагине | Не в пакете плагина в build-docs | Preview в extension |
| **LSP** | `.lsp.json` / `lspServers` | Не в модели плагина в build-docs | Не в extension reference как у Claude |
| **Marketplace** | `.claude-plugin/marketplace.json` в корне репо маркетплейса | `.agents/plugins/marketplace.json` | Галерея + `gemini extensions install` |
| **Dev workflow** | `claude --plugin-dir`; кэш маркетплейса `~/.claude/plugins/cache` | marketplace + `/plugins`; кэш `~/.codex/plugins/cache/...` | `gemini extensions link` |

Первоисточники: [Plugins reference](https://docs.anthropic.com/en/docs/claude-code/plugins-reference), [Codex Build plugins](https://developers.openai.com/codex/plugins/build), [Gemini Extensions](https://www.geminicli.com/docs/extensions/).

Полнее матрицы по всем целям: [Codex — справочник](../codex-cli-plugins/README.md), [Gemini — справочник](../gemini-cli-extensions/README.md).

---

## 10. Примеры репозиториев

- **Официально:** [anthropics/claude-plugins-official](https://github.com/anthropics/claude-plugins-official) — множество плагинов под `plugins/*` и `external_plugins/*`; у части external есть **`.mcp.json`** (например Supabase)
- **Комьюнити (манифест + skills/MCP встречаются):** [wshobson/agents](https://github.com/wshobson/agents), [cased/claude-code-plugins](https://github.com/cased/claude-code-plugins), [obra/superpowers](https://github.com/obra/superpowers), [futuregerald/futuregerald-claude-plugin](https://github.com/futuregerald/futuregerald-claude-plugin), [cbrake/claude-plugins](https://github.com/cbrake/claude-plugins), [hesreallyhim/claude-code-json-schema](https://github.com/hesreallyhim/claude-code-json-schema), [yu-iskw/claude-plugin-builder](https://github.com/yu-iskw/claude-plugin-builder)

Список не исчерпывающий; перед доверием проверяйте содержимое репо.

---

## 11. Связь с plugin-kit-ai

- Пример пакета: **`examples/plugins/claude-basic-prod/.claude-plugin/plugin.json`**
- Рендер цели **claude**: `cli/plugin-kit-ai/internal/platformexec/claude.go` — `Generate` вызывает **`renderManagedPluginArtifacts`** для `.claude-plugin/plugin.json`, опционально копирует hooks/commands/agents из `targets/claude/`

---

## 12. Практические рекомендации

1. **Структура каталогов:** всё кроме `plugin.json` — в **корне** плагина; типичные промахи — вложение компонентов в `.claude-plugin/`. **Уверенность 9/10**, **надёжность 9/10** (прямо в Create plugins).

2. **Быстро меняющийся релиз:** закрепить набор URL + `claude --version` + smoke `claude plugin validate` / минимальный `--plugin-dir`. **Уверенность 8/10**, **надёжность 8/10**.

---

## См. также

- [OpenAI Codex CLI plugins](../codex-cli-plugins/README.md)
- [Gemini CLI extensions](../gemini-cli-extensions/README.md)

---

## Лицензия заметок

Внутренний research-документ репозитория plugin-kit-ai. Описание продуктов Anthropic и третьих сторон — по публичным источникам на дату сборки.
