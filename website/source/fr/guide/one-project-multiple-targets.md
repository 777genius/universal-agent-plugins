---
title: "Un projet, plusieurs cibles"
description: "Comment décider quand un dépôt doit atteindre davantage de résultats, quand il doit rester étroit et quand il est temps de le diviser."
canonicalId: "page:guide:one-project-multiple-targets"
section: "guide"
locale: "fr"
generated: false
translationRequired: true
aside: true
outline: [2, 3]
---

[Utiliser des plugins](/fr/use/) · [Créer des plugins](/fr/build/) — Préparation, pas une version publiée. La migration des projets vers v2 est indisponible.
<p class="locale-historical-identity">Un projet, plusieurs cibles</p>

[Instantané historique de la branche plugin-kit-ai v1. 1.2.4 est la référence des commandes, pas la version exacte de cet instantané.](/en/legacy/v1/)

<details><summary>Instantané historique de la branche plugin-kit-ai v1. 1.2.4 est la référence des commandes, pas la version exacte de cet instantané.</summary>

# Un projet, plusieurs cibles

Utilisez cette page après le premier dépôt fonctionnel, lorsque la vraie question devient : ce même dépôt doit-il croître, et si oui, jusqu'où ?

## La règle courte

Un dépôt peut couvrir en toute sécurité plusieurs sorties lorsque la même logique de plugin, la même intention de publication et le même modèle de propriété sont toujours valables.

## Quand un dépôt devrait croître

Développez le même dépôt lorsque :

- le comportement du plugin est toujours un produit cohérent
- la nouvelle sortie est une autre façon de fournir le même plugin
- une équipe peut toujours posséder proprement la source créée
- la régénération et la validation permettent toujours au dépôt d'être facilement révisable

## Quand un dépôt doit rester étroit

Gardez le référentiel concentré lorsque la sortie actuelle résout déjà le besoin réel et que des sorties supplémentaires ne feraient qu'ajouter des frais de maintenance.

## Quand diviser les dépôts

Divisez les dépôts lorsque le produit cesse d'être une seule chose en pratique :

- différentes équipes sont propriétaires du travail
- le moment de la sortie diverge
- le comportement diverge au-delà de la simple adaptation de la cible
- le repo deviendrait plus difficile à raisonner que deux pensions plus petites

## Le modèle mental sûr

Commencez de manière étroite, validez une sortie de travail, puis développez ensuite le dépôt avec une autre sortie prise en charge.
</details>
