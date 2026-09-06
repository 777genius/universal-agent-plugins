---
title: "Choisissez le modèle de livraison"
description: "Comment choisir entre les assistants fournis et le package d'exécution partagé pour les plugins Python et Node."
canonicalId: "page:guide:choose-delivery-model"
section: "guide"
locale: "fr"
generated: false
translationRequired: true
---

[Utiliser des plugins](/fr/use/) · [Créer des plugins](/fr/build/) — Préparation, pas une version publiée. La migration des projets vers v2 est indisponible.
<details><summary>Documentation historique plugin-kit-ai v1 · 1.2.4 ; les anciennes instructions ne constituent pas le parcours Build actuel.</summary>

# Choisissez le modèle de livraison

Les plugins Python et Node ont deux méthodes prises en charge pour expédier la logique d'assistance. Ils résolvent différents problèmes pratiques.

<MermaidDiagram
  :chart="`
flowchart TD
  Start[Python or Node plugin] --> Partagé{Besoin d'une dépendance réutilisable entre les dépôts}
  Partagé -->|Oui| Package[package d'exécution partagé]
  Partagé -->|Non| Smooth {Besoin du démarrage autonome le plus fluide}
  Lisse -->|Oui| Vendu[assistant vendu]
  Lisse -->|Non| Forfait
`"
/>

## Règle pratique rapide

Si vous souhaitez simplement le dépôt Python ou Node fonctionnel le plus simple aujourd'hui, utilisez d'abord l'échafaudage par défaut.

Si vous savez déjà que plusieurs dépôts doivent partager une dépendance d'assistance, commencez par `--runtime-package`.

## Les deux modes

- `vendored helper` : l'échafaudage par défaut écrit les fichiers d'assistance dans le dépôt lui-même
- `shared runtime package` : `--runtime-package` importe `plugin-kit-ai-runtime` en tant que dépendance au lieu d'écrire l'assistant dans `plugin/`

## Le même projet dans les deux modes

Chemin d'assistance local par défaut :

```bash
plugin-kit-ai init my-plugin --platform codex-runtime --runtime python
```

Chemin du package partagé :

```bash
plugin-kit-ai init my-plugin --platform codex-runtime --runtime python --runtime-package
```

## Choisissez l'assistant du fournisseur lorsque

- vous voulez le chemin de première exécution le plus fluide
- vous voulez que le dépôt reste autonome
- vous voulez que l'implémentation de l'assistant soit visible dans le dépôt
- votre équipe ne standardise pas encore une version partagée de PyPI ou d'assistance npm

Il s'agit de la valeur par défaut car c'est le point de départ le plus simple pour les projets Python et Node.

## Choisissez le package d'exécution partagé lorsque

- vous voulez une dépendance d'assistance réutilisable sur plusieurs dépôts de plugins
- vous préférez mettre à niveau le comportement de l'assistant via des modifications normales de la version du package
- votre équipe est à l'aise pour épingler les versions dans `requirements.txt` ou `package.json`
- vous savez déjà que le dépôt doit suivre le chemin des dépendances partagées dès le premier jour

## Ce que les gens veulent dire habituellement dans la pratique

- choisissez l'assistant du fournisseur lorsque l'objectif principal est de "faire fonctionner un dépôt rapidement"
- choisissez le package d'exécution partagé lorsque l'objectif principal est de "réutiliser le même package d'assistance dans tous les dépôts"
- ne choisissez pas le package partagé simplement parce qu'il ressemble davantage à une production ; il ne supprime pas l'exigence d'exécution Python ou Node de la machine d'exécution

## Ce qui ne change pas

- Go est toujours la valeur par défaut recommandée lorsque vous souhaitez le chemin de production le plus solide
- Python nécessite toujours Python `3.10+` sur la machine d'exécution
- Node nécessite toujours Node.js `20+` sur la machine d'exécution
- `validate --strict` reste le principal contrôle de préparation
- Les packages d'installation CLI ne deviennent toujours pas des API d'exécution.

## Politique d'équipe recommandée

- choisissez Go lorsque vous souhaitez le chemin pris en charge à long terme le plus solide
- choisissez les aides du fournisseur lorsque vous souhaitez démarrer Python ou Node le plus en douceur
- choisissez le package d'exécution partagé lorsque vous savez déjà que vous souhaitez une stratégie de dépendance réutilisable entre les dépôts

Associez cette page à [Créer un plugin d'exécution Python](/fr/guide/python-runtime), [Choisissez un dépôt de démarrage](/fr/guide/choose-a-starter), [Modèles de démarrage](/fr/guide/starter-templates) et [Préparation à la production](/fr/guide/production-readiness).

</details>
