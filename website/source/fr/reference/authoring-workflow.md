---
title: "Flux de travail de création"
description: "Le flux de travail principal depuis init pour générer, valider, tester et transférer."
canonicalId: "page:reference:authoring-workflow"
section: "reference"
locale: "fr"
generated: false
translationRequired: true
---

[Utiliser des plugins](/fr/use/) · [Créer des plugins](/fr/build/) — Préparation, pas une version publiée. La migration des projets vers v2 est indisponible.
<details><summary>Documentation historique plugin-kit-ai v1 · 1.2.4 ; les anciennes instructions ne constituent pas le parcours Build actuel.</summary>

# Flux de travail de création

Le flux de travail recommandé est volontairement simple :

```text
init -> generate -> validate --strict -> test -> handoff
```

<MermaidDiagram
  :chart="`
flowchart LR
  Init[init] --> Générer[générer]
  Générer --> Valider[validate --strict]
  Valider --> Test[test ou contrôles de fumée]
  Test --> Transfert[handoff]
  Bootstrap[médecin ou bootstrap si nécessaire] -. prend en charge .-> Générer
  Amorçage -. prend en charge .-> Valider
`"
/>

## Ce que signifie chaque étape

| Étape | Objectif |
| --- | --- |
| `init` | Créer une présentation de projet standard pour les packages |
| `generate` | Générer des artefacts cibles à partir de la source du projet |
| `validate --strict` | Exécutez le contrôle de préparation principal |
| `test` | Effectuez des tests de fumée stables, le cas échéant |
| `export` / flux groupé | Produire des artefacts de transfert pour les cas Python et Node pris en charge |

## Règles qui maintiennent le dépôt en bonne santé

- la source du projet réside dans la présentation du projet standard du package
- les fichiers cibles générés sont des sorties, pas la source de vérité à long terme
- une validation stricte est une vérification obligatoire et non un supplément facultatif

Ce flux de travail est également important pour les dépôts à cible unique et multi-cibles.

La seule différence est que dans un projet multi-cible, les boucles `generate` et `validate` sont répétées pour chaque cible que le dépôt promet réellement de prendre en charge.

## Quand le flux de travail change

Le flux de travail peut s'élargir pour des cas particuliers :

- `doctor` et `bootstrap` sont importants pour les chemins d'exécution Python et Node
- `import` et `normalize` sont importants lors de la consolidation des fichiers cibles gérés manuellement dans le modèle de projet géré
- les commandes groupées sont importantes pour les flux de transfert portables Python et Node

Commencez par [Démarrage rapide](/fr/guide/quickstart) lorsque vous avez besoin du chemin le plus court.

</details>
