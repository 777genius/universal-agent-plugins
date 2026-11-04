---
title: "Визначення агента"
description: "Визначення Claude або Codex за даними hook."
canonicalId: "page:sdk:host-detection"
section: "sdk"
locale: "uk"
generated: false
translationRequired: true
localeDisposition: "current-translation"
---

# Визначення агента для hooks

`sdk/hostdetect` визначає, **який агент викликав поточний hook**, а не чи встановлений агент.

## Правила

`DefaultRegistry()` містить тільки **Codex** і **Claude Code**: верхній ключ `turn_id` означає Codex; `hook_event_name` без `turn_id` означає Claude. Некоректний, завеликий або невідомий payload повертає `PlatformUnknown`. Порядок — спершу Codex.

Змінні `PLUGIN_ROOT` і `CLAUDE_PLUGIN_ROOT` ненадійні, бо Codex може встановлювати обидві; тому стандартний Registry не містить env-маркерів. Старий Codex notify з argv-ключем `turn-id` не входить до розпізнавання stdin JSON.

## Приклад

```go
package main

import (
    "fmt"

    "github.com/777genius/plugin-kit-ai/sdk/hostdetect"
)

func main() {
    payload := []byte(`{"hook_event_name":"Stop","session_id":"example"}`)
    host, err := hostdetect.Detect(hostdetect.DefaultRegistry(), "", nil, payload)
    if err != nil {
        panic(err)
    }
    switch host {
    case hostdetect.PlatformClaude:
        fmt.Println("Claude hook")
    case hostdetect.PlatformCodex:
        fmt.Println("Codex hook")
    default:
        fmt.Println("Unknown; do not assume a default agent")
    }
}
```

`Detect(registry, override, env, payload)` перевіряє явний коректний override, потім маркери власного впорядкованого Registry, потім обмежений JSON. Невідомий override — **помилка**, відсутність збігу — `PlatformUnknown` без помилки. Додавайте `Signal` лише з перевіреною відмінною ознакою. Перевірте, чи є цей пакет у вашому опублікованому Go-тегу.

Для пошуку **встановлених клієнтів** використовуйте [`discoverTargets`](./typescript-management) через `manage discover` або [`Engine.Discover`](./go-installer) для явно зареєстрованих Go-адаптерів. Ці методи не встановлюють плагіни.

[Вихідний код](https://github.com/777genius/universal-agent-plugins/tree/main/sdk/hostdetect).
