---
title: "Стандарт репозитория"
description: "Как должен выглядеть здоровый plugin-kit-ai repo и как отделять исходное состояние проекта от generated outputs."
canonicalId: "page:reference:repository-standard"
section: "reference"
locale: "ru"
generated: false
translationRequired: true
---

[Использовать плагины](/ru/use/) · [Создавать плагины](/ru/build/) — Подготовка, не релиз. Миграция проектов в v2 пока недоступна.
<p class="locale-historical-identity">Стандарт репозитория</p>

[Исторический снимок ветки plugin-kit-ai v1. 1.2.4 — базовая версия команд, а не точная версия этого снимка.](/en/legacy/v1/)

<details><summary>Исторический снимок ветки plugin-kit-ai v1. 1.2.4 — базовая версия команд, а не точная версия этого снимка.</summary>


# Стандарт репозитория

Эта страница описывает публичную форму здорового `plugin-kit-ai` репозитория.

## Главное правило

Репозиторий должен делать устройство проекта очевидным, а generated outputs — воспроизводимыми.

На практике это значит:

- исходное состояние проекта легко найти
- generated target files явно являются outputs
- основной target или набор target'ов в scope видимы
- выбранный runtime или runtime-политика видимы
- validation command задокументирована

## Что должно быть легко найти

В здоровом репозитории без копания должны быть понятны такие вещи:

- основной target или target'ы в scope
- выбранный runtime или политика runtime по target'ам
- canonical команда `validate --strict` или набор validation-команд, если target'ов несколько
- runtime prerequisites вроде Go, Python или Node
- использует ли repo Go SDK path или shared runtime package

## Что не должно быть главным источником истины

Вот это не должно становиться главным источником истины:

- hand-edited generated target files
- install packages, замаскированные под runtime API
- скрытое знание о “той самой команде, которую на самом деле нужно запускать”

## Признаки здорового репозитория

- `generate` воспроизводит target outputs
- `validate --strict` чисто проходит для intended target или для каждого target’а, который repo публично обещает
- repo объясняет выбранный путь в публичных docs или README
- CI использует тот же public readiness flow, что и локальная разработка

## Признаки слабого репозитория

- target files патчатся вручную после генерации
- выбор пути неявен или плавает между машинами
- другому пользователю нужен maintainer, чтобы повторить базовый flow
- repo обещает поддержку частей системы вне заявленной границы поддержки

## Связь с этим docs site

В этой публичной документации стандарт repo — это место, где:

- authoring guidance становится практикой
- support boundaries становятся проверяемыми
- handoff становится правдоподобным

Свяжите эту страницу с [Процессом авторинга](/ru/reference/authoring-workflow), [Готовностью к продакшену](/ru/guide/production-readiness) и [Словарём терминов](/ru/reference/glossary).

</details>
