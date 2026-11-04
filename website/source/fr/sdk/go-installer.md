---
title: "SDK d'installation Go"
description: "Intégrer l'installation, la validation et la récupération."
canonicalId: "page:sdk:go-installer"
section: "sdk"
locale: "fr"
generated: false
translationRequired: true
localeDisposition: "current-translation"
---

# SDK d'installation Go

Ce facade Go intègre **install, update, repair, refresh projection, remove, inspect et recover** dans votre processus. Il est distinct du runtime des hooks.

## Module

```text
github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/installer
```

Le module `install/integrationctl/agentplugins` est distinct de `sdk`. Épinglez une révision et ses dépendances vérifiées ; ne lui attribuez **pas** la version `v1.3.0` des hooks.

## Construire le moteur

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

`StateRoot` doit être un chemin absolu ; `Registry` contient uniquement les adaptateurs explicitement sélectionnés. `New` valide la configuration mais **ne crée aucun répertoire et n'exécute aucun client**. `Discover` signale les capacités et les exécutables enregistrés, sans promettre leur activation. Configurez helper, runner, profil et politique de confiance pour la livraison choisie.

## Préparer, présenter, appliquer

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

L'extrait requiert `ctx`, `engine` et les imports de l'[exemple complet](https://github.com/777genius/universal-agent-plugins/blob/main/install/integrationctl/agentplugins/installer/example/main.go). Présentez le **plan immuable** avant `Decision{Confirmed: true}`. `Apply` revalide les droits et les digests ; `Inspect` est en lecture seule ; `Recover` reçoit exactement l'observation typée correspondante.

`ClientConfigRoot` ne se déduit pas de `HOME` ou `CODEX_HOME`. Le contenu local exige un `Assess` ou `Request.Assessment` lié au digest ; `TrustedLocalPackages` est réservé aux octets locaux explicitement préapprouvés. `Request.Targets` gère certains groupes, avec des limites pour OpenCode et la suppression. La migration d'un ancien état se fait via `PrepareStateMigration` et `ApplyStateMigration` après revue et sauvegarde.

[Contrat détaillé](https://github.com/777genius/universal-agent-plugins/blob/main/install/integrationctl/agentplugins/installer/README.md).
