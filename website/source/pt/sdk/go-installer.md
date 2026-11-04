---
title: "SDK Go Installer"
description: "Integrar instalações com revisão e recuperação."
canonicalId: "page:sdk:go-installer"
section: "sdk"
locale: "pt"
generated: false
translationRequired: true
localeDisposition: "current-translation"
---

# SDK Go Installer

O facade Go incorpora **install, update, repair, refresh projection, remove, inspect e recover** ao aplicativo. É distinto do SDK de hooks.

## Módulo

```text
github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/installer
```

O módulo `install/integrationctl/agentplugins` tem seu próprio contrato. Fixe uma revisão e dependências revisadas; não presuma a versão `v1.3.0` do módulo de hooks.

## Criação

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

`StateRoot` deve ser absoluto e `Registry` deve registrar explicitamente os clientes permitidos. `New` valida, mas **não cria pastas nem executa agentes**. `Discover` mostra capacidade e presença de executáveis registrados, sem provar ativação. Configure helper, runner, perfil e política de confiança para a entrega escolhida.

## Preparar, revisar e aplicar

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

O trecho depende de `ctx`, `engine` e dos imports do [exemplo completo](https://github.com/777genius/universal-agent-plugins/blob/main/install/integrationctl/agentplugins/installer/example/main.go). Mostre o **plano imutável** antes de `Decision{Confirmed: true}`. `Apply` revalida autoridade e digests; `Inspect` é somente leitura; `Recover` exige a observação tipada exata.

`ClientConfigRoot` não vem automaticamente de `HOME` ou `CODEX_HOME`. Bytes locais exigem `Assess` ou `Request.Assessment` vinculado ao digest; `TrustedLocalPackages` vale apenas para material explicitamente pré-autorizado. `Request.Targets` suporta grupos com limites específicos para OpenCode/remoção. A migração de estado antigo é explícita via `PrepareStateMigration` / `ApplyStateMigration`, com revisão e backup.

[Contrato completo](https://github.com/777genius/universal-agent-plugins/blob/main/install/integrationctl/agentplugins/installer/README.md).
