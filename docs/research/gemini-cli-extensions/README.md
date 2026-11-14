# Gemini CLI extensions — справочник

Консолидированные заметки по официальной документации [google-gemini/gemini-cli](https://github.com/google-gemini/gemini-cli) и смежным источникам. Продукт называет расширения **extensions** (не «plugins»): каталог с **`gemini-extension.json`**, установка через команду **`gemini extensions`**.

**Дата сборки:** 2026-03-28. Версии CLI меняются быстро; в тексте сайта встречается привязка к конкретным релизам (например v0.4.0) — сверяйте с актуальной веткой доков.

---

## Официальные URL

| Ресурс | URL |
|--------|-----|
| Репозиторий | https://github.com/google-gemini/gemini-cli |
| Обзор расширений (сайт) | https://google-gemini.github.io/gemini-cli/docs/extensions/ |
| Обзор (зеркало geminicli.com) | https://www.geminicli.com/docs/extensions/ |
| Extension reference | https://geminicli.com/docs/extensions/reference/ |
| Writing / build extensions | https://geminicli.com/docs/extensions/writing-extensions |
| Getting started (extensions) | https://google-gemini.github.io/gemini-cli/docs/extensions/getting-started-extensions.html |
| Releasing / галерея | https://github.com/google-gemini/gemini-cli/blob/main/docs/extensions/releasing.md |
| Extension releasing (сайт) | https://google-gemini.github.io/gemini-cli/docs/extensions/extension-releasing.html |
| Agent Skills | https://geminicli.com/docs/cli/skills |
| Creating skills | https://geminicli.com/docs/cli/creating-skills/ |
| Custom commands | https://www.geminicli.com/docs/cli/custom-commands |
| Configuration (в т.ч. MCP) | https://google-gemini.github.io/gemini-cli/docs/get-started/configuration.html |
| Hooks | https://www.geminicli.com/docs/hooks/ |
| Subagents | https://www.geminicli.com/docs/core/subagents/ |
| Remote subagents | https://www.geminicli.com/docs/core/remote-agents |
| Extension best practices | https://www.geminicli.com/docs/extensions/best-practices |
| Галерея | https://geminicli.com/extensions/browse/ |
| О расширениях (маркетинг) | https://www.geminicli.com/extensions/about |
| Стандарт Agent Skills | https://agentskills.io/ |
| Исходники доков в репо | `docs/extensions/index.md`, `reference.md`, `writing-extensions.md` |

**Посты Google (контекст UX):**

- [Introducing Gemini CLI extensions](https://blog.google/innovation-and-ai/technology/developers-tools/gemini-cli-extensions/) (Oct 2025)
- [Making Gemini CLI extensions easier to use](https://developers.googleblog.com/making-gemini-cli-extensions-easier-to-use/) (Feb 2026, settings / keychain / scopes)

---

## Манифест `gemini-extension.json`

Расширение — директория с **`gemini-extension.json` в корне**. После установки копия (или symlink при `link`) живёт под **`~/.gemini/extensions`**.

### Поля манифеста (по reference)

| Поле | Назначение |
|------|------------|
| `name` | Уникальный id: строчные буквы, цифры, дефисы; ожидается согласованность с именем каталога расширения |
| `version` | Версия расширения |
| `description` | Краткое описание; может отображаться в галерее |
| redirect URL field | URL нового репозитория; CLI может перенаправить установку при обновлениях |
| `mcpServers` | Карта MCP-серверов; формат как в `settings.json`, **кроме опции `trust`** (в манифесте расширения недоступна) |
| `contextFileName` | Имя файла контекста в каталоге расширения; если не задано и есть **`GEMINI.md`**, он подхватывается |
| `excludeTools` | Список **встроенных** инструментов CLI для исключения (например паттерны вроде `run_shell_command(rm -rf)`). **Не то же самое**, что `excludeTools` у конкретного MCP-сервера |
| `plan` | Настройки планирования: **`plan.directory`** — каталог артефактов планов (fallback, если у пользователя не задано) |
| `settings` | Массив настроек при установке: `name`, `description`, `envVar`, `sensitive` (при `true` — keychain и маскировка в UI); значения попадают в **`.env` в каталоге расширения** |
| `themes` | Массив кастомных тем TUI; в UI имя дополняется именем расширения в скобках |

### Подстановки переменных

Используются в **`gemini-extension.json`** и **`hooks/hooks.json`**:

- **`${extensionPath}`** — абсолютный путь к каталогу расширения (симлинки могут не раскрываться — уточнять в доке версии)
- **`${workspacePath}`** — абсолютный путь текущего workspace
- **`${/}`** или **`pathSeparator`** — разделитель путей ОС

Реализация: `packages/cli/src/config/extensions/variables.ts` в репозитории gemini-cli.

---

## Структура каталога расширения

Помимо манифеста в корне:

| Путь | Назначение |
|------|------------|
| `commands/**/*.toml` | Slash-команды; вложенность задаёт имя (`deploy.toml` → `/deploy`, `gcs/sync.toml` → `/gcs:sync`) |
| `skills/<skill-id>/SKILL.md` | Agent Skills; автообнаружение при включённом расширении |
| `hooks/hooks.json` | Хуки жизненного цикла (не внутри `gemini-extension.json`) |
| `agents/*.md` | Субагенты расширения — в reference помечены как **preview / under active development** |
| `policies/*.toml` | Политики (policy engine); у расширений есть ограничения: нельзя тихо обходить safeguards |
| Контекст | Обычно **`GEMINI.md`** или файл из `contextFileName` — подмешивается в контекст сессии |

Шаблоны CLI: `packages/cli/src/commands/extensions/examples` в репозитории.

---

## Команды CLI для расширений

Из **Extension reference** и обзора:

- **`gemini extensions install <source>`** — GitHub URL или локальный путь; для Git нужен `git`
- Флаги (часть): `--ref`, `--auto-update`, `--pre-release`, `--consent`, `--skip-settings`
- **`gemini extensions link <path>`** — symlink для разработки без постоянного `update`
- **`gemini extensions update`** — подтянуть изменения с источника (для копии, не link)
- **`gemini extensions enable` / `disable`** — с опцией **`--scope user|workspace`**
- **`gemini extensions uninstall`**, **`gemini extensions config`**
- **`gemini extensions new <path> [template]`** — шаблоны: `mcp-server`, `context`, `custom-commands`, `exclude-tools`, …

Ограничение: **`gemini extensions install` не работает из интерактивного режима CLI**; **`/extensions list`** в сессии доступен; после изменений расширений часто нужен **перезапуск сессии**.

---

## Agent Skills в расширениях

- Формат: **`skills/<id>/SKILL.md`** с YAML front matter: **`name`**, **`description`**; тело — процедурные инструкции
- Опционально: `scripts/`, `references/`, `assets/` (рекомендации в creating-skills)
- Работа: progressive disclosure → метаданные в системном промпте → модель вызывает **`activate_skill`** → подтверждение в UI → загрузка тела и контекста каталога
- Уровни обнаружения: **Workspace** (`.gemini/skills/` или `.agents/skills/`) → **User** (`~/.gemini/skills/` или `~/.agents/skills/`) → **skills внутри extensions**
- При конфликте имён: **Workspace > User > Extension**; на одном уровне **`.agents/skills/`** приоритетнее **`.gemini/skills/`**
- Документация связывает формат со стандартом [agentskills.io](https://agentskills.io/)

---

## MCP в расширениях

- **`mcpServers`** задаётся **внутри `gemini-extension.json`** (не отдельным файлом, в отличие от Codex `.mcp.json`)
- Семантика полей — как в [Configuration → mcpServers](https://google-gemini.github.io/gemini-cli/docs/get-started/configuration.html):
  - **Stdio:** `command`, `args`, `env`, `cwd`
  - **SSE:** `url`
  - **Streamable HTTP:** `httpUrl`
  - Приоритет при нескольких транспортах: в доке указана цепочка (httpUrl → url → command)
- **Конфликт имён** с `settings.json`: побеждает конфигурация из **`settings.json`**
- Инструменты MCP именуются квалифицированно (`mcp_<serverAlias>_...`); в доке предупреждение про **`_` в алиасах серверов**
- **Два смысла `excludeTools`:** топ-уровень в манифесте — встроенные tools CLI; внутри записи сервера — фильтр инструментов MCP
- Слияние конфигов MCP (детали): см. `packages/core/src/tools/mcp-client-manager.ts` (`mergeMcpConfigs`, пересечение `includeTools`, объединение `excludeTools`)

---

## Хуки, субагенты, Plan Mode, темы

### Hooks

- Конфигурация: **`hooks/hooks.json`** в расширении; события и поля — [Hooks](https://www.geminicli.com/docs/hooks/), [Writing hooks](https://www.geminicli.com/docs/hooks/writing-hooks), [Hooks reference](https://www.geminicli.com/docs/hooks/reference)
- Слияние слоёв: project → user → system → **extensions** (низший приоритет у расширений)
- Модель: внешние команды, JSON на stdin/stdout

### Subagents

- Общие субагенты CLI: страница помечена как **experimental**; `browser_agent` — experimental/preview
- Remote A2A: **Remote Subagents (experimental)**
- Субагенты **внутри extension**: в Extension reference — **preview, under active development**

### Plan

- Пользователь: `general.plan.directory` в settings; Plan Mode связан с experimental-настройками
- Расширение: **`plan.directory`** в манифесте как fallback

### Themes

- Массив **`themes`** в `gemini-extension.json`; выбор через `/theme` или `ui.theme` в settings

---

## Дистрибуция, галерея, версии

- Основные источники установки: **URL репозитория GitHub** и **локальный путь**
- Галерея обнаружения: https://geminicli.com/extensions/browse/
- Для индексации в экосистеме: публичный репо, топик **`gemini-cli-extension`**, валидный `gemini-extension.json`, краулер (подробности в releasing)
- **Git install:** в releasing указано, что при установке из git **HEAD может трактоваться как последняя версия** независимо от поля `version` в JSON — важно для воспроизводимости; для фиксации использовать **`--ref`** на тег/коммит
- **GitHub Releases:** обновления смотрят на релиз, помеченный Latest; возможны нюансы с pre-release (сверять с версией CLI и флагом `--pre-release`)

---

## Безопасность и доверие (по докам)

- Установка: флаг **`--consent`** — подтверждение рисков без интерактивного prompt
- MCP в манифесте расширения: **нет `trust`**, в отличие от пользовательского `settings.json`
- Policy engine: расширение **не может** через политики тихо одобрять опасные действия / обходить safeguards (формулировка в Extension reference)
- Секреты: **`sensitive: true`** в `settings` → keychain и маскировка в UI (см. Developers Blog Feb 2026)
- Авторам: least privilege, `excludeTools` для опасных shell-паттернов, валидация на стороне MCP-сервера — [Best practices](https://www.geminicli.com/docs/extensions/best-practices)

---

## Примеры репозиториев

Организация **https://github.com/gemini-cli-extensions** — основной набор официальных расширений, например:

- **workspace** — MCP + набор skills (Gmail, Calendar, Docs, …)
- **security** — MCP + skills dependency/patcher/poc
- **cicd** — MCP + GCP CI/CD skills
- **alloydb-omni**, **cloud-sql-postgresql** — БД / GCP
- **developer-knowledge** — HTTP MCP + OAuth
- **mcp-toolbox**, **firebase**, **flutter** — MCP и контекст

Сторонний пример: **https://github.com/markmcd/gemini-docs-ext** (MCP для доков API).

### Пример в этом репозитории (plugin-kit-ai)

См. **`examples/plugins/gemini-extension-package/gemini-extension.json`**: `mcpServers` с `${extensionPath}`, `settings` с `sensitive`, `themes`, `plan.directory`, `excludeTools`, `contextFileName: GEMINI.md`.

---

## Краткое сравнение с другими стеками

| Измерение | Gemini CLI extensions | OpenAI Codex plugins | Claude Code plugins | OpenCode | Cursor |
|-----------|------------------------|----------------------|---------------------|----------|--------|
| Манифест | `gemini-extension.json` (корень) | `.codex-plugin/plugin.json` | `.claude-plugin/plugin.json` | `opencode.json` + JS plugins | `.cursor-plugin` (IDE) |
| MCP в пакете | Объект `mcpServers` в JSON | Ссылка на `./.mcp.json` | `.mcp.json` / inline | Блок `mcp` в конфиге | `mcp.json` + маркетплейс |
| Skills | `skills/`, tier Extension | `./skills/` в plugin.json | `skills/`, `/plugin:skill` | `.opencode/skills/` + совместимость | В плагине IDE |
| Команды | TOML `commands/` | Не тот же слой в build-docs | `commands/` (md) | Иное | В плагине IDE |
| Хуки | `hooks/hooks.json` | Экспериментально отдельно от структуры плагина | Богатые hooks + inline | In-process JS hooks | В плагине IDE |
| Уникально | `themes`, top-level `excludeTools`, `plan`, policies, GEMINI.md, галерея geminicli.com | Apps `.app.json`, `@`, marketplace.json | LSP, `settings.json` default agent | npm/Bun plugins, custom tools | CLI: полные плагины не поддерживаются ([Plugins](https://cursor.com/docs/plugins)) |

Источники сравнения: официальные доки перечисленных продуктов (см. ссылки выше и в разделе URL).

---

## Практические рекомендации

1. **Документация + версия CLI:** держать закладки на geminicli.com / github docs и перепроверять после обновления `gemini` — **уверенность ~9/10**, **надёжность ~8/10** (доки иногда отстают от релиза).
2. **Эталонное расширение в репозитории + `gemini extensions link`:** смоук после апдейта CLI — **уверенность ~8/10**, **надёжность ~9/10** для регрессий установки и MCP.

---

## Лицензия заметок

Внутренний research-документ репозитория plugin-kit-ai. Факты о продукте принадлежат Google / OpenAI / Anthropic / Cursor / OpenCode и описаны по публичным источникам на дату сборки.
