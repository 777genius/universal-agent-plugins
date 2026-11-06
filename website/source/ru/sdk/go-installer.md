---
title: "Go Installer SDK"
description: "Встраивание установки с планом, подтверждением и восстановлением."
canonicalId: "page:sdk:go-installer"
section: "sdk"
locale: "ru"
generated: false
translationRequired: true
localeDisposition: "current-translation"
---

# Go Installer SDK

Этот SDK нужен, когда ваше Go-приложение встраивает установку, обновление, исправление, удаление, просмотр состояния и восстановление **Agent Plugins**. Он не связан с обработкой hooks.

## Модуль и граница релиза

Публичный facade расположен в `github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/installer`; модуль — `github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins`. Изучите [README](https://github.com/777genius/universal-agent-plugins/blob/main/install/integrationctl/agentplugins/installer/README.md) и [внешний пример](https://github.com/777genius/universal-agent-plugins/blob/main/install/integrationctl/agentplugins/installer/example/main.go). Это **текущий исходный контракт**: фиксируйте ревизию и зависимости, не переносите сюда версию Hooks SDK `v1.3.0`.

## Создание Engine

`Config.StateRoot` должен быть абсолютным чистым путём. `Registry` должен содержать явно выбранные адаптеры. `New` только проверяет настройки: он **не** создаёт директории и не запускает клиент. `Discover` показывает адаптеры, capabilities, scopes и наличие executable, но это не гарантия нативной поддержки всего пакета.

```go
registry, err := clients.NewRegistry(codex.New(), claude.New())
if err != nil { return err }
engine, err := uapinstaller.New(uapinstaller.Config{
    StateRoot: "/absolute/private/uap-state",
    Registry: registry,
})
if err != nil { return err }
_ = engine.Discover()
```

Фрагмент использует импорты из [полного примера](https://github.com/777genius/universal-agent-plugins/blob/main/install/integrationctl/agentplugins/installer/example/main.go). Для реальной доставки MCP задайте нужные `HelperExecutable`, `Runner` и политику доверия исходным файлам.

## Жизненный цикл с подтверждением

```go
req := uapinstaller.Request{
    Operation:        uapinstaller.OpInstall,
    PackageRoot:      "/absolute/reviewed/package",
    ClientID:         "codex",
    ClientConfigRoot: "/absolute/codex-profile",
    ClientExecutable: "/absolute/path/to/codex",
    InstallationID:   "00000000-0000-4000-8000-000000000099",
    OperationID:      "unique-host-operation-id",
}
prepared, err := engine.Prepare(ctx, req)
if err != nil { return err }
defer prepared.Close()
plan := prepared.Plan()
// Покажите пользователю изменения, риски, цели и дайджесты плана.
_ = plan
// Только после осознанного подтверждения:
result, err := engine.Apply(ctx, prepared, uapinstaller.Decision{Confirmed: true})
if err != nil { return err }
_ = result
view, err := engine.Inspect(ctx)
if err != nil { return err }
if view.Recovery.Required {
    _, err = engine.Recover(ctx, view)
    if err != nil { return err }
}
```

Это **фрагмент**, требующий `ctx`, `engine` и импортов. `Prepare` возвращает неизменяемый handle; `Apply` перепроверяет scope, профили, снапшот и согласованный план. `Inspect` читает состояние без мутации, `Recover` принимает именно его типизированное наблюдение. При незавершённой операции нельзя вслепую повторять другой запрос.

## Поддержка и безопасность

- `OpInstall`, `OpUpdate`, `OpRepair`, `OpRefreshProjection`, `OpRemove` — отдельные операции с отдельными планами. `Request.Targets` выбирает двух зарегистрированных клиентов; операции `install-group` нет. Для группы OpenCode и некоторых типов удаления действуют ограничения.
- `ClientConfigRoot` не выводится из `HOME`, `CODEX_HOME` или `CLAUDE_CONFIG_DIR`. Project scope требует явных прав и корня проекта.
- Проверяйте пакет через `Config.Assess` или дайджест-bound `Request.Assessment`. `TrustedLocalPackages: true` уместен только для **заранее доверенных локальных байтов**.
- Регистрация адаптера не доказывает поддержку всех компонентов, native activation или доступ к MCP-сервису. Отсутствующий binding нельзя считать установкой при `repair`.
- Для старого состояния используйте `PrepareStateMigration` / `ApplyStateMigration` после отдельного просмотра; сохраняется резервная копия. Старые незавершённые работы нужно восстановить до миграции.

[Подробный контракт](https://github.com/777genius/universal-agent-plugins/blob/main/install/integrationctl/agentplugins/installer/README.md) · [TypeScript SDK](./typescript-management)
