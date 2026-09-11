---
title: "Соберите Python runtime-плагин"
description: "Простой end-to-end путь для локального Python-плагина."
canonicalId: "page:guide:python-runtime"
section: "guide"
locale: "ru"
generated: false
translationRequired: true
---

[Использовать плагины](/ru/use/) · [Создавать плагины](/ru/build/) — Подготовка, не релиз. Миграция проектов в v2 пока недоступна.
<p class="locale-historical-identity">Соберите Python runtime-плагин</p>

[Исторический снимок ветки plugin-kit-ai v1. 1.2.4 — базовая версия команд, а не точная версия этого снимка.](/en/legacy/v1/)

<details><summary>Исторический снимок ветки plugin-kit-ai v1. 1.2.4 — базовая версия команд, а не точная версия этого снимка.</summary>


# Соберите Python runtime-плагин

Используйте этот путь, когда команда уже пишет на Python и вы хотите запускать плагин прямо из этого репозитория.

Если нужен один скомпилированный бинарник и самый простой путь к распространению, лучше выбрать Go. Python здесь подходит для сценария, где репозиторий остаётся основным местом разработки и запуска плагина.

## Выберите Python-путь за 10 секунд

Используйте обычный Python-путь, когда нужен самый простой первый репозиторий:

```bash
plugin-kit-ai init my-plugin --platform codex-runtime --runtime python
```

Используйте путь с общим пакетом, когда хотите импортировать `plugin_kit_ai_runtime` из `requirements.txt` в нескольких репозиториях:

```bash
plugin-kit-ai init my-plugin --platform codex-runtime --runtime python --runtime-package
```

Если сомневаетесь, сначала берите обычный путь по умолчанию.

## Что даёт этот путь

- один репозиторий плагина
- Python `3.10+` на машине, где будет запускаться плагин
- локальный `.venv`
- поддерживаемый Python-сценарий для `codex-runtime` или `claude`
- одна главная проверка перед коммитом или передачей репозитория: `validate --strict`

## Если нужен только самый короткий путь

Скопируйте это и дойдите до первого зелёного прогона:

```bash
brew install 777genius/homebrew-plugin-kit-ai/plugin-kit-ai
plugin-kit-ai init my-plugin --platform codex-runtime --runtime python
cd my-plugin
plugin-kit-ai doctor .
plugin-kit-ai bootstrap .
plugin-kit-ai generate .
plugin-kit-ai validate . --platform codex-runtime --strict
plugin-kit-ai test . --platform codex-runtime --event notify
```

На `--runtime-package` переходите только тогда, когда требование общего dependency уже реально появилось.

## 1. Установите CLI

```bash
brew install 777genius/homebrew-plugin-kit-ai/plugin-kit-ai
plugin-kit-ai version
```

## 2. Создайте Python-проект

Для обычного Python-first пути под Codex:

```bash
plugin-kit-ai init my-plugin --platform codex-runtime --runtime python
cd my-plugin
```

Если первым реальным требованием являются Claude hooks, создайте сразу Claude-проект:

```bash
plugin-kit-ai init my-plugin --platform claude --runtime python
cd my-plugin
```

## 3. Подготовьте локальное Python-окружение

```bash
plugin-kit-ai doctor .
plugin-kit-ai bootstrap .
```

`doctor` показывает, готов ли репозиторий к запуску.

`bootstrap` создаёт `.venv`, когда это нужно, и ставит `requirements.txt`.

## 4. Сгенерируйте и провалидируйте проект

```bash
plugin-kit-ai generate .
plugin-kit-ai validate . --platform codex-runtime --strict
```

`generate` обновляет сгенерированные launcher и config-файлы из ваших исходных файлов.

Если вы начали с Claude, поменяйте target у `validate`:

```bash
plugin-kit-ai validate . --platform claude --strict
```

## 5. Добавьте свою Python-логику

Скаффолд по умолчанию хранит helper локально в `plugin/plugin_runtime.py`, поэтому первый проект остаётся самодостаточным.

Типовая форма Codex starter:

```python
from plugin_runtime import CodexApp, continue_

app = CodexApp()


@app.on_notify
def on_notify(event):
    _ = event
    return continue_()


if __name__ == "__main__":
    raise SystemExit(app.run())
```

Основную логику редактируйте в `plugin/main.py`. Оставляйте stdout только для ответов инструмента, а диагностику пишите в stderr.

## 6. Запустите smoke test

Для пути `codex-runtime`:

```bash
plugin-kit-ai test . --platform codex-runtime --event notify
```

Можно и напрямую запустить сгенерированный launcher:

```bash
./bin/my-plugin notify '{"client":"codex-tui"}'
```

Для Claude самая простая проверка такая:

```bash
plugin-kit-ai test . --platform claude --all
```

## 7. Когда нужен общий Python package

Оставайтесь на локальном helper по умолчанию, если нужен самый простой первый репозиторий.

Переключайтесь на shared dependency path, если хотите использовать один и тот же helper package в нескольких репозиториях:

```bash
plugin-kit-ai init my-plugin --platform codex-runtime --runtime python --runtime-package
```

В этом режиме проект импортирует [`plugin_kit_ai_runtime`](/ru/api/runtime-python/plugin-kit-ai-runtime) из опубликованного пакета [`plugin-kit-ai-runtime`](https://github.com/777genius/plugin-kit-ai/tree/main/python/plugin-kit-ai-runtime), а не генерирует локальный `plugin/plugin_runtime.py`.

Если вы используете локальную development-сборку CLI из этого исходного дерева, передавайте `--runtime-package-version` явно во время `init`.
Стабильные released CLI подбирают подходящую версию helper package автоматически.

## Короткое правило

- выбирайте Python, когда команда уже живёт в Python и плагин остаётся локальным для репозитория
- выбирайте Go, когда нужен самый чистый packaging и distribution story
- используйте `doctor -> bootstrap -> generate -> validate --strict` как основной Python-сценарий
- переходите на `--runtime-package` только тогда, когда действительно нужен shared dependency

## Что читать дальше

- Прочитайте [Выбор runtime](/ru/concepts/choosing-runtime), чтобы понять tradeoffs.
- Прочитайте [Выбор модели поставки](/ru/guide/choose-delivery-model) для решения между local helper и shared package.
- Откройте [Python Runtime API](/ru/api/runtime-python/), когда понадобится справочник по helper API.

</details>
