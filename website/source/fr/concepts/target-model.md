---
title: "Modèle cible"
description: "En quoi les sorties d'exécution, de package, d'extension et d'intégration appartenant au référentiel diffèrent, et comment choisir le bon chemin."
canonicalId: "page:concepts:target-model"
section: "concepts"
locale: "fr"
generated: false
translationRequired: true
aside: true
outline: [2, 3]
---

[Utiliser des plugins](/fr/use/) · [Créer des plugins](/fr/build/) — Préparation, pas une version publiée. La migration des projets vers v2 est indisponible.
<details><summary>Documentation historique plugin-kit-ai v1 · 1.2.4 ; les anciennes instructions ne constituent pas le parcours Build actuel.</summary>

# Modèle cible

Une cible est le type de sortie que vous souhaitez que le dépôt produise.

Le choix important n’est pas la taxonomie abstraite. Le choix important est ce que vous essayez d’expédier.

## Règle rapide

- Choisissez un chemin d'exécution lorsque vous souhaitez un plugin exécutable.
- Choisissez un chemin de package lorsqu'un autre système chargera votre sortie packagée.
- Choisissez un chemin d'extension lorsque l'hôte attend un artefact d'extension.
- Choisissez une configuration d'intégration appartenant au référentiel lorsque le référentiel a principalement besoin d'une configuration enregistrée pour un autre outil.

## Chemins d'exécution

Les cibles d'exécution produisent quelque chose d'exécutable. Il s'agit du point de départ par défaut pour la plupart des équipes, car c'est le moyen le plus clair de s'approprier le comportement, de valider le résultat et de développer le dépôt plus tard.

## Chemins des packages

Les cibles de package produisent une sortie packagée au lieu de la forme d’exécution exécutable principale. Utilisez-les lorsque l’emballage constitue la véritable exigence de livraison, et pas seulement une exportation supplémentaire dont vous pourriez avoir besoin plus tard.

## Chemins d'extension

Les cibles d'extension correspondent aux hôtes qui attendent un artefact d'extension spécifique ou une forme de package installable.

## Configuration de l'intégration appartenant au dépôt

Certaines sorties sont pour la plupart une configuration enregistrée qui aide un autre outil ou espace de travail à utiliser le plugin. Ce sont toujours des chemins pris en charge utiles, mais ils répondent à une question de livraison différente de celle d’un runtime exécutable.

## Le modèle mental sûr

Commencez par le résultat dont vous avez besoin en premier. Si le référentiel s'agrandit plus tard, vous pouvez ajouter une autre sortie prise en charge sans modifier le fait qu'un projet reste faisant autorité.
</details>
