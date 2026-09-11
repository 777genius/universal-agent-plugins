---
title: "Интеграция с CI"
description: "Как превратить публичный authored flow в стабильный CI gate для plugin-kit-ai проектов."
canonicalId: "page:guide:ci-integration"
section: "guide"
locale: "ru"
generated: false
translationRequired: true
---

[Использовать плагины](/ru/use/) · [Создавать плагины](/ru/build/) — Подготовка, не релиз. Миграция проектов в v2 пока недоступна.
<p class="locale-historical-identity">Интеграция с CI</p>

[Исторический снимок ветки plugin-kit-ai v1. 1.2.4 — базовая версия команд, а не точная версия этого снимка.](/en/legacy/v1/)

<details><summary>Исторический снимок ветки plugin-kit-ai v1. 1.2.4 — базовая версия команд, а не точная версия этого снимка.</summary>


# Интеграция с CI

Самая безопасная история для CI не обязана быть сложной. Она просто должна строго проверять публичный контракт проекта.

<MermaidDiagram
  :chart="`
flowchart LR
  Doctor[doctor] --> Bootstrap[bootstrap when needed]
  Bootstrap --> Generate[generate]
  Generate --> Validate[validate --strict]
  Validate --> Smoke[smoke or bundle checks]
`"
/>

## Минимальный CI gate

Для большинства authored projects базовый путь такой:

```bash
plugin-kit-ai doctor .
plugin-kit-ai generate .
plugin-kit-ai validate . --platform <target> --strict
```

Если у вашего path есть stable smoke tests или bundle checks, добавляйте их после validation gate, а не вместо него.

## Почему это работает

- `doctor` рано ловит отсутствующие runtime prerequisites
- `generate` доказывает, что generated outputs можно воспроизвести из исходного состояния проекта
- `validate --strict` доказывает, что repo внутренне согласован для выбранного target
- для multi-target repo эта же логика должна выполняться по каждому target’у в support scope

## Заметки по runtime

### Go

Go — самый чистый CI path, потому что execution machine не обязана иметь Python или Node просто для удовлетворения runtime path.

Для launcher-based Go repo сначала собирайте проверяемый launcher:

```bash
go build -o bin/my-plugin ./cmd/my-plugin
plugin-kit-ai doctor .
plugin-kit-ai generate .
plugin-kit-ai validate . --platform codex-runtime --strict
```

### Node/TypeScript

Явно добавляйте bootstrap:

```bash
plugin-kit-ai doctor .
plugin-kit-ai bootstrap .
plugin-kit-ai generate .
plugin-kit-ai validate . --platform codex-runtime --strict
```

### Python

Используйте тот же паттерн, что и для Node, и явно фиксируйте версию Python в CI.

## Частые ошибки в CI

- запускать `validate --strict` без `generate`
- воспринимать generated artifacts как вручную поддерживаемые файлы
- забывать про runtime prerequisites для Node или Python paths
- обещать совместимость для target, который находится вне stable support boundary

## Рекомендуемое правило

Если CI не может воспроизвести authored outputs и пройти `validate --strict`, repo не готов к стабильному handoff. Для multi-target repo нужен явный зелёный прогон по каждому target’у в support scope.

Свяжите эту страницу с [Готовностью к продакшену](/ru/guide/production-readiness), [Границей поддержки](/ru/reference/support-boundary) и [Диагностикой проблем](/ru/reference/troubleshooting).

</details>
