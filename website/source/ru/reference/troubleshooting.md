---
title: "Диагностика проблем"
description: "Быстрые recovery steps для самых частых проблем с install, generate, validate и bootstrap."
canonicalId: "page:reference:troubleshooting"
section: "reference"
locale: "ru"
generated: false
translationRequired: true
---

[Использовать плагины](/ru/use/) · [Создавать плагины](/ru/build/) — Подготовка, не релиз. Миграция проектов в v2 пока недоступна.
<p class="locale-historical-identity">Диагностика проблем</p>

[Исторический снимок ветки plugin-kit-ai v1. 1.2.4 — базовая версия команд, а не точная версия этого снимка.](/en/legacy/v1/)

<details><summary>Исторический снимок ветки plugin-kit-ai v1. 1.2.4 — базовая версия команд, а не точная версия этого снимка.</summary>


# Диагностика проблем

Используйте эту страницу, когда workflow перестал двигаться. Начинайте с самой простой проверки.

## CLI установился, но не запускается

Проверьте, что binary действительно находится в shell `PATH`.

Если вы ставили CLI через npm или PyPI, убедитесь, что пакет реально скачал опубликованный binary. Не воспринимайте wrapper package как runtime.

## Python или Node runtime-repo падают слишком рано

Сначала проверьте сам runtime:

- Python runtime repo требуют Python `3.10+`
- Node runtime repo требуют Node.js `20+`

Используйте `plugin-kit-ai doctor <path>`, прежде чем считать, что сломан сам repo.

Типовой recovery flow:

```bash
plugin-kit-ai doctor ./my-plugin
plugin-kit-ai bootstrap ./my-plugin
plugin-kit-ai generate ./my-plugin
plugin-kit-ai validate ./my-plugin --platform codex-runtime --strict
```

## Падает `validate --strict`

Воспринимайте это как сигнал, а не как шум.

Частые причины:

- generated artifacts устарели, потому что был пропущен `generate`
- выбранная platform не соответствует project source
- runtime path всё ещё требует bootstrap или исправления окружения

## `generate` выдаёт не то, что ожидалось

Обычно это значит, что project source и ваша ментальная модель разошлись.

Сначала перепроверьте package-standard layout, а не редактируйте generated target files вручную, чтобы силой подогнать output.

## Я не понимаю, какой path выбрать

Начинайте с default Go path, если нужен самый сильный контракт.

Переходите на Node/TypeScript или Python только тогда, когда локальный runtime tradeoff действительно осознан и нужен.

См. [Python runtime-плагин](/ru/guide/python-runtime), [Процесс авторинга](/ru/reference/authoring-workflow) и [Частые вопросы](/ru/reference/faq).

</details>
