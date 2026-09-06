---
title: "Transfert du bundle"
description: "Comment exporter, installer, récupérer et publier des bundles portables Python et Node pour les flux de transfert pris en charge."
canonicalId: "page:guide:bundle-handoff"
section: "guide"
locale: "fr"
generated: false
translationRequired: true
---

[Utiliser des plugins](/fr/use/) · [Créer des plugins](/fr/build/) — Préparation, pas une version publiée. La migration des projets vers v2 est indisponible.
<p class="locale-historical-identity">Transfert du bundle</p>

[Instantané historique de la branche plugin-kit-ai v1. 1.2.4 est la référence des commandes, pas la version exacte de cet instantané.](/en/legacy/v1/)

<details><summary>Instantané historique de la branche plugin-kit-ai v1. 1.2.4 est la référence des commandes, pas la version exacte de cet instantané.</summary>

# Transfert du bundle

Utilisez ce guide lorsqu'un plugin Python ou Node doit voyager comme un artefact portable plutôt que comme une extraction de dépôt en direct.

Il s'agit d'une véritable capacité publique, mais elle est intentionnellement plus étroite que le chemin principal Go.

## Ce que cela couvre

Le sous-ensemble de transfert de bundle stable est destiné :

- exporté des bundles `python` sur `codex-runtime` et `claude`
- exporté des bundles `node` sur `codex-runtime` et `claude`
- installation du bundle local
- récupération du bundle à distance
- GitHub Publication du bundle de versions

C'est la solution idéale lorsque :

- une autre équipe devrait recevoir un artefact prêt au lieu de votre dépôt complet
- votre flux de versions utilise déjà les versions GitHub
- vous voulez une histoire de transfert plus propre pour les environnements d'exécution Python ou Node

## Le flux pratique

Du côté des producteurs, c'est :

```bash
plugin-kit-ai export . --platform <codex-runtime|claude>
plugin-kit-ai bundle publish . --platform <codex-runtime|claude> --repo <owner/repo> --tag <tag>
```

Le côté consommateur est soit :

```bash
plugin-kit-ai bundle install <bundle.tar.gz> --dest <path>
```

ou :

```bash
plugin-kit-ai bundle fetch <owner/repo> --tag <tag> --platform <codex-runtime|claude> --runtime <python|node> --dest <path>
```

Après l'installation ou la récupération, le dépôt résultant a toujours besoin de ses vérifications normales de démarrage et de préparation au moment de l'exécution.

## Ce qui ne se produit pas automatiquement

`bundle install` et `bundle fetch` ne transforment pas silencieusement le bundle en un plugin entièrement validé.

Traitez le bundle installé comme le début de l'installation en aval :

1. installer les prérequis d'exécution
2. exécutez `plugin-kit-ai doctor .`
3. exécutez toute étape d'amorçage requise
4. exécutez `plugin-kit-ai validate . --platform <target> --strict`

## Quand le transfert de bundle est meilleur qu'un live repo

Choisissez le transfert du bundle lorsque :

- les artefacts de version constituent le véritable contrat de livraison
- les consommateurs en aval ne doivent pas cloner le dépôt source
- vous souhaitez une distribution reproductible des versions GitHub pour les voies Python ou Node

Restez sur le chemin du dépôt en direct lorsque :

- l'équipe édite toujours directement la source du projet
- le principal besoin est la collaboration au sein d'un seul dépôt
- Go vous donne déjà le transfert binaire compilé propre dont vous avez besoin

## Limite importante

Le transfert de bundles n’est pas un « package universel pour chaque cible ».

Il s’agit d’un flux de transfert portable pris en charge pour les sous-ensembles Python et Node exportés sur `codex-runtime` et `claude`.

Ne présumez pas que le même contrat s’applique à :

- Go SDK dépôts
- cibles de configuration d'espace de travail telles que Cursor ou OpenCode
- cibles uniquement liées à l'emballage telles que Gemini
- CLI packages d'installation

## Ordre de lecture recommandé

Associez cette page à [Choisir le modèle de livraison](/fr/guide/choose-delivery-model), [Préparation à la production](/fr/guide/production-readiness) et [Limite de support](/fr/reference/support-boundary).

</details>
