---
title: "Norme de référentiel"
description: "À quoi devrait ressembler un dépôt plugin-kit-ai sain et comment séparer la source du projet des sorties générées."
canonicalId: "page:reference:repository-standard"
section: "reference"
locale: "fr"
generated: false
translationRequired: true
---

[Utiliser des plugins](/fr/use/) · [Créer des plugins](/fr/build/) — Préparation, pas une version publiée. La migration des projets vers v2 est indisponible.
<details><summary>Documentation historique plugin-kit-ai v1 · 1.2.4 ; les anciennes instructions ne constituent pas le parcours Build actuel.</summary>

# Norme de référentiel

Cette page définit la forme publique d'un référentiel `plugin-kit-ai` sain.

## La règle principale

Le référentiel doit rendre sa configuration prévue évidente et ses sorties générées reproductibles.

En pratique, cela signifie :

- la source du projet est facile à localiser
- les fichiers cibles générés sont clairement des sorties
- la ou les cibles principales dans le champ d'application sont visibles
- le choix d'exécution ou la politique d'exécution est visible
- la commande de validation est documentée

## Ce qui devrait être facile à trouver

Un dépôt sain devrait rendre ces éléments visibles sans creuser :

- la ou les cibles principales dans le champ d'application
- le runtime choisi ou la politique d'exécution par cible
- la commande canonique `validate --strict`, ou les commandes de validation s'il y a plusieurs cibles
- prérequis d'exécution tels que Go, Python ou Node
- si le dépôt utilise un chemin Go SDK ou un package d'exécution partagé

## Ce qui ne devrait pas être la source de la vérité

Ceux-ci ne devraient pas constituer la principale source de vérité :

- fichiers cibles générés manuellement
- packages d'installation du wrapper traités comme des API d'exécution
- connaissances tribales sur « la commande dont vous avez réellement besoin pour exécuter »

## Signaux de référentiel sains

- `generate` peut reproduire les sorties cibles
- `validate --strict` passe proprement pour la cible prévue, ou pour chaque cible que le dépôt prétend publiquement prendre en charge
- le dépôt explique le chemin choisi dans des documents publics ou dans du matériel README
- CI utilise le même flux de préparation publique que le développement local

## Signaux de référentiel faibles

- les fichiers cibles sont corrigés manuellement après génération
- le choix d'exécution ou de cible est implicite ou incohérent sur toutes les machines
- les utilisateurs en aval ont besoin des conseils du responsable pour reproduire le flux de base
- le repo promet un support pour les zones en dehors de la limite de support déclarée

## Relation avec ce site de documentation

Ce site de documentation publique traite la norme de référentiel comme l'endroit où :

- les conseils de rédaction deviennent opérationnels
- les limites de soutien deviennent exécutoires
- le transfert devient crédible

Associez cette page à [Workflow de création](/fr/reference/authoring-workflow), [Préparation à la production](/fr/guide/production-readiness) et [Glossaire](/fr/reference/glossary).

</details>
