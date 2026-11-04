---
title: "SDK de hooks en Go"
description: "Controladores tipados para eventos nativos de agentes."
canonicalId: "page:sdk:go-hooks"
section: "sdk"
locale: "es"
generated: false
translationRequired: true
localeDisposition: "current-translation"
---

# SDK de hooks en Go

El SDK registra **controladores tipados de hooks nativos**. No instala plugins ni convierte automáticamente hooks en componentes portables Skills/MCP.

## Instalar el módulo publicado

```bash
go get github.com/777genius/plugin-kit-ai/sdk@v1.3.0
```

El módulo declara Go 1.22 y toolchain Go 1.26.10. Lo disponible en `main` puede no existir en `v1.3.0`.

## Ejemplo

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

`New(Config)` crea la aplicación; `Claude()`, `Codex()`, `Gemini()` y `Qoder()` proporcionan registros; `Use` agrega middleware y `Run()` / `RunContext(ctx)` atiende una invocación. El agente debe configurar por separado el ejecutable, el evento y el payload; este ejemplo no prueba activación.

## Eventos y compatibilidad

La estabilidad se define **por evento**. El [README](https://github.com/777genius/universal-agent-plugins/blob/main/sdk/README.md) documenta Claude `Stop` / `PreToolUse` / `UserPromptSubmit`, Codex `Notify`, nueve hooks de Gemini y los hooks Qoder indicados; otros pueden ser `public-beta`. El observer de parada de Cursor continúa en **beta**, sin cualificación completa de instalación nativa.

`Supported()` describe eventos de runtime, no soporte de instalador. La entrada está limitada por `MaxPayloadBytes`. Deja stdout reservado para la respuesta del protocolo; registra diagnósticos en stderr. Para un ejecutable compartido consulta la [detección](./host-detection), sin consumir stdin dos veces salvo que implementes un `Config.IO` reutilizable.

[Política de estabilidad](https://github.com/777genius/universal-agent-plugins/blob/main/sdk/STABILITY.md).
