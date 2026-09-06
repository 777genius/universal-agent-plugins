---
title: "Glossaire"
description: "Brèves définitions des termes publics utilisés dans les documents plugin-kit-ai."
canonicalId: "page:reference:glossary"
section: "reference"
locale: "fr"
generated: false
translationRequired: true
---

[Utiliser des plugins](/fr/use/) · [Créer des plugins](/fr/build/) — Préparation, pas une version publiée. La migration des projets vers v2 est indisponible.
<details><summary>Documentation historique plugin-kit-ai v1 · 1.2.4 ; les anciennes instructions ne constituent pas le parcours Build actuel.</summary>

# Glossaire

Utilisez cette page lorsqu'un terme de documentation vous ralentit. Le but n’est pas une théorie parfaite. Le but est un sens partagé rapidement.

## État d'auteur

La partie du dépôt que votre équipe possède directement. `generate` transforme cette source en sortie spécifique à la cible.

## Fichiers cibles générés

Fichiers produits pour une cible spécifique après génération. Il s’agit de véritables résultats, mais ils ne constituent pas une source de vérité à long terme.

## Chemin

Un moyen pratique de créer et de livrer le plugin. Les exemples incluent le chemin d'exécution par défaut Go, le chemin local Node/TypeScript et la configuration d'intégration appartenant au référentiel.

## Cible

Le résultat que vous visez, tel que `codex-runtime`, `claude`, `codex-package`, `gemini`, `opencode` ou `cursor`.

## Chemin d'exécution

Un chemin où le dépôt possède directement le comportement du plugin exécutable.

## Chemin du package ou de l'extension

Un chemin axé sur la production du bon package ou de l’artefact d’extension au lieu de la forme d’exécution exécutable principale.

## Configuration de l'intégration appartenant au dépôt

Un chemin où le dépôt expédie principalement la configuration enregistrée pour un autre outil ou espace de travail.

## Installer la chaîne

Un moyen d'installer le CLI, tel que Homebrew, npm, PyPI ou le script vérifié. Il ne s'agit pas d'un environnement d'exécution public API.

## Package d'exécution partagé

La dépendance `plugin-kit-ai-runtime` utilisée par les flux Python et Node approuvés au lieu de copier les fichiers d'assistance dans chaque dépôt.

## Limite de support

La frontière publique entre ce que le projet recommande par défaut, ce qu'il prend en charge avec plus de soin et ce qui reste expérimental.

## Porte de préparation

Le chèque que vous devez traiter comme le signal qu’un repo est suffisamment sain pour être transféré. Pour la plupart des dépôts, il s'agit de `validate --strict`.

## Transfert

Le point où un autre coéquipier, une autre machine ou un autre utilisateur peut utiliser le dépôt sans connaissances cachées en matière de configuration.

Pages associées : [Modèle cible](/fr/concepts/target-model), [Limite de support](/fr/reference/support-boundary) et [Préparation à la production](/fr/guide/production-readiness).
</details>
