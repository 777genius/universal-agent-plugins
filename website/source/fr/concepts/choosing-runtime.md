---
title: "Choisir l'environnement d'exécution"
description: "Comment choisir entre Go, Python, Node et les chemins de création du shell."
canonicalId: "page:concepts:choosing-runtime"
section: "concepts"
locale: "fr"
generated: false
translationRequired: true
---

[Utiliser des plugins](/fr/use/) · [Créer des plugins](/fr/build/) — Préparation, pas une version publiée. La migration des projets vers v2 est indisponible.
<details><summary>Documentation historique plugin-kit-ai v1 · 1.2.4 ; les anciennes instructions ne constituent pas le parcours Build actuel.</summary>

# Choisir l'environnement d'exécution

Le choix du runtime ne concerne pas seulement la préférence linguistique. Cela change la façon dont le plugin fonctionne, ce que la machine d'exécution doit avoir installé et la simplicité du CI et du transfert.

<MermaidDiagram
  :chart="`
flowchart TD
  Start[Need a runtime lane] --> Prod{Besoin de la voie d'exécution la plus puissante}
  Prod -->|Oui| Go[aller]
  Prod -->|Non| Local {Le dépôt de plugins est-il local de par sa conception}
  Local -->|Oui| Équipe{L'équipe Python est-elle première ou Node première}
  Équipe --> Python[python]
  Équipe --> Node[nœud ou nœud --typescript]
  Local -->|Non| Échapper{Besoin seulement d'une trappe d'évacuation}
  Échapper -> Shell[shell]
`"
/>

## Choisissez Go Quand

- vous voulez la voie d'exécution la plus solide
- vous voulez des gestionnaires typés et l'histoire de version la plus propre
- vous voulez le moins de frictions d'amorçage dans CI et sur d'autres machines

## Choisissez Python ou Node Quand

- le plugin est de par sa conception repo-local
- votre équipe vit déjà dans ce runtime
- vous acceptez de posséder vous-même le bootstrap du runtime
- vous êtes à l'aise avec la présence de Python `3.10+` ou Node.js `20+` sur la machine d'exécution

## Choisissez Shell uniquement lorsque

- vous avez besoin d'une trappe de secours étroite
- vous acceptez explicitement un compromis expérimental ou avancé

## Matrice par défaut sécurisée

| Situation | Choix recommandé |
| --- | --- |
| Voie d'exécution la plus puissante | `go` |
| Voie d'exécution principale non-Go | `node --typescript` |
| Local Python-première équipe | `python` |
| Trappe de secours | `shell` |

</details>
