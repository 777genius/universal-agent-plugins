---
title: "Choisissez une cible"
description: "Un guide public pratique pour choisir la cible qui correspond à la manière dont vous souhaitez expédier le plugin."
canonicalId: "page:guide:choose-a-target"
section: "guide"
locale: "fr"
generated: false
translationRequired: true
---

[Utiliser des plugins](/fr/use/) · [Créer des plugins](/fr/build/) — Préparation, pas une version publiée. La migration des projets vers v2 est indisponible.
<details><summary>Documentation historique plugin-kit-ai v1 · 1.2.4 ; les anciennes instructions ne constituent pas le parcours Build actuel.</summary>

# Choisissez une cible

Utilisez cette page lorsque vous savez déjà que vous voulez `plugin-kit-ai`, mais que vous devez toujours faire correspondre le dépôt à la manière dont vous souhaitez expédier le plugin.

Choisir une cible signifie choisir le chemin principal dont le produit a besoin aujourd'hui, et non verrouiller le dépôt pour toujours.

<MermaidDiagram
  :chart="`
flowchart TD
  Need[What does the product need right now] --> Exec{Comportement exécutable}
  Besoin --> Artefact{Package ou extension}
  Besoin --> Config{Intégration gérée par Repo}
  Exec --> Codex[codex-runtime]
  Exécutif --> Claude[claude]
  Artefact --> CodexPackage[codex-package]
  Artefact --> Gemini[gemini]
  Configuration --> OpenCode[opencode]
  Configuration --> Cursor[curseur]
`"
/>

## Règle courte

- choisissez `codex-runtime` lorsque vous souhaitez le chemin d'exécution par défaut le plus puissant
- choisissez `claude` lorsque les crochets Claude sont la véritable exigence du produit
- choisissez `codex-package` lorsque le produit est un package officiel Codex
- choisissez `gemini` lorsque le produit est un package d'extension Gemini
- choisissez `opencode` ou `cursor` lorsque le dépôt doit posséder la configuration d'intégration/configuration

## Répertoire cible

| Cible | Choisissez-le quand | Voie |
| --- | --- | --- |
| `codex-runtime` | Vous voulez le chemin du plugin exécutable par défaut | Chemin d'exécution recommandé |
| `claude` | Vous avez spécifiquement besoin de crochets Claude | Chemin Claude recommandé |
| `codex-package` | Vous avez besoin de Codex sortie d'emballage | Chemin de package recommandé |
| `gemini` | Vous expédiez un package d'extension Gemini | Chemin d'extension recommandé |
| `opencode` | Vous souhaitez une configuration d'intégration OpenCode appartenant au dépôt | Configuration de l'intégration appartenant au dépôt |
| `cursor` | Vous souhaitez une configuration d'intégration Cursor appartenant au dépôt | Configuration de l'intégration appartenant au dépôt |

## Valeur par défaut sûre

Si vous n'êtes pas sûr, commencez par `codex-runtime` et le chemin par défaut Go.

Cela vous donne le point de départ de production le plus propre avant de choisir une voie plus étroite ou plus spécialisée.

Lorsque vous passerez plus tard à `codex-package`, la voie officielle des packages suivra la disposition officielle du bundle `.codex-plugin/plugin.json`.

Si vous démarrez intentionnellement sur Node/TypeScript ou Python pris en charge, cela modifie le choix de la langue, et non la nécessité de décider de chaque détail d'emballage ou d'intégration dès le premier jour.

## Que faire lorsque vous avez besoin de plus d'une cible

- choisir le chemin principal qui définit le produit aujourd'hui
- garder le repo unifié
- ajouter plus de cibles uniquement lorsqu'une réelle exigence de livraison ou d'intégration apparaît

Lisez [Un projet, plusieurs cibles](/fr/guide/one-project-multiple-targets) lorsque vous souhaitez un modèle mental multi-cibles plus large.

</details>
