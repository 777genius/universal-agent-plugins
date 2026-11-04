---
title: "SDK Go Hooks"
description: "Handlers tipados para hooks nativos em Go."
canonicalId: "page:sdk:go-hooks"
section: "sdk"
locale: "pt"
generated: false
translationRequired: true
localeDisposition: "current-translation"
---

# SDK Go Hooks

O SDK Go registra **handlers tipados para hooks nativos**. Ele não instala plugins nem transforma hooks automaticamente em componentes portáteis Skills/MCP.

## Instale o módulo publicado

```bash
go get github.com/777genius/plugin-kit-ai/sdk@v1.3.0
```

O módulo declara Go 1.22 e toolchain Go 1.26.10. O código em `main` pode ter eventos ausentes da versão `v1.3.0`.

## Exemplo

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

`New(Config)` cria o aplicativo; `Claude()`, `Codex()`, `Gemini()` e `Qoder()` disponibilizam registradores; `Use` adiciona middleware e `Run()` / `RunContext(ctx)` processa uma chamada. O agente deve configurar separadamente o executável, evento e payload; isso não comprova instalação nativa.

## Suporte e execução

A estabilidade é **por evento**. A [documentação](https://github.com/777genius/universal-agent-plugins/blob/main/sdk/README.md) descreve Claude `Stop` / `PreToolUse` / `UserPromptSubmit`, Codex `Notify`, nove hooks Gemini e hooks Qoder; outros são `public-beta`. O observador de stop do Cursor continua em **beta** e não comprova suporte nativo completo.

`Supported()` reflete o runtime, não o instalador. `MaxPayloadBytes` limita a entrada. Reserve stdout para a resposta do protocolo e stderr para logs. Um executável compartilhado pode usar [detecção](./host-detection), desde que não leia stdin duas vezes sem um `Config.IO` que reproduza os dados.

[Política de estabilidade](https://github.com/777genius/universal-agent-plugins/blob/main/sdk/STABILITY.md).
