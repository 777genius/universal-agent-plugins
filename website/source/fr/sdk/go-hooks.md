---
title: "SDK Go Hooks"
description: "Écrire des gestionnaires d'événements natifs typés en Go."
canonicalId: "page:sdk:go-hooks"
section: "sdk"
locale: "fr"
generated: false
translationRequired: true
localeDisposition: "current-translation"
---

# SDK Go Hooks

Le SDK Go enregistre des **handlers typés pour les hooks natifs**. Il ne procède pas à l'installation des plugins et ne transforme pas les hooks en composants Skills/MCP portables.

## Installation du module publié

```bash
go get github.com/777genius/plugin-kit-ai/sdk@v1.3.0
```

Le module déclare Go 1.22 avec une directive de toolchain Go 1.26.10. Le code de `main` peut contenir des événements absents de `v1.3.0`.

## Exemple

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

`New(Config)` crée une application ; `Claude()`, `Codex()`, `Gemini()` et `Qoder()` exposent les registres ; `Use` ajoute des middleware ; `Run()` ou `RunContext(ctx)` traite une invocation. L'agent doit lui-même configurer le lancement de l'exécutable, le nom de l'événement et les données. Ce code n'atteste pas une activation native.

## Compatibilité et exécution

Les événements stables documentés incluent Claude `Stop` / `PreToolUse` / `UserPromptSubmit`, Codex `Notify`, les neuf hooks Gemini et les hooks Qoder documentés. D'autres sont `public-beta` ; l'observer stop de Cursor reste **beta**, sans qualification d'installation native. Consultez le [contrat exact](https://github.com/777genius/universal-agent-plugins/blob/main/sdk/README.md) pour le module épinglé.

`Supported()` renseigne sur les **événements runtime**, pas sur les installateurs. `MaxPayloadBytes` borne l'entrée ; stdout reste réservé au protocole de réponse, stderr aux diagnostics. Pour un exécutable partagé, utilisez la [détection](./host-detection) en évitant de consommer stdin deux fois, sauf avec un `Config.IO` adapté.

[Politique de stabilité](https://github.com/777genius/universal-agent-plugins/blob/main/sdk/STABILITY.md).
