---
title: "v1.0.4 Go SDK"
description: "Patch release notes для исправления Go SDK module path."
canonicalId: "page:releases:v1-0-4-go-sdk"
section: "releases"
locale: "ru"
generated: false
translationRequired: true
---

[Использовать плагины](/ru/use/) · [Создавать плагины](/ru/build/) — Подготовка, не релиз. Миграция проектов в v2 пока недоступна.
<details><summary>Исторические материалы plugin-kit-ai v1 · 1.2.4; старые инструкции не являются текущим Build.</summary>


# v1.0.4 Go SDK

Дата релиза: `2026-03-29`

## Почему этот patch важен

Этот patch сделал public Go SDK module path корректным для нормального Go consumption.

## Что изменилось

- корень Go SDK module был перенесён с `sdk/plugin-kit-ai/` в `sdk/`
- public module path `github.com/777genius/plugin-kit-ai/sdk` теперь совпадает с реальным layout репозитория
- starter repos, examples и templates перестали учить новичков `replace`-workaround’ам

## Практический вывод

- используйте `github.com/777genius/plugin-kit-ai/sdk@v1.0.4` или новее для нормального Go module consumption
- считайте `v1.0.3` known-bad релизом для Go SDK module path

## Почему это важно пользователям

Этот patch убрал лишнее трение для обычных Go consumers и сделал рекомендуемый SDK path похожим на нормальный public module, а не на special-case workaround.

</details>
