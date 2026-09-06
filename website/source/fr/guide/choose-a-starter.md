---
title: "Choisissez un dépôt de démarrage"
description: "Une matrice pratique pour choisir le bon démarreur officiel par cible, durée d'exécution et chemin de livraison."
canonicalId: "page:guide:choose-a-starter"
section: "guide"
locale: "fr"
generated: false
translationRequired: true
---

[Utiliser des plugins](/fr/use/) · [Créer des plugins](/fr/build/) — Préparation, pas une version publiée. La migration des projets vers v2 est indisponible.
<details><summary>Documentation historique plugin-kit-ai v1 · 1.2.4 ; les anciennes instructions ne constituent pas le parcours Build actuel.</summary>

# Choisissez un dépôt de démarrage

Utilisez cette page lorsque vous souhaitez accéder au chemin le plus rapide vers un référentiel pouvant ensuite être étendu à des sorties plus prises en charge.

<MermaidDiagram
  :chart="`
flowchart TD
  Start[Need a starter] --> Produit{Le chemin principal est Codex ou Claude}
  Produit --> Codex[Codex famille de démarrage]
  Produit --> Claude[Claude famille de démarrage]
  Codex --> Exécution{Go, Node ou Python}
  Claude --> Runtime2{Go, Node ou Python}
`"
/>

Avant de choisir, rappelez-vous une règle importante :

- le démarreur vous indique par où commencer
- ce n'est pas la limite finale du produit
- et cela n'empêche pas un dépôt de prendre en charge plus tard plus de cibles

Si cette distinction est encore floue, lisez d'abord [Un projet, plusieurs cibles](/fr/guide/one-project-multiple-targets).

## Choisissez rapidement, puis développez plus tard

- choisissez Go lorsque vous souhaitez le chemin de production le plus solide
- choisissez Node/TypeScript lorsque vous souhaitez le chemin principal non-Go pris en charge
- choisissez Python lorsque le dépôt est intentionnellement Python-en premier et reste local au dépôt
- choisissez les démarreurs Claude uniquement lorsque les crochets Claude constituent l'exigence réelle du produit

Choisissez le démarreur pour le premier chemin correct, et non pour une limite de produit permanente imaginaire.

## Ce qui reste vrai après votre choix

- Vous conservez toujours un dépôt.
- Vous conservez toujours le même flux de travail de base.
- Vous pouvez ajouter des cibles prises en charge ultérieurement, à mesure que le produit se développe.
- La profondeur du support dépend de la cible que vous ajoutez.

## Matrice de démarrage

| Si tu veux | Meilleure entrée | Pourquoi |
| --- | --- | --- |
| Chemin de production Codex le plus solide | `plugin-kit-ai-starter-codex-go` | Go-premier chemin de production avec l'histoire de transfert la plus propre |
| Plugin Repo-local Codex dans Python | `plugin-kit-ai-starter-codex-python` | Sous-ensemble Python stable avec une disposition de dépôt connue |
| Plugin Repo-local Codex dans Node/TS | `plugin-kit-ai-starter-codex-node-typescript` | Chemin principal non-Go pris en charge |
| Chemin de production Claude le plus solide | `plugin-kit-ai-starter-claude-go` | Sous-ensemble Claude stable et chemin de production le plus propre |
| Plugin Repo-local Claude dans Python | `plugin-kit-ai-starter-claude-python` | Sous-ensemble de crochets Claude stable avec assistants Python |
| Plugin Repo-local Claude dans Node/TS | `plugin-kit-ai-starter-claude-node-typescript` | Sous-ensemble de crochets Claude stable pour les premières équipes TypeScript |

## Variantes de packages partagés

Ignorez cette section, sauf si vous savez déjà que votre équipe souhaite `plugin-kit-ai-runtime` comme dépendance réutilisable au lieu des fichiers d'assistance fournis.

Utilisez les variantes de package partagé lorsque :

- vous souhaitez une dépendance partagée entre plusieurs dépôts de plugins
- vous êtes à l'aise pour épingler et mettre à niveau explicitement le package d'exécution
- vous ne voulez pas que les fichiers d'assistance soient copiés dans chaque dépôt

Démarreurs actuels de packages partagés :

- [`plugin-kit-ai-starter-codex-python-runtime-package`](https://github.com/777genius/plugin-kit-ai-starter-codex-python-runtime-package) : Python Codex démarreur avec `plugin-kit-ai-runtime` épinglé dans `requirements.txt`
- [`plugin-kit-ai-starter-claude-node-typescript-runtime-package`](https://github.com/777genius/plugin-kit-ai-starter-claude-node-typescript-runtime-package) : Node/TypeScript Claude démarreur avec `plugin-kit-ai-runtime` épinglé dans `package.json`

Si vous choisissez entre le démarreur Python normal et le démarreur du package d'exécution Python, lisez d'abord [Créer un plugin d'exécution Python](/fr/guide/python-runtime), puis [Choisir le modèle de livraison](/fr/guide/choose-delivery-model).

## Quand éviter de sur-optimiser le choix

Ne passez pas trop de temps à chercher l’entrée parfaite.

Si vous n'êtes pas sûr :

1. commencez par le démarreur Go pour le défaut le plus fort
2. commencez par le démarreur Node/TypeScript pour le chemin principal non-Go pris en charge
3. ne choisissez Python ou des variantes de package partagé que lorsque le compromis de l'équipe est déjà réel

## Bonne politique d'équipe

Un choix de partant à l’échelle de l’équipe doit rester cohérent suffisamment longtemps pour que :

- tout le monde reconnaît la disposition du dépôt
- CI utilise le même flux de préparation
- le transfert ne dépend pas de l'explication du responsable

Mais un choix de démarrage stable n’empêche toujours pas un dépôt d’ajouter d’autres cibles ultérieurement si le produit les nécessite.

Associez cette page à [Modèles de démarrage](/fr/guide/starter-templates), [Choisir le modèle de livraison](/fr/guide/choose-delivery-model) et [Norme de référentiel](/fr/reference/repository-standard).

</details>
