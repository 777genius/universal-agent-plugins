---
title: "Maintain installed plugins"
description: "Перевод недоступен; читайте актуальную английскую страницу"
canonicalId: "page:use:manage"
section: "use"
locale: "ru"
generated: false
translationRequired: true
---

# Maintain installed plugins

[Перевод недоступен; читайте актуальную английскую страницу](/en/use/manage). Авторинг Agent Plugins доступен через Agent Plugins в `universal-agent-plugins`.

[Использовать плагины](/ru/use/) · [Создавать плагины](/ru/build/)

## Обслуживание проекта в текущем исходном коде

**Только сборка из исходников; функция ещё не выпущена.** В каждой команде
обслуживания указывайте ту же область и тот же проект:

```sh
agentplugins list --scope project --project-root /path/to/project
agentplugins install --locked --scope project --project-root /path/to/project
agentplugins repair review --target cursor --scope project --project-root /path/to/project
agentplugins deps recover --scope project --project-root /path/to/project
```

Для совместного использования зависимостей сохраняйте переносимые `uap.toml`
и `uap.lock` в системе контроля версий. Записи владения, восстановление и данные
в `.uap/` остаются локальными; данные копируйте отдельно. Удаление сохраняет
данные и общий кэш содержимого. Repair восстанавливает отсутствующие компоненты
и повреждённые управляемые пакеты; изменённые файлы агента требуют проверки.
Восстановление выполняйте для исходного выбранного проекта.

[Руководство по установке в проект](https://github.com/777genius/universal-agent-plugins/blob/main/docs/PROJECT_INSTALLATION.md) · [Текущая поддержка проектов и ограничения](https://github.com/777genius/universal-agent-plugins/blob/main/docs/PROJECT_CLIENT_SUPPORT.md)
