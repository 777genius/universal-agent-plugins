---
title: "v1.0.4 Go SDK"
description: "Notes de version du correctif pour la correction du chemin du module Go SDK."
canonicalId: "page:releases:v1-0-4-go-sdk"
section: "releases"
locale: "fr"
generated: false
translationRequired: true
---

[Utiliser des plugins](/fr/use/) · [Créer des plugins](/fr/build/) — Préparation, pas une version publiée. La migration des projets vers v2 est indisponible.
<p class="locale-historical-identity">v1.0.4 Go SDK</p>

[Instantané historique de la branche plugin-kit-ai v1. 1.2.4 est la référence des commandes, pas la version exacte de cet instantané.](/en/legacy/v1/)

<details><summary>Instantané historique de la branche plugin-kit-ai v1. 1.2.4 est la référence des commandes, pas la version exacte de cet instantané.</summary>

# v1.0.4 Go SDK

Date de sortie : `2026-03-29`

## Pourquoi ce patch est important

Ce correctif a rendu le chemin public du module Go SDK véridique pour une consommation normale de Go.

## Ce qui a changé

- la racine du module Go SDK déplacée de `sdk/plugin-kit-ai/` à `sdk/`
- le chemin du module public `github.com/777genius/plugin-kit-ai/sdk` correspond désormais à la disposition réelle du dépôt
- Les référentiels de démarrage, les exemples et les modèles ont été mis à jour pour cesser d'enseigner aux nouveaux arrivants des solutions de contournement basées sur `replace`.

## Conseils pratiques

- utilisez `github.com/777genius/plugin-kit-ai/sdk@v1.0.4` ou plus récent pour une consommation normale du module Go
- traiter `v1.0.3` comme connu comme mauvais pour le chemin du module Go SDK

## Pourquoi les utilisateurs devraient s'en soucier

Ce correctif a réduit les frictions pour les consommateurs Go normaux et a fait ressembler le chemin SDK recommandé à un module public normal au lieu d'une solution de contournement particulière.
</details>
