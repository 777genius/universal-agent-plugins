---
title: "Comment fonctionne plugin-kit-ai"
description: "Comment un dépôt reste la source de vérité pendant que vous générez des sorties, validez strictement et transmettez un résultat propre."
canonicalId: "page:concepts:managed-project-model"
section: "concepts"
locale: "fr"
generated: false
translationRequired: true
aside: true
outline: [2, 3]
---

[Utiliser des plugins](/fr/use/) · [Créer des plugins](/fr/build/) — Préparation, pas une version publiée. La migration des projets vers v2 est indisponible.
<p class="locale-historical-identity">Comment fonctionne plugin-kit-ai</p>

[Instantané historique de la branche plugin-kit-ai v1. 1.2.4 est la référence des commandes, pas la version exacte de cet instantané.](/en/legacy/v1/)

<details><summary>Instantané historique de la branche plugin-kit-ai v1. 1.2.4 est la référence des commandes, pas la version exacte de cet instantané.</summary>

# Comment fonctionne plugin-kit-ai

plugin-kit-ai conserve un dépôt comme source de vérité pour votre plugin. Vous modifiez les fichiers que vous possédez, générez les sorties dont vous avez besoin, validez strictement le résultat et transmettez un dépôt qui reste prévisible dans le temps.

## La version courte

La boucle principale est simple :

```text
source -> generate -> validate --strict -> handoff
```

Cette boucle est importante car le projet n'est pas seulement un modèle de démarrage. La sortie générée peut changer à mesure que la cible évolue, tandis que votre source créée reste claire et maintenable.

## Un dépôt comme source de vérité

Le dépôt est l’endroit où le plugin vit réellement.

- les fichiers créés restent sous votre contrôle
- les sorties générées sont reconstruites à partir de cette source
- la validation vérifie la sortie que vous prévoyez d'expédier
- le transfert n'a lieu qu'une fois que le résultat généré est propre

Cela permet à un projet de se développer avec précaution au lieu de disperser la même logique de plugin sur plusieurs dépôts.

## Ce que vous modifiez réellement

Vous continuez à modifier la source du projet et le code du plugin que vous possédez. Vous ne considérez pas la sortie générée comme le lieu où le projet vit réellement.

Cette limite est ce qui permet de gérer les mises à niveau, les modifications d’objectifs et les travaux de maintenance.

## Pourquoi c'est plus que des modèles de démarrage

Un modèle de démarrage vous donne une forme initiale. plugin-kit-ai continue de gérer la boucle après le premier jour :

- il régénère la sortie spécifique à la cible à partir de la même source
- il valide ce que vous vous apprêtez à expédier
- il maintient les fichiers créés et les fichiers générés clairement séparés
- il permet à un référentiel de s'étendre ultérieurement à plusieurs sorties sans réécrire l'ensemble du modèle de projet

## Où aller ensuite

- Lisez [Source et résultats du projet](/fr/concepts/authoring-architecture) pour connaître la limite entre création et génération.
- Lisez [Modèle cible](/fr/concepts/target-model) pour les différents types de sortie.
- Lisez [Un projet, plusieurs cibles](/fr/guide/one-project-multiple-targets) lorsque vous souhaitez développer davantage un dépôt.

</details>
