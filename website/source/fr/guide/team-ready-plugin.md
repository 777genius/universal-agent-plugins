---
title: "Créez un plugin prêt pour l'équipe"
description: "Un didacticiel public phare pour faire passer un plugin d'un échafaudage à une forme prête pour CI, prête à être transférée et lisible par l'équipe."
canonicalId: "page:guide:team-ready-plugin"
section: "guide"
locale: "fr"
generated: false
translationRequired: true
---

[Utiliser des plugins](/fr/use/) · [Créer des plugins](/fr/build/) — Préparation, pas une version publiée. La migration des projets vers v2 est indisponible.
<p class="locale-historical-identity">Créez un plugin prêt pour l&#x27;équipe</p>

[Instantané historique de la branche plugin-kit-ai v1. 1.2.4 est la référence des commandes, pas la version exacte de cet instantané.](/en/legacy/v1/)

<details><summary>Instantané historique de la branche plugin-kit-ai v1. 1.2.4 est la référence des commandes, pas la version exacte de cet instantané.</summary>

# Créez un plugin prêt pour l'équipe

Ce didacticiel reprend là où s'arrête le premier plugin réussi. L'objectif n'est pas seulement « cela fonctionne sur ma machine », mais un dépôt qu'un autre coéquipier peut cloner, valider et expédier sans connaissances cachées.

<MermaidDiagram
  :chart="`
flowchart LR
  scaffold[Dépôt initial] --> explicit[Chemin et portée rendus explicites]
  explicit --> honest[Fichiers générés gardés honnêtes]
  honest --> ci[Porte CI répétable]
  ci --> handoff[Transfert visible pour les coéquipiers]
  handoff --> ready[Dépôt prêt pour une équipe]
`"
/>

## Résultat

À la fin, vous devriez avoir :

- un dépôt créé selon les normes du package
- fichiers générés reproduits à partir de la source du projet
- un contrôle de validation strict qui passe proprement
- un ou plusieurs objectifs principaux clairs et documentés pour les coéquipiers
- un choix d'exécution clair ou une politique d'exécution par cible
- un chemin compatible CI qui peut être répété sur une autre machine

## 1. Commencez par le chemin stable le plus étroit

Commencez par le chemin le plus étroit qui correspond réellement au besoin :

```bash
plugin-kit-ai init my-plugin --template custom-logic
cd my-plugin
plugin-kit-ai inspect . --authoring
go mod tidy
plugin-kit-ai validate . --platform codex-runtime --strict
```

Cela vous donne la base la plus propre pour un transfert ultérieur.

## 2. Rendre le choix explicite

Un dépôt prêt pour l'équipe devrait dire, au minimum :

- quelle cible est principale et quelles cibles supplémentaires sont véritablement prises en charge
- quel moteur d'exécution il utilise et si cela change selon la cible
- quelle est la commande de validation principale ou quelles commandes de validation sont requises pour un dépôt multi-cible
- si cela dépend d'un chemin Go SDK ou d'un package d'exécution partagé

Si cette information n'est que dans la tête d'un seul responsable, le dépôt n'est pas prêt.

## 3. Gardez le référentiel honnête

Avant de développer le projet, appliquez trois règles :

- la source du projet réside dans la présentation standard du package
- les fichiers cibles générés sont des sorties
- `generate` et `validate --strict` font toujours partie du flux de travail normal

Ne corrigez pas les fichiers générés à la main et espérez ensuite que l'équipe ne réexécutera jamais la génération.

## 4. Ajouter une porte CI répétable

Pour le chemin Go launcher par défaut, la porte minimale devrait ressembler à ceci :

```bash
go build -o bin/my-plugin ./cmd/my-plugin
plugin-kit-ai doctor .
plugin-kit-ai generate .
plugin-kit-ai validate . --platform codex-runtime --strict
```

Si le chemin choisi est Node ou Python, incluez `bootstrap` et épinglez la version d'exécution dans CI :

```bash
plugin-kit-ai doctor .
plugin-kit-ai bootstrap .
plugin-kit-ai generate .
plugin-kit-ai validate . --platform codex-runtime --strict
```

Si le dépôt prend en charge plusieurs cibles, la porte CI doit vérifier explicitement chaque cible prise en charge plutôt que d'assumer une couverture indirecte.

## 5. Vérifiez si vous avez réellement besoin d'un chemin différent

Ne vous éloignez du chemin par défaut que lorsque le compromis est réel :

- utiliser `claude` lorsque les crochets Claude sont une exigence du produit
- utilisez `node --typescript` lorsque l'équipe est TypeScript en premier et que le compromis d'exécution local est acceptable
- utilisez `python` lorsque le projet est intentionnellement local au dépôt et Python-first

Changer de voie devrait résoudre un problème de produit ou d’équipe, et pas seulement refléter une préférence linguistique. Si le produit est véritablement multi-cible, dites-le directement : le dépôt a un chemin principal et des cibles supplémentaires à l'intérieur de la portée prise en charge.

## 6. Rendre le transfert visible

Un nouveau coéquipier devrait être en mesure de répondre à ces questions à partir du dépôt et de la documentation :

- comment installer les prérequis ?
- quelle commande prouve que le dépôt est sain ?
- pour quelle cible je valide ?
- quels fichiers sont créés et lesquels sont générés ?

Si la réponse à l’une de ces questions est « demandez à l’auteur original », le dépôt n’est toujours pas prêt.

## 7. Reliez le repo au contrat public

Un dépôt de plugin prêt pour l’équipe devrait diriger les gens vers :

- [Préparation à la production](/fr/guide/production-readiness)
- [Intégration CI](/fr/guide/ci-integration)
- [Norme de référentiel](/fr/reference/repository-standard)
- la note de version publique actuelle, maintenant [v1.1.2](/fr/releases/v1-1-2)

## Règle finale

Le dépôt est prêt lorsqu'un autre coéquipier peut le cloner, comprendre le chemin et la portée cible, reproduire les sorties générées et passer la porte de validation stricte sans improvisation.

Associez ce didacticiel à [Construisez votre premier plugin](/fr/guide/first-plugin), [Architecture de création](/fr/concepts/authoring-architecture) et [Limite de support](/fr/reference/support-boundary).

</details>
