---
title: "Go Hooks SDK"
description: "Типизированные нативные hooks Claude, Codex, Gemini и границы стабильности."
canonicalId: "page:sdk:go-hooks"
section: "sdk"
locale: "ru"
generated: false
translationRequired: true
localeDisposition: "current-translation"
---

# Go Hooks SDK

SDK Go для hooks предназначен для **типизированных нативных событий** Claude Code, Codex и других поддерживаемых клиентов. Он не устанавливает плагин и не делает hooks частью переносимого пакета Skills/MCP.

## Установка и закрепление версии

```bash
go get github.com/777genius/plugin-kit-ai/sdk@v1.3.0
```

Модуль объявляет Go 1.22 и toolchain Go 1.26.10. Версия `v1.3.0` — подтверждённый опубликованный контракт в [README SDK](https://github.com/777genius/universal-agent-plugins/blob/main/sdk/README.md). События из более свежей ветки `main` не обязательно доступны в этом релизе.

## Минимальный обработчик

```go
package main

import (
    "os"
    uap "github.com/777genius/plugin-kit-ai/sdk"
    "github.com/777genius/plugin-kit-ai/sdk/claude"
    "github.com/777genius/plugin-kit-ai/sdk/codex"
)

func main() {
    app := uap.New(uap.Config{Name: "review-hooks"})
    app.Claude().OnStop(func(event *claude.StopEvent) *claude.Response {
        return claude.Allow()
    })
    app.Codex().OnNotify(func(event *codex.NotifyEvent) *codex.Response {
        return codex.Continue()
    })
    os.Exit(app.Run())
}
```

`New(Config)` создаёт приложение; регистраторы подключают обработчики; `Run()` обрабатывает **один вызов** процесса. Нативный клиент должен **сам** быть настроен на запуск с правильным именем события и payload. Этот пример доказывает API регистрации, а не успешную установку в агент. См. [проверенные примеры](https://github.com/777genius/universal-agent-plugins/blob/main/sdk/example_test.go).

## API и уровень стабильности

- Корневые функции: `New`, `App.Use`, `App.Run`, `App.RunContext`, `Supported()`; регистраторы `Claude()`, `Codex()`, `Gemini()`, `Qoder()`.
- Пакеты типов: `sdk/claude`, `sdk/codex`, `sdk/gemini`, `sdk/qoder`. Наличие пакета в `main` не означает его включение в выбранный релиз.
- В документации исходников стабильно поддержаны отдельные hooks Claude (`Stop`, `PreToolUse`, `UserPromptSubmit`), Codex `Notify`, девять hooks Gemini и описанные Qoder hooks. Прочие события могут иметь статус `public-beta`.
- `Cursor` stop observer остаётся **beta**. Его протокольная проверка **не** подтверждает наличие полноценной нативной интеграции Cursor.

Точную поддержку проверяйте по `Supported()` и [матрице событий](https://github.com/777genius/universal-agent-plugins/blob/main/docs/generated/support_matrix.md) **с учётом версии**. `Supported()` описывает runtime, не поддержку установщика или авторизации.

## Как работает вызов

1. Агент запускает hook executable и передаёт нативное событие.
2. `RunContext(ctx)` разрешает platform/event, декодирует ограниченный payload и выполняет middleware/handler.
3. SDK записывает ожидаемый ответ и код выхода. **Не выводите произвольные логи в stdout**: он занят wire-протоколом. Для диагностики используйте stderr.
4. Отдельно проверяйте реальную конфигурацию хоста, доступность executable, payload и повторную загрузку агента.

`Config{Args, IO, Env, Logger}` помогает внедрить окружение при тестировании. `App.Use` добавляет middleware до запуска; `MaxPayloadBytes` ограничивает размер данных. Для общего бинарника разных агентов используйте [Host detection](./host-detection), но не читайте stdin дважды без replayable `Config.IO`.

## Если что-то не работает

Проверьте имя события и версию SDK, нативную конфигурацию hooks, путь и права executable, транспорт stdin/argv и формат JSON. Успешный Go unit test **не** подтверждает активацию в установленном клиенте.

[Исходники и ограничения](https://github.com/777genius/universal-agent-plugins/blob/main/sdk/README.md) · [Определение агента](./host-detection)
