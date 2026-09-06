---
title: "Dépannage"
description: "Étapes de récupération rapides pour les problèmes d'installation, de génération, de validation et d'amorçage les plus courants."
canonicalId: "page:reference:troubleshooting"
section: "reference"
locale: "fr"
generated: false
translationRequired: true
---

[Utiliser des plugins](/fr/use/) · [Créer des plugins](/fr/build/) — Préparation, pas une version publiée. La migration des projets vers v2 est indisponible.
<p class="locale-historical-identity">Dépannage</p>

[Instantané historique de la branche plugin-kit-ai v1. 1.2.4 est la référence des commandes, pas la version exacte de cet instantané.](/en/legacy/v1/)

<details><summary>Instantané historique de la branche plugin-kit-ai v1. 1.2.4 est la référence des commandes, pas la version exacte de cet instantané.</summary>

# Dépannage

Utilisez cette page lorsque le flux de travail s'arrête. Commencez par la vérification la plus simple.

## Le CLI s'installe mais ne s'exécute pas

Vérifiez que le binaire est bien sur votre shell `PATH`.

Si vous avez installé via npm ou PyPI, assurez-vous que le package a réellement téléchargé le binaire publié. Ne traitez pas le package wrapper lui-même comme le moteur d’exécution.

## Les projets runtime Python ou Node échouent trop tôt

Vérifiez d'abord le temps d'exécution réel :

- Les dépôts d'exécution Python nécessitent Python `3.10+`
- Les dépôts d'exécution Node nécessitent Node.js `20+`

Utilisez `plugin-kit-ai doctor <path>` avant de supposer que le dépôt lui-même est cassé.

Flux de récupération typique :

```bash
plugin-kit-ai doctor ./my-plugin
plugin-kit-ai bootstrap ./my-plugin
plugin-kit-ai generate ./my-plugin
plugin-kit-ai validate ./my-plugin --platform codex-runtime --strict
```

## Échec de `validate --strict`

Traitez cela comme un signal et non comme du bruit.

Causes courantes :

- les artefacts générés sont obsolètes car `generate` a été ignoré
- la plateforme sélectionnée ne correspond pas à la source du projet
- le chemin d'exécution nécessite encore des correctifs d'amorçage ou d'environnement

## `generate` produit un résultat inattendu

Cela signifie généralement que la source du projet et votre modèle mental se sont séparés.

Revérifiez la disposition standard du package au lieu de modifier manuellement les fichiers cibles générés pour forcer la sortie attendue.

## Je ne sais pas quel chemin je dois utiliser

Commencez par le chemin par défaut Go si vous voulez le contrat le plus solide.

Passez à Node/TypeScript ou Python uniquement lorsque le compromis d'exécution locale est réel et intentionnel.

Voir [Créer un plugin d'exécution Python](/fr/guide/python-runtime), [Flux de travail de création](/fr/reference/authoring-workflow) et [FAQ](/fr/reference/faq).

</details>
