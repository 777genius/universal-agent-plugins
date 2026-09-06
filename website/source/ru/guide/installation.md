---
title: "Установка"
description: "Установка plugin-kit-ai через поддерживаемые каналы."
canonicalId: "page:guide:installation"
section: "guide"
locale: "ru"
generated: false
translationRequired: true
---

[Использовать плагины](/ru/use/) · [Создавать плагины](/ru/build/) — Подготовка, не релиз. Миграция проектов в v2 пока недоступна.
<details><summary>Исторические материалы plugin-kit-ai v1 · 1.2.4; старые инструкции не являются текущим Build.</summary>


# Установка

Используйте `npx`, если хотите максимально быстро попробовать первую установку плагина. Используйте Homebrew, если plugin-kit-ai нужен для ежедневной работы.

## Самая быстрая первая установка плагина

Это опциональная проверка без собственного repo, что опубликованный install flow действительно жив.
Она не создаёт repo плагина, который вы будете редактировать.

```bash
npx plugin-kit-ai@latest add notion
```

- Эта команда ставит все поддерживаемые outputs этого плагина.
- Если ваша цель - авторский plugin repo, переходите в [Быстрый старт](/ru/guide/quickstart) и начинайте с `plugin-kit-ai init ...`.

## Поддерживаемые каналы

- Homebrew для самого чистого CLI пути.
- npm, если у вас среда уже завязана на npm.
- PyPI / pipx, если у вас среда уже завязана на Python.
- Verified install script как запасной путь.

## Рекомендуемые команды

### Homebrew

```bash
brew install 777genius/homebrew-plugin-kit-ai/plugin-kit-ai
plugin-kit-ai version
```

### npm

```bash
npm i -g plugin-kit-ai
plugin-kit-ai version
```

### PyPI / pipx

```bash
pipx install plugin-kit-ai
plugin-kit-ai version
```

### Verified Script

```bash
curl -fsSL https://raw.githubusercontent.com/777genius/plugin-kit-ai/main/scripts/install.sh | sh
plugin-kit-ai version
```

Чтобы установить CLI и сразу посмотреть план установки реального universal plugin без Node/npm:

```bash
curl -fsSL https://raw.githubusercontent.com/777genius/plugin-kit-ai/main/scripts/install.sh | sh -s -- add notion --dry-run
```

## Что выбирать большинству людей?

- Выбирайте `npx`, если хотите самый короткий первый запуск без постоянной установки.
- Выбирайте Homebrew, если вы на macOS и хотите самый удобный путь для ежедневной работы.
- Выбирайте npm или pipx только тогда, когда это уже соответствует среде вашей команды.
- Используйте verified script как запасной путь вне сценариев, где всё уже крутится вокруг пакетного менеджера, в том числе для one-shot команд через `sh -s -- ...`.

## Путь для CI

Для CI лучше использовать dedicated setup action, а не учить каждый workflow вручную скачивать CLI.

## Что читать после установки

Большинству людей стоит сразу перейти к [Быстрому старту](/ru/guide/quickstart), сначала попробовать реальный плагин, а потом создать первый repo на job-first пути под свою задачу.

Если вы выбрали `pipx`, потому что команда уже Python-first и вам заранее нужен Python-путь, переходите к [Python runtime-плагину](/ru/guide/python-runtime).

## Важная граница

npm и PyPI пакеты — это способы установить CLI binary. Они не считаются публичным runtime API и не являются SDK.

См. [Справочник > Каналы установки](/ru/reference/install-channels) для формальной границы контракта.

</details>
