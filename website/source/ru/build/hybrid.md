---
title: "Создать гибридный пакет"
description: "Объедините инструкции Skill и конфигурацию MCP в одном пакете."
canonicalId: "page:build:hybrid"
section: "build"
locale: "ru"
generated: false
translationRequired: true
localeDisposition: "current-translation"
---

# Создать гибридный пакет

**Средства из исходного кода:** команды требуют
[соответствующей сборки](./#build-from-source). В текущем установщике нет
`agentplugins author`.

Гибридный пакет полезен, когда инструкции объясняют, как и когда вызывать
инструменты. Оба компонента находятся под одним корневым манифестом. Поддержка
в клиенте может различаться: сохраняйте отдельные результаты для каждого
компонента.

## Явно выберите MCP-компонент

Флаг `--mcp-template` выбирает `mcp-remote` или `mcp-stdio`. Затем укажите
обязательные параметры выбранного шаблона. Транспорт не выводится из URL,
локальных файлов или установленных сред исполнения.

Пример с удалённым MCP:

```bash
agentplugins author init ./research-helper \
  --template hybrid \
  --name research-helper \
  --description 'Работа с командным MCP-сервисом для исследований' \
  --skill-name research-guide \
  --mcp-template mcp-remote \
  --url https://mcp.example.com/mcp
```

URL — пример, замените его настоящей конечной точкой. Прочитайте
[требования удалённого MCP](./mcp-remote).

## Либо выберите локальный процесс

Для другого отсутствующего каталога:

```bash
agentplugins author init ./local-research \
  --template hybrid \
  --name local-research \
  --description 'Исследования с локальными инструментами Node' \
  --skill-name research-guide \
  --mcp-template mcp-stdio \
  --runtime node
```

Не добавляйте `--url` к stdio-варианту. Изучите
[требования и границы stdio](./mcp-stdio).

## Свяжите инструкции с инструментами

Основная структура удалённого гибридного пакета:

```text
research-helper/
  plugin.json
  mcp.json
  skills/
    research-guide/
      SKILL.md
```

Также создаются README и `.gitignore`. Вариант stdio добавляет Node-исходник и
файлы зависимостей из соответствующего руководства. В `SKILL.md` объясните,
когда вызывать инструменты, какие входные данные нужны и как сообщать об
отсутствующем доступе. Текст Skill не является разрешением на действия.

## Проверьте оба компонента

```bash
agentplugins author skills validate ./research-helper
agentplugins author validate ./research-helper
agentplugins author inspect ./research-helper --target codex,claude
agentplugins author test ./research-helper
agentplugins author compat ./research-helper --target codex,claude
```

Для локального варианта выберите `./local-research`. Поддержка Skill не означает
поддержку транспорта MCP или активацию сервиса. Объединение в один пакет не
устраняет эту границу.

Далее: [дополнительные Skills](./skills) или [передача](./handoff).
