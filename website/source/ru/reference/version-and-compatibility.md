---
title: "Политика версий и совместимости"
description: "Как мыслить про релизы, compatibility promises, wrappers, SDK и vocabulary поддержки в plugin-kit-ai."
canonicalId: "page:reference:version-and-compatibility"
section: "reference"
locale: "ru"
generated: false
translationRequired: true
---

[Использовать плагины](/ru/use/) · [Создавать плагины](/ru/build/) — Подготовка, не релиз. Миграция проектов в v2 пока недоступна.
<p class="locale-historical-identity">Политика версий и совместимости</p>

[Исторический снимок ветки plugin-kit-ai v1. 1.2.4 — базовая версия команд, а не точная версия этого снимка.](/en/legacy/v1/)

<details><summary>Исторический снимок ветки plugin-kit-ai v1. 1.2.4 — базовая версия команд, а не точная версия этого снимка.</summary>


# Политика версий и совместимости

Эта страница нужна для одного практического командного решения: что мы стандартизируем и насколько сильное это обещание?

## Выбор за 60 секунд

- читайте эту страницу, когда команде нужен один компактный policy doc про релизы, wrappers, SDK, runtimes и promises совместимости
- читайте [Границу поддержки](/ru/reference/support-boundary), когда нужен самый короткий практический ответ про поддержку
- читайте [Релизы](/ru/releases/), когда нужна история конкретного релиза

## Публичный baseline

Полезно думать о стандартизации в трёх слоях:

- release line, которую вы выбираете между repo
- support level path внутри этой release line
- install или delivery mechanism вокруг этого path

Эти слои связаны, но они не взаимозаменяемы.

## Recommended lanes и formal tiers

Используйте одну и ту же простую трансляцию между docs и policy:

- `Recommended` обычно означает promoted `public-stable` production path
- `Advanced` означает поддерживаемую surface с более узким или специализированным контрактом
- `Experimental` означает opt-in churn вне нормального compatibility expectation

Главные recommended paths сегодня:

- `Codex runtime Go`
- `Codex package`
- `Gemini packaging`
- `Gemini Go runtime`
- `Claude default stable lane`
- `Python` и `Node` local runtime paths как поддерживаемый и рекомендуемый non-Go authoring choice на поддерживаемых target'ах

## Что именно покрывает совместимость

Самое сильное публичное обещание относится к:

- заявленному публичному CLI contract
- рекомендуемому Go SDK path и перечисленным выше recommended production paths
- рекомендуемым локальным Python и Node runtime paths на поддерживаемых target'ах
- задокументированному поведению `public-stable` generated outputs

Совместимость не означает, что каждый wrapper, convenience path или специализированная surface движутся с одинаковой силой обещаний.

## Публичный язык и формальные термины

Используйте такую трансляцию, когда говорите с командой:

- `Recommended` обычно означает, что path находится внутри самого сильного текущего `public-stable` контракта
- `Advanced` означает, что surface поддерживается, но уже или специализированнее первого default
- `Experimental` означает opt-in churn без нормального compatibility expectation

Когда команде нужен точный policy layer, используйте формальные термины `public-stable`, `public-beta` и `public-experimental`.

## Wrappers, SDK и runtime APIs

Не стандартизируйте эти категории так, будто это одно и то же.

- Homebrew, npm, PyPI и verified script - это install channels для CLI
- Go SDK - это публичная SDK surface
- runtime APIs привязаны к своим declared runtime paths

Если относиться к install wrappers так, будто они несут то же обещание, что SDK или runtime path, команда стандартизирует не тот слой.

## Что командам стоит стандартизировать

Здоровые команды обычно стандартизируют:

- одну явную release baseline
- один основной path с понятной support story
- один validation gate перед handoff и rollout
- одно общее понимание формальных compatibility terms

## Финальное правило

Стандартизируйте только ту release line и тот path, чьё публичное обещание команда действительно готова защищать в CI, handoff и rollout.

</details>
