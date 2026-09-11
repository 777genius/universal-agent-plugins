---
title: "Ce que vous pouvez construire"
description: "Utilisez cette page comme carte du produit : quelles sorties existent, à quoi ressemble le démarrage par défaut et comment un dépôt peut se développer ultérieurement."
canonicalId: "page:guide:what-you-can-build"
section: "guide"
locale: "fr"
generated: false
translationRequired: true
aside: true
outline: [2, 3]
---

[Utiliser des plugins](/fr/use/) · [Créer des plugins](/fr/build/) — Préparation, pas une version publiée. La migration des projets vers v2 est indisponible.
<p class="locale-historical-identity">Ce que vous pouvez construire</p>

[Instantané historique de la branche plugin-kit-ai v1. 1.2.4 est la référence des commandes, pas la version exacte de cet instantané.](/en/legacy/v1/)

<details><summary>Instantané historique de la branche plugin-kit-ai v1. 1.2.4 est la référence des commandes, pas la version exacte de cet instantané.</summary>

# Ce que vous pouvez construire

Utilisez cette page comme carte du produit. Il montre quels types de résultats existent, et non quand un dépôt doit croître ou se diviser plus tard.

plugin-kit-ai peut démarrer avec un plugin exécutable et s'étendre vers des sorties supplémentaires prises en charge au fil du temps.

## Forme de départ recommandée

Commencez par un chemin d'exécution, généralement le runtime Codex avec Go. Cela simplifie le premier dépôt et vous offre la boucle de validation et d'expédition la plus claire.

Si votre équipe travaille déjà dans Node/TypeScript ou Python, ces chemins de départ sont également pris en charge.

## Un dépôt, de nombreuses sorties prises en charge

A partir d'un même projet, vous pouvez évoluer vers :

- sorties d'exécution pour les hôtes pris en charge
- les sorties emballées lorsque l'emballage est la véritable exigence de livraison
- sorties d'extension pour les hôtes qui attendent un artefact d'extension
- configuration d'intégration appartenant au référentiel lorsque le référentiel a principalement besoin d'une configuration enregistrée pour un autre outil

## À quoi cette page n'est-elle pas destinée

Choisir Node ou Python ne vous oblige pas à décider de chaque détail d'emballage ou d'intégration dès le premier jour.

Cette page est l'aperçu. Si votre question est de savoir si un dépôt doit continuer à croître, lisez [Un projet, plusieurs cibles](/fr/guide/one-project-multiple-targets).

</details>
