---
title: "Packages et configuration de l'intégration"
description: "Lorsque l'empaquetage ou la configuration de l'intégration enregistrée est la bonne réponse au lieu d'un plugin d'exécution exécutable."
canonicalId: "page:guide:package-and-workspace-targets"
section: "guide"
locale: "fr"
generated: false
translationRequired: true
aside: true
outline: [2, 3]
---

[Utiliser des plugins](/fr/use/) · [Créer des plugins](/fr/build/) — Préparation, pas une version publiée. La migration des projets vers v2 est indisponible.
<p class="locale-historical-identity">Packages et configuration de l&#x27;intégration</p>

[Instantané historique de la branche plugin-kit-ai v1. 1.2.4 est la référence des commandes, pas la version exacte de cet instantané.](/en/legacy/v1/)

<details><summary>Instantané historique de la branche plugin-kit-ai v1. 1.2.4 est la référence des commandes, pas la version exacte de cet instantané.</summary>

# Packages et configuration de l'intégration

Tous les projets ne doivent pas être livrés sous forme de plugin d'exécution exécutable.

Parfois, la véritable exigence est un package qu'un autre système chargera, un artefact d'extension ou une configuration d'intégration enregistrée qui réside dans le dépôt.

## La règle courte

Choisissez des packages ou une configuration d'intégration lorsque la forme de livraison compte plus que l'exécution directe du plugin.

## Choisissez cette page quand

C'est la bonne voie lorsque :

- l'emballage est la véritable exigence de livraison
- l'hôte attend une extension ou un artefact packagé
- le dépôt a principalement besoin d'une configuration d'intégration enregistrée pour un autre outil
- un runtime exécutable ajouterait du travail opérationnel inutile

## Qu'est-ce qui différencie cela d'un chemin d'exécution

Un chemin d'exécution est généralement la valeur par défaut la plus claire lorsque vous souhaitez un plugin exécutable.

Les packages et la configuration de l'intégration répondent à une question différente : comment ce plugin doit-il être livré ou connecté à un autre système ?

## Le modèle mental sûr

Choisissez le runtime lorsque vous souhaitez exécuter le plugin directement. Choisissez des packages ou une configuration d'intégration lorsque la forme de la livraison est la principale exigence.

## Codex Limite du colis

Pour le package officiel Codex, gardez la disposition du bundle explicite et étroite :

- `.codex-plugin/` contient uniquement `plugin.json`
- facultatif `.app.json` et `.mcp.json` restent à la racine du plugin

Ce chemin de package est destiné à la surface officielle du bundle de plugins Codex, et non à mélanger le câblage du runtime du référentiel local dans la présentation du package.
</details>
