---
title: "Создавать плагины"
description: "Подготовьте пакет Skills и MCP вручную или средствами из исходного кода."
canonicalId: "page:build:index"
section: "build"
locale: "ru"
generated: false
translationRequired: true
localeDisposition: "current-translation"
---

# Создавать плагины

Пакет Agent Plugins 1.0 — это каталог с корневым `plugin.json` и Skills,
конфигурацией MCP или обоими компонентами. Выберите минимальный состав,
который решает задачу пользователей. Skill задаёт инструкции, MCP подключает
инструменты.

## Доступность

Текущий нативный установщик,
[CLI 0.1.94](https://github.com/777genius/universal-agent-plugins/releases/tag/agentplugins-v0.1.94),
проверяет и устанавливает стандартные пакеты. Пакетные каналы могут
обновляться позднее; узнайте установленную версию через `agentplugins version`.
В нём **нет**
`agentplugins author`. Команды разработки в этом руководстве относятся к
**текущей сборке из исходного кода**, в которой эти средства включены.
Доказательства для старого выпуска 0.1.65 сохраняют свою версию и не описывают
нынешний установщик.

Вы можете написать файлы вручную и использовать опубликованный установщик либо
собрать средства для создания шаблонов, статических проверок и явного запуска
MCP. Успешная статическая проверка не доказывает активацию в клиенте или связь
с работающим сервисом.

## Подготовьте небольшой пакет вручную

Создайте структуру:

```text
review-helper/
  plugin.json
  skills/
    review-docs/
      SKILL.md
```

Укажите сведения о пакете в `plugin.json`:

```json
{
  "$schema": "https://agent-plugins.org/schemas/1.0.0/plugin.schema.json",
  "name": "review-helper",
  "version": "0.1.0",
  "description": "Проверка ясности и обоснованности изменений документации"
}
```

Начните `skills/review-docs/SKILL.md` так:

```markdown
---
name: review-docs
description: Используйте при проверке ясности и обоснованности документации.
---

# Проверка документации

Определите читателя и задачу. Проверьте входные данные и действие каждой команды.
Отделите проверенное поведение от требований и непроверенных утверждений.
Верните замечания с расположением в файлах и предложенной правкой.
```

Установите опубликованный менеджер, проверьте пакет и план установки:

```bash
npm install -g universal-agent-plugins
agentplugins validate ./review-helper
agentplugins add ./review-helper --target codex --dry-run
```

Изучите план и перейдите к [установке](../use/install), когда будете готовы
применить изменения. Назначение файлов описано в [структуре пакета](./layout).

## Сборка из исходного кода {#build-from-source}

Следующие команды собирают текущую CLI со средствами создания пакетов.
Нужны Git и версия Go из репозитория — сейчас Go 1.26.10.
Сохраняйте коммит собранной версии вместе с результатами проверок.

```bash
git clone https://github.com/777genius/universal-agent-plugins.git
cd universal-agent-plugins/cli
mkdir -p ../bin
authoring_module=$(go list -f '{{.Module.Path}}' ./cmd/agentplugins)
authoring_revision=$(git rev-parse HEAD)
go build -ldflags "-X ${authoring_module}/internal/authoring/commands.Enabled=release-cli-contract-v1 -X ${authoring_module}/internal/authoring/commands.Revision=${authoring_revision}" \
  -o ../bin/agentplugins ./cmd/agentplugins
export PATH="$(cd ../bin && pwd):$PATH"
agentplugins author --help
```

Создавайте учебные пакеты в отдельной рабочей папке. Полученный файл — сборка
для разработки; сама её работоспособность не подтверждает квалификацию
публичного выпуска. Если CLI по-прежнему не знает `author`, проверьте путь
запускаемого исполняемого файла.

## Выберите шаблон

| Задача | Руководство | Обязательные параметры |
| --- | --- | --- |
| Инструкции для агента | [Отдельный Skill](./skill) | `--template skill` |
| Подключение существующего сервиса | [Удалённый MCP](./mcp-remote) | `--template mcp-remote --url <endpoint>` |
| Локальный MCP-процесс Node | [MCP через stdio](./mcp-stdio) | `--template mcp-stdio --runtime node` |
| Инструкции вместе с инструментами | [Гибридный пакет](./hybrid) | `--template hybrid --mcp-template <choice>` и параметры выбранного MCP-шаблона |

Пакет с одним Skill всё равно содержит корневой `plugin.json`. Установка чужих
отдельных Skills — другая [операция установщика](../use/install).

## Рабочий цикл

1. Создайте пакет в ещё не существующем каталоге и отредактируйте файлы.
2. Запустите [validate, inspect и статический test](./checks) для точного корня пакета.
3. Проверьте выбранные клиенты через `compat`, изучите сведения `doctor`.
4. Отдельно проверьте MCP явным запуском в сборке из исходного кода.
5. Передайте пакет и сведения о проверках через [установщик](./handoff).

В текущем исходном коде также есть `bootstrap` по lock-файлу, непрерывный `dev`,
нормализация JSON через `normalize` и явный импорт Claude MCP через
`import native`. См. [проверки](./checks) и [MCP через stdio](./mcp-stdio).
Генерация представлений для клиентов, экспорт архивов и публикация пакетов
не входят в текущий интерфейс этих команд. Клиентские hooks остаются отдельными
расширениями и не становятся переносимыми компонентами Skills или MCP.
