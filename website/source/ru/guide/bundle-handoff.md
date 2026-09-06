---
title: "Передача bundle"
description: "Как экспортировать, устанавливать, загружать и публиковать portable bundles для поддерживаемых Python и Node сценариев."
canonicalId: "page:guide:bundle-handoff"
section: "guide"
locale: "ru"
generated: false
translationRequired: true
---

[Использовать плагины](/ru/use/) · [Создавать плагины](/ru/build/) — Подготовка, не релиз. Миграция проектов в v2 пока недоступна.
<p class="locale-historical-identity">Передача bundle</p>

[Исторический снимок ветки plugin-kit-ai v1. 1.2.4 — базовая версия команд, а не точная версия этого снимка.](/en/legacy/v1/)

<details><summary>Исторический снимок ветки plugin-kit-ai v1. 1.2.4 — базовая версия команд, а не точная версия этого снимка.</summary>


# Передача bundle

Используйте этот гайд, когда Python или Node плагин нужно передавать как готовый portable artifact, а не как живой checkout репозитория.

Это реальная публичная возможность продукта, но она уже и строже, чем основной путь через Go.

## Что сюда входит

Стабильный subset bundle handoff покрывает:

- exported `python` bundles на `codex-runtime` и `claude`
- exported `node` bundles на `codex-runtime` и `claude`
- локальную установку bundle
- удалённую загрузку bundle
- публикацию bundle в GitHub Releases

Этот путь подходит, когда:

- другой команде нужно передать готовый артефакт, а не весь репозиторий
- ваш release flow уже использует GitHub Releases
- для Python или Node runtime нужен более чистый handoff-сценарий

## Практический поток

Со стороны автора:

```bash
plugin-kit-ai export . --platform <codex-runtime|claude>
plugin-kit-ai bundle publish . --platform <codex-runtime|claude> --repo <owner/repo> --tag <tag>
```

Со стороны получателя есть два пути:

```bash
plugin-kit-ai bundle install <bundle.tar.gz> --dest <path>
```

или:

```bash
plugin-kit-ai bundle fetch <owner/repo> --tag <tag> --platform <codex-runtime|claude> --runtime <python|node> --dest <path>
```

После `install` или `fetch` получившийся репозиторий всё равно должен пройти обычный bootstrap и проверки готовности.

## Что не происходит автоматически

`bundle install` и `bundle fetch` не превращают bundle в полностью готовый и проверенный плагин сами по себе.

Считайте установленный bundle началом дальнейшей настройки:

1. поставьте runtime prerequisites
2. выполните `plugin-kit-ai doctor .`
3. выполните нужный bootstrap step
4. выполните `plugin-kit-ai validate . --platform <target> --strict`

## Когда bundle handoff лучше, чем live repo

Выбирайте bundle handoff, когда:

- именно release artifacts являются вашим договором поставки
- downstream-пользователям не нужно клонировать исходный репозиторий
- нужен повторяемый GitHub Releases flow для Python или Node путей

Оставайтесь на пути с живым репозиторием, когда:

- команда продолжает редактировать исходное состояние проекта напрямую
- главная задача — совместная работа внутри одного репозитория
- Go уже даёт тот чистый compiled-binary handoff, который вам нужен

## Важная граница

Bundle handoff не означает «универсальная упаковка для любых targets».

Это поддерживаемый portable handoff flow только для exported Python и Node subset на `codex-runtime` и `claude`.

Не переносите этот контракт автоматически на:

- Go SDK repos
- workspace-config target’ы вроде Cursor и OpenCode
- packaging-only targets вроде Gemini
- install packages для CLI

## Что читать дальше

Свяжите эту страницу с [Выбором модели поставки](/ru/guide/choose-delivery-model), [Готовностью к продакшену](/ru/guide/production-readiness) и [Границей поддержки](/ru/reference/support-boundary).

</details>
