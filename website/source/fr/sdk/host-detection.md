---
title: "Détection de l'agent"
description: "Distinguer Claude et Codex à partir du hook."
canonicalId: "page:sdk:host-detection"
section: "sdk"
locale: "fr"
generated: false
translationRequired: true
localeDisposition: "current-translation"
---

# Détecter l'agent qui invoque un hook

`sdk/hostdetect` identifie l'agent **qui appelle le processus de hook** ; ce n'est pas un inventaire des agents installés.

## Signaux

`DefaultRegistry()` reconnaît **Codex** et **Claude Code** uniquement. La clé JSON de premier niveau `turn_id` indique Codex ; `hook_event_name` sans `turn_id` indique Claude. Un payload invalide, trop grand (`MaxPayloadBytes`) ou inconnu produit `PlatformUnknown`.

Le registre examine Codex avant Claude. Il ne se fie **pas** à `PLUGIN_ROOT` ni `CLAUDE_PLUGIN_ROOT`, car Codex peut définir les deux. L'ancien JSON argv de Codex notify avec `turn-id` n'est pas reconnu par ce détecteur de stdin.

## Exemple

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

`Detect(registry, override, env, payload)` privilégie un override valide, puis les marqueurs d'un registre personnalisé et enfin les clés de premier niveau JSON. Un override inconnu est une **erreur** ; aucune correspondance renvoie `PlatformUnknown` sans erreur. Ajoutez des `Signal` uniquement avec des indices fiables. Vérifiez que ce package figure dans votre tag Go réellement publié.

Pour détecter **les profils et clients installés**, utilisez [`discoverTargets`](./typescript-management) via `manage discover`, ou [`Engine.Discover`](./go-installer) pour les adaptateurs Go explicitement enregistrés. Rien n'est installé par ces appels.

[Code et tests](https://github.com/777genius/universal-agent-plugins/tree/main/sdk/hostdetect).
