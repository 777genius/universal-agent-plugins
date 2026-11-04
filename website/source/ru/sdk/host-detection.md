---
title: "Определение агента"
description: "Отличие Claude и Codex по payload вызванного hook."
canonicalId: "page:sdk:host-detection"
section: "sdk"
locale: "ru"
generated: false
translationRequired: true
localeDisposition: "current-translation"
---

# Как определить вызвавшего агента

`github.com/777genius/plugin-kit-ai/sdk/hostdetect` распознаёт **хост текущего hook-процесса**. Это **не** обнаружение установленных клиентов и не поиск профилей.

## Встроенные сигналы

`DefaultRegistry()` знает только **Codex** и **Claude Code**:

| Признак в верхнем уровне JSON | Результат |
| --- | --- |
| `turn_id` | `PlatformCodex` |
| `hook_event_name` без `turn_id` | `PlatformClaude` |
| Нет подходящего сигнала или JSON некорректен/превышает `MaxPayloadBytes` | `PlatformUnknown` |

Сначала проверяется Codex, затем Claude. Переменные `PLUGIN_ROOT` и `CLAUDE_PLUGIN_ROOT` **не подходят** для такого определения: Codex может устанавливать обе. Поэтому в `DefaultRegistry` нет env-маркеров. Старый Codex notify в аргументах командной строки, где ключ называется `turn-id`, этим детектором stdin JSON тоже не распознаётся.

## Пример

```go
payload := []byte("{\"hook_event_name\":\"Stop\",\"session_id\":\"example\"}")
host, err := hostdetect.Detect(hostdetect.DefaultRegistry(), "", nil, payload)
if err != nil { return err }
switch host {
case hostdetect.PlatformClaude:
    // Claude hook
case hostdetect.PlatformCodex:
    // Codex hook
case hostdetect.PlatformUnknown:
    // Не выбирайте произвольный агент по умолчанию.
}
```

Подключите `github.com/777genius/plugin-kit-ai/sdk/hostdetect` и используйте пример внутри функции, возвращающей error. Он демонстрирует API, но не читает реальный stdin. При совместном использовании с Hooks SDK сохраняйте payload и передавайте его через согласованный `Config.IO`.

## Порядок Detect и расширение

`Detect(registry, override, env, payload)` применяет: **(1)** непустой корректный override; **(2)** env-маркеры из упорядоченного пользовательского Registry; **(3)** сигналы верхнего уровня JSON. Некорректный override приводит к **ошибке**, неизвестное содержимое — к `PlatformUnknown` без ошибки. Типы `Registry`, `Signal`, `Platform` позволяют добавить свои сигналы только при наличии надёжного отличительного признака. Первый совпавший сигнал в одном уровне побеждает.

Пакет существует в **текущем исходном коде**; перед указанием публичной версии убедитесь, что ваш конкретный тег Go-модуля содержит его.

## Если нужна диагностика установленных агентов

Используйте [`discoverTargets` из TypeScript Management SDK](./typescript-management) — он вызывает `manage discover` у закреплённого Go-движка и читает доступность клиентов и приватные профили, не выполняя установку. Для встраивания непосредственно в Go есть [`Engine.Discover`](./go-installer) с ограничением явного списка адаптеров. Ни один вариант не гарантирует успешную нативную активацию.

[Исходники и тесты](https://github.com/777genius/universal-agent-plugins/tree/main/sdk/hostdetect)
