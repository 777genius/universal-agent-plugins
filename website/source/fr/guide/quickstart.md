---
title: "Démarrage rapide"
description: "Installez des plugins disponibles ou préparez un paquet portable Agent Plugins 1.0."
canonicalId: "page:guide:quickstart"
section: "guide"
locale: "fr"
generated: false
translationRequired: true
---

<a id="demarrage-rapide"></a>

# Utiliser des plugins / Créer des plugins

Installez des plugins disponibles ou préparez un paquet portable Agent Plugins 1.0.

## Utiliser des plugins {#use-plugins}

Disponible maintenant : installez des plugins avec Universal Agent Plugins. Avec Node.js 22+, exécutez :

```bash
npx universal-agent-plugins add context7
```

Pour une installation native sur macOS, Linux ou Windows, consultez le guide. Le CLI demande quels agents compatibles utiliser ; chaque plugin peut nécessiter son propre environnement d’exécution.

[Guide d’installation](https://github.com/777genius/universal-agent-plugins#quick-start)

Versions vérifiées le 2026-09-07 : universal-agent-plugins 0.1.53 (npm), plugin-kit-ai 1.2.4 (npm/PyPI), version stable GitHub agentplugins-v0.1.53.

La compatibilité dépend du paquet. La validation du schéma ne prouve ni l’exécution, ni OAuth, ni l’activation. Codex ne prend pas en charge MCP SSE déclaré ; stdio et Streamable HTTP conservent leur prise en charge par les adaptateurs existants.

[Compatibilité des clients (en anglais)](https://github.com/777genius/universal-agent-plugins#supported-clients)

## Créer des plugins {#build-plugins}

Créez un paquet portable Agent Plugins 1.0 autour de plugin.json, avec skills/ et mcp.json facultatifs. La prise en charge dépend du paquet et de ses composants.

**En préparation — non publié**

Le CLI de création fondé sur le standard est en préparation et n’est pas encore publié. plugin-kit-ai 1.2.4, publié sur npm et PyPI, est l’outil historique v1, pas standard-first v2. Installer plugin-kit-ai@latest ne fournit pas le futur parcours de création.

[Spécification Agent Plugins 1.0](https://agent-plugins.org/specification)

## Maintenance historique v1 {#historical-v1}

<a id="si-vous-ne-lisez-qu-une-chose"></a>
<a id="valeur-par-defaut-recommandee"></a>
<a id="pourquoi-c-est-la-valeur-par-defaut"></a>

Maintenez les projets plugin.yaml existants avec les instructions v1 ci-dessous. Ces modèles et sorties générées relèvent des parcours historiques v1 ; ils ne créent pas le nouveau parcours fondé sur le standard.

```bash
brew install 777genius/homebrew-plugin-kit-ai/plugin-kit-ai
plugin-kit-ai version
plugin-kit-ai init my-plugin
cd my-plugin
go mod tidy
plugin-kit-ai generate .
plugin-kit-ai validate . --platform codex-runtime --strict
```

### Ce que vous obtenez

- un dépôt de plugin dès le premier jour
- fichiers créés sous `plugin/`
- généré une sortie d'exécution Codex à partir du même dépôt
- un contrôle de préparation propre via `validate --strict`

### Chemins Node et Python pris en charge

Si votre équipe habite déjà à Node/TypeScript ou Python, ces chemins sont pris en charge et visibles dès le départ :

- `codex-runtime --runtime node --typescript`
- `codex-runtime --runtime python`
- les deux sont des chemins d'exécution interprétés localement, donc la machine cible a toujours besoin de Node.js `20+` ou Python `3.10+`
- Go reste toujours la valeur par défaut lorsque vous souhaitez l'histoire de production générale la plus forte

### Si vous commencez intentionnellement le Node ou Python

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

### Que faire ensuite

- modifiez le plugin sous `plugin/`
- exécutez à nouveau `plugin-kit-ai generate ./my-plugin` après les modifications
- exécutez à nouveau `plugin-kit-ai validate ./my-plugin --platform codex-runtime --strict`
- ensuite seulement, ajoutez un autre moyen de l'expédier si le produit en a besoin

### Développer plus tard

| Si tu veux | Ajoutez ceci plus tard |
| --- | --- |
| Claude crochets comme produit réel | `claude` |
| Forfait officiel Codex | `codex-package` |
| Gemini package d'extension | `gemini` |
| Configuration de l'intégration appartenant au dépôt | `opencode` ou `cursor` |

Choisissez d'abord `claude` uniquement lorsque les crochets Claude constituent déjà la véritable exigence du produit.

### Ce qui se développera plus tard

- le dépôt reste unifié à mesure que vous ajoutez plus de voies
- les voies de package et d'extension proviennent de la même source d'auteur
- OpenCode et Cursor conviennent lorsque le dépôt doit posséder la configuration d'intégration
- la limite exacte du support reste dans les documents de référence, pas dans votre flux de premier démarrage

### Après le démarrage rapide

- Continuez avec [Créez votre premier plugin](/fr/guide/first-plugin) pour le tutoriel historique v1.
- Continuez avec [Ce que vous pouvez construire](/fr/guide/what-you-can-build) si vous souhaitez la carte complète des produits.
- Continuez avec [Choisir une cible](/fr/guide/choose-a-target) lorsque vous êtes prêt à faire correspondre le dépôt à la manière dont vous souhaitez l'expédier.
- Continuez avec [Un projet, plusieurs cibles](/fr/guide/one-project-multiple-targets) lorsque vous êtes prêt à vous développer au-delà du premier chemin.

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
