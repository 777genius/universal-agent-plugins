---
title: "Detección del agente"
description: "Distinguir Claude y Codex por la carga del hook."
canonicalId: "page:sdk:host-detection"
section: "sdk"
locale: "es"
generated: false
translationRequired: true
localeDisposition: "current-translation"
---

# Detección del agente que invoca un hook

`sdk/hostdetect` identifica el **producto que invoca el proceso hook**, no las aplicaciones instaladas.

## Señales del registro predeterminado

`DefaultRegistry()` solo reconoce **Codex** y **Claude Code**: `turn_id` en el nivel superior del JSON indica Codex; `hook_event_name` sin `turn_id` indica Claude. Payloads inválidos, demasiado grandes (más de `MaxPayloadBytes`) o sin coincidencia producen `PlatformUnknown`.

El orden es Codex antes de Claude. El registro predeterminado **no** confía en `PLUGIN_ROOT` ni `CLAUDE_PLUGIN_ROOT`, ya que Codex puede definir ambas. Tampoco reconoce el antiguo JSON argv de Codex notify con `turn-id`.

## Ejemplo

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

`Detect(registry, override, env, payload)` prioriza un override válido, después los marcadores del registro personalizado y finalmente las claves superiores del JSON. Un override desconocido provoca **error**; sin coincidencia devuelve `PlatformUnknown` sin error. Extiende `Registry`/`Signal` solo con señales fiables. Comprueba que tu **tag publicado** contenga este paquete antes de distribuirlo.

Para encontrar **perfiles y clientes instalados**, usa [`discoverTargets`](./typescript-management) que invoca `manage discover`, o [`Engine.Discover`](./go-installer) de adaptadores Go registrados. Ninguno realiza instalación automática.

[Código y tests](https://github.com/777genius/universal-agent-plugins/tree/main/sdk/hostdetect).
