---
title: "Créer un paquet MCP stdio"
description: "Préparez un serveur MCP local en Node et évaluez séparément dépendances et exécution."
canonicalId: "page:build:mcp-stdio"
section: "build"
locale: "fr"
generated: false
translationRequired: true
localeDisposition: "current-translation"
---

# Créer un paquet MCP stdio

**Création depuis le code source :** ces commandes nécessitent la [compilation où la création est activée](./#build-from-source). Elles ne figurent pas dans l’installateur actuel.

Choisissez stdio si le paquet fournit un processus local communiquant par l’entrée et la sortie standard. La structure initiale prend en charge Node.js 22 ou une version ultérieure et utilise le SDK MCP officiel. Sa création n’installe pas les dépendances et ne démarre pas de serveur.

## Créez la structure initiale

Choisissez une destination absente sous un dossier parent existant :

```bash
agentplugins author init ./local-tools \
  --template mcp-stdio \
  --name local-tools \
  --description 'Provide local tools through a Node MCP server' \
  --runtime node
```

`node` est l’environnement d’exécution accepté par ce modèle portable. Une option Python ou Go ne fait pas partie du contrat actuel de cette commande.

## Examinez les fichiers

```text
local-tools/
  plugin.json
  mcp.json
  package.json
  package-lock.json
  src/
    server.mjs
  README.md
  .gitignore
```

`mcp.json` déclare le serveur `local-tools`, avec la commande `node` et l’argument `${PLUGIN_ROOT}/src/server.mjs`. Le paramètre du paquet est résolu dans le paquet sélectionné ; il ne désigne pas le dossier courant du terminal. Le code commence par un petit outil `hello`. Adaptez-le à votre tâche et envoyez les journaux applicatifs sur stderr pour réserver stdout au protocole MCP.

Examinez le manifeste de dépendances et son fichier de verrouillage. `init` les écrit, mais n’exécute pas npm et ne prouve pas que la machine peut importer le SDK.

## Recueillez des preuves statiques

```bash
agentplugins author validate ./local-tools
agentplugins author inspect ./local-tools
agentplugins author test ./local-tools
agentplugins author compat ./local-tools --target codex,claude
agentplugins author doctor ./local-tools
```

Ces vérifications sont statiques. `doctor` examine des métadonnées délimitées du projet et des exécutables ; il ne prouve ni l’installation des dépendances, ni une négociation MCP réussie, ni le fonctionnement d’un outil. Interprétez séparément [chaque niveau de preuve](./checks).

## Installez les dépendances du modèle reconnu

Pour la structure Node générée et son verrouillage correspondant, le code source actuel permet de prévisualiser et d’installer les dépendances :

```bash
agentplugins author bootstrap ./local-tools --dry-run
agentplugins author bootstrap ./local-tools
```

Examinez l’aperçu avant d’appliquer. `bootstrap` utilise une installation npm verrouillée avec les scripts de cycle de vie désactivés. Il accepte uniquement la structure générée reconnue, les fichiers de paquet correspondants et l’absence de `node_modules` ; il ne remplace pas un gestionnaire de paquets général. Des dépendances modifiées, des verrouillages concurrents ou des sorties existantes peuvent provoquer un refus. Corrigez l’exigence indiquée plutôt que de chercher une option d’écrasement. L’acquisition des dépendances peut utiliser le réseau.

L’application de `bootstrap` nécessite Linux et un accès valide aux dossiers par des descripteurs `/proc` ; sinon, elle renvoie `bootstrap_platform_unsupported`. Sur macOS et Windows, `--dry-run` permet d’obtenir le plan, mais cette commande n’applique pas l’installation des dépendances.

## Lancez une vérification MCP explicite

```bash
agentplugins author test ./local-tools --runtime mcp --server local-tools
```

L’exécution stdio nécessite Linux, une délégation cgroup v2 satisfaisant le contrôle de confinement des processus et bubblewrap (`bwrap`) du système avec des espaces de noms utilisables. Node doit être résolu dans les dossiers système `/usr`, `/bin`, `/lib` ou `/lib64`. Si ces prérequis manquent, le test et `dev` refusent le démarrage ; ils ne lancent pas le serveur sans confinement.

Cette commande exécute le processus local sélectionné et vérifie l’initialisation MCP et la découverte des outils. Un appel à `hello` exige un fichier JSON d’arguments explicite ; consultez [l’exemple d’exécution](./checks#evaluate-mcp-runtime-explicitly). Pour une itération continue :

```bash
agentplugins author dev ./local-tools --server local-tools
```

Conservez l’identité exacte du moteur, du paquet et de la plateforme avec le résultat. Une vérification directe de création ne prouve ni que le serveur fonctionne dans tous les agents natifs, ni que le client a accordé sa confiance ou terminé l’authentification.

## Préparez la transmission

Expliquez dans le README les outils, le prérequis Node.js, les fichiers de dépendances et les preuves d’exécution. Indiquez le serveur sélectionné et les outils ou environnements non testés. Partagez le paquet par [la transmission à l’installateur](./handoff). Ajoutez [des Skills](./skills) si les utilisateurs ont besoin de consignes avec les outils.
