---
title: "Стартовые шаблоны"
description: "Официальные starter repositories для типовых входных путей в plugin-kit-ai, а не граница managed project model."
canonicalId: "page:guide:starter-templates"
section: "guide"
locale: "ru"
generated: false
translationRequired: true
---

[Использовать плагины](/ru/use/) · [Создавать плагины](/ru/build/) — Подготовка, не релиз. Миграция проектов в v2 пока недоступна.
<details><summary>Исторические материалы plugin-kit-ai v1 · 1.2.4; старые инструкции не являются текущим Build.</summary>


# Стартовые шаблоны

Если нужен проверенный старт вместо пустой директории, используйте официальные starter repositories.

## Важно: starter’ы — это точки входа

Названия starter’ов специально разделены по основному пути, например Codex или Claude.

Это **не** означает, что модель продукта навсегда запирается в одном agent family.

Starter помогает выбрать правильную первую форму для:

- основного runtime-требования
- языка команды
- первого поддерживаемого target’а

После этого сохраняйте repo в managed project model и расширяйте его по реальной необходимости.

Для более широкой картины прочитайте [Один проект, несколько target’ов](/ru/guide/one-project-multiple-targets).

## Codex Runtime

- [plugin-kit-ai-starter-codex-go](https://github.com/777genius/plugin-kit-ai-starter-codex-go)
- [plugin-kit-ai-starter-codex-python](https://github.com/777genius/plugin-kit-ai-starter-codex-python)
- [plugin-kit-ai-starter-codex-node-typescript](https://github.com/777genius/plugin-kit-ai-starter-codex-node-typescript)
- [plugin-kit-ai-starter-codex-python-runtime-package](https://github.com/777genius/plugin-kit-ai-starter-codex-python-runtime-package)

## Claude

- [plugin-kit-ai-starter-claude-go](https://github.com/777genius/plugin-kit-ai-starter-claude-go)
- [plugin-kit-ai-starter-claude-python](https://github.com/777genius/plugin-kit-ai-starter-claude-python)
- [plugin-kit-ai-starter-claude-node-typescript](https://github.com/777genius/plugin-kit-ai-starter-claude-node-typescript)
- [plugin-kit-ai-starter-claude-node-typescript-runtime-package](https://github.com/777genius/plugin-kit-ai-starter-claude-node-typescript-runtime-package)

## Когда лучше брать starter

Используйте starter, когда:

- хотите сразу получить проверенный repo layout
- хотите сравнить свой проект с минимальным поддерживаемым примером
- хотите быстрее онбордить команду без догадок по scaffolding

Используйте `plugin-kit-ai init` напрямую, когда:

- нужен fresh repo from first principles
- нужно явно выбрать флаги и path
- вы встраиваете plugin-kit-ai в уже существующую структуру репозитория

## Безопасная mental model

- выбирайте starter под **первый** правильный путь
- не считайте семейство starter’а окончательной границей repo
- считайте managed project model долгосрочным source of truth

</details>
