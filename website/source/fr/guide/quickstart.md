---
title: "Démarrage rapide"
description: "Installez des paquets publiés ou préparez votre propre paquet portable Agent Plugins 1.0."
canonicalId: "page:guide:quickstart"
section: "guide"
locale: "fr"
generated: false
translationRequired: true
localeDisposition: "current-translation"
---

<a id="demarrage-rapide"></a>

# Utiliser des plugins / Créer des plugins

Installez un paquet pour vos agents ou créez un paquet portable Agent Plugins 1.0 avec des instructions et une configuration d’outils.

## Utiliser des plugins {#use-plugins}

Avec Node.js 22 ou une version ultérieure :

```bash
npx universal-agent-plugins add context7
```

Le CLI vous demande quels agents compatibles utiliser et indique les étapes d’activation restantes. Pour examiner et prévisualiser d’abord :

```bash
npx universal-agent-plugins info context7
npx universal-agent-plugins add context7 --target codex,cursor --dry-run
```

Le [guide d’installation native](https://github.com/777genius/universal-agent-plugins/blob/main/docs/NATIVE_INSTALL.md) fournit des exécutables pour macOS, Linux et Windows. Le gestionnaire natif ne nécessite pas Node.js, même si certains plugins ont besoin de leur propre environnement d’exécution.

[CLI 0.1.95](https://github.com/777genius/universal-agent-plugins/releases/tag/agentplugins-v0.1.95) conserve la portée projet explicite et la sélection automatique pour `install` introduite dans 0.1.94. Il ajoute aussi une [API de gestion locale](../use/manage#local-management-api) et des [extensions natives Pi](../reference/client-compatibility#pi-native-extensions). Pour prévisualiser l’ajout d’un paquet à un projet :

```bash
npx universal-agent-plugins add context7 --target cursor --scope project \
  --project-root /path/to/project --dry-run
```

Choisissez un dossier de projet existant. `add` utilise par défaut la portée utilisateur : sélectionnez projet explicitement pour cette opération. Depuis [CLI natif 0.1.94](https://github.com/777genius/universal-agent-plugins/releases/tag/agentplugins-v0.1.94), `install` sans argument de source détecte le projet uniquement si un fichier `uap.toml` ordinaire existe dans le dossier courant exact, sans recherche dans les parents. Une portée explicite reste prioritaire. Cette sélection automatique nécessite 0.1.94 ou une version ultérieure : vérifiez `agentplugins version` et utilisez l’installateur natif si votre canal est antérieur. Les canaux peuvent être mis à jour à des moments différents ; les exemples `npx` avec portée explicite de cette page restent compatibles avec 0.1.93. Examinez le plan avant d’omettre `--dry-run`. La découverte native, la confiance, l’authentification et les appels MCP réussis restent distincts de l’écriture des fichiers du paquet.

[Parcours complet d’installation](../use/install) · [Compatibilité des clients et portée des tests](../reference/client-compatibility)

### Restaurez des dépendances enregistrées

Vérifiez ce que l’environnement utilisateur sélectionné enregistre :

```bash
npx universal-agent-plugins deps status
```

Pour un couple `uap.toml`/`uap.lock` enregistré, exportez depuis un dossier d’échange :

```bash
npx universal-agent-plugins deps export
```

Copiez le dossier sur une autre machine, installez les clients cibles, puis exécutez :

```bash
npx universal-agent-plugins deps import
npx universal-agent-plugins install --locked --scope user
```

Seuls l’accord et le verrouillage validés sont transférés. Profils, réglages, identifiants, reçus et données du plugin restent séparés. L’installation nécessite toujours les sources exactes ou des paquets vérifiés en cache. Pour un projet, versionnez les deux fichiers et restaurez avec `install --locked --scope project` explicite. Consultez [la maintenance et la restauration](../use/manage) pour la récupération et le transfert.

## Créer des plugins {#build-plugins}

Un paquet standard commence avec `plugin.json` à sa racine et peut inclure `skills/<name>/SKILL.md`, `mcp.json` ou les deux. Vous pouvez écrire ces fichiers à la main et valider le paquet avec l’installateur publié :

```bash
npx universal-agent-plugins validate ./my-plugin
npx universal-agent-plugins add ./my-plugin --target codex --dry-run
```

Ces exemples supposent que `./my-plugin` contient déjà un paquet complet. Le [guide de création](../build/) décrit un paquet Skill manuel et les outils source de génération d’une structure initiale.

**Disponibilité de la création :** l’installateur actuel npm ou natif n’inclut pas `agentplugins author`. Le code source actuel implémente la génération initiale, les vérifications statiques, les tests MCP explicites, les cycles de développement, l’installation des dépendances et la maintenance JSON. Utilisez [la préparation depuis le code source](../build/#build-from-source) pour ces commandes. Les preuves d’anciennes publications restent liées à leurs versions ; elles ne promettent pas les mêmes fonctions dans l’installateur actuel.

La validation statique ne prouve ni l’exécution, ni OAuth, ni l’activation. Les transports MCP pris en charge varient aussi par client : par exemple, Codex ne prend pas en charge MCP SSE déclaré. Indiquez les prérequis du paquet et les vérifications restantes dans son README.

[Spécification Agent Plugins 1.0](https://agent-plugins.org/specification)


<!-- locale-historical-source:start
locale-historical-frontmatter:start
---
title: "Démarrage rapide"
description: "Le chemin recommandé le plus rapide vers un projet plugin-kit-ai fonctionnel."
canonicalId: "page:guide:quickstart"
section: "guide"
locale: "fr"
generated: false
translationRequired: true
---
locale-historical-frontmatter:end
# Démarrage rapide

Il s'agit du chemin recommandé le plus court lorsque vous souhaitez un dépôt de plugin qui peut ensuite évoluer vers d'autres façons d'expédier le plugin.

Commencez par un chemin solide. Ajoutez ultérieurement des packages, des extensions ou une configuration d'intégration appartenant au référentiel, lorsque le produit en a réellement besoin.

## Si vous ne lisez qu'une chose

Commencez par le chemin Go par défaut, sauf si vous savez déjà que les crochets Claude, Node/TypeScript ou Python définissent les exigences du produit.

Votre premier choix est le point de départ, et non la limite permanente du repo.

## Valeur par défaut recommandée

Si vous n’avez pas de bonnes raisons de choisir une autre voie, commencez ici :

```bash
brew install 777genius/homebrew-plugin-kit-ai/plugin-kit-ai
plugin-kit-ai version
plugin-kit-ai init my-plugin
cd my-plugin
go mod tidy
plugin-kit-ai generate .
plugin-kit-ai validate . --platform codex-runtime --strict
```

Cela vous donne le chemin par défaut le plus solide aujourd'hui : un référentiel d'exécution Go basé sur Codex qui reste facile à valider, à transmettre et à développer ultérieurement.

## Pourquoi c'est la valeur par défaut

- un dépôt dès le premier jour
- l'histoire d'exécution et de version la plus propre aujourd'hui
- la base la plus simple pour les packages, extensions et intégrations ultérieurs

## Ce que vous obtenez

- un dépôt de plugin dès le premier jour
- fichiers créés sous `plugin/`
- généré une sortie d'exécution Codex à partir du même dépôt
- un contrôle de préparation propre via `validate --strict`

## Chemins Node et Python pris en charge

Si votre équipe habite déjà à Node/TypeScript ou Python, ces chemins sont pris en charge et visibles dès le départ :

- `codex-runtime --runtime node --typescript`
- `codex-runtime --runtime python`
- les deux sont des chemins d'exécution interprétés localement, donc la machine cible a toujours besoin de Node.js `20+` ou Python `3.10+`
- Go reste toujours la valeur par défaut lorsque vous souhaitez l'histoire de production générale la plus forte

## Si vous commencez intentionnellement le Node ou Python

Utilisez ce flux alternatif uniquement lorsque le choix de la langue fait déjà partie des exigences du produit :

```bash
plugin-kit-ai init my-plugin --platform codex-runtime --runtime node --typescript
plugin-kit-ai doctor ./my-plugin
plugin-kit-ai bootstrap ./my-plugin
plugin-kit-ai generate ./my-plugin
plugin-kit-ai validate ./my-plugin --platform codex-runtime --strict
```

Ou commencez par Python :

```bash
plugin-kit-ai init my-plugin --platform codex-runtime --runtime python
plugin-kit-ai doctor ./my-plugin
plugin-kit-ai bootstrap ./my-plugin
plugin-kit-ai generate ./my-plugin
plugin-kit-ai validate ./my-plugin --platform codex-runtime --strict
```

## Que faire ensuite

- modifiez le plugin sous `plugin/`
- exécutez à nouveau `plugin-kit-ai generate ./my-plugin` après les modifications
- exécutez à nouveau `plugin-kit-ai validate ./my-plugin --platform codex-runtime --strict`
- ensuite seulement, ajoutez un autre moyen de l'expédier si le produit en a besoin

## Développer plus tard

| Si tu veux | Ajoutez ceci plus tard |
| --- | --- |
| Claude crochets comme produit réel | `claude` |
| Forfait officiel Codex | `codex-package` |
| Gemini package d'extension | `gemini` |
| Configuration de l'intégration appartenant au dépôt | `opencode` ou `cursor` |

Choisissez d'abord `claude` uniquement lorsque les crochets Claude constituent déjà la véritable exigence du produit.

## Ce qui se développera plus tard

- le dépôt reste unifié à mesure que vous ajoutez plus de voies
- les voies de package et d'extension proviennent de la même source d'auteur
- OpenCode et Cursor conviennent lorsque le dépôt doit posséder la configuration d'intégration
- la limite exacte du support reste dans les documents de référence, pas dans votre flux de premier démarrage

## Après le démarrage rapide

- Continuez avec [Créez votre premier plugin](/fr/guide/first-plugin) si vous souhaitez le didacticiel recommandé le plus restreint.
- Continuez avec [Ce que vous pouvez construire](/fr/guide/what-you-can-build) si vous souhaitez la carte complète des produits.
- Continuez avec [Choisir une cible](/fr/guide/choose-a-target) lorsque vous êtes prêt à faire correspondre le dépôt à la manière dont vous souhaitez l'expédier.
- Continuez avec [Un projet, plusieurs cibles](/fr/guide/one-project-multiple-targets) lorsque vous êtes prêt à vous développer au-delà du premier chemin.
locale-historical-source:end -->
