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

La portée projet explicite est disponible depuis [CLI 0.1.93](https://github.com/777genius/universal-agent-plugins/releases/tag/agentplugins-v0.1.93). Dans la version native actuelle, 0.1.94, `agentplugins install` sans argument de source sélectionne le projet si un fichier `uap.toml` ordinaire existe dans le dossier courant exact ; il ne recherche pas les parents. Une portée explicite reste prioritaire. `add` et les autres commandes utilisent toujours la portée utilisateur par défaut : sélectionnez `--scope project` pour ces opérations visant un projet. La référence native et Homebrew est 0.1.94 ; les canaux peuvent être mis à jour à des moments différents. Vérifiez `agentplugins version` ; la sélection automatique de projet nécessite 0.1.94 ou une version ultérieure, disponible via l’installateur natif ou Homebrew. Les composants pris en charge et l’activation dépendent du client.

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
