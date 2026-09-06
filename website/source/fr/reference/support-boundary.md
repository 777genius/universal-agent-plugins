---
title: "Limite de support"
description: "La réponse pratique la plus courte à ce que plugin-kit-ai recommande, soutient avec soin et reste expérimental."
canonicalId: "page:reference:support-boundary"
section: "reference"
locale: "fr"
generated: false
translationRequired: true
---

[Utiliser des plugins](/fr/use/) · [Créer des plugins](/fr/build/) — Préparation, pas une version publiée. La migration des projets vers v2 est indisponible.
<p class="locale-historical-identity">Limite de support</p>

[Instantané historique de la branche plugin-kit-ai v1. 1.2.4 est la référence des commandes, pas la version exacte de cet instantané.](/en/legacy/v1/)

<details><summary>Instantané historique de la branche plugin-kit-ai v1. 1.2.4 est la référence des commandes, pas la version exacte de cet instantané.</summary>

# Limite de support

Utilisez cette page lorsque vous avez besoin de la réponse honnête la plus courte concernant l'assistance.

Il répond à trois questions d’équipe :

- qu'est-ce qu'il est prudent de recommander par défaut
- ce qui est pris en charge, mais doit être choisi exprès
- ce qui est encore expérimental et ne devrait pas devenir une politique d'équipe

## Paramètres par défaut sécurisés

Voici les valeurs par défaut les plus sûres aujourd’hui :

- Go est le chemin d'exécution par défaut recommandé.
- `validate --strict` est la principale porte de préparation pour les dépôts d'exécution locaux Python et Node.
- `Codex runtime Go`, `Codex package`, `Gemini packaging`, `Gemini Go runtime` et la voie stable par défaut Claude sont les principales voies de production recommandées.
- `Python` et `Node` sont des chemins non-Go pris en charge et le choix non-Go recommandé lorsque le compromis d'exécution interprété localement est intentionnel.

## Comment cela correspond au contrat formel

Les documents publics utilisent d'abord trois mots simples :

- `Recommended` correspond généralement aux voies de production `public-stable` actuelles les plus fortes.
- `Advanced` désigne une surface supportée avec un contrat plus étroit, plus spécialisé ou plus soigné.
- `Experimental` signifie une désabonnement opt-in en dehors des attentes normales de compatibilité.

Lorsqu'une équipe a besoin d'un langage politique précis, les termes formels l'emportent : `public-stable`, `public-beta` et `public-experimental`.

## Recommandé aujourd'hui

Si vous avez besoin d’une réponse pratique, commencez ici :

- Claude est recommandé sur le chemin de hook stable par défaut.
- Codex est recommandé à la fois pour le chemin d'exécution `Notify` et pour le chemin officiel `codex-package`.
- L'emballage Gemini est recommandé et le runtime promu Gemini Go est également prêt pour la production.
- OpenCode et Cursor sont des chemins de configuration d'intégration appartenant au dépôt. Ils sont utiles, mais ils ne constituent pas le démarrage par défaut du runtime exécutable.

## Surfaces avancées

Choisissez des surfaces avancées uniquement lorsque le compromis est explicite et en vaut la peine.

Exemples typiques :

- OpenCode et Cursor lorsque le dépôt doit posséder la configuration d'intégration au lieu de fournir un chemin d'exécution
- extensions d'exécution plus étroites ou spécialisées au-delà des principaux chemins recommandés
- installer des wrappers lorsque le véritable problème est la livraison CLI, et non l'exécution APIs ou SDKs
- des surfaces de configuration spécialisées qui sont utiles, mais ne constituent pas le premier défaut pour la plupart des équipes

## Surfaces expérimentales

Traitez les zones expérimentales comme des zones opt-in et à fort taux de désabonnement.

Ils peuvent être utiles aux premiers utilisateurs, mais ils ne devraient pas devenir une norme à long terme pour l’équipe.

## Règle pratique

Si vous choisissez une équipe, standardisez le chemin le plus étroit dont vous êtes réellement prêt à défendre la promesse en matière d'IC, de déploiement et de transfert.

</details>
