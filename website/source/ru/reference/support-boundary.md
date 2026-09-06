---
title: "Граница поддержки"
description: "Самый короткий практический ответ о том, что plugin-kit-ai рекомендует, поддерживает осторожно и оставляет experimental."
canonicalId: "page:reference:support-boundary"
section: "reference"
locale: "ru"
generated: false
translationRequired: true
---

[Использовать плагины](/ru/use/) · [Создавать плагины](/ru/build/) — Подготовка, не релиз. Миграция проектов в v2 пока недоступна.
<details><summary>Исторические материалы plugin-kit-ai v1 · 1.2.4; старые инструкции не являются текущим Build.</summary>


# Граница поддержки

Используйте эту страницу, когда нужен самый короткий честный ответ про поддержку.

Она отвечает на три командных вопроса:

- что безопасно рекомендовать по умолчанию
- что поддерживается, но должно выбираться осознанно
- что всё ещё experimental и не должно незаметно становиться team policy

## Безопасные defaults

Вот самые безопасные defaults на сегодня:

- Go - рекомендуемый runtime path по умолчанию.
- `validate --strict` - главная проверка готовности для локальных Python и Node runtime repos.
- `Codex runtime Go`, `Codex package`, `Gemini packaging`, `Gemini Go runtime` и Claude default stable lane - главные Recommended production lanes.
- `Python` и `Node` - поддерживаемые non-Go paths и рекомендуемый non-Go выбор, когда компромисс локального interpreted runtime выбран осознанно.

## Как это маппится на формальный контракт

Сначала публичные docs используют три простых слова:

- `Recommended` обычно маппится на самые сильные текущие `public-stable` production lanes.
- `Advanced` означает поддерживаемую surface с более узким, специализированным или осторожным контрактом.
- `Experimental` означает opt-in churn вне нормального compatibility expectation.

Когда команде нужен точный policy language, важнее формальные термины: `public-stable`, `public-beta` и `public-experimental`.

## Что рекомендуется сегодня

Если нужен практический ответ, начинайте отсюда:

- Claude рекомендуется на default stable hook path.
- Codex рекомендуется и для `Notify` runtime path, и для официального `codex-package` path.
- Gemini packaging рекомендуется, и promoted Gemini Go runtime тоже production-ready.
- OpenCode и Cursor - это repo-owned integration setup paths. Они полезны, но это не default executable runtime start.

## Advanced surfaces

Выбирайте advanced surfaces только тогда, когда компромисс ясен и действительно нужен.

Типичные примеры:

- OpenCode и Cursor, когда repo должен владеть integration config вместо runtime path
- более узкие или специализированные runtime expansions вне основных рекомендуемых paths
- install wrappers, если реальная задача - доставка CLI, а не runtime APIs или SDKs
- специальные config surfaces, которые полезны, но не являются первым default для большинства команд

## Experimental surfaces

Относитесь к experimental областям как к opt-in и high-churn.

Они могут быть полезны ранним пользователям, но не должны тихо становиться долгосрочным стандартом для команды.

## Практическое правило

Если выбираете за команду, стандартизируйте самый узкий path, чьё обещание вы действительно готовы защищать в CI, rollout и handoff.

</details>
