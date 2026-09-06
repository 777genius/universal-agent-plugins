---
title: "Modèle de stabilité"
description: "Comment plugin-kit-ai classe les zones publiques stables, publiques bêta et publiques expérimentales."
canonicalId: "page:concepts:stability-model"
section: "concepts"
locale: "fr"
generated: false
translationRequired: true
---

[Utiliser des plugins](/fr/use/) · [Créer des plugins](/fr/build/) — Préparation, pas une version publiée. La migration des projets vers v2 est indisponible.
<details><summary>Documentation historique plugin-kit-ai v1 · 1.2.4 ; les anciennes instructions ne constituent pas le parcours Build actuel.</summary>

# Modèle de stabilité

`plugin-kit-ai` utilise des termes contractuels formels afin que les équipes puissent décider exactement ce qu'elles souhaitent normaliser.

<MermaidDiagram
  :chart="`
flowchart TD
  Stable[public stable] --> Bêta [bêta publique]
  Bêta -> Expérimental [expérimental public]
  StableNote[Attentes normales de production] -> Stable
  BetaNote[Supporté mais pas gelé] ->.-> Bêta
  ExperimentalNote[Opt in churn] -> Expérimental
`"
/>

## Langage public versus langage formel

Les documents publics utilisent un vocabulaire de premier passage plus simple :

- `Recommended` pointe généralement vers les chemins `public-stable` de courant le plus fort
- `Advanced` pointe vers des surfaces supportées plus étroites ou plus spécialisées
- `Experimental` correspond à `public-experimental`

Lorsque vous définissez une politique de compatibilité, les termes formels l'emportent.

## Comment lire recommandé

`Recommended` est le langage du produit et ne remplace pas le contrat formel.

- cela signifie généralement un chemin de production promu `public-stable`
- cela ne signifie pas la parité entre toutes les cibles
- il ne met pas à niveau les surfaces `public-beta` ou `public-experimental` par le seul libellé

## Public-Stable

Considérez `public-stable` comme le niveau sur lequel vous pouvez construire avec des attentes de production normales.

Il s’agit du niveau que la plupart des équipes devraient préférer pour les normes par défaut et le déploiement à long terme.

## Bêta publique

Traitez `public-beta` comme pris en charge, mais pas gelé.

Utilisez la version bêta uniquement lorsque le compromis est explicite et en vaut la peine pour le produit.

## Public-Expérimental

Traitez `public-experimental` comme une désabonnement opt-in en dehors des attentes normales de compatibilité.

Cela peut être utile pour l’apprentissage ou l’adoption précoce, mais cela ne devrait pas devenir la valeur par défaut de l’équipe.

## Règle pratique

1. Préférez le chemin recommandé pour le produit que vous créez.
2. Utilisez les termes formels exacts uniquement lorsque vous avez besoin de précisions en matière de politique ou de compatibilité.
3. Utilisez `validate --strict` comme porte de préparation pour le dépôt que vous prévoyez d'expédier.
</details>
