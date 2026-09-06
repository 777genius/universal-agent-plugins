---
title: "Модель стабильности"
description: "Как plugin-kit-ai различает public-stable, public-beta и public-experimental области."
canonicalId: "page:concepts:stability-model"
section: "concepts"
locale: "ru"
generated: false
translationRequired: true
---

[Использовать плагины](/ru/use/) · [Создавать плагины](/ru/build/) — Подготовка, не релиз. Миграция проектов в v2 пока недоступна.
<p class="locale-historical-identity">Модель стабильности</p>

[Исторический снимок ветки plugin-kit-ai v1. 1.2.4 — базовая версия команд, а не точная версия этого снимка.](/en/legacy/v1/)

<details><summary>Исторический снимок ветки plugin-kit-ai v1. 1.2.4 — базовая версия команд, а не точная версия этого снимка.</summary>


# Модель стабильности

`plugin-kit-ai` использует формальные contract terms, чтобы команды могли точно понять, что именно они хотят стандартизировать.

<MermaidDiagram
  :chart="`
flowchart TD
  Stable[public stable] --> Beta[public beta]
  Beta --> Experimental[public experimental]
  StableNote[Normal production expectations] -.-> Stable
  BetaNote[Supported but not frozen] -.-> Beta
  ExperimentalNote[Opt in churn] -.-> Experimental
`"
/>

## Публичный язык и формальный язык

Публичные docs сначала используют более простой vocabulary:

- `Recommended` обычно указывает на самые сильные текущие `public-stable` paths
- `Advanced` указывает на поддерживаемые surfaces, которые уже или специализированнее
- `Experimental` соответствует `public-experimental`

Когда вы задаёте compatibility policy, формальные термины важнее.

## Как читать Recommended

`Recommended` - это продуктовый язык, а не замена формального контракта.

- обычно это promoted `public-stable` production path
- это не означает parity между всеми target'ами
- сама формулировка не поднимает `public-beta` или `public-experimental` surfaces выше

## Public-Stable

Воспринимайте `public-stable` как уровень, на который можно опираться с нормальными production expectations.

Это tier, который большинству команд стоит предпочитать для default standards и долгого rollout.

## Public-Beta

Воспринимайте `public-beta` как поддерживаемый, но ещё не замороженный контракт.

Используйте beta только тогда, когда компромисс осознан и действительно оправдан для продукта.

## Public-Experimental

Воспринимайте `public-experimental` как opt-in churn вне нормального compatibility expectation.

Это может быть полезно для learning или раннего тестирования, но не должно тихо становиться командным default.

## Практическое правило

1. Предпочитайте рекомендуемый path для того продукта, который вы строите.
2. Используйте точные формальные terms только тогда, когда нужна policy или compatibility precision.
3. Используйте `validate --strict` как readiness gate для repo, который собираетесь выпускать.

</details>
