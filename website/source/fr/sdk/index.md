---
title: "SDK et intégrations"
description: "Choisir le SDK adapté aux hooks, à la détection ou à la gestion."
canonicalId: "page:sdk:index"
section: "sdk"
locale: "fr"
generated: false
translationRequired: true
localeDisposition: "current-translation"
---

# SDK et intégrations

Ces SDK servent à **intégrer Universal Agent Plugins dans une application** ou à gérer des hooks natifs. Pour une installation ordinaire, consultez [Utiliser des plugins](../use/).

## Choisir l'interface

| Besoin | Guide | API |
| --- | --- | --- |
| Traiter des hooks d'agent | [Go Hooks](./go-hooks) | `sdk` |
| Identifier l'hôte d'un hook | [Détection](./host-detection) | `sdk/hostdetect` |
| Intégrer l'installation en Go | [Go Installer](./go-installer) | `install/integrationctl/agentplugins/installer` |
| Gérer les plugins depuis Node/Electron | [TypeScript Management](./typescript-management) | `universal-agent-plugins-management-sdk` |

## Versions et frontières

- SDK Go Hooks publié : `github.com/777genius/plugin-kit-ai/sdk@v1.3.0`. La stabilité dépend **de chaque événement**.
- `sdk/hostdetect` existe dans le code source actuel ; vérifiez sa présence dans le tag exact que vous consommez.
- Le facade d'installation Go est un module distinct : épinglez la révision et ses dépendances, sans réutiliser la version des hooks.
- Management SDK TypeScript : version source **0.1.1**, Node.js **22.18+**, API de gestion **v1** ; vérifiez la publication npm.
- CLI **0.1.95+** offre `manage serve` et `manage discover`, mais `checkUpdates` n'existe **pas dans le CLI 0.1.95 ni les versions antérieures**.

`hostdetect.Detect` identifie l'agent **qui invoque un hook** ; `discoverTargets` inspecte **les clients natifs et les profils privés** ; `Engine.Discover` décrit les adaptateurs Go enregistrés. Aucune de ces opérations n'installe un plugin. Validation, installation des fichiers, activation native et authentification restent des vérifications distinctes.

Lire la suite : [Go Hooks](./go-hooks), [détection](./host-detection), [Go Installer](./go-installer), [TypeScript SDK](./typescript-management).
