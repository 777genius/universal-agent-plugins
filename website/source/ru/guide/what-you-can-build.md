---
title: "Что можно собрать"
description: "Используйте эту страницу как product map: какие outputs существуют, как выглядит default start и куда один repo может вырасти позже."
canonicalId: "page:guide:what-you-can-build"
section: "guide"
locale: "ru"
generated: false
translationRequired: true
aside: true
outline: [2, 3]
---

[Использовать плагины](/ru/use/) · [Создавать плагины](/ru/build/) — Подготовка, не релиз. Миграция проектов в v2 пока недоступна.
<p class="locale-historical-identity">Что можно собрать</p>

[Исторический снимок ветки plugin-kit-ai v1. 1.2.4 — базовая версия команд, а не точная версия этого снимка.](/en/legacy/v1/)

<details><summary>Исторический снимок ветки plugin-kit-ai v1. 1.2.4 — базовая версия команд, а не точная версия этого снимка.</summary>


# Что можно собрать

Используйте эту страницу как product map. Она показывает, какие outputs вообще существуют, а не отвечает на вопрос, когда одному repo пора расти или делиться позже.

plugin-kit-ai может стартовать с одного исполняемого plugin и со временем вырасти в дополнительные supported outputs.

## Рекомендуемая стартовая форма

Начните с одного runtime path, обычно с Codex runtime на Go. Так первый repo остаётся простым и даёт самый понятный validate-and-ship loop.

Если ваша команда уже работает на Node/TypeScript или Python, это тоже supported starting paths.

## Один repo, много supported outputs

Из одного и того же проекта можно вырасти в:

- runtime outputs для supported hosts
- packaged outputs, если packaging - это реальное требование поставки
- extension outputs для host'ов, которым нужен extension artifact
- repo-owned integration setup, если repo в основном должен хранить checked-in configuration для другого tool

## Для чего эта страница не нужна

Выбор Node или Python не заставляет вас в первый же день решать все вопросы про packaging или integration setup.

Эта страница - обзор. Если вопрос в том, стоит ли одному repo продолжать расти, читайте [Один проект, несколько target'ов](/ru/guide/one-project-multiple-targets).

</details>
