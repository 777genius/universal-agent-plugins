---
title: "Utiliser des plugins"
description: "Installez des paquets dans vos agents et séparez les dépendances de chaque projet."
canonicalId: "page:use:index"
section: "use"
locale: "fr"
generated: false
translationRequired: true
localeDisposition: "current-translation"
---

# Utiliser des plugins

Universal Agent Plugins installe des paquets standard pour les agents d’IA de votre choix. Un paquet peut contenir des Skills, une configuration de serveurs MCP ou les deux. Vous pouvez l’installer pour votre compte ou garder les dépendances d’un projet séparées.

## Obtenez le CLI

Avec Node.js 22 ou une version ultérieure, lancez directement le wrapper npm :

```bash
npx universal-agent-plugins search docs
```

Pour un usage régulier, installez-le globalement :

```bash
npm install -g universal-agent-plugins
agentplugins version
```

Homebrew est une autre possibilité :

```bash
brew install 777genius/agentplugins/agentplugins
```

Le [guide d’installation native](https://github.com/777genius/universal-agent-plugins/blob/main/docs/NATIVE_INSTALL.md) donne les instructions pour macOS, Linux et Windows. Le gestionnaire natif ne nécessite pas Node.js, mais certains plugins peuvent avoir besoin de leur propre environnement d’exécution. Les exemples de ce parcours utilisent `agentplugins` ; `npx universal-agent-plugins` exécute les mêmes commandes d’installation.

## Suivez le parcours d’installation

1. [Recherchez et examinez un paquet](./install). Vérifiez sa provenance et ses prérequis.
2. Prévisualisez les cibles et la portée prévues avec `--dry-run`.
3. Appliquez l’installation examinée et suivez les instructions d’activation.
4. [Maintenez l’état installé](./manage) : liste, mise à jour, réparation et suppression.
5. Au besoin, restaurez des dépendances partagées depuis `uap.toml` et `uap.lock`.

[CLI 0.1.95](https://github.com/777genius/universal-agent-plugins/releases/tag/agentplugins-v0.1.95) est l’installateur natif actuel. Il ajoute une API de gestion locale et des extensions natives Pi en portée utilisateur. Consultez [la gestion locale](./manage#local-management-api) et [la compatibilité Pi](../reference/client-compatibility#pi-native-extensions). Les versions peuvent varier selon le canal ; vérifiez `agentplugins version` et utilisez l’installateur natif pour obtenir la version vérifiée.

La portée projet explicite est disponible depuis 0.1.93. La plupart des commandes utilisent par défaut la portée utilisateur. Depuis 0.1.94, `install` sans argument de source ni portée explicite sélectionne le projet uniquement si le dossier courant exact contient un fichier `uap.toml` ordinaire ; il ne recherche pas dans les parents. Une portée explicite reste prioritaire. Sélectionnez `--scope project` pour la maintenance d’un projet. La prise en charge des composants et l’activation dépendent du client.

## Interprétez chaque résultat

Un rechargement, une demande de confiance, une activation manuelle ou une étape OAuth peuvent rester nécessaires après l’installation. Le résultat Prepared signifie que des étapes restent à effectuer dans le client. Lisez le résultat de chaque cible : l’opération complète ne prouve pas à elle seule que le plugin fonctionne à l’exécution. Consultez la [compatibilité des clients](../reference/client-compatibility) pour connaître la portée et les versions des observations natives.

## Si vous créez un paquet

Commencez par [Créer des plugins](../build/). Le livrable commun est un dossier avec `plugin.json` à la racine et les composants du paquet. La personne qui installe ce paquet n’a pas besoin d’exécuter les outils de création compilés depuis le code source.

| Commande | Rôle |
| --- | --- |
| `agentplugins validate ./my-plugin` | Lire et valider un paquet local pour l’installation. |
| `agentplugins doctor` | Diagnostiquer l’état géré par l’installateur. |
| `agentplugins author validate ./my-plugin` | Vérifier la conformité et la préparation d’un paquet dans une compilation où la création est activée. |
| `agentplugins author doctor ./my-plugin` | Examiner les preuves du projet et de sa chaîne d’outils dans cette compilation. |

La version actuelle de l’installateur npm ou natif n’inclut pas `agentplugins author`.
