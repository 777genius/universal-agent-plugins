---
title: "Exemples et recettes"
description: "Une carte guidée des exemples de dépôts publics, des dépôts de démarrage, des références d'exécution locales et des exemples de compétences dans plugin-kit-ai."
canonicalId: "page:guide:examples-and-recipes"
section: "guide"
locale: "fr"
generated: false
translationRequired: true
---

[Utiliser des plugins](/fr/use/) · [Créer des plugins](/fr/build/) — Préparation, pas une version publiée. La migration des projets vers v2 est indisponible.
<details><summary>Documentation historique plugin-kit-ai v1 · 1.2.4 ; les anciennes instructions ne constituent pas le parcours Build actuel.</summary>

# Exemples et recettes

Utilisez cette page lorsque vous souhaitez voir à quoi ressemble `plugin-kit-ai` dans de vrais référentiels au lieu de simplement lire des conseils abstraits.

## 1. Exemples de plugins de production

Voici les exemples les plus clairs de formes publiques finies :

- [`codex-basic-prod`](https://github.com/777genius/plugin-kit-ai/tree/main/examples/plugins/codex-basic-prod) : dépôt de production Go plus `codex-runtime`
- [`claude-basic-prod`](https://github.com/777genius/plugin-kit-ai/tree/main/examples/plugins/claude-basic-prod) : dépôt de production Go plus `claude`
- [`codex-package-prod`](https://github.com/777genius/plugin-kit-ai/tree/main/examples/plugins/codex-package-prod) : cible `codex-package`
- [`gemini-extension-package`](https://github.com/777genius/plugin-kit-ai/tree/main/examples/plugins/gemini-extension-package) : cible de packaging `gemini`
- [`cursor-basic`](https://github.com/777genius/plugin-kit-ai/tree/main/examples/plugins/cursor-basic) : cible `cursor` de configuration d'espace de travail
- [`opencode-basic`](https://github.com/777genius/plugin-kit-ai/tree/main/examples/plugins/opencode-basic) : cible `opencode` de configuration d'espace de travail

Lisez-les quand vous le souhaitez :

- une structure concrète de dépôt
- sorties réelles générées
- un exemple public véridique de ce à quoi ressemble le terme « sain »

Important : ces exemples montrent des formes distinctes de produits publics. Ils n'impliquent pas qu'un système réel doit être divisé en un dépôt distinct pour chaque cible.

## 2. Dépôts de démarrage

Utilisez les dépôts de démarrage lorsque vous souhaitez commencer à partir d'une base de référence connue plutôt que d'un répertoire vide.

Ils sont les meilleurs pour :

- première configuration
- intégration de l'équipe
- choisir entre les points de départ Go, Python, Node, Claude et Codex

Les liens code-first les plus directs sont :

- [`plugin-kit-ai-starter-codex-go`](https://github.com/777genius/plugin-kit-ai-starter-codex-go)
- [`plugin-kit-ai-starter-codex-python`](https://github.com/777genius/plugin-kit-ai-starter-codex-python)
- [`plugin-kit-ai-starter-codex-node-typescript`](https://github.com/777genius/plugin-kit-ai-starter-codex-node-typescript)
- [`plugin-kit-ai-starter-claude-go`](https://github.com/777genius/plugin-kit-ai-starter-claude-go)
- [`plugin-kit-ai-starter-claude-python`](https://github.com/777genius/plugin-kit-ai-starter-claude-python)
- [`plugin-kit-ai-starter-claude-node-typescript`](https://github.com/777genius/plugin-kit-ai-starter-claude-node-typescript)

Si vous êtes toujours en train de choisir, associez cette page à [Choisissez un dépôt de démarrage](/fr/guide/choose-a-starter).

## 3. Références d'exécution locales

La zone `examples/local` affiche les références d'exécution Python et Node pour les dépôts qui restent d'abord locaux.

Ceux-ci sont utiles lorsque :

- vous souhaitez comprendre plus en profondeur l'histoire d'exécution interprétée
- vous souhaitez comparer les configurations d'exécution locale JavaScript, TypeScript et Python
- vous avez besoin d'une référence concrète au-delà des dépôts de démarrage

Commencez par :

- [`codex-node-typescript-local`](https://github.com/777genius/plugin-kit-ai/tree/main/examples/local/codex-node-typescript-local)
- [`codex-python-local`](https://github.com/777genius/plugin-kit-ai/tree/main/examples/local/codex-python-local)

## 4. Exemples de compétences

La zone `examples/skills` montre des exemples de compétences de support et des intégrations d'assistance.

Ce ne sont pas le principal point d’entrée pour la plupart des auteurs de plugins, mais ils sont utiles lorsque :

- vous souhaitez intégrer des documents, des aides à la révision ou au formatage dans le flux de travail plus large
- vous voulez comprendre comment les compétences adjacentes peuvent s'adapter aux dépôts de plugins

## Lecture suggérée selon l'objectif

- Vous voulez l'exemple d'exécution le plus performant : commencez par l'exemple de production Codex ou Claude, puis lisez [Créez un plugin prêt pour l'équipe](/fr/guide/team-ready-plugin).
- Vous voulez un exemple code-first par langage et cible : commencez par le dépôt de démarrage Go, Python ou Node lié ci-dessus, puis ouvrez [Build Custom Plugin Logic](/en/guide/build-custom-plugin-logic).
- Vous souhaitez des exemples d'empaquetage ou de configuration d'espace de travail : commencez par les exemples Codex package, Gemini, Cursor ou OpenCode, puis lisez [Packages et configuration de l'intégration](/fr/guide/package-and-workspace-targets).
- Vous voulez un point de départ clair, pas un exemple fini : accédez à [Modèles de démarrage](/fr/guide/starter-templates).
- Vous souhaitez choisir la cible avant de consulter les dépôts : lisez [Choisissez une cible](/fr/guide/choose-a-target).
- Vous voulez d'abord connaître l'histoire complète de l'expansion d'un seul dépôt : lisez [Ce que vous pouvez construire](/fr/guide/what-you-can-build).

## Règle finale

Les exemples doivent clarifier le marché public et non le remplacer.

Utilisez des exemples de dépôts pour voir la forme et les résultats sains. Pour le modèle mental multi-cibles à dépôt unique, lisez [Un projet, plusieurs cibles](/fr/guide/one-project-multiple-targets).

</details>
