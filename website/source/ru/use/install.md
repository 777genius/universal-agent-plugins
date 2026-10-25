---
title: "Find and install a plugin"
description: "Перевод недоступен; читайте актуальную английскую страницу"
canonicalId: "page:use:install"
section: "use"
locale: "ru"
generated: false
translationRequired: true
---

# Find and install a plugin

[Перевод недоступен; читайте актуальную английскую страницу](/en/use/install). Авторинг Agent Plugins доступен через Agent Plugins в `universal-agent-plugins`.

[Использовать плагины](/ru/use/) · [Создавать плагины](/ru/build/)

## Установка в проект в текущем исходном коде

**Только сборка из исходников; функция ещё не выпущена.** Совместимая
сборка позволяет явно выбрать проект для пакетов и Agent Skills:

```sh
agentplugins add owner/repo --target cursor \
  --scope project --project-root /path/to/project
agentplugins skills add owner/repository --ui \
  --scope project --project-root /path/to/project
```

Каталог должен существовать. По умолчанию используется `user`; область
`project` без `--project-root` выбирает текущий каталог без поиска в родительских
папках. Браузерный и терминальный выбор используют одну область и показывают
доступные для неё агенты. После подготовки файлов агенту могут потребоваться
перезапуск, доверие к проекту или явный выбор конфигурации сеанса.

[Руководство по установке в проект](https://github.com/777genius/universal-agent-plugins/blob/main/docs/PROJECT_INSTALLATION.md) · [Текущая поддержка проектов и ограничения](https://github.com/777genius/universal-agent-plugins/blob/main/docs/PROJECT_CLIENT_SUPPORT.md)
