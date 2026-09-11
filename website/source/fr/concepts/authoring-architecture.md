---
title: "Source et résultats du projet"
description: "Comment les fichiers créés, les sorties générées, la validation stricte et le transfert s'intègrent dans plugin-kit-ai."
canonicalId: "page:concepts:authoring-architecture"
section: "concepts"
locale: "fr"
generated: false
translationRequired: true
aside: true
outline: [2, 3]
---

[Utiliser des plugins](/fr/use/) · [Créer des plugins](/fr/build/) — Préparation, pas une version publiée. La migration des projets vers v2 est indisponible.
<p class="locale-historical-identity">Source et résultats du projet</p>

[Instantané historique de la branche plugin-kit-ai v1. 1.2.4 est la référence des commandes, pas la version exacte de cet instantané.](/en/legacy/v1/)

<details><summary>Instantané historique de la branche plugin-kit-ai v1. 1.2.4 est la référence des commandes, pas la version exacte de cet instantané.</summary>

# Source et résultats du projet

Cette page est plus étroite que le modèle de produit principal. Il explique la limite de travail à l'intérieur du dépôt : ce que vous créez, ce qui est généré et pourquoi cette division maintient le projet maintenable.

## La forme du noyau

```text
project source -> generate -> target outputs -> validate --strict -> handoff
```

La source reste stable. Les sorties peuvent changer par cible. La validation garantit que le résultat généré peut toujours être transmis en toute sécurité.

## Fichiers créés par rapport aux fichiers générés

Les fichiers créés constituent la partie du référentiel que vous êtes censé gérer directement.

Les fichiers générés sont des artefacts de build pour les cibles que vous avez choisies. Ce sont de véritables résultats de livraison, mais ce n’est pas le lieu où la vérité du projet devrait dériver.

Cette distinction maintient le dépôt lisible et sécurise la régénération.

## Pourquoi la scission est importante

Sans une répartition claire, les équipes finissent par modifier les résultats générés, perdant en répétabilité et rendant les mises à niveau plus difficiles qu'elles ne devraient l'être.

Avec une répartition claire, vous pouvez :

- examiner directement les modifications de la source
- régénérer la production en toute confiance
- valider à chaque fois la même forme de livraison
- ajouter une autre sortie prise en charge plus tard sans reconstruire le dépôt à partir de zéro

## Quel est le rapport avec le modèle plus grand

Si vous souhaitez une explication de niveau supérieur, commencez par [Comment fonctionne plugin-kit-ai](/fr/concepts/managed-project-model).
</details>
