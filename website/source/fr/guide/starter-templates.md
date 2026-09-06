---
title: "Modèles de démarrage"
description: "Dépôts de démarrage officiels pour les premiers chemins communs dans plugin-kit-ai."
canonicalId: "page:guide:starter-templates"
section: "guide"
locale: "fr"
generated: false
translationRequired: true
---

[Utiliser des plugins](/fr/use/) · [Créer des plugins](/fr/build/) — Préparation, pas une version publiée. La migration des projets vers v2 est indisponible.
<details><summary>Documentation historique plugin-kit-ai v1 · 1.2.4 ; les anciennes instructions ne constituent pas le parcours Build actuel.</summary>

# Modèles de démarrage

Si vous souhaitez un bon point de départ connu au lieu d'un échafaudage à partir d'un répertoire vierge, utilisez les référentiels de démarrage officiels.

## Important : les starters sont des points d'entrée

Les noms de démarreur sont intentionnellement divisés par chemin principal tel que Codex ou Claude.

Cela ne signifie **pas** que le dépôt est définitivement verrouillé sur une seule famille d'agents.

Un starter vous aide à choisir la meilleure première forme pour :

- votre principale exigence d'exécution
- le choix de la langue de votre équipe
- votre première cible prise en charge

Après cela, gardez le dépôt unifié et développez-le selon vos besoins.

Lisez [Un projet, plusieurs cibles](/fr/guide/one-project-multiple-targets) si vous souhaitez une vue plus large du système.

## Runtime Codex

- [plugin-kit-ai-starter-codex-go](https://github.com/777genius/plugin-kit-ai-starter-codex-go)
- [plugin-kit-ai-starter-codex-python](https://github.com/777genius/plugin-kit-ai-starter-codex-python)
- [plugin-kit-ai-starter-codex-node-typescript](https://github.com/777genius/plugin-kit-ai-starter-codex-node-typescript)
- [plugin-kit-ai-starter-codex-python-runtime-package](https://github.com/777genius/plugin-kit-ai-starter-codex-python-runtime-package)

## Claude

- [plugin-kit-ai-starter-claude-go](https://github.com/777genius/plugin-kit-ai-starter-claude-go)
- [plugin-kit-ai-starter-claude-python](https://github.com/777genius/plugin-kit-ai-starter-claude-python)
- [plugin-kit-ai-starter-claude-node-typescript](https://github.com/777genius/plugin-kit-ai-starter-claude-node-typescript)
- [plugin-kit-ai-starter-claude-node-typescript-runtime-package](https://github.com/777genius/plugin-kit-ai-starter-claude-node-typescript-runtime-package)

## Quand préférer une entrée

Utilisez un démarreur lorsque :

- vous voulez immédiatement une mise en page de repo connue
- vous souhaitez comparer votre projet à un exemple minimal pris en charge
- vous souhaitez intégrer des coéquipiers avec moins de conjectures d'échafaudage

Utilisez `plugin-kit-ai init` directement lorsque :

- vous voulez un nouveau dépôt à partir des premiers principes
- vous devez choisir explicitement les drapeaux
- vous construisez autour d'une structure de référentiel existante

## Modèle mental sûr

- choisissez un démarreur pour le **premier** chemin correct
- ne pas traiter la famille starter comme la limite finale du repo
- conserver un dépôt et l'étendre uniquement lorsque le produit a vraiment besoin de plus de sorties

</details>
