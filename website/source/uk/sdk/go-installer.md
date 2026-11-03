---
title: "Go Installer SDK"
description: "Вбудоване встановлення з переглядом плану й відновленням."
canonicalId: "page:sdk:go-installer"
section: "sdk"
locale: "uk"
generated: false
translationRequired: true
localeDisposition: "current-translation"
---

# Go Installer SDK

Цей Go facade вбудовує **встановлення, update, repair, refresh projection, remove, inspect і recover** в окремий Go-хост. Це не Hooks SDK.

## Модуль

```text
github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/installer
```

Модуль `install/integrationctl/agentplugins` має окремі версії й контракти. Для інтеграції зафіксуйте перевірену ревізію; версія hooks `v1.3.0` тут не застосовується.

## Створення та виявлення

```go
registry, err := clients.NewRegistry(codex.New(), claude.New())
if err != nil { return err }

engine, err := uapinstaller.New(uapinstaller.Config{
    StateRoot: "/absolute/private/uap-state",
    Registry: registry,
})
if err != nil { return err }

available := engine.Discover()
_ = available // inspect scopes, capabilities and detected executables
```

Потрібні **абсолютний** `StateRoot` та явно зареєстрований `Registry`. `New` не створює файлів і не запускає клієнтів. `Discover` повертає можливості, scopes та присутність executable тільки для цих адаптерів. Для конкретного пакета налаштуйте helper, runner, профілі та політику довіри.

## Перегляд → підтвердження → виконання

```go
req := uapinstaller.Request{
    Operation:        uapinstaller.OpInstall,
    PackageRoot:      "/absolute/reviewed/package",
    ClientID:         "codex",
    ClientConfigRoot: "/absolute/codex-profile",
    ClientExecutable: "/absolute/path/to/codex",
    InstallationID:   "00000000-0000-4000-8000-000000000099",
    OperationID:      "host-generated-unique-operation-id",
}
prepared, err := engine.Prepare(ctx, req)
if err != nil { return err }
defer prepared.Close()

plan := prepared.Plan()
// Show plan, targets, components, warnings and digests to the user.
// Only after explicit approval, with immutable prepared inputs:
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

Фрагмент вимагає `ctx`, `engine` та імпортів із [повного прикладу](https://github.com/777genius/universal-agent-plugins/blob/main/install/integrationctl/agentplugins/installer/example/main.go). Перед `Decision{Confirmed:true}` **покажіть користувачеві незмінний Plan**. `Apply` повторно перевіряє права і дайджести; `Inspect` не змінює стан; `Recover` приймає саме типізоване спостереження.

Не виводьте `ClientConfigRoot` з `HOME`, `CODEX_HOME` або `CLAUDE_CONFIG_DIR`. Для локальних байтів потрібен `Assess` або digest-bound `Request.Assessment`; `TrustedLocalPackages` дозволений лише для явно довіреного комплекту. `Request.Targets` обирає групу, але груповий OpenCode і видалення мають окремі обмеження. Для старих схем — явні `PrepareStateMigration` / `ApplyStateMigration` і резервна копія.

[Повний контракт](https://github.com/777genius/universal-agent-plugins/blob/main/install/integrationctl/agentplugins/installer/README.md).
