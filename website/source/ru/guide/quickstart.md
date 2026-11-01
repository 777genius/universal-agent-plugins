---
title: "Быстрый старт"
description: "Установите готовый пакет или создайте переносимый пакет Agent Plugins 1.0."
canonicalId: "page:guide:quickstart"
section: "guide"
locale: "ru"
generated: false
translationRequired: true
localeDisposition: "current-translation"
---

<a id="быстрыи-старт"></a>

# Используйте плагины / Создавайте плагины

Установите пакет для своих агентов или подготовьте переносимый пакет Agent
Plugins 1.0 с инструкциями и подключением инструментов.

## Используйте плагины {#use-plugins}

<a id="опциональная-быстрая-проверка"></a>

При наличии Node.js 22 или новее:

```bash
npx universal-agent-plugins add context7
```

CLI предложит совместимые агенты и покажет оставшиеся действия активации.
Для предварительного изучения и просмотра плана:

```bash
npx universal-agent-plugins info context7
npx universal-agent-plugins add context7 --target codex,cursor --dry-run
```

[Инструкция по нативной установке](https://github.com/777genius/universal-agent-plugins/blob/main/docs/NATIVE_INSTALL.md)
предлагает исполняемые файлы для macOS, Linux и Windows. Самому нативному
менеджеру Node.js не нужен; отдельным плагинам могут требоваться свои среды.
Для npm проверьте `npx universal-agent-plugins version`: пакетный канал может
ещё поставлять предыдущий CLI. Автоматический выбор ниже требует 0.1.94;
для проверенного выпуска используйте нативный установщик. Явные проектные
команды этого руководства также работают в 0.1.93.

[CLI 0.1.95](https://github.com/777genius/universal-agent-plugins/releases/tag/agentplugins-v0.1.95)
сохраняет явную область проекта и автоматический выбор для `install`, добавленный
в 0.1.94. Также доступны [локальный API управления](../use/manage#local-management-api)
и [нативные расширения Pi](../reference/client-compatibility#pi-native-extensions).
План добавления пакета в проект:

Для проекта возьмите из проверенной карточки каталога точный Git-источник с полным SHA коммита и совместимый агент. Псевдонимы Directory v1 (например, `context7`) предназначены только для пользовательской области. Замените `OWNER/REPOSITORY`, `FULL_COMMIT_SHA` и `PACKAGE_PATH`; для пакета в корне репозитория уберите `//PACKAGE_PATH`.

```bash
npx universal-agent-plugins add 'github:OWNER/REPOSITORY@FULL_COMMIT_SHA//PACKAGE_PATH' --target codex --scope project \
  --project-root /path/to/project --dry-run
```

Выберите существующую папку. Добавление и обслуживание по умолчанию используют
пользовательскую область: выбирайте проект явно. В 0.1.94 `install` без аргумента источника
выбирает проект только при обычном файле `uap.toml` непосредственно в текущей папке. Явная
область имеет приоритет, родители не просматриваются. Прочитайте план перед
удалением `--dry-run`. Обнаружение клиентом, доверие, аутентификация и вызовы MCP
остаются отдельными проверками после записи файлов.

[Полный путь установки](../use/install) ·
[Совместимость и границы проверок](../reference/client-compatibility)

### Восстановите сохранённые зависимости

Проверьте записи выбранного пользовательского окружения:

```bash
npx universal-agent-plugins deps status
```

Для сохранённой пары `uap.toml`/`uap.lock` выполните экспорт из папки обмена:

```bash
npx universal-agent-plugins deps export
```

Скопируйте папку на другой компьютер, установите нужные клиенты и выполните там:

```bash
npx universal-agent-plugins deps import
npx universal-agent-plugins install --locked --scope user
```

Переносятся только проверенные файлы соглашения и lock. Профили, настройки,
учётные данные, квитанции и данные плагинов остаются отдельными. Установке нужны
точные источники или проверенный кэш. Для проекта добавьте пару в контроль версий
и восстанавливайте через явный `install --locked --scope project`.
Подробности переноса и восстановления — в [обслуживании](../use/manage).

## Создавайте плагины {#build-plugins}

<a id="рекомендуемыи-путь"></a>
<a id="если-прочитать-только-одно"></a>

Стандартный пакет начинается с корневого `plugin.json`; можно добавить
`skills/<name>/SKILL.md`, `mcp.json` или оба компонента. Файлы можно написать
вручную и проверить опубликованным установщиком:

```bash
npx universal-agent-plugins validate ./my-plugin
npx universal-agent-plugins add ./my-plugin --target codex --dry-run
```

Предполагается, что `./my-plugin` уже содержит полный пакет.
[Руководство по созданию](../build/) показывает минимальный пакет Skill вручную
и средства создания шаблонов из исходного кода.

**Доступность средств разработки:** в текущем npm- и нативном выпуске
установщика нет `agentplugins author`. В исходном коде реализованы шаблоны,
статические проверки, явные запуски MCP, циклы разработки, установка зависимостей
и обслуживание JSON. Для этих команд нужна [сборка из исходного кода](../build/#build-from-source).
Доказательства для старых выпусков относятся к их версиям и не обещают те же
команды в нынешнем установщике.

Статическая проверка не доказывает исполнение, OAuth или активацию. Поддержка
транспортов MCP зависит от клиента: например, Codex не поддерживает объявленный
MCP SSE. Укажите требования и непроведённые проверки в README пакета.

[Спецификация Agent Plugins 1.0](https://agent-plugins.org/specification)

<!-- locale-historical-source:start
locale-historical-frontmatter:start
---
title: "Быстрый старт"
description: "Самый быстрый рекомендуемый путь к рабочему проекту на plugin-kit-ai."
canonicalId: "page:guide:quickstart"
section: "guide"
locale: "ru"
generated: false
translationRequired: true
---
locale-historical-frontmatter:end

# Быстрый старт

Это самый короткий рекомендуемый путь, если вам нужен один plugin repo, который потом можно расширять новыми способами поставки.

Сначала выберите один сильный стартовый путь. Package, extension и настройку интеграций в самом repo можно добавить позже, когда они действительно понадобятся продукту.

## Начните с задачи

Выберите форму repo по тому, что именно вы собираете:

- online service: `plugin-kit-ai init my-plugin --template online-service`
- local tool: `plugin-kit-ai init my-plugin --template local-tool`
- custom logic - Advanced: `plugin-kit-ai init my-plugin --template custom-logic`

Если хотите сначала короткое объяснение, откройте [Что именно вы собираете](/ru/guide/choose-what-you-are-building).

## Опциональная быстрая проверка

Если хотите максимально быстро убедиться, что опубликованный install flow реально работает, начните отсюда:

```bash
npx plugin-kit-ai@latest add notion
```

- Эта команда ставит все поддерживаемые outputs этого плагина.
- Эта опциональная проверка не создаёт repo, который вы будете редактировать дальше.
- Если ваша цель - авторский plugin repo, пропустите этот шаг и идите сразу в job-first `init` выше.

## Если читать только одно

Начинайте с job-first пути выше.

Первый выбор - это стартовая точка, а не вечная граница репозитория.

## Legacy path для совместимости

Используйте этот путь только тогда, когда вы осознанно поддерживаете старый Codex runtime Go flow или подстраиваетесь под существующие docs и scripts:

```bash
brew install 777genius/homebrew-plugin-kit-ai/plugin-kit-ai
plugin-kit-ai version
plugin-kit-ai init my-plugin
cd my-plugin
go mod tidy
plugin-kit-ai generate .
plugin-kit-ai validate . --platform codex-runtime --strict
```

Это сохраняет старый путь для Codex runtime на Go, но это уже не рекомендуемый first-run путь для новых repo.

## Установите CLI для ежедневной работы

Если планируете пользоваться plugin-kit-ai постоянно, установите CLI отдельно:

```bash
brew install 777genius/homebrew-plugin-kit-ai/plugin-kit-ai
plugin-kit-ai version
```

## Почему этот путь всё ещё существует

- backward compatibility для старых docs и scripts
- стабильный Codex runtime Go path, когда он вам уже действительно нужен
- мост для миграции, а не главная рекомендация для новых пользователей

## Что вы получите

- один plugin repo с первого дня
- authored files под `plugin/`
- generated output для Codex runtime из того же repo
- понятную проверку готовности через `validate --strict`

## Поддерживаемые пути для Node и Python

Если команда уже живёт в Node/TypeScript или Python, эти пути поддерживаются и видны с самого начала:

- `codex-runtime --runtime node --typescript`
- `codex-runtime --runtime python`
- оба варианта являются локальными interpreted runtime paths, поэтому на машине исполнения всё равно нужен Node.js `20+` или Python `3.10+`
- Go всё равно остаётся путём по умолчанию, когда нужен самый сильный общий сценарий для продакшна

## Если вы осознанно начинаете с Node или Python

Используйте этот альтернативный flow только тогда, когда выбор языка уже является частью продуктового требования:

```bash
plugin-kit-ai init my-plugin --platform codex-runtime --runtime node --typescript
plugin-kit-ai doctor ./my-plugin
plugin-kit-ai bootstrap ./my-plugin
plugin-kit-ai generate ./my-plugin
plugin-kit-ai validate ./my-plugin --platform codex-runtime --strict
```

Или стартуйте с Python:

```bash
plugin-kit-ai init my-plugin --platform codex-runtime --runtime python
plugin-kit-ai doctor ./my-plugin
plugin-kit-ai bootstrap ./my-plugin
plugin-kit-ai generate ./my-plugin
plugin-kit-ai validate ./my-plugin --platform codex-runtime --strict
```

## Что делать дальше

- правьте плагин под `plugin/`
- после изменений снова запускайте `plugin-kit-ai generate ./my-plugin`
- потом снова запускайте `plugin-kit-ai validate ./my-plugin --platform codex-runtime --strict`
- и только после этого добавляйте другие способы поставки, если продукту это действительно нужно

## Что добавлять потом

| Цель | Что добавлять позже |
| --- | --- |
| Claude hooks как реальный продукт | `claude` |
| Официальный пакет Codex | `codex-package` |
| Пакет расширения Gemini | `gemini` |
| Настройка интеграции в самом repo | `opencode` или `cursor` |

`claude` выбирайте первым только тогда, когда hooks Claude уже являются реальным требованием продукта.

## Что расширяется потом

- repo остаётся единым, когда вы добавляете новые lanes
- package и extension lanes идут из того же authored source
- OpenCode и Cursor нужны тогда, когда repo должен хранить и вести настройку интеграции
- точная support boundary живёт в reference docs, а не в вашем первом стартовом flow

## Что читать дальше

- [Что именно вы собираете](/ru/guide/choose-what-you-are-building)
- [Соберите собственную логику плагина](/ru/guide/build-custom-plugin-logic)
- [Создайте первый plugin](/ru/guide/first-plugin)
- [Что можно собрать](/ru/guide/what-you-can-build)
- [Выбор target](/ru/guide/choose-a-target)
locale-historical-source:end -->
