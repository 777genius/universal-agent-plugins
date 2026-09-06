---
title: "Créez votre premier plugin"
description: "Un tutoriel minimal de bout en bout, de l'initialisation à la validation stricte."
canonicalId: "page:guide:first-plugin"
section: "guide"
locale: "fr"
generated: false
translationRequired: true
---

[Utiliser des plugins](/fr/use/) · [Créer des plugins](/fr/build/) — Préparation, pas une version publiée. La migration des projets vers v2 est indisponible.
<p class="locale-historical-identity">Créez votre premier plugin</p>

[Instantané historique de la branche plugin-kit-ai v1. 1.2.4 est la référence des commandes, pas la version exacte de cet instantané.](/en/legacy/v1/)

<details><summary>Instantané historique de la branche plugin-kit-ai v1. 1.2.4 est la référence des commandes, pas la version exacte de cet instantané.</summary>

# Créez votre premier plugin

Ce didacticiel vous donne le premier dépôt de travail le plus simple sur le chemin par défaut le plus fort.

Le champ d’application est intentionnellement restreint :

- première cible : `codex-runtime`
- première langue : `go`
- première porte de préparation : `validate --strict`

Cette forme étroite est réservée au premier passage. Si l'histoire plus large d'un dépôt unique et de plusieurs résultats est la principale chose qui vous intéresse, lisez [Un projet, plusieurs cibles](/fr/guide/one-project-multiple-targets) juste après ce didacticiel.

## 1. Installez le CLI

```bash
brew install 777genius/homebrew-plugin-kit-ai/plugin-kit-ai
plugin-kit-ai version
```

## 2. Échafauder un projet

```bash
plugin-kit-ai init my-plugin
cd my-plugin
go mod tidy
```

Le chemin par défaut `init` est déjà le point de départ de production recommandé.

## 3. Générer les fichiers cibles

```bash
plugin-kit-ai generate .
```

Traitez les fichiers cibles générés comme des sorties. Continuez à modifier le dépôt via `plugin-kit-ai` au lieu de gérer manuellement les fichiers générés.

## 4. Exécutez la porte de préparation

```bash
plugin-kit-ai validate . --platform codex-runtime --strict
```

Utilisez-le comme porte principale de qualité CI pour un projet de plugin local.

## Ce que vous avez maintenant

- un dépôt de plugin
- fichiers créés sous `plugin/` pour les nouveaux dépôts
- sortie d'exécution générée Codex
- une porte de préparation claire via `validate --strict`

## 5. Quand changer de chemin

Passez à un autre chemin uniquement lorsque vous en avez réellement besoin :

- choisissez `claude` pour les plugins Claude
- choisissez `--runtime node --typescript` pour le chemin principal non-Go pris en charge
- choisissez `--runtime python` lorsque le projet reste local par rapport au dépôt et que votre équipe est Python-en premier
- choisissez `codex-package`, `gemini`, `opencode` ou `cursor` uniquement lorsque vous avez vraiment besoin d'une manière différente d'expédier le plugin

Cela ne signifie pas que le dépôt doit rester éternellement à cible unique : commencez par la cible la plus importante aujourd'hui et ajoutez les autres uniquement lorsque le produit se développe réellement.

## Prochaines étapes

- Lisez [Choisir l'environnement d'exécution](/fr/concepts/choosing-runtime) avant de quitter le chemin par défaut.
- Lisez [Un projet, plusieurs cibles](/fr/guide/one-project-multiple-targets) si l'idée d'un dépôt unique et de plusieurs résultats est l'une des principales raisons pour lesquelles vous vous souciez du produit.
- Utilisez [Modèles de démarrage](/fr/guide/starter-templates) lorsque vous souhaitez un exemple de dépôt connu.
- Parcourez [Référence CLI](/fr/api/cli/) lorsque vous avez besoin d'un comportement de commande exact.

</details>
