---
title: "SDK de instalación en Go"
description: "Integrar instalación y recuperación con confirmación."
canonicalId: "page:sdk:go-installer"
section: "sdk"
locale: "es"
generated: false
translationRequired: true
localeDisposition: "current-translation"
---

# SDK de instalación en Go

El facade de Go integra **install, update, repair, refresh projection, remove, inspect y recover** dentro de una aplicación Go. No procesa hooks nativos.

## Módulo y publicación

```text
github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/installer
```

El módulo `install/integrationctl/agentplugins` es independiente del SDK de hooks. Fija una revisión y dependencias verificadas; **no presupongas** que comparte la versión `v1.3.0` de hooks.

## Crear el engine

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

`StateRoot` debe ser una ruta absoluta y `Registry` un conjunto explícito de adaptadores. `New` solo valida, **no** crea directorios ni ejecuta clientes. `Discover` muestra capacidades y presencia de ejecutables registrados, sin garantizar activación. Selecciona helper, runner, perfil y política de contenido según el plan real.

## Preparar, revisar, aplicar, recuperar

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

El fragmento necesita `ctx`, `engine` e imports del [ejemplo completo](https://github.com/777genius/universal-agent-plugins/blob/main/install/integrationctl/agentplugins/installer/example/main.go). Enseña al usuario el **plan inmutable** antes de `Decision{Confirmed: true}`. `Apply` vuelve a comprobar autoridad y digests. `Inspect` no modifica estado; `Recover` requiere exactamente la observación tipada anterior.

`ClientConfigRoot` nunca se deduce de `HOME`/`CODEX_HOME`. Los bytes locales deben pasar `Assess` o `Request.Assessment` ligada al digest; `TrustedLocalPackages` solo corresponde a material local preautorizado. `Request.Targets` permite grupos, con límites específicos para OpenCode/removal. Las migraciones de estado antiguo utilizan `PrepareStateMigration` y `ApplyStateMigration` con revisión y copia de seguridad.

[Contrato detallado](https://github.com/777genius/universal-agent-plugins/blob/main/install/integrationctl/agentplugins/installer/README.md).
