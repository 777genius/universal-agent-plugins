---
title: "Installation"
description: "Installez plugin-kit-ai en utilisant les canaux pris en charge."
canonicalId: "page:guide:installation"
section: "guide"
locale: "fr"
generated: false
translationRequired: true
---

[Utiliser des plugins](/fr/use/) · [Créer des plugins](/fr/build/) — Préparation, pas une version publiée. La migration des projets vers v2 est indisponible.
<details><summary>Documentation historique plugin-kit-ai v1 · 1.2.4 ; les anciennes instructions ne constituent pas le parcours Build actuel.</summary>

# Installation

Utilisez Homebrew par défaut lorsqu'il correspond à votre environnement. L'objectif ici est simple : installez le CLI et accédez rapidement à votre premier dépôt fonctionnel.

## Chaînes prises en charge

- Homebrew pour le chemin CLI par défaut le plus propre.
- npm lorsque votre environnement est déjà centré autour de npm.
- PyPI / pipx lorsque votre environnement est déjà centré autour de Python.
- Script d'installation vérifié comme chemin de secours.

## Commandes recommandées

### Homebrew

```bash
brew install 777genius/homebrew-plugin-kit-ai/plugin-kit-ai
plugin-kit-ai version
```

### npm

```bash
npm i -g plugin-kit-ai
plugin-kit-ai version
```

### PyPI / pipx

```bash
pipx install plugin-kit-ai
plugin-kit-ai version
```

### Script vérifié

```bash
curl -fsSL https://raw.githubusercontent.com/777genius/plugin-kit-ai/main/scripts/install.sh | sh
plugin-kit-ai version
```

## Lequel la plupart des gens devraient-ils utiliser ?

- Utilisez Homebrew si vous êtes sur macOS et souhaitez le chemin par défaut le plus fluide.
- Utilisez npm ou pipx uniquement lorsque cela correspond déjà à l'environnement de votre équipe.
- Utilisez le script vérifié lorsque vous avez besoin d'une solution de secours en dehors des configurations axées sur le gestionnaire de packages.

## Après l'installation

La plupart des gens devraient continuer directement vers [Démarrage rapide](/fr/guide/quickstart) et créer le premier dépôt sur le chemin par défaut Go.

Si vous avez choisi `pipx` parce que votre équipe est Python-en premier et que vous savez déjà que vous voulez le chemin Python, continuez avec [Créer un plugin d'exécution Python](/fr/guide/python-runtime).

## Chemin d'installation de CI

Pour CI, préférez l'action de configuration dédiée au lieu d'apprendre à chaque flux de travail comment télécharger le CLI manuellement.

## Limite importante

Les packages npm et PyPI sont des canaux d'installation pour le CLI. Ce ne sont pas des API d’exécution et ce ne sont pas des SDK.

Voir [Référence > Installer les canaux](/fr/reference/install-channels) pour connaître les limites du contrat.

</details>
