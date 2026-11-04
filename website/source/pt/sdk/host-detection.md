---
title: "Detecção do agente"
description: "Distinguir Claude e Codex pelo payload de um hook."
canonicalId: "page:sdk:host-detection"
section: "sdk"
locale: "pt"
generated: false
translationRequired: true
localeDisposition: "current-translation"
---

# Detectar o agente que chamou um hook

`sdk/hostdetect` identifica o **host que iniciou o hook**, não os agentes instalados.

## Sinais

`DefaultRegistry()` reconhece apenas **Codex** e **Claude Code**: `turn_id` no nível superior do JSON indica Codex; `hook_event_name` sem `turn_id` indica Claude. Payload inválido, maior que `MaxPayloadBytes` ou sem sinal resulta em `PlatformUnknown`.

A prioridade é Codex antes de Claude. O registro padrão **não usa** `PLUGIN_ROOT` nem `CLAUDE_PLUGIN_ROOT` porque Codex pode definir ambos. O JSON argv antigo do Codex notify com `turn-id` também não é reconhecido por esse detector stdin.

## Exemplo

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

`Detect(registry, override, env, payload)` considera primeiro override válido, depois variáveis do registro personalizado e por fim as chaves superiores do JSON. Override inválido é **erro**; sem correspondência retorna `PlatformUnknown` sem erro. Adicione `Registry`/`Signal` apenas com sinais confiáveis. Verifique que o módulo publicado no tag escolhido contém esse pacote.

Para encontrar **clientes instalados e perfis**, use [`discoverTargets`](./typescript-management) via `manage discover` ou [`Engine.Discover`](./go-installer) para adaptadores registrados. Eles não fazem instalação.

[Código e testes](https://github.com/777genius/universal-agent-plugins/tree/main/sdk/hostdetect).
