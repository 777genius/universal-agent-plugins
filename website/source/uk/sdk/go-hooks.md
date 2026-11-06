---
title: "Go Hooks SDK"
description: "Типізовані hooks і контракти подій агентів."
canonicalId: "page:sdk:go-hooks"
section: "sdk"
locale: "uk"
generated: false
translationRequired: true
localeDisposition: "current-translation"
---

# Go Hooks SDK

Go runtime SDK реєструє **типізовані нативні обробники hooks**; він не встановлює пакети. Встановіть перевірену версію:

```bash
go get github.com/777genius/plugin-kit-ai/sdk@v1.3.0
```

Модуль вимагає Go 1.22 і зазначає toolchain Go 1.26.10. Функції у поточному `main` можуть бути відсутніми у `v1.3.0`.

## Приклад

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

`New(Config)` створює застосунок, `Claude()` / `Codex()` / `Gemini()` / `Qoder()` повертають реєстратори, `Use` додає middleware, а `Run()` або `RunContext(ctx)` обробляє виклик. Нативний агент повинен окремо запустити executable з належною подією та payload — приклад не доводить інсталяції.

## Сумісність і виконання

Стабільність залежить від події: у вихідній [матриці](https://github.com/777genius/universal-agent-plugins/blob/main/docs/generated/support_matrix.md) описано Claude `Stop` / `PreToolUse` / `UserPromptSubmit`, Codex `Notify`, дев'ять hooks Gemini та Qoder; інші можуть бути `public-beta`. Cursor stop observer — **beta**, не повноцінно кваліфікована інтеграція. Перевіряйте саме вибраний реліз.

`Supported()` повідомляє статус **runtime-подій**, а не можливості інсталятора. SDK читає обмежений payload (`MaxPayloadBytes`), виконує обробник і записує відповідь. Не виводьте логів до stdout, залиште його wire-протоколу. Для спільного executable використовуйте [детектор](./host-detection), але не читайте stdin повторно без власного `Config.IO`.

[Вихідний код і політика стабільності](https://github.com/777genius/universal-agent-plugins/blob/main/sdk/README.md).
