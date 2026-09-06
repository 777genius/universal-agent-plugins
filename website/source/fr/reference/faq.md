---
title: "FAQ"
description: "Des réponses courtes aux questions que les équipes posent le plus souvent lors du démarrage et de la mise à l'échelle des dépôts plugin-kit-ai."
canonicalId: "page:reference:faq"
section: "reference"
locale: "fr"
generated: false
translationRequired: true
---

[Utiliser des plugins](/fr/use/) · [Créer des plugins](/fr/build/) — Préparation, pas une version publiée. La migration des projets vers v2 est indisponible.
<p class="locale-historical-identity">FAQ</p>

[Instantané historique de la branche plugin-kit-ai v1. 1.2.4 est la référence des commandes, pas la version exacte de cet instantané.](/en/legacy/v1/)

<details><summary>Instantané historique de la branche plugin-kit-ai v1. 1.2.4 est la référence des commandes, pas la version exacte de cet instantané.</summary>

# FAQ

## Dois-je commencer par Go, Python ou Node ?

Commencez par Go, sauf si vous avez une vraie raison de ne pas le faire.

Choisissez Node/TypeScript comme principal chemin non-Go pris en charge. Choisissez Python lorsque le plugin reste local dans le dépôt et que votre équipe est déjà Python-en premier.

## Quelle est la configuration Python la plus simple ?

Utilisez d'abord l'échafaudage Python par défaut :

```bash
plugin-kit-ai init my-plugin --platform codex-runtime --runtime python
plugin-kit-ai doctor ./my-plugin
plugin-kit-ai bootstrap ./my-plugin
plugin-kit-ai generate ./my-plugin
plugin-kit-ai validate ./my-plugin --platform codex-runtime --strict
```

Modifiez ensuite le plugin, régénérez-le et validez à nouveau.

Voir [Créer un plugin d'exécution Python](/fr/guide/python-runtime).

## Quand dois-je utiliser `--runtime-package` ?

Utilisez `--runtime-package` uniquement lorsque vous souhaitez intentionnellement une dépendance d'assistance partagée sur plusieurs dépôts.

La plupart des équipes devraient commencer par l'assistant local par défaut.

## Les packages npm et PyPI `plugin-kit-ai` Runtime API sont-ils ?

Non, ils installent le CLI. Ce ne sont pas des API d’exécution et ce ne sont pas des SDK.

## Quand dois-je utiliser les commandes groupées ?

Utilisez les commandes bundle lorsqu'une autre machine a besoin d'artefacts portables Python ou Node pour récupérer ou installer.

Ne confondez pas la livraison du bundle avec le chemin d'installation principal CLI.

## Puis-je conserver les fichiers cibles natifs comme source de vérité ?

Non. Le modèle à long terme prévu est de conserver la source de vérité dans la présentation standard du package et de traiter les fichiers cibles comme une sortie générée.

## `generate` est-il facultatif ?

Non, pas si vous souhaitez le flux de projet géré. `generate` fait partie du flux de travail.

## `validate --strict` est-il facultatif ?

Considérez-le comme le principal contrôle de préparation, en particulier pour les dépôts d'exécution locaux Python et Node.

## Un dépôt peut-il posséder plusieurs cibles ?

Oui.

La règle pratique est la suivante :

- conserver l'état créé dans un dépôt géré
- commencez par la cible principale dont vous avez besoin aujourd'hui
- ajoutez plus de cibles uniquement lorsqu'un réel besoin de produit, de livraison ou d'intégration apparaît

Voir [Un projet, plusieurs cibles](/fr/guide/one-project-multiple-targets) et [Modèle cible](/fr/concepts/target-model).

## Toutes les cibles sont-elles également stables ?

Non.

Différents chemins comportent différentes promesses de soutien. Utilisez [Limite de support](/fr/reference/support-boundary) pour la réponse courte et [Support des cibles](/fr/reference/target-support) pour la matrice exacte.

</details>
